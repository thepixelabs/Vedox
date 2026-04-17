package ai

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestRaceJobStoreSnapshot verifies that Snapshot can be called concurrently
// with run()'s field mutations without triggering the race detector.
//
// Prior to the fix (WS-Q-17), HTTP handlers called Get(id) → writeJSON(job),
// which reads GenerationJob fields from a goroutine that does not hold js.mu.
// Meanwhile run() mutates job.Status, job.Names, job.Error, etc. under the
// write lock. That aliasing is a data race detected by `go test -race`.
//
// We submit many jobs (each backed by a non-existent provider so the run()
// goroutine fails fast) and pound Snapshot from several reader goroutines
// while run() is writing terminal state. Snapshot returns a value copy taken
// under the lock, so subsequent field reads cannot race.
func TestRaceJobStoreSnapshot(t *testing.T) {
	t.Parallel()

	js := NewJobStore(8)

	const submissions = 16
	jobIDs := make([]string, 0, submissions)
	for i := 0; i < submissions; i++ {
		j := js.Submit(nonExistentRequest())
		jobIDs = append(jobIDs, j.ID)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Reader goroutines: hammer Snapshot for every known job and touch every
	// field that run() might be mutating. Field reads happen on the value
	// copy returned by Snapshot, not on the pointer in the map, so -race
	// stays quiet regardless of writer state.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for _, id := range jobIDs {
					snap, ok := js.Snapshot(id)
					if !ok {
						continue
					}
					_ = snap.Status
					_ = snap.Names
					_ = snap.Error
					_ = snap.ProviderUsed
					_ = snap.AccountUsed
					_ = snap.DurationMs
					if snap.CompletedAt != nil {
						_ = *snap.CompletedAt
					}
				}
				runtime.Gosched()
			}
		}()
	}

	// Give run() goroutines time to transition through the lifecycle.
	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Drain to terminal state before exit so semaphore slots are released
	// and no goroutine outlives the test.
	for _, id := range jobIDs {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			snap, ok := js.Snapshot(id)
			if ok && (snap.Status == JobDone || snap.Status == JobError) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}
