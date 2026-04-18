package ai

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// getStatus returns the current status of a job, holding the read lock for the
// entire field access. Accessing job.Status via Get's returned pointer without
// the lock is a data race — this helper avoids that.
func getStatus(js *JobStore, id string) (JobStatus, bool) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	job, ok := js.jobs[id]
	if !ok {
		return "", false
	}
	return job.Status, true
}

// waitForStatus polls a job's Status until it equals want or the deadline expires.
// All reads go through getStatus which holds js.mu.RLock, so no data race.
func waitForStatus(t *testing.T, js *JobStore, id string, want JobStatus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s, ok := getStatus(js, id); ok && s == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	s, ok := getStatus(js, id)
	if !ok {
		t.Fatalf("waitForStatus(%s): job %q not found after %s", want, id, timeout)
	} else {
		t.Fatalf("waitForStatus: timed out after %s; final status = %q, want %q", timeout, s, want)
	}
}

// nonExistentProvider returns a ProviderID whose binary will never be on PATH
// so RunGeneration fails quickly without network or subprocess side-effects.
func nonExistentProvider() ProviderID {
	return ProviderID("__vedox_test_no_such_provider__")
}

// nonExistentRequest builds a GenerationRequest that uses a fake provider and
// a short timeout so the job completes (with an error) quickly.
func nonExistentRequest() GenerationRequest {
	return GenerationRequest{
		Provider: nonExistentProvider(),
		Timeout:  200 * time.Millisecond,
		Count:    5,
	}
}

// TestNewJobStore_DefaultCapacity verifies that NewJobStore initialises a usable
// store and that zero/negative maxConcurrent is treated as 3.
func TestNewJobStore_DefaultCapacity(t *testing.T) {
	for _, maxConcurrent := range []int{0, -1, -100} {
		js := NewJobStore(maxConcurrent)
		if js == nil {
			t.Fatalf("NewJobStore(%d) returned nil", maxConcurrent)
		}
		if cap(js.sem) != 3 {
			t.Errorf("NewJobStore(%d): semaphore capacity = %d, want 3", maxConcurrent, cap(js.sem))
		}
	}
}

// TestNewJobStore_ExplicitCapacity verifies that a positive maxConcurrent value
// is applied directly to the semaphore.
func TestNewJobStore_ExplicitCapacity(t *testing.T) {
	for _, n := range []int{1, 5, 10} {
		js := NewJobStore(n)
		if cap(js.sem) != n {
			t.Errorf("NewJobStore(%d): semaphore capacity = %d, want %d", n, cap(js.sem), n)
		}
	}
}

// TestSubmit_ReturnsPendingJob verifies that Submit returns a job immediately
// with status JobPending before the background goroutine has run.
func TestSubmit_ReturnsPendingJob(t *testing.T) {
	// Use a semaphore capacity of 0-equivalent: pin the semaphore so the
	// background goroutine blocks on sem <- struct{}{}, keeping the job pending
	// long enough for us to observe it.
	js := NewJobStore(1)
	// Fill the semaphore slot so run() blocks immediately.
	js.sem <- struct{}{}

	job := js.Submit(nonExistentRequest())

	if job == nil {
		t.Fatal("Submit returned nil")
	}
	if job.ID == "" {
		t.Error("Submit: job ID must not be empty")
	}
	// Read Status under the lock so the -race detector does not flag the
	// concurrent write that will happen once the semaphore slot is released.
	s, ok := getStatus(js, job.ID)
	if !ok {
		t.Fatal("job not found in store immediately after Submit")
	}
	if s != JobPending {
		t.Errorf("Submit: expected status %q immediately after Submit, got %q", JobPending, s)
	}

	// Release the semaphore to allow background goroutines to drain. We do not
	// assert the final state here — that is covered by TestSubmit_FailsWithBadProvider.
	<-js.sem
}

// TestSubmit_FailsWithBadProvider verifies the full pending → running → error
// lifecycle when the AI CLI binary does not exist on PATH.
func TestSubmit_FailsWithBadProvider(t *testing.T) {
	js := NewJobStore(1)
	job := js.Submit(nonExistentRequest())

	if job == nil {
		t.Fatal("Submit returned nil")
	}

	// The job must eventually reach JobError because the provider binary is absent.
	waitForStatus(t, js, job.ID, JobError, 5*time.Second)

	// Read all fields under the read lock to avoid a data race with the
	// background goroutine that may have just finished writing them.
	js.mu.RLock()
	gotJob := js.jobs[job.ID]
	var errMsg string
	var completedAt *time.Time
	if gotJob != nil {
		errMsg = gotJob.Error
		completedAt = gotJob.CompletedAt
	}
	js.mu.RUnlock()

	if gotJob == nil {
		t.Fatal("Get: expected job, got nil")
	}
	if errMsg == "" {
		t.Error("job.Error must be non-empty when status is JobError")
	}
	if completedAt == nil {
		t.Error("job.CompletedAt must be set when status is JobError")
	}
}

// TestGet_KnownID returns the job for a registered ID.
func TestGet_KnownID(t *testing.T) {
	js := NewJobStore(1)
	job := js.Submit(nonExistentRequest())
	wantID := job.ID // ID is set before the goroutine runs and never mutated.

	// Read the stored ID under the lock to avoid a race with the goroutine
	// that may be writing other fields on the same struct concurrently.
	js.mu.RLock()
	stored, exists := js.jobs[wantID]
	var gotID string
	if stored != nil {
		gotID = stored.ID
	}
	js.mu.RUnlock()

	if !exists || stored == nil {
		t.Fatalf("Get(%q): job not found in store", wantID)
	}
	if gotID != wantID {
		t.Errorf("Get: ID mismatch: got %q, want %q", gotID, wantID)
	}
}

// TestGet_UnknownID returns nil for an unregistered ID.
func TestGet_UnknownID(t *testing.T) {
	js := NewJobStore(1)

	got := js.Get("no-such-id")
	if got != nil {
		t.Errorf("Get(unknown): expected nil, got %+v", got)
	}
}

// TestGet_EmptyID returns nil for an empty string ID.
func TestGet_EmptyID(t *testing.T) {
	js := NewJobStore(1)

	got := js.Get("")
	if got != nil {
		t.Errorf("Get(\"\"): expected nil, got %+v", got)
	}
}

// TestJobStatus_PendingToRunningToDone exercises the full lifecycle by injecting
// a job directly into the store map and simulating transitions that the
// background goroutine would perform, verifying invariants at each step.
// We test observable state through Get, which acquires the read lock.
func TestJobStatus_PendingToRunningToDone(t *testing.T) {
	js := NewJobStore(1)

	// Inject a job in pending state directly (same package access).
	job := &GenerationJob{
		ID:        "test-lifecycle",
		Status:    JobPending,
		StartedAt: time.Now().UTC(),
	}
	js.mu.Lock()
	js.jobs[job.ID] = job
	js.mu.Unlock()

	// Verify pending.
	got := js.Get("test-lifecycle")
	if got == nil || got.Status != JobPending {
		t.Fatalf("expected pending status, got %v", got)
	}

	// Transition to running.
	js.mu.Lock()
	job.Status = JobRunning
	js.mu.Unlock()

	got = js.Get("test-lifecycle")
	if got.Status != JobRunning {
		t.Errorf("expected running, got %q", got.Status)
	}

	// Transition to done.
	now := time.Now().UTC()
	js.mu.Lock()
	job.Status = JobDone
	job.Names = []string{"Vedox", "Nexus"}
	job.CompletedAt = &now
	job.DurationMs = 42
	js.mu.Unlock()

	got = js.Get("test-lifecycle")
	if got.Status != JobDone {
		t.Errorf("expected done, got %q", got.Status)
	}
	if len(got.Names) != 2 {
		t.Errorf("expected 2 names, got %d", len(got.Names))
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt must be set on done job")
	}
}

// TestJobStatus_PendingToRunningToFailed exercises the error transition path
// via direct store mutation (same package access).
func TestJobStatus_PendingToRunningToFailed(t *testing.T) {
	js := NewJobStore(1)

	job := &GenerationJob{
		ID:        "test-failure",
		Status:    JobPending,
		StartedAt: time.Now().UTC(),
	}
	js.mu.Lock()
	js.jobs[job.ID] = job
	js.mu.Unlock()

	now := time.Now().UTC()
	js.mu.Lock()
	job.Status = JobRunning
	js.mu.Unlock()

	js.mu.Lock()
	job.Status = JobError
	job.Error = "AI CLI not found"
	job.CompletedAt = &now
	js.mu.Unlock()

	got := js.Get("test-failure")
	if got.Status != JobError {
		t.Errorf("expected error status, got %q", got.Status)
	}
	if got.Error == "" {
		t.Error("Error field must be set on failed job")
	}
}

// TestConcurrentSubmitAndGet launches many goroutines submitting jobs and
// reading them back to confirm no data race under -race.
func TestConcurrentSubmitAndGet(t *testing.T) {
	js := NewJobStore(5)

	const goroutines = 50
	ids := make([]string, goroutines)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Submit goroutines.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			job := js.Submit(nonExistentRequest())
			mu.Lock()
			ids = append(ids, job.ID)
			mu.Unlock()
		}()
	}
	wg.Wait()

	// Get goroutines: read all jobs concurrently.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			mu.Lock()
			idsCopy := make([]string, len(ids))
			copy(idsCopy, ids)
			mu.Unlock()
			for _, id := range idsCopy {
				_ = js.Get(id)
			}
		}()
	}
	wg.Wait()
}

// TestConcurrentSubmitAndComplete mixes submitting jobs and marking them
// complete via direct map access to exercise the mutex under -race.
func TestConcurrentSubmitAndComplete(t *testing.T) {
	js := NewJobStore(10)

	const n = 30
	var wg sync.WaitGroup

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			job := &GenerationJob{
				ID:        fmt.Sprintf("concurrent-job-%d", i),
				Status:    JobPending,
				StartedAt: time.Now().UTC(),
			}
			js.mu.Lock()
			js.jobs[job.ID] = job
			js.mu.Unlock()

			// Simulate a status transition.
			now := time.Now().UTC()
			js.mu.Lock()
			job.Status = JobDone
			job.CompletedAt = &now
			js.mu.Unlock()

			// Read it back through the public API.
			_ = js.Get(job.ID)
		}(i)
	}
	wg.Wait()
}

// TestNewAIJobID_UniqueAndNonEmpty verifies that generated IDs are non-empty
// and distinct across calls.
func TestNewAIJobID_UniqueAndNonEmpty(t *testing.T) {
	const n = 100
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := newAIJobID()
		if id == "" {
			t.Error("newAIJobID returned empty string")
		}
		if _, dup := seen[id]; dup {
			t.Errorf("newAIJobID produced duplicate: %q", id)
		}
		seen[id] = struct{}{}
	}
}
