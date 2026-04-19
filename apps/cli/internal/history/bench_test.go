package history

// bench_test.go — hot-path benchmark for the history differ.
//
// DiffDocs is called on every doc-save that triggers a history snapshot.
// The Myers diff runs at O(ND) in edit distance; the block split is O(n) in
// line count. Both paths need to be fast enough that synchronous diffing
// on the commit path is acceptable (target: <1ms for a 200-line doc).
//
// Package: history (internal — white-box; uses unexported helpers splitBlocks
// and myersDiff to benchmark each stage independently).
//
// Run with:
//
//	go test -bench=. -benchmem -run=^$ ./internal/history/...
import (
	"fmt"
	"strings"
	"testing"
)

// buildTypicalDoc returns a realistic 200-line Markdown document with a
// mix of headings, paragraphs, code fences, and list items. The structure
// matches what a documentation author would write: one top-level section,
// four sub-sections each with prose and a code block.
func buildTypicalDoc200Lines(seed string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "---\ntitle: %s\ndate: 2026-04-01\nstatus: published\n---\n\n", seed)
	fmt.Fprintf(&sb, "# %s Overview\n\n", seed)

	sections := []string{"Installation", "Configuration", "Usage", "Troubleshooting"}
	for _, sec := range sections {
		fmt.Fprintf(&sb, "## %s\n\n", sec)
		// Three paragraphs per section (~8 lines each).
		for p := 0; p < 3; p++ {
			fmt.Fprintf(&sb, "This is paragraph %d of the %s section. "+
				"It contains realistic prose to simulate a documentation file. "+
				"The content is deterministic so benchmark results are reproducible.\n\n",
				p+1, sec)
		}
		// One code fence per section.
		fmt.Fprintf(&sb, "```bash\n# %s example\necho 'hello from %s'\n```\n\n", sec, sec)
		// One list per section.
		fmt.Fprintf(&sb, "Key points:\n\n- First: configure the %s option.\n- Second: restart the daemon.\n- Third: verify with status check.\n\n", sec)
	}
	return sb.String()
}

// buildTypicalChange returns a modified version of the 200-line doc with a
// 10-line change: one paragraph replaced and one code block updated.
// This represents a realistic edit distance for a documentation PR.
func buildTypicalChange(original string) string {
	// Replace the first "paragraph 2 of the Installation section" with different prose.
	modified := strings.Replace(
		original,
		"This is paragraph 2 of the Installation section. "+
			"It contains realistic prose to simulate a documentation file. "+
			"The content is deterministic so benchmark results are reproducible.",
		"The installation process has been updated for v2. "+
			"Please refer to the migration guide before proceeding. "+
			"Legacy configurations will require a one-time upgrade step.",
		1,
	)
	// Update the Installation code fence content.
	modified = strings.Replace(
		modified,
		"```bash\n# Installation example\necho 'hello from Installation'\n```",
		"```bash\n# Installation v2 example\nvdx install --migrate\necho 'upgrade complete'\n```",
		1,
	)
	return modified
}

// BenchmarkDiffDocs_TypicalEdit measures DiffDocs on a realistic 200-line
// documentation file with a 10-line change (one paragraph + one code block).
// This is the modal edit for a documentation PR.
func BenchmarkDiffDocs_TypicalEdit(b *testing.B) {
	oldContent := buildTypicalDoc200Lines("MyService")
	newContent := buildTypicalChange(oldContent)

	// Sanity: must produce at least one change. Fails fast if the test data
	// is constructed incorrectly (e.g. strings.Replace found nothing).
	if changes := DiffDocs(oldContent, newContent); len(changes) == 0 {
		b.Fatal("benchmark setup error: DiffDocs returned no changes for the seeded edit")
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = DiffDocs(oldContent, newContent)
	}
}

// BenchmarkSplitBlocks_200Lines benchmarks the block-splitting pass alone.
// This isolates whether the linear scan or the Myers diff is the bottleneck —
// if SplitBlocks is slow, the optimisation target is the parser, not the diff.
func BenchmarkSplitBlocks_200Lines(b *testing.B) {
	content := buildTypicalDoc200Lines("BenchmarkService")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = splitBlocks(content)
	}
}

// BenchmarkDiffDocs_IdenticalDocs measures the fast path where old == new.
// This should be O(n) (single pass through equal blocks) and is the common
// case when the daemon re-indexes a file that has not changed.
func BenchmarkDiffDocs_IdenticalDocs(b *testing.B) {
	content := buildTypicalDoc200Lines("IdenticalService")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		changes := DiffDocs(content, content)
		if len(changes) != 0 {
			b.Fatalf("expected 0 changes for identical docs, got %d", len(changes))
		}
	}
}
