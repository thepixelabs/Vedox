# Performance Baselines

This directory contains tooling for comparing benchmark results against captured
baselines. It is a local development aid, not part of CI.

## Why not in CI?

Shared CI runners (GitHub Actions, etc.) have variable CPU performance due to
noisy-neighbour workloads, NUMA topology differences, and thermal throttling.
A 20% tolerance band calibrated on an Apple M1 Pro is not meaningful on a
different machine class. Adding this to CI without dedicated bare-metal runners
would produce a flaky gate that blocks legitimate PRs based on scheduler noise.

If the project moves to dedicated performance runners in future, the baselines
should be re-captured on those runners and the CI flag added at that point.

## Files

| File | Purpose |
|---|---|
| `check-baselines.sh` | Runs the benchmark suite and compares results to baselines |
| `../../testdata/perf-baselines.json` | Captured baseline values + 20% tolerance band |

## Running locally

From the repository root:

```sh
cd apps/cli
bash ../../tests/perf/check-baselines.sh
```

Or with a longer benchtime for more stable numbers:

```sh
BENCH_TIME=10s bash ../../tests/perf/check-baselines.sh
```

Requirements: `go` (1.22+) and `jq` in PATH.

## Running benchmarks without the comparison script

```sh
cd apps/cli
go test -bench=. -benchmem -run=^$ \
  ./internal/secretscan/... \
  ./internal/docgraph/... \
  ./internal/history/... \
  ./internal/api/...
```

## Updating baselines

After a justified performance change (algorithm improvement, struct layout
change, etc.):

1. Run the benchmarks on the same machine class as the original capture.
2. Record the new `ns_op` and `allocs_op` for each benchmark.
3. Compute `ns_op_max = ns_op * 1.20` and `allocs_op_max = allocs_op * 1.20`.
4. Update `testdata/perf-baselines.json`.
5. Commit with a message explaining why the baseline changed.

## Benchmark inventory

| Benchmark | Package | What it measures |
|---|---|---|
| `BenchmarkScanSmallFile` | `secretscan` | Scan 1 KB clean file through all 15 rules |
| `BenchmarkScanLargeFile` | `secretscan` | Scan 1 MB file with 50 seeded findings |
| `BenchmarkGatePreCommit_100Files` | `secretscan` | Full gate across 100 clean files |
| `BenchmarkExtractor_TypicalDoc` | `docgraph` | Parse a realistic ADR (all four link types) |
| `BenchmarkSaveRefs_HotPath` | `docgraph` | SQLite transaction: 8 refs, count refresh |
| `BenchmarkDiffDocs_TypicalEdit` | `history` | Myers diff: 200-line doc, 10-line change |
| `BenchmarkHandleHealthz` | `api` | Minimal JSON handler round-trip |
| `BenchmarkHandleGraph_Empty` | `api` | Graph handler with empty project (DB read) |
