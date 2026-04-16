package secretscan

import "unicode/utf8"

// Redact returns a safe preview of a matched secret for display in UI alerts,
// audit logs, and terminal output. The full secret is never stored.
//
// Redaction rules:
//   - If the match is 8 characters or fewer, return all asterisks of the same length.
//   - Otherwise, return the first 4 characters followed by "…" followed by asterisks
//     representing the remaining characters (capped at 8 asterisks so the preview
//     does not reconstruct meaningful structure from the tail).
//
// The redacted string reveals enough to help the user identify which secret
// triggered the block without exposing the secret's value. For example:
//
//	"AKIAIOSFODNN7EXAMPLE" → "AKIA…********"
//	"ghp_abcdefghijklmnopqrstuvwxyz123456" → "ghp_…********"
//	"abc" → "***"
func Redact(match string) string {
	if match == "" {
		return ""
	}

	runeCount := utf8.RuneCountInString(match)

	if runeCount <= 8 {
		stars := make([]byte, runeCount)
		for i := range stars {
			stars[i] = '*'
		}
		return string(stars)
	}

	// Take first 4 runes.
	prefix := firstNRunes(match, 4)
	return prefix + "…" + "********"
}

// firstNRunes returns the first n runes of s as a string. If s has fewer
// than n runes, the full string is returned.
func firstNRunes(s string, n int) string {
	count := 0
	for i, _ := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
