package history

import (
	"fmt"
	"strings"
)

// Summarize converts a []Change into a single human-readable sentence that
// describes the overall nature of the edit. The output is deterministic —
// identical inputs always produce identical output.
//
// Examples:
//
//	"Added 2 paragraphs and a code block under '## Setup'"
//	"Removed heading 'Old approach'"
//	"Major rewrite: 14 blocks changed across 3 sections"
//	"No changes"
func Summarize(changes []Change) string {
	if len(changes) == 0 {
		return "No changes"
	}

	// If a single change, emit a verbatim description.
	if len(changes) == 1 {
		return changes[0].Summary
	}

	// Major-rewrite threshold: if more than 10 changes, aggregate.
	if len(changes) > 10 {
		sections := uniqueSections(changes)
		return fmt.Sprintf("Major rewrite: %d blocks changed across %d %s",
			len(changes), len(sections), pluralise("section", len(sections)))
	}

	// Bucket by (changeType, blockKind) and by section for a readable summary.
	type bucket struct {
		ct      ChangeType
		kind    BlockKind
		lang    string
		section string
	}
	counts := make(map[bucket]int)
	var order []bucket

	for _, c := range changes {
		// Only code fences carry a language; leave it empty for every other
		// kind so buckets dedup by (type, kind, section) and not by
		// accidentally-overlapping content text.
		lang := ""
		if c.Kind == BlockCodeFence {
			lang = extractFenceLang(c.NewContent)
			if lang == "" {
				lang = extractFenceLang(c.OldContent)
			}
		}
		b := bucket{ct: c.Type, kind: c.Kind, lang: lang, section: c.Section}
		if _, seen := counts[b]; !seen {
			order = append(order, b)
		}
		counts[b]++
	}

	var parts []string
	for _, b := range order {
		n := counts[b]
		label := blockKindLabel(b.kind, b.lang)
		if n > 1 {
			label = pluraliseKind(b.kind, b.lang, n)
		}
		section := ""
		if b.section != "" {
			section = fmt.Sprintf(" under '%s'", b.section)
		}
		verb := changeVerb(b.ct)
		parts = append(parts, fmt.Sprintf("%s %s%s", verb, label, section))
	}

	return strings.Join(parts, ", ")
}

// changeVerb returns the past-tense verb for a ChangeType.
func changeVerb(ct ChangeType) string {
	switch ct {
	case ChangeAdded:
		return "added"
	case ChangeRemoved:
		return "removed"
	case ChangeModified:
		return "modified"
	case ChangeMoved:
		return "moved"
	default:
		return "changed"
	}
}

// pluralise returns "word" or "words" depending on n.
func pluralise(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// pluraliseKind returns the plural label for n blocks of the given kind.
func pluraliseKind(kind BlockKind, lang string, n int) string {
	switch kind {
	case BlockFrontmatter:
		return "frontmatter blocks"
	case BlockHeading:
		return fmt.Sprintf("%d headings", n)
	case BlockParagraph:
		return fmt.Sprintf("%d paragraphs", n)
	case BlockCodeFence:
		if lang != "" {
			return fmt.Sprintf("%d %s code blocks", n, lang)
		}
		return fmt.Sprintf("%d code blocks", n)
	case BlockListItem:
		return fmt.Sprintf("%d lists", n)
	default:
		return fmt.Sprintf("%d blocks", n)
	}
}

// uniqueSections returns the deduplicated set of section names across changes.
func uniqueSections(changes []Change) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, c := range changes {
		if _, ok := seen[c.Section]; !ok {
			seen[c.Section] = struct{}{}
			out = append(out, c.Section)
		}
	}
	return out
}

// extractFenceLang reads the language specifier from the opening line of a
// fenced code block (e.g. "```go\n..." → "go"). Returns "" for non-fences.
func extractFenceLang(raw string) string {
	if !strings.HasPrefix(raw, "```") && !strings.HasPrefix(raw, "~~~") {
		return ""
	}
	first, _, _ := strings.Cut(raw, "\n")
	lang := strings.TrimSpace(first[3:])
	return lang
}
