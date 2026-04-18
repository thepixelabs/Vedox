package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// JobStatus mirrors the lifecycle states used by the scanner job store.
type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobError   JobStatus = "error"
)

// GenerationJob holds the state of a single async name generation run.
// All fields are written by the background goroutine and read by HTTP handlers;
// access is protected by the embedding JobStore's mutex.
type GenerationJob struct {
	ID           string     `json:"id"`
	Status       JobStatus  `json:"status"`
	Names        []string   `json:"names,omitempty"`
	Error        string     `json:"error,omitempty"`
	ProviderUsed string     `json:"providerUsed,omitempty"`
	AccountUsed  string     `json:"accountUsed,omitempty"`
	DurationMs   int64      `json:"durationMs,omitempty"`
	StartedAt    time.Time  `json:"startedAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

// JobStore is a mutex-protected in-memory store for GenerationJobs.
// The zero value is not usable; construct with NewJobStore.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*GenerationJob
	// sem caps how many AI CLI subprocesses may run simultaneously.
	// This prevents runaway resource use when many requests come in at once.
	sem chan struct{}
}

// NewJobStore returns an initialised JobStore.
// maxConcurrent controls how many AI CLI subprocesses may run simultaneously;
// values less than 1 are treated as 3.
func NewJobStore(maxConcurrent int) *JobStore {
	if maxConcurrent < 1 {
		maxConcurrent = 3
	}
	return &JobStore{
		jobs: make(map[string]*GenerationJob),
		sem:  make(chan struct{}, maxConcurrent),
	}
}

// Submit creates a new GenerationJob, stores it, and starts the generation in
// a background goroutine. Returns the job immediately with status "pending".
func (js *JobStore) Submit(req GenerationRequest) *GenerationJob {
	job := &GenerationJob{
		ID:        newAIJobID(),
		Status:    JobPending,
		StartedAt: time.Now().UTC(),
	}

	js.mu.Lock()
	js.jobs[job.ID] = job
	js.mu.Unlock()

	go js.run(job, req)

	return job
}

// run executes the generation, respecting the semaphore capacity.
// It owns the transition pending → running → done/error.
func (js *JobStore) run(job *GenerationJob, req GenerationRequest) {
	// Block until a semaphore slot is free. This is the backpressure valve
	// that prevents spinning up more than maxConcurrent AI CLI processes.
	js.sem <- struct{}{}
	defer func() { <-js.sem }()

	js.mu.Lock()
	job.Status = JobRunning
	js.mu.Unlock()

	// Each job gets its own background context with the standard timeout.
	// We use context.Background() rather than leaking a caller context
	// so the job can outlive the HTTP request that submitted it.
	timeout := req.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := RunGeneration(ctx, req)

	js.mu.Lock()
	defer js.mu.Unlock()

	now := time.Now().UTC()
	job.CompletedAt = &now

	if err != nil {
		job.Status = JobError
		job.Error = err.Error()
		return
	}

	job.Status = JobDone
	job.Names = result.Names
	job.ProviderUsed = result.ProviderUsed
	job.AccountUsed = result.AccountUsed
	job.DurationMs = result.DurationMs
}

// Get returns the GenerationJob for id, or nil if not found.
//
// WARNING: the returned pointer aliases the same *GenerationJob that the
// background run() goroutine mutates under js.mu. Reading fields from the
// returned pointer without holding js.mu races with run()'s state
// transitions, which is a data race under -race. Callers that need to read
// job fields (e.g. HTTP handlers encoding JSON) MUST use Snapshot instead.
// Get is retained for callers that only need identity comparison or that
// take js.mu themselves.
func (js *JobStore) Get(id string) *GenerationJob {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.jobs[id]
}

// Snapshot returns a value copy of the job identified by id. The copy is
// taken under js.mu so field reads cannot race with run()'s concurrent
// mutations. Returns (GenerationJob{}, false) if the id is not found.
//
// This is the safe accessor for HTTP handlers and any other caller that
// reads the job fields (Status, Names, Error, ProviderUsed, AccountUsed,
// DurationMs, CompletedAt) from a goroutine other than run() itself.
//
// run() assigns Names and CompletedAt exactly once in its terminal critical
// section, so the shallow value copy taken here captures a stable view:
// either pre-terminal (nil slice, nil CompletedAt) or post-terminal
// (immutable slice, non-nil CompletedAt). Callers must not mutate the
// returned slice headers.
func (js *JobStore) Snapshot(id string) (GenerationJob, bool) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	p, ok := js.jobs[id]
	if !ok {
		return GenerationJob{}, false
	}
	return *p, true
}

// newAIJobID generates a random 16-byte hex string for use as a job ID.
// Mirrors the implementation in scanner/progress.go.
func newAIJobID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.999999999")))
	}
	return hex.EncodeToString(b)
}
