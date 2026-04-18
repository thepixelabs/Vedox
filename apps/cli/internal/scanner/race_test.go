package scanner

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestRaceJobStoreSnapshot verifies that Snapshot can be called concurrently
// with runScan's field mutations without triggering the race detector.
//
// Prior to this fix, HTTP handlers called Get(id) → writeJSON(job) which
// reads ScanJob fields from a goroutine that does not hold js.mu. That
// aliased the same *ScanJob that runScan mutates under the lock, which is
// a data race detected by `go test -race`.
//
// We spawn several scans and hammer Snapshot from a second goroutine while
// runScan is writing job.Status / job.Total / job.Scanned.  Snapshot takes
// the lock, so -race stays quiet.  Using Get() here (uncomment to verify)
// would still work but every field access afterwards (e.g. snap.Status)
// would race.
func TestRaceJobStoreSnapshot(t *testing.T) {
	t.Parallel()

	js := NewJobStore()

	const scans = 10
	var jobIDs []string
	for i := 0; i < scans; i++ {
		ws := t.TempDir()
		mkGitDir(t, ws+"/proj")
		job := js.StartScan(ws)
		jobIDs = append(jobIDs, job.ID)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Reader goroutine — repeatedly Snapshot every known job. Fields are
	// read from the VALUE copy returned, not from the pointer, so there is
	// no aliasing race even while runScan mutates its own *ScanJob.
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
				// Touch every field that runScan might be mutating. Value
				// access — no race regardless of writer state.
				_ = snap.Status
				_ = snap.Total
				_ = snap.Scanned
				_ = snap.Error
				_ = snap.Projects
				if snap.CompletedAt != nil {
					_ = *snap.CompletedAt
				}
			}
			runtime.Gosched()
		}
	}()

	// Also pound on LastCompletedSnapshot.
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
				_ = id
				_, _ = js.LastCompletedSnapshot("irrelevant")
			}
			runtime.Gosched()
		}
	}()

	// Give runScan some time to progress through the states.
	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Wait for all scans to reach terminal state before tempdir cleanup.
	for _, id := range jobIDs {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			snap, ok := js.Snapshot(id)
			if ok && (snap.Status == JobStatusDone || snap.Status == JobStatusError) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}
