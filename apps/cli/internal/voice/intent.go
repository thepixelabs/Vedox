// Package voice implements the intent parser and agent dispatch layer for the
// Vedox voice ingress pipeline.
//
// Pipeline:
//
//	[Whisper.cpp transcript] → [ParseIntent] → [Intent] → [Dispatch]
//
// ParseIntent is deterministic, table-driven, and LLM-free. It tolerates
// common Whisper STT mishearings of the wake word ("vedocks", "vee docs",
// "vdox") using Levenshtein distance ≤ 2.
//
// Command constants and confidence scoring:
//   - Exact wake-word + phrase match: Confidence = 1.0
//   - Fuzzy wake-word (Levenshtein ≤ 2) + exact phrase: Confidence = 0.7
//   - Partial / suffix-only match (no explicit wake word): Confidence = 0.5
//   - No match: Command = CommandUnknown, Confidence = 0.0
package voice

import (
	"strings"
	"unicode"
)

// Command is a structured intent identifier extracted from a voice transcript.
type Command string

const (
	// CommandDocumentEverything documents all files in the current project.
	CommandDocumentEverything Command = "document_everything"

	// CommandDocumentFolder documents all files in the current folder.
	CommandDocumentFolder Command = "document_folder"

	// CommandDocumentChanges documents only files that have changed (git diff).
	CommandDocumentChanges Command = "document_changes"

	// CommandDocumentFile documents a specific file at the path held in Target.
	CommandDocumentFile Command = "document_file"

	// CommandStatus queries the daemon for its current running state.
	CommandStatus Command = "status"

	// CommandStop cancels the current agent job.
	CommandStop Command = "stop"

	// CommandUnknown means the transcript did not match any known pattern.
	CommandUnknown Command = "unknown"
)

// Intent is the structured result produced by ParseIntent.
type Intent struct {
	// Command is the recognised command, or CommandUnknown.
	Command Command

	// Target holds the path argument for CommandDocumentFile, and is empty for
	// all other commands.
	Target string

	// Confidence is a score in [0, 1] indicating how confident the parser is.
	//   1.0 — exact wake word + exact phrase match
	//   0.7 — fuzzy wake word (Levenshtein ≤ 2) + exact phrase match
	//   0.5 — partial match (phrase matched without wake word)
	//   0.0 — no match (CommandUnknown)
	Confidence float64

	// RawText is the original transcribed string, preserved verbatim.
	RawText string
}

// canonicalWakeWords is the set of exact spellings we accept for the wake word.
// All comparisons are performed on lowercased input.
var canonicalWakeWords = []string{
	"vedox",
}

// commonMishearings is the list of known Whisper STT errors for "vedox".
// These are tried after exact matches fail; they produce Confidence = 0.7.
// Levenshtein distance from "vedox":
//
//	"vedocks" → 2  (insertion of 'c', transposition)
//	"veedox"  → 1  (insertion)
//	"vee docs"→ 2  (split + insertion; collapsed to "veedocs" during normalise)
//	"vdox"    → 1  (deletion)
//	"vedocks" → 2
//	"vedoc"   → 1  (deletion)
//	"veedocs" → 2
var fuzzyWakeThreshold = 2

// knownWakeVariants is a pre-seeded list used as a fast path before we invoke
// the full Levenshtein check.  This avoids the O(n*m) computation for the
// most common mishearings.
var knownWakeVariants = []string{
	"vedocks",
	"veedox",
	"veedocs",
	"vdox",
	"vedoc",
	"vedocs",
	"vedex",
	"vidox",
}

// phraseRule associates a set of trigger strings with a Command and a
// flag indicating whether a target path may follow.
type phraseRule struct {
	// triggers is the set of lowercased phrase strings that activate this rule.
	// Compared after the wake word token has been stripped from the input.
	triggers []string

	// command is produced when any trigger matches.
	command Command

	// captureTarget, if true, means any remaining tokens after the trigger
	// are joined and stored as Intent.Target.
	captureTarget bool
}

// rules is the ordered priority list of phrase rules.
// First match wins.  The wake word itself is stripped before matching; rules
// operate on what follows the wake word.
//
// Additional entries for non-wake-word partial matches come AFTER all
// wake-word rules so that partial matches only fire when no exact rule wins.
var rules = []phraseRule{
	{
		triggers:      []string{"document everything", "doc everything", "document all", "doc all"},
		command:       CommandDocumentEverything,
		captureTarget: false,
	},
	{
		triggers: []string{
			"document this folder",
			"doc folder",
			"document this directory",
			"document the folder",
			"doc this folder",
			"document folder",
		},
		command:       CommandDocumentFolder,
		captureTarget: false,
	},
	{
		triggers: []string{
			"document these changes",
			"doc changes",
			"document the changes",
			"document changes",
			"doc these changes",
			"document my changes",
		},
		command:       CommandDocumentChanges,
		captureTarget: false,
	},
	{
		// "document <path>" — any remaining text becomes Target.
		// Must come after the more-specific folder/changes rules.
		triggers:      []string{"document", "doc"},
		command:       CommandDocumentFile,
		captureTarget: true,
	},
	{
		triggers:      []string{"status", "what's running", "whats running", "what is running"},
		command:       CommandStatus,
		captureTarget: false,
	},
	{
		triggers:      []string{"stop", "cancel", "halt", "abort"},
		command:       CommandStop,
		captureTarget: false,
	},
}

// partialOnlyRules are matched WITHOUT requiring a wake word, producing
// Confidence = 0.5. They mirror the main rules but use the full phrase
// (wake word already absent from caller perspective).
var partialOnlyRules = []phraseRule{
	{
		triggers:      []string{"document everything", "doc everything", "document all", "doc all"},
		command:       CommandDocumentEverything,
		captureTarget: false,
	},
	{
		triggers: []string{
			"document this folder",
			"document folder",
			"doc folder",
			"document this directory",
			"document the folder",
		},
		command:       CommandDocumentFolder,
		captureTarget: false,
	},
	{
		triggers: []string{
			"document these changes",
			"document the changes",
			"doc changes",
			"document changes",
		},
		command:       CommandDocumentChanges,
		captureTarget: false,
	},
	{
		triggers:      []string{"vedox status", "status check"},
		command:       CommandStatus,
		captureTarget: false,
	},
	{
		triggers:      []string{"vedox stop", "vedox cancel"},
		command:       CommandStop,
		captureTarget: false,
	},
}

// ParseIntent parses a raw Whisper transcript into a structured Intent.
//
// Algorithm:
//  1. Normalise input (lowercase, collapse whitespace, strip punctuation).
//  2. Try to strip the wake word from the front (exact, then fuzzy).
//  3. Match the remainder against ordered phrase rules.
//  4. If no wake word was found, try partial-only rules at Confidence = 0.5.
//  5. Return CommandUnknown with Confidence = 0.0 if nothing matches.
func ParseIntent(text string) Intent {
	raw := text
	norm := normalise(text)

	// --- Step 1: attempt to strip wake word ---
	remainder, wakeConf := stripWakeWord(norm)

	if wakeConf > 0 {
		// Wake word was detected — match against main rules.
		cmd, target, matched := matchRules(remainder, rules)
		if matched {
			return Intent{
				Command:    cmd,
				Target:     target,
				Confidence: wakeConf,
				RawText:    raw,
			}
		}
	}

	// --- Step 2: no wake word found — try partial-only rules ---
	cmd, target, matched := matchRules(norm, partialOnlyRules)
	if matched {
		return Intent{
			Command:    cmd,
			Target:     target,
			Confidence: 0.5,
			RawText:    raw,
		}
	}

	return Intent{
		Command:    CommandUnknown,
		Confidence: 0.0,
		RawText:    raw,
	}
}

// stripWakeWord attempts to find and remove the wake word from the beginning
// of the normalised input string.
//
// Returns (remainder, confidence):
//   - confidence = 1.0 on exact match
//   - confidence = 0.7 on fuzzy match (Levenshtein ≤ 2 or known variant)
//   - confidence = 0.0 if no wake word found (remainder is the full input)
func stripWakeWord(norm string) (remainder string, confidence float64) {
	tokens := strings.Fields(norm)
	if len(tokens) == 0 {
		return norm, 0
	}

	first := tokens[0]
	rest := strings.TrimSpace(strings.Join(tokens[1:], " "))

	// Exact match.
	for _, w := range canonicalWakeWords {
		if first == w {
			return rest, 1.0
		}
	}

	// Known STT mishearing fast-path.
	for _, v := range knownWakeVariants {
		if first == v {
			return rest, 0.7
		}
	}

	// Full Levenshtein check against each canonical wake word.
	for _, w := range canonicalWakeWords {
		if levenshtein(first, w) <= fuzzyWakeThreshold {
			return rest, 0.7
		}
	}

	// Two-token collapsing: "vee docs" → try ["vee", "docs"] collapsed to "veedocs".
	if len(tokens) >= 2 {
		collapsed := tokens[0] + tokens[1]
		for _, w := range canonicalWakeWords {
			if collapsed == w {
				return strings.TrimSpace(strings.Join(tokens[2:], " ")), 1.0
			}
		}
		for _, v := range knownWakeVariants {
			if collapsed == v {
				return strings.TrimSpace(strings.Join(tokens[2:], " ")), 0.7
			}
		}
		for _, w := range canonicalWakeWords {
			if levenshtein(collapsed, w) <= fuzzyWakeThreshold {
				return strings.TrimSpace(strings.Join(tokens[2:], " ")), 0.7
			}
		}
	}

	return norm, 0
}

// matchRules iterates the rule list in priority order and returns the first
// matching command plus any captured target.
func matchRules(input string, ruleset []phraseRule) (cmd Command, target string, matched bool) {
	for _, rule := range ruleset {
		for _, trigger := range rule.triggers {
			if rule.captureTarget {
				// The trigger is a prefix; any suffix becomes the target.
				if input == trigger {
					return rule.command, "", true
				}
				if strings.HasPrefix(input, trigger+" ") {
					remainder := strings.TrimPrefix(input, trigger+" ")
					return rule.command, strings.TrimSpace(remainder), true
				}
			} else {
				if input == trigger {
					return rule.command, "", true
				}
			}
		}
	}
	return CommandUnknown, "", false
}

// normalise converts text to lowercase, collapses whitespace, and strips
// leading/trailing punctuation from the whole string (but preserves internal
// path separators such as / and .).
func normalise(text string) string {
	// Lowercase.
	s := strings.ToLower(text)

	// Replace non-printable and control characters with spaces.
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	s = b.String()

	// Strip sentence-level punctuation that Whisper adds, but preserve dots
	// inside paths (e.g. "main.go") and apostrophes inside words (e.g. "what's").
	runes := []rune(s)
	var stripped strings.Builder
	for i, r := range runes {
		switch r {
		case '.':
			// Keep dot if it's between two word characters (path separator).
			if i > 0 && i < len(runes)-1 && unicode.IsLetter(runes[i-1]) && unicode.IsLetter(runes[i+1]) {
				stripped.WriteRune(r)
			} else {
				stripped.WriteRune(' ')
			}
		case '\'':
			// Keep apostrophe if it's between two letters (contraction).
			if i > 0 && i < len(runes)-1 && unicode.IsLetter(runes[i-1]) && unicode.IsLetter(runes[i+1]) {
				stripped.WriteRune(r)
			} else {
				stripped.WriteRune(' ')
			}
		case ',', '?', '!', ';', ':', '"':
			stripped.WriteRune(' ')
		default:
			stripped.WriteRune(r)
		}
	}
	s = stripped.String()

	// Collapse multiple spaces.
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// levenshtein computes the edit distance between two strings using the standard
// dynamic-programming algorithm.  Both inputs are assumed to be short (≤ 20
// characters) so the O(n*m) cost is acceptable in the hot path.
func levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	na, nb := len(ra), len(rb)

	if na == 0 {
		return nb
	}
	if nb == 0 {
		return na
	}

	// Allocate two rows.
	prev := make([]int, nb+1)
	curr := make([]int, nb+1)

	for j := 0; j <= nb; j++ {
		prev[j] = j
	}

	for i := 1; i <= na; i++ {
		curr[0] = i
		for j := 1; j <= nb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}

	return prev[nb]
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
