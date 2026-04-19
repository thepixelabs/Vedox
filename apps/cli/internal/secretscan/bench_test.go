package secretscan_test

// bench_test.go — hot-path benchmarks for the secretscan package.
//
// These benchmarks cover the three latency-sensitive call sites in the
// pre-commit gate:
//
//  1. Scan a small (~1 KB) clean file — the common case for documentation.
//  2. Scan a large (~1 MB) file with 50 seeded findings — stress test.
//  3. GatePreCommit across 100 files — the realistic hook invocation path.
//
// Run with:
//
//	go test -bench=. -benchmem -run=^$ ./internal/secretscan/...
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/secretscan"
)

// benchScanner is a package-level Scanner so New() cost is not amortised into
// each iteration. Initialised in the first benchmark via sync.Once would be
// cleaner but global init is acceptable for benchmarks — callers never mutate
// scanner state after construction.
var benchScanner = secretscan.New(secretscan.DefaultRules())

// cleanLine returns a 64-byte line of documentation prose with no secrets.
// Repeated until the desired file size is reached.
const cleanLine = "The quick brown fox jumps over the lazy dog — documentation content.\n"

// buildCleanBody returns approximately targetBytes of clean markdown prose.
func buildCleanBody(targetBytes int) []byte {
	var sb strings.Builder
	for sb.Len() < targetBytes {
		sb.WriteString(cleanLine)
	}
	return []byte(sb.String())
}

// buildLargeBodyWithFindings returns ~1 MB of markdown with exactly 50 lines
// that match the AWS Access Key ID rule. Findings are seeded every 20 KB so
// they are spread across the file, matching a realistic distribution where a
// key appears in multiple places (duplicated config blocks, copy-pasted examples).
func buildLargeBodyWithFindings(targetBytes, findingCount int) []byte {
	totalLines := targetBytes / len(cleanLine)
	findingEvery := totalLines / findingCount

	var sb strings.Builder
	finding := 0
	for i := 0; i < totalLines; i++ {
		if finding < findingCount && i%findingEvery == 0 {
			// AKIA + 16 uppercase chars — matches AWS-ACCESS-KEY-ID.
			sb.WriteString(fmt.Sprintf("aws_access_key_id = AKIA%016X\n", finding))
			finding++
		} else {
			sb.WriteString(cleanLine)
		}
	}
	return []byte(sb.String())
}

// BenchmarkScanSmallFile measures scanning a 1 KB clean documentation file.
// This is the modal case: most files in a documentation repo contain no secrets.
func BenchmarkScanSmallFile(b *testing.B) {
	body := buildCleanBody(1024)
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		findings := benchScanner.Scan("docs/readme.md", body)
		if len(findings) != 0 {
			b.Fatalf("unexpected finding in clean file: %v", findings[0])
		}
	}
}

// BenchmarkScanLargeFile measures scanning a ~1 MB file that contains 50
// seeded AWS key findings. It exercises the per-line regex fan-out for all
// 15 default rules across a large input and the finding accumulation path.
func BenchmarkScanLargeFile(b *testing.B) {
	const targetBytes = 1 << 20 // 1 MiB
	const findings = 50
	body := buildLargeBodyWithFindings(targetBytes, findings)
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		got := benchScanner.Scan("large-config.md", body)
		if len(got) < findings {
			b.Fatalf("expected >=%d findings, got %d", findings, len(got))
		}
	}
}

// BenchmarkGatePreCommit_100Files measures the full GatePreCommit path for 100
// small clean files. This reflects a typical pre-commit hook invocation on a
// feature branch with many changed documentation files.
//
// Each benchmark iteration writes the 100 files into b.TempDir() once during
// setup; subsequent iterations reuse the same file paths so disk write cost is
// outside the measured region.
func BenchmarkGatePreCommit_100Files(b *testing.B) {
	const fileCount = 100
	dir := b.TempDir()

	// Pre-write 100 clean 1 KB files.
	paths := make([]string, fileCount)
	body := buildCleanBody(1024)
	for i := 0; i < fileCount; i++ {
		p := filepath.Join(dir, fmt.Sprintf("doc-%03d.md", i))
		if err := os.WriteFile(p, body, 0o600); err != nil {
			b.Fatalf("write fixture file: %v", err)
		}
		paths[i] = p
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := secretscan.GatePreCommit(paths)
		if err != nil {
			b.Fatalf("GatePreCommit returned error for clean files: %v", err)
		}
	}
}
