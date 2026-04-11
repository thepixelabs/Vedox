package db

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// bench_test.go — VDX-P4-DB benchmarks.
//
// These benchmarks exist so optimization decisions (FTS5 tokenizer, prefix
// indexes, columnsize=0, pragma tuning, extra indexes) are driven by
// measured numbers, not guesses. See the ticket file for before/after
// snapshots. Run with:
//
//   go test -bench . -benchmem -benchtime=3s ./internal/db/...
//
// All sizes are kept small enough (<= 10k) that the suite still fits in a
// reasonable CI window. Larger workloads should be driven by the scanner
// package benchmarks, not from here.

// benchDoc builds a deterministic document. The body contains a handful of
// common English words plus a unique token so token-N queries narrow to one
// row — that's what BenchmarkSearch relies on.
func benchDoc(i int) *Doc {
	return &Doc{
		ID:             fmt.Sprintf("bench/doc-%06d.md", i),
		Project:        "bench",
		Title:          fmt.Sprintf("Document %06d", i),
		Type:           "how-to",
		Status:         "published",
		Date:           "2026-04-07",
		Tags:           []string{"alpha", "beta", "gamma"},
		Author:         "bench",
		ContentHash:    fmt.Sprintf("%064x", i),
		ModTime:        "2026-04-07T00:00:00Z",
		Size:           512,
		RawFrontmatter: `{"title":"bench"}`,
		Body: fmt.Sprintf(
			"Quantum flux and reticulated splines — token-%06d. "+
				"The quick brown fox jumps over the lazy dog. "+
				"Distributed systems require careful tradeoffs between "+
				"consistency, availability, and partition tolerance. "+
				"Indexing strategies include btree, hash, gin, and gist. ",
			i,
		),
	}
}

func benchOpen(b *testing.B) *Store {
	b.Helper()
	root := b.TempDir()
	s, err := Open(Options{WorkspaceRoot: root})
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	b.Cleanup(func() { _ = s.Close() })
	return s
}

// seedDocs fills the store with n documents outside of the timed region.
func seedDocs(b *testing.B, s *Store, n int) {
	b.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		if err := s.UpsertDoc(ctx, benchDoc(i)); err != nil {
			b.Fatalf("seed upsert: %v", err)
		}
	}
}

// ─── BenchmarkUpsertDoc ─────────────────────────────────────────────────────
//
// Measures the single-writer serial upsert path at several population sizes.
// Each iteration writes a fresh row (i is monotonic across b.N) so we are
// measuring insert cost, not conflict resolution. The pre-seed establishes
// table size so FTS5 index growth is part of the measurement.
func BenchmarkUpsertDoc(b *testing.B) {
	sizes := []int{10, 100, 1_000, 10_000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			s := benchOpen(b)
			seedDocs(b, s, n)
			ctx := context.Background()
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d := benchDoc(n + i)
				if err := s.UpsertDoc(ctx, d); err != nil {
					b.Fatalf("upsert: %v", err)
				}
			}
		})
	}
}

// ─── BenchmarkUpsertDocConcurrent ────────────────────────────────────────────
//
// Eight goroutines submit concurrently. Because all writes funnel through a
// single goroutine, throughput should be comparable to the serial benchmark
// (minus channel overhead); the purpose is to quantify the contention tax.
func BenchmarkUpsertDocConcurrent(b *testing.B) {
	sizes := []int{10, 100, 1_000, 10_000}
	const goroutines = 8
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			s := benchOpen(b)
			seedDocs(b, s, n)
			ctx := context.Background()
			var idx int64 = int64(n)
			b.ResetTimer()
			b.ReportAllocs()
			var wg sync.WaitGroup
			per := b.N / goroutines
			if per == 0 {
				per = 1
			}
			for g := 0; g < goroutines; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < per; i++ {
						j := atomic.AddInt64(&idx, 1)
						if err := s.UpsertDoc(ctx, benchDoc(int(j))); err != nil {
							b.Errorf("upsert: %v", err)
							return
						}
					}
				}()
			}
			wg.Wait()
		})
	}
}

// ─── BenchmarkSearch ────────────────────────────────────────────────────────
//
// Measures FTS5 query latency and reports p50/p95/p99 as custom metrics.
// The 1/3/10 token query sizes reflect realistic search bar usage.
func BenchmarkSearch(b *testing.B) {
	sizes := []int{100, 1_000, 10_000}
	queries := map[string]string{
		"q=1tok":  "reticulated",
		"q=3tok":  "quantum reticulated splines",
		"q=10tok": "quantum flux reticulated splines distributed systems indexing btree hash gin",
	}
	for _, n := range sizes {
		s := openForSearch(b, n)
		ctx := context.Background()
		for name, q := range queries {
			b.Run(fmt.Sprintf("n=%d/%s", n, name), func(b *testing.B) {
				samples := make([]time.Duration, 0, b.N)
				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					t0 := time.Now()
					if _, err := s.Search(ctx, q, SearchFilters{}); err != nil {
						b.Fatalf("search: %v", err)
					}
					samples = append(samples, time.Since(t0))
				}
				b.StopTimer()
				reportPercentiles(b, samples)
			})
		}
	}
}

// openForSearch opens a Store and seeds it with n docs. Kept outside
// BenchmarkSearch so the seed cost isn't repeated for every query variant.
func openForSearch(b *testing.B, n int) *Store {
	b.Helper()
	root := b.TempDir()
	s, err := Open(Options{WorkspaceRoot: root})
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	b.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	for i := 0; i < n; i++ {
		if err := s.UpsertDoc(ctx, benchDoc(i)); err != nil {
			b.Fatalf("seed: %v", err)
		}
	}
	return s
}

func reportPercentiles(b *testing.B, samples []time.Duration) {
	if len(samples) == 0 {
		return
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	pct := func(p float64) float64 {
		idx := int(float64(len(samples)-1) * p)
		return float64(samples[idx].Nanoseconds())
	}
	b.ReportMetric(pct(0.50), "p50-ns")
	b.ReportMetric(pct(0.95), "p95-ns")
	b.ReportMetric(pct(0.99), "p99-ns")
}

// ─── BenchmarkReindex ───────────────────────────────────────────────────────
//
// Full reindex path (truncate + walk + upsert). Uses an in-memory DocStore
// so disk I/O outside .vedox/index.db does not pollute the number.
type memDocStore struct{ docs []*Doc }

func (m *memDocStore) WalkDocs(_ string, fn func(*Doc) error) error {
	for _, d := range m.docs {
		if err := fn(d); err != nil {
			return err
		}
	}
	return nil
}

func BenchmarkReindex(b *testing.B) {
	sizes := []int{100, 1_000, 10_000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			store := &memDocStore{docs: make([]*Doc, n)}
			for i := 0; i < n; i++ {
				store.docs[i] = benchDoc(i)
			}
			ctx := context.Background()
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := benchOpen(b)
				b.StartTimer()
				if err := s.Reindex(ctx, store, ""); err != nil {
					b.Fatalf("reindex: %v", err)
				}
			}
		})
	}
}

// ─── BenchmarkListTasks ─────────────────────────────────────────────────────
//
// Read-path microbenchmark for the per-project task list. Two sizes bracket
// a realistic workspace (tens) and a pathological backlog (thousands).
func BenchmarkListTasks(b *testing.B) {
	sizes := []int{100, 1_000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			s := benchOpen(b)
			ctx := context.Background()
			now := "2026-04-07T00:00:00Z"
			for i := 0; i < n; i++ {
				if err := s.InsertTask(ctx, Task{
					ID:        fmt.Sprintf("t-%05d", i),
					Project:   "bench",
					Title:     "task",
					Status:    "todo",
					Position:  float64(i + 1),
					CreatedAt: now,
					UpdatedAt: now,
				}); err != nil {
					b.Fatalf("insert task: %v", err)
				}
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := s.ListTasks(ctx, "bench"); err != nil {
					b.Fatalf("list: %v", err)
				}
			}
		})
	}
}
