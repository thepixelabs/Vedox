package voice

import (
	"testing"
)

// intentCase is a single test vector for ParseIntent.
type intentCase struct {
	name           string
	input          string
	wantCommand    Command
	wantTarget     string
	wantMinConf    float64 // confidence must be >= this value
	wantMaxConf    float64 // confidence must be <= this value
	wantConfExact  *float64 // if non-nil, confidence must equal this exactly
}

func f(v float64) *float64 { return &v }

var intentCases = []intentCase{
	// -----------------------------------------------------------------------
	// Exact wake-word + exact phrase → Confidence = 1.0
	// -----------------------------------------------------------------------
	{
		name:          "exact: document everything",
		input:         "vedox document everything",
		wantCommand:   CommandDocumentEverything,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: doc everything",
		input:         "vedox doc everything",
		wantCommand:   CommandDocumentEverything,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: document all",
		input:         "vedox document all",
		wantCommand:   CommandDocumentEverything,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: document this folder",
		input:         "vedox document this folder",
		wantCommand:   CommandDocumentFolder,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: doc folder",
		input:         "vedox doc folder",
		wantCommand:   CommandDocumentFolder,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: document this directory",
		input:         "vedox document this directory",
		wantCommand:   CommandDocumentFolder,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: document these changes",
		input:         "vedox document these changes",
		wantCommand:   CommandDocumentChanges,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: doc changes",
		input:         "vedox doc changes",
		wantCommand:   CommandDocumentChanges,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: document the changes",
		input:         "vedox document the changes",
		wantCommand:   CommandDocumentChanges,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: status",
		input:         "vedox status",
		wantCommand:   CommandStatus,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: whats running",
		input:         "vedox whats running",
		wantCommand:   CommandStatus,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: what's running (apostrophe stripped)",
		input:         "vedox what's running",
		wantCommand:   CommandStatus,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: stop",
		input:         "vedox stop",
		wantCommand:   CommandStop,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: cancel",
		input:         "vedox cancel",
		wantCommand:   CommandStop,
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: document file with path",
		input:         "vedox document apps/cli/main.go",
		wantCommand:   CommandDocumentFile,
		wantTarget:    "apps/cli/main.go",
		wantConfExact: f(1.0),
	},
	{
		name:          "exact: document file with deep path",
		input:         "vedox document apps/cli/internal/voice/intent.go",
		wantCommand:   CommandDocumentFile,
		wantTarget:    "apps/cli/internal/voice/intent.go",
		wantConfExact: f(1.0),
	},
	// -----------------------------------------------------------------------
	// Capitalisation and punctuation tolerance
	// -----------------------------------------------------------------------
	{
		name:          "capitalised wake word",
		input:         "Vedox document everything",
		wantCommand:   CommandDocumentEverything,
		wantConfExact: f(1.0),
	},
	{
		name:          "all caps",
		input:         "VEDOX DOCUMENT EVERYTHING",
		wantCommand:   CommandDocumentEverything,
		wantConfExact: f(1.0),
	},
	{
		name:          "trailing period (Whisper punctuation)",
		input:         "vedox document everything.",
		wantCommand:   CommandDocumentEverything,
		wantConfExact: f(1.0),
	},
	{
		name:          "comma in phrase",
		input:         "vedox, document this folder",
		wantCommand:   CommandDocumentFolder,
		wantConfExact: f(1.0),
	},
	// -----------------------------------------------------------------------
	// Fuzzy wake-word matching → Confidence = 0.7
	// -----------------------------------------------------------------------
	{
		name:        "fuzzy: vedocks document everything",
		input:       "vedocks document everything",
		wantCommand: CommandDocumentEverything,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "fuzzy: vdox document everything",
		input:       "vdox document everything",
		wantCommand: CommandDocumentEverything,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "fuzzy: veedox document everything",
		input:       "veedox document everything",
		wantCommand: CommandDocumentEverything,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "fuzzy: vee docs document everything (two-token collapse)",
		input:       "vee docs document everything",
		wantCommand: CommandDocumentEverything,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "fuzzy: vedocks stop",
		input:       "vedocks stop",
		wantCommand: CommandStop,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "fuzzy: vedoc document this folder",
		input:       "vedoc document this folder",
		wantCommand: CommandDocumentFolder,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "fuzzy: vidox status",
		input:       "vidox status",
		wantCommand: CommandStatus,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	// -----------------------------------------------------------------------
	// Partial match (no wake word) → Confidence = 0.5
	// -----------------------------------------------------------------------
	{
		name:        "partial: document everything no wake word",
		input:       "document everything",
		wantCommand: CommandDocumentEverything,
		wantMinConf: 0.49,
		wantMaxConf: 0.51,
	},
	{
		name:        "partial: document this folder no wake word",
		input:       "document this folder",
		wantCommand: CommandDocumentFolder,
		wantMinConf: 0.49,
		wantMaxConf: 0.51,
	},
	{
		name:        "partial: document the changes no wake word",
		input:       "document the changes",
		wantCommand: CommandDocumentChanges,
		wantMinConf: 0.49,
		wantMaxConf: 0.51,
	},
	// -----------------------------------------------------------------------
	// Unknown / no match → Confidence = 0.0
	// -----------------------------------------------------------------------
	{
		name:        "unknown: empty string",
		input:       "",
		wantCommand: CommandUnknown,
		wantMinConf: 0,
		wantMaxConf: 0,
	},
	{
		name:        "unknown: random noise",
		input:       "hello world",
		wantCommand: CommandUnknown,
		wantMinConf: 0,
		wantMaxConf: 0,
	},
	{
		name:        "unknown: wake word only",
		input:       "vedox",
		wantCommand: CommandUnknown,
		wantMinConf: 0,
		wantMaxConf: 0,
	},
	{
		name:        "unknown: complete gibberish from STT",
		input:       "the weather today is really nice",
		wantCommand: CommandUnknown,
		wantMinConf: 0,
		wantMaxConf: 0,
	},
	// -----------------------------------------------------------------------
	// Real STT error strings observed in Whisper output
	// -----------------------------------------------------------------------
	{
		name:        "stt error: veedocs doc everything",
		input:       "veedocs doc everything",
		wantCommand: CommandDocumentEverything,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "stt error: vedocs document this folder",
		input:       "vedocs document this folder",
		wantCommand: CommandDocumentFolder,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "stt error: vedex status",
		input:       "vedex status",
		wantCommand: CommandStatus,
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	// -----------------------------------------------------------------------
	// Edge cases for document_file path capture
	// -----------------------------------------------------------------------
	{
		name:        "document_file: path with spaces should be captured",
		input:       "vedox document my project readme",
		wantCommand: CommandDocumentFile,
		wantTarget:  "my project readme",
		wantMinConf: 1.0,
		wantMaxConf: 1.0,
	},
	{
		name:        "document_file: fuzzy wake + path",
		input:       "vdox document README.md",
		wantCommand: CommandDocumentFile,
		wantTarget:  "readme.md", // dots inside paths are preserved
		wantMinConf: 0.69,
		wantMaxConf: 0.71,
	},
	{
		name:        "halt alias for stop",
		input:       "vedox halt",
		wantCommand: CommandStop,
		wantMinConf: 1.0,
		wantMaxConf: 1.0,
	},
	{
		name:        "abort alias for stop",
		input:       "vedox abort",
		wantCommand: CommandStop,
		wantMinConf: 1.0,
		wantMaxConf: 1.0,
	},
	{
		name:          "document changes via my changes alias",
		input:         "vedox document my changes",
		wantCommand:   CommandDocumentChanges,
		wantConfExact: f(1.0),
	},
}

func TestParseIntent(t *testing.T) {
	t.Parallel()

	for _, tc := range intentCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ParseIntent(tc.input)

			if got.Command != tc.wantCommand {
				t.Errorf("ParseIntent(%q).Command = %q, want %q", tc.input, got.Command, tc.wantCommand)
			}

			if tc.wantTarget != "" && got.Target != tc.wantTarget {
				t.Errorf("ParseIntent(%q).Target = %q, want %q", tc.input, got.Target, tc.wantTarget)
			}

			if tc.wantConfExact != nil {
				if got.Confidence != *tc.wantConfExact {
					t.Errorf("ParseIntent(%q).Confidence = %v, want exact %v", tc.input, got.Confidence, *tc.wantConfExact)
				}
			} else {
				if got.Confidence < tc.wantMinConf || got.Confidence > tc.wantMaxConf {
					t.Errorf("ParseIntent(%q).Confidence = %v, want in [%v, %v]",
						tc.input, got.Confidence, tc.wantMinConf, tc.wantMaxConf)
				}
			}

			if got.RawText != tc.input {
				t.Errorf("ParseIntent(%q).RawText = %q, want %q", tc.input, got.RawText, tc.input)
			}
		})
	}
}

// TestLevenshtein verifies the distance function against known pairs.
func TestLevenshtein(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want int
	}{
		{"vedox", "vedox", 0},
		{"vedox", "vedocks", 3}, // 3 edits (insert c, x→k, insert s); caught by knownWakeVariants fast-path, not Levenshtein
		{"vedox", "vdox", 1},
		{"vedox", "veedox", 1},
		{"vedox", "vidox", 1},
		{"vedox", "vedex", 1},
		{"vedox", "vedoc", 1},
		{"vedox", "vedocs", 2},
		{"", "vedox", 5},
		{"vedox", "", 5},
		{"abc", "abc", 0},
		{"abc", "axc", 1},
	}

	for _, tc := range cases {
		got := levenshtein(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestNormalise checks that normalise produces consistent output.
func TestNormalise(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"Vedox Document Everything", "vedox document everything"},
		{"  vedox   document   everything  ", "vedox document everything"},
		{"vedox document everything.", "vedox document everything"},
		{"vedox, document this folder", "vedox document this folder"},
		{"VEDOX STOP", "vedox stop"},
		{"", ""},
	}

	for _, tc := range cases {
		got := normalise(tc.input)
		if got != tc.want {
			t.Errorf("normalise(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestCommandCount verifies we have the expected set of command constants.
func TestCommandConstants(t *testing.T) {
	t.Parallel()

	commands := []Command{
		CommandDocumentEverything,
		CommandDocumentFolder,
		CommandDocumentChanges,
		CommandDocumentFile,
		CommandStatus,
		CommandStop,
		CommandUnknown,
	}

	if len(commands) != 7 {
		t.Errorf("expected 7 command constants, got %d", len(commands))
	}

	// Verify none are empty strings.
	for _, c := range commands {
		if c == "" {
			t.Error("found empty Command constant")
		}
	}
}

// TestIntentRawTextPreserved verifies that RawText always matches the input
// exactly, regardless of parse outcome.
func TestIntentRawTextPreserved(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"vedox document everything",
		"VEDOX DOCUMENT EVERYTHING",
		"completely random noise",
		"",
		"vedocks stop",
	}

	for _, input := range inputs {
		got := ParseIntent(input)
		if got.RawText != input {
			t.Errorf("ParseIntent(%q).RawText = %q, want %q", input, got.RawText, input)
		}
	}
}

// TestUnknownConfidenceIsZero verifies that CommandUnknown always has Confidence = 0.
func TestUnknownConfidenceIsZero(t *testing.T) {
	t.Parallel()

	unknownInputs := []string{
		"",
		"hello world",
		"vedox",
		"what time is it",
		"the quick brown fox",
	}

	for _, input := range unknownInputs {
		got := ParseIntent(input)
		if got.Command == CommandUnknown && got.Confidence != 0 {
			t.Errorf("ParseIntent(%q): unknown command but Confidence = %v (want 0)", input, got.Confidence)
		}
	}
}

// TestCaseCount is a meta-test that fails if we drop below 30 test vectors.
// This enforces the D5-01 requirement of "at least 30 test cases".
func TestCaseCount(t *testing.T) {
	if len(intentCases) < 30 {
		t.Errorf("intent test case count = %d, requirement is >= 30", len(intentCases))
	}
}
