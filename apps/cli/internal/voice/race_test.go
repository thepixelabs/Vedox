package voice

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRaceOnActivityConcurrent reproduces the race between OnActivity() called
// from one goroutine and notify() invoked from the pipeline loop goroutine.
//
// Without the lifecycleMu protection around activityCb, `go test -race` flags
// the concurrent read/write of Pipeline.activityCb here; with the fix, this
// test passes cleanly under -race.
func TestRaceOnActivityConcurrent(t *testing.T) {
	t.Parallel()

	trans := NewStubTranscriber(nil)
	src := newControlledSource()

	p, err := NewPipeline(PipelineConfig{
		Source:       src,
		Transcriber:  trans,
		DaemonURL:    "http://127.0.0.1:4711",
		DispatchFunc: func(context.Context, Intent, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = p.Stop() })

	// Pound on OnActivity from one goroutine while the loop calls notify()
	// in its own goroutine via SetPTT → state transitions. Gosched yields
	// between operations to maximise interleaving under the race detector.
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			p.OnActivity(func(VoiceState) {})
			runtime.Gosched()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-stop:
				return
			default:
			}
			p.SetPTT(i%2 == 0)
			runtime.Gosched()
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestRaceStubTranscriberCloseConcurrent exercises Close() and Transcribe()
// from different goroutines.  Before the atomic.Bool fix, the `closed` bool
// was written by Close() and read by Transcribe() with no synchronisation,
// which `go test -race` flags as a data race.
func TestRaceStubTranscriberCloseConcurrent(t *testing.T) {
	t.Parallel()

	s := NewStubTranscriber(nil)

	var wg sync.WaitGroup
	var calls int64

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, _ = s.Transcribe(context.Background(), silenceChunk(4))
			atomic.AddInt64(&calls, 1)
			runtime.Gosched()
		}
	}()

	// Close from a different goroutine partway through.
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Millisecond)
		_ = s.Close()
	}()

	wg.Wait()

	if atomic.LoadInt64(&calls) == 0 {
		t.Fatal("transcriber was never called")
	}
}

// TestRaceStartStopInterleaved verifies Start and Stop may be called from
// different goroutines without racing on p.cancel / p.doneCh. A buggy
// implementation where cancel is a plain field would trigger -race here.
func TestRaceStartStopInterleaved(t *testing.T) {
	t.Parallel()

	for i := 0; i < 10; i++ {
		trans := NewStubTranscriber(nil)
		src := newControlledSource()

		p, err := NewPipeline(PipelineConfig{
			Source:       src,
			Transcriber:  trans,
			DaemonURL:    "http://127.0.0.1:4711",
			DispatchFunc: func(context.Context, Intent, string) error { return nil },
		})
		if err != nil {
			t.Fatalf("NewPipeline: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = p.Start(ctx)
		}()
		go func() {
			defer wg.Done()
			runtime.Gosched()
			_ = p.Stop()
		}()
		wg.Wait()

		// Ensure the pipeline is fully stopped (handles the case where Start
		// won the race — leaving the loop running — and the subsequent Stop
		// ran before Start finished).
		_ = p.Stop()
		cancel()
	}
}
