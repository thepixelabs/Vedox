package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

// writeOp is any mutating SQLite operation. All writes to the index
// DB are serialised through a single goroutine (the "writer funnel")
// regardless of how many callers submit concurrently. This is an
// architectural rule from the CTO audit: readers can run concurrently
// in WAL mode, but exactly one goroutine ever holds a write handle.
type writeOp struct {
	ctx  context.Context
	fn   func(tx *sql.Tx) error
	resp chan error
}

// writer owns the single *sql.DB write connection and a job channel.
// Callers submit a writeOp and block on its response channel. This
// gives us strict serialisation without sprinkling mutexes through
// the codebase, and keeps the contention story explicit.
type writer struct {
	db        *sql.DB
	jobs      chan writeOp
	quit      chan struct{}
	doneWG    sync.WaitGroup
	closeOnce sync.Once
}

func newWriter(db *sql.DB) *writer {
	w := &writer{
		db:   db,
		jobs: make(chan writeOp, 128),
		quit: make(chan struct{}),
	}
	w.doneWG.Add(1)
	go w.loop()
	return w
}

func (w *writer) loop() {
	defer w.doneWG.Done()
	for {
		select {
		case <-w.quit:
			// Drain any pending jobs with a shutdown error so
			// submitters don't block forever on resp.
			for {
				select {
				case j := <-w.jobs:
					j.resp <- errors.New("vedox: db writer shutting down")
				default:
					return
				}
			}
		case j := <-w.jobs:
			j.resp <- w.exec(j)
		}
	}
}

func (w *writer) exec(j writeOp) error {
	if err := j.ctx.Err(); err != nil {
		return err
	}
	tx, err := w.db.BeginTx(j.ctx, nil)
	if err != nil {
		return fmt.Errorf("begin write tx: %w", err)
	}
	if err := j.fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit write tx: %w", err)
	}
	return nil
}

// submit enqueues an operation and blocks until it has been executed
// (or the context is cancelled / the writer has shut down).
func (w *writer) submit(ctx context.Context, fn func(tx *sql.Tx) error) error {
	resp := make(chan error, 1)
	select {
	case <-w.quit:
		return errors.New("vedox: db writer closed")
	case <-ctx.Done():
		return ctx.Err()
	case w.jobs <- writeOp{ctx: ctx, fn: fn, resp: resp}:
	}
	select {
	case err := <-resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// close shuts down the writer goroutine. Idempotent — safe to call multiple
// times. Test fixtures and Store.Close may both invoke this; sync.Once
// guarantees the underlying channel is only closed once.
func (w *writer) close() {
	w.closeOnce.Do(func() {
		close(w.quit)
		w.doneWG.Wait()
	})
}
