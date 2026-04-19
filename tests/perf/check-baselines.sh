#!/usr/bin/env bash
# check-baselines.sh — run the hot-path benchmarks, parse ns/op and allocs/op,
# and compare each result against the baselines in testdata/perf-baselines.json.
#
# Exit codes:
#   0 — all benchmarks within tolerance
#   1 — one or more regressions detected (details printed to stdout)
#
# Usage:
#   cd <repo-root>/apps/cli
#   bash ../../tests/perf/check-baselines.sh
#
# Environment variables:
#   BENCH_TIME   — go test -benchtime value (default: 3s)
#   BASELINES    — path to the JSON baselines file (default: <repo-root>/testdata/perf-baselines.json)
#
# Requirements:
#   - go (1.22+)
#   - jq (https://jqlang.github.io/jq/)
#
# This script is NOT intended for CI use. Shared CI runners produce unreliable
# timing numbers. See tests/perf/README.md for the reasoning.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CLI_DIR="${REPO_ROOT}/apps/cli"
BASELINES="${BASELINES:-${REPO_ROOT}/testdata/perf-baselines.json}"
BENCH_TIME="${BENCH_TIME:-3s}"

BENCH_PACKAGES=(
  "./internal/secretscan/..."
  "./internal/docgraph/..."
  "./internal/history/..."
  "./internal/api/..."
)

# ── preflight checks ──────────────────────────────────────────────────────────

if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required but not found in PATH." >&2
  echo "  Install: brew install jq  (macOS) or apt-get install jq (Linux)" >&2
  exit 1
fi

if [[ ! -f "${BASELINES}" ]]; then
  echo "ERROR: baselines file not found: ${BASELINES}" >&2
  echo "  Run benchmarks and populate testdata/perf-baselines.json first." >&2
  exit 1
fi

# ── run benchmarks ────────────────────────────────────────────────────────────

BENCH_OUTPUT_FILE="$(mktemp)"
trap 'rm -f "${BENCH_OUTPUT_FILE}"' EXIT

echo "==> Running benchmarks (benchtime=${BENCH_TIME}) ..."
cd "${CLI_DIR}"

go test \
  -bench=. \
  -benchmem \
  -benchtime="${BENCH_TIME}" \
  -run='^$' \
  "${BENCH_PACKAGES[@]}" \
  2>&1 | tee "${BENCH_OUTPUT_FILE}"

echo ""
echo "==> Comparing against baselines (tolerance: $(jq -r '._meta.tolerance_pct' "${BASELINES}")%) ..."
echo ""

# ── parse and compare ─────────────────────────────────────────────────────────

FAILURES=0

# Each benchmark in the JSON has a name and max values for ns/op and allocs/op.
# We parse the go test output line by line looking for Benchmark* lines.
#
# go test -benchmem output format:
#   BenchmarkFoo-8   12345   9876 ns/op   456 B/op   7 allocs/op

while IFS= read -r line; do
  # Match lines starting with "Benchmark"
  if [[ ! "${line}" =~ ^Benchmark ]]; then
    continue
  fi

  # Extract benchmark name (strip goroutine count suffix, e.g. -10)
  bench_name="$(echo "${line}" | awk '{print $1}' | sed 's/-[0-9]*$//')"
  ns_op="$(echo "${line}" | awk '{print $3}')"
  allocs_op="$(echo "${line}" | awk '{print $NF}')"  # last field after "allocs/op"

  # Validate that we got numbers.
  if ! [[ "${ns_op}" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
    echo "  SKIP  ${bench_name}: could not parse ns/op from: ${line}"
    continue
  fi

  # Look up this benchmark in the baselines file.
  baseline="$(jq --arg name "${bench_name}" '.benchmarks[] | select(.name == $name)' "${BASELINES}")"

  if [[ -z "${baseline}" ]]; then
    echo "  SKIP  ${bench_name}: no baseline entry found"
    continue
  fi

  ns_max="$(echo "${baseline}" | jq -r '.ns_op_max')"
  allocs_max="$(echo "${baseline}" | jq -r '.allocs_op_max')"
  ns_base="$(echo "${baseline}" | jq -r '.ns_op')"

  # Compare ns/op against the max (baseline + 20%).
  # Use awk for float comparison since ns_op can be a decimal (e.g. 837.2).
  ns_ok="$(awk -v actual="${ns_op}" -v max="${ns_max}" 'BEGIN {print (actual <= max) ? "yes" : "no"}')"
  allocs_ok="$(awk -v actual="${allocs_op}" -v max="${allocs_max}" 'BEGIN {print (actual <= max) ? "yes" : "no"}')"

  if [[ "${ns_ok}" == "yes" && "${allocs_ok}" == "yes" ]]; then
    printf "  PASS  %-45s  %8.0f ns/op (max %8.0f)  %4s allocs/op (max %4s)\n" \
      "${bench_name}" "${ns_op}" "${ns_max}" "${allocs_op}" "${allocs_max}"
  else
    printf "  FAIL  %-45s  %8.0f ns/op (max %8.0f)  %4s allocs/op (max %4s)\n" \
      "${bench_name}" "${ns_op}" "${ns_max}" "${allocs_op}" "${allocs_max}"
    if [[ "${ns_ok}" == "no" ]]; then
      pct="$(awk -v actual="${ns_op}" -v base="${ns_base}" 'BEGIN {printf "%.1f", (actual - base) / base * 100}')"
      echo "        ^ ns/op regression: +${pct}% over baseline (${ns_base} ns/op)"
    fi
    if [[ "${allocs_ok}" == "no" ]]; then
      echo "        ^ allocs/op regression: ${allocs_op} > max ${allocs_max}"
    fi
    FAILURES=$((FAILURES + 1))
  fi

done < "${BENCH_OUTPUT_FILE}"

echo ""
if [[ "${FAILURES}" -eq 0 ]]; then
  echo "==> All benchmarks within tolerance. No regressions detected."
  exit 0
else
  echo "==> ${FAILURES} benchmark(s) exceeded tolerance. Review the regressions above."
  echo "    To update baselines after a justified performance change:"
  echo "      1. Run benchmarks and record new numbers."
  echo "      2. Update testdata/perf-baselines.json with the new ns_op and allocs_op values."
  echo "      3. Recompute *_max = value * 1.20."
  echo "      4. Commit with a message explaining the performance change."
  exit 1
fi
