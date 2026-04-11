package db

// dupcheck.go — lightweight duplicate detection helpers for the agent queue.
//
// No external dependencies. Levenshtein is intentionally naïve (O(n*m) time,
// O(min(n,m)) space). Queue entries have short titles; this is never called
// in a tight loop. A small constant over two strings of a few dozen runes is
// negligible on any modern CPU.

// levenshtein returns the edit distance between two strings.
// The implementation uses a single row of the DP table and advances in-place,
// keeping space proportional to the shorter of the two inputs.
func levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)

	// Ensure ra is the shorter string so the working row stays small.
	if len(ra) > len(rb) {
		ra, rb = rb, ra
	}

	m := len(ra)
	n := len(rb)

	// row[j] = distance between ra[0:i] and rb[0:j] after step i.
	row := make([]int, m+1)
	for j := range row {
		row[j] = j
	}

	for i := 1; i <= n; i++ {
		prev := row[0]
		row[0] = i
		for j := 1; j <= m; j++ {
			old := row[j]
			if ra[j-1] == rb[i-1] {
				row[j] = prev
			} else {
				row[j] = 1 + min3(prev, row[j], row[j-1])
			}
			prev = old
		}
	}
	return row[m]
}

// titleSimilarity returns a value in [0, 1] measuring how similar two strings
// are based on normalised Levenshtein distance:
//
//	similarity = 1 - (editDistance / max(len(a), len(b)))
//
// Empty strings return 1.0 (identical by convention — both are absent titles).
func titleSimilarity(a, b string) float64 {
	ra := []rune(a)
	rb := []rune(b)
	maxLen := len(ra)
	if len(rb) > maxLen {
		maxLen = len(rb)
	}
	if maxLen == 0 {
		return 1.0
	}
	dist := levenshtein(a, b)
	return 1.0 - float64(dist)/float64(maxLen)
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
