package analytics

// Tests for the full 30-event taxonomy registered in AllEventKinds.
//
// These are separate from events_test.go's bootstrap-10 checks so the
// original bootstrap contract remains untouched while the expanded
// taxonomy (FINAL_PLAN.md OQ-K, changelog item 3) gets its own assertions.

import (
	"testing"
	"time"
)

// TestTaxonomyLength pins the closed taxonomy at exactly 30 entries
// (FINAL_PLAN.md OQ-K). Adding or removing an event kind requires updating
// both AllEventKinds and this constant, per the OQ-K rule 3 contract.
func TestTaxonomyLength(t *testing.T) {
	const want = 30
	if got := len(AllEventKinds); got != want {
		t.Errorf("AllEventKinds length = %d, want %d (FINAL_PLAN.md OQ-K 30-event taxonomy)", got, want)
	}
}

// TestTaxonomy_AllKindsValidate exercises every kind in AllEventKinds
// through Event.Validate. Any typo or regex regression surfaces here as a
// failed sub-test named after the offending wire value.
func TestTaxonomy_AllKindsValidate(t *testing.T) {
	for _, k := range AllEventKinds {
		k := k
		t.Run(k, func(t *testing.T) {
			e := Event{Kind: k, Timestamp: time.Now(), SessionID: "taxonomy-session"}
			if err := e.Validate(); err != nil {
				t.Errorf("taxonomy kind %q failed Validate: %v", k, err)
			}
		})
	}
}

// TestTaxonomy_AllKindsDistinct verifies no two entries in AllEventKinds
// share the same wire value — a duplicate here would route analytics events
// under the wrong bucket silently.
func TestTaxonomy_AllKindsDistinct(t *testing.T) {
	seen := make(map[string]bool, len(AllEventKinds))
	for _, k := range AllEventKinds {
		if seen[k] {
			t.Errorf("duplicate taxonomy entry %q", k)
		}
		seen[k] = true
	}
	if len(seen) != len(AllEventKinds) {
		t.Errorf("expected %d distinct kinds, got %d (check for copy-paste duplicates)",
			len(AllEventKinds), len(seen))
	}
}

// TestTaxonomy_NewConstantsPresent verifies the 20 constants added by
// FIX-ARCH-10 are all present and non-empty. If any is accidentally deleted
// during a merge, this test names the offender.
func TestTaxonomy_NewConstantsPresent(t *testing.T) {
	added := map[string]string{
		"EventKindDaemonStarted":           EventKindDaemonStarted,
		"EventKindDaemonStopped":           EventKindDaemonStopped,
		"EventKindDaemonReloaded":          EventKindDaemonReloaded,
		"EventKindRepoRemoved":             EventKindRepoRemoved,
		"EventKindRepoSetDefault":          EventKindRepoSetDefault,
		"EventKindAgentUninstalled":        EventKindAgentUninstalled,
		"EventKindAgentRepaired":           EventKindAgentRepaired,
		"EventKindVoiceActivated":          EventKindVoiceActivated,
		"EventKindVoiceTranscribed":        EventKindVoiceTranscribed,
		"EventKindVoiceDispatched":         EventKindVoiceDispatched,
		"EventKindDoctreeFiltered":         EventKindDoctreeFiltered,
		"EventKindDoctreeExpanded":         EventKindDoctreeExpanded,
		"EventKindGraphViewed":             EventKindGraphViewed,
		"EventKindGraphNodeClicked":        EventKindGraphNodeClicked,
		"EventKindHistoryViewed":           EventKindHistoryViewed,
		"EventKindHistoryEntryExpanded":    EventKindHistoryEntryExpanded,
		"EventKindPreviewHovered":          EventKindPreviewHovered,
		"EventKindSettingsReset":           EventKindSettingsReset,
		"EventKindOnboardingStepCompleted": EventKindOnboardingStepCompleted,
		"EventKindOnboardingSkipped":       EventKindOnboardingSkipped,
	}
	if got := len(added); got != 20 {
		t.Errorf("expected 20 new FIX-ARCH-10 constants, have %d", got)
	}
	for name, wire := range added {
		if wire == "" {
			t.Errorf("%s is empty", name)
		}
	}
}
