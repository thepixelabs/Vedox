package ai

import (
	"strings"
	"unicode"
)

// ParseNames extracts product names from the numbered-list output of an AI CLI.
// It handles the common formats "1. Name", "1) Name", and "1: Name".
// Blank lines and lines that don't start with a digit are skipped.
// Duplicates are removed using case-insensitive comparison; the first-seen
// casing is preserved in the output.
func ParseNames(stdout string) []string {
	lines := strings.Split(stdout, "\n")
	seen := make(map[string]bool)
	var names []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip lines that don't start with a digit.
		if !unicode.IsDigit(rune(line[0])) {
			continue
		}

		// Find the separator after the number: '.', ')', or ':'.
		rest := ""
		for i, ch := range line {
			if ch == '.' || ch == ')' || ch == ':' {
				rest = strings.TrimSpace(line[i+1:])
				break
			}
			if !unicode.IsDigit(ch) {
				break
			}
		}

		if rest == "" {
			continue
		}

		// Strip trailing parenthetical notes or em-dash qualifiers that some
		// models append (e.g. "Nexio (cloud platform)" → "Nexio").
		// We only strip if there's enough content before the marker so we don't
		// truncate a name that happens to contain a hyphen (e.g. "X-Ray").
		if idx := strings.IndexAny(rest, "([{—"); idx > 2 {
			rest = strings.TrimSpace(rest[:idx])
		}

		// Skip empty results or suspiciously long strings (likely a description
		// that leaked through rather than a single product name).
		if rest == "" || len(rest) > 60 {
			continue
		}

		key := strings.ToLower(rest)
		if !seen[key] {
			seen[key] = true
			names = append(names, rest)
		}
	}

	return names
}
