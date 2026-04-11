// Package scanner — async scan job infrastructure.
//
// StartScan launches a scan in a background goroutine and returns a *ScanJob
// that callers can poll for progress. The JobStore holds all active and recently
// completed jobs in memory.
//
// Jobs are never persisted to disk. A server restart means in-flight jobs are
// lost, which is acceptable — the frontend can simply re-issue POST /api/scan.
package scanner

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// JobStatus is the lifecycle state of a ScanJob.
type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusError   JobStatus = "error"
)

// ScanJob holds the state of a single asynchronous workspace scan.
// All fields are set by the background goroutine and read by HTTP handlers.
// Access is protected by the embedding JobStore's mutex.
type ScanJob struct {
	// ID is the unique job identifier. Format: UUID v4.
	ID string `json:"id"`

	// WorkspaceRoot is the directory this job is scanning.
	WorkspaceRoot string `json:"workspaceRoot"`

	// Status is the current lifecycle state.
	Status JobStatus `json:"status"`

	// Total is the number of projects discovered so far.
	// Updated incrementally as the scan progresses.
	Total int `json:"total"`

	// Scanned is the number of projects fully processed (framework detected,
	// doc count completed). Scanned <= Total.
	Scanned int `json:"scanned"`

	// Projects holds the results once Status == "done".
	// During the scan it may be nil or partially populated.
	Projects []*Project `json:"projects"`

	// Error holds the user-facing error message when Status == "error".
	Error string `json:"error,omitempty"`

	// StartedAt and CompletedAt are informational timestamps.
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// JobStore is a mutex-protected in-memory store for ScanJobs.
// The zero value is not usable; use NewJobStore().
type JobStore struct {
	mu      sync.RWMutex
	jobs    map[string]*ScanJob
	scanner *Scanner
	// lastDone tracks the most recent completed job per workspaceRoot so
	// GET /api/projects can find cached results without knowing the job ID.
	lastDone map[string]*ScanJob
}

// NewJobStore returns an initialised JobStore backed by a new Scanner.
func NewJobStore() *JobStore {
	return &JobStore{
		jobs:     make(map[string]*ScanJob),
		scanner:  NewScanner(),
		lastDone: make(map[string]*ScanJob),
	}
}

// StartScan creates a new ScanJob for workspaceRoot, stores it, starts the
// scan in a goroutine, and returns the job immediately (status "pending").
// The job transitions pending → running → done (or error) asynchronously.
func (js *JobStore) StartScan(workspaceRoot string) *ScanJob {
	job := &ScanJob{
		ID:            newJobID(),
		WorkspaceRoot: workspaceRoot,
		Status:        JobStatusPending,
		StartedAt:     time.Now().UTC(),
	}

	js.mu.Lock()
	js.jobs[job.ID] = job
	js.mu.Unlock()

	go js.runScan(job)

	return job
}

// runScan executes the scan and updates job state. Called in a goroutine.
func (js *JobStore) runScan(job *ScanJob) {
	// Transition to running.
	js.mu.Lock()
	job.Status = JobStatusRunning
	js.mu.Unlock()

	projects, err := js.scanner.Scan(job.WorkspaceRoot)

	js.mu.Lock()
	defer js.mu.Unlock()

	now := time.Now().UTC()
	job.CompletedAt = &now

	if err != nil {
		job.Status = JobStatusError
		job.Error = "workspace scan failed: " + err.Error()
		return
	}

	job.Status = JobStatusDone
	job.Projects = projects
	job.Total = len(projects)
	job.Scanned = len(projects)

	// Record as the last completed job for this workspace root.
	js.lastDone[job.WorkspaceRoot] = job
}

// Get returns the ScanJob for id. Returns nil if not found.
// The returned pointer is safe to read under the store's lock; callers that
// need a stable snapshot should copy the value.
func (js *JobStore) Get(id string) *ScanJob {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.jobs[id]
}

// LastCompleted returns the most recent completed ScanJob for workspaceRoot,
// or nil if no scan has completed for that root in this process lifetime.
func (js *JobStore) LastCompleted(workspaceRoot string) *ScanJob {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.lastDone[workspaceRoot]
}

// Scanner returns the underlying Scanner so callers (e.g. HTTP handlers that
// need a synchronous fallback) can invoke Scan directly.
func (js *JobStore) Scanner() *Scanner {
	return js.scanner
}

// InvalidateCache removes the cached completed scan for workspaceRoot so the
// next call to LastCompleted returns nil and handleListProjects is forced to
// run a fresh scan. Call this after any mutation that adds or removes projects
// from the workspace (e.g. after a successful import).
func (js *JobStore) InvalidateCache(workspaceRoot string) {
	js.mu.Lock()
	defer js.mu.Unlock()
	delete(js.lastDone, workspaceRoot)
}

// newJobID generates a random 16-byte hex string suitable for use as a job ID.
// We use crypto/rand directly to avoid importing the uuid package as a direct
// dependency (it is currently indirect in go.mod).
func newJobID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to a timestamp-based ID.
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.999999999")))
	}
	// Format as UUID-like string: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx (32 hex chars).
	return hex.EncodeToString(b)
}
