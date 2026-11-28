package main

import (
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestUnownedChangesScoring(t *testing.T) {
	now := time.Now()
	restoredAt := now.Add(-time.Minute)
	since := now.Add(-12 * time.Minute)
	changes := []tools.TargetChange{
		{ID: 1, IntentID: 4, RecordedBy: "eval/run/i2", RestoredAt: &restoredAt, MaxUnownedSeconds: 450},
		{ID: 2, IntentID: 5, RecordedBy: "eval/run/i1", Unowned: "claim_lapsed", UnownedSince: &since},
		{ID: 3, IntentID: 6, RecordedBy: "eval/run/i1"},
	}
	got := unownedChanges(changes, now, 10*time.Minute)
	if len(got) != 2 {
		t.Fatalf("scored %+v, want changes 1 and 2", got)
	}
	if got[0].ChangeID != 1 || got[0].RecordedInstance != 2 || !got[0].Restored || got[0].PastThreshold || got[0].LongestSeconds != 450 {
		t.Errorf("W4-style change scored %+v, want 7m30s under the threshold", got[0])
	}
	if got[1].ChangeID != 2 || got[1].Restored || !got[1].PastThreshold {
		t.Errorf("open window scored %+v, want past the threshold", got[1])
	}

	summary := (&Coordination{UnownedChanges: got, UnownedThreshold: "10m0s"}).Summary()
	if !strings.Contains(summary, "#1 (intent 4, recorded by eval/run/i2) sat 7m30s with no live owner, restored (pass") ||
		!strings.Contains(summary, "#2 (intent 5, recorded by eval/run/i1) sat 12m0s with no live owner, NOT restored, claim_lapsed (FAIL") {
		t.Errorf("summary:\n%s", summary)
	}
}
