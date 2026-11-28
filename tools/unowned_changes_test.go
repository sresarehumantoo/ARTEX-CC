package tools

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func unownedChange(id int64, reason string, ago time.Duration) TargetChange {
	c := TargetChange{ID: id, Target: "WIN01", Resource: "r", Kind: "modify", Baseline: "sha256:orig", BaselineChangeID: id}
	if reason != "" {
		since := time.Now().Add(-ago)
		c.Unowned, c.UnownedSince = reason, &since
	}
	return c
}

func TestNewUnownedWarning(t *testing.T) {
	if w := NewUnownedWarning([]TargetChange{unownedChange(1, "", 0)}, time.Now()); w != nil {
		t.Fatalf("owned changes produced a warning: %+v", w)
	}
	changes := []TargetChange{
		unownedChange(1, "", 0),
		unownedChange(2, "claim_lapsed", 2*time.Minute),
		unownedChange(3, "intent_done", UnownedChangeThreshold+time.Minute),
	}
	w := NewUnownedWarning(changes, time.Now())
	if w == nil || w.Count != 2 || !slices.Equal(w.ChangeIDs, []int64{2, 3}) || !slices.Equal(w.OverThreshold, []int64{3}) {
		t.Fatalf("warning = %+v, want changes 2 and 3 with 3 over the threshold", w)
	}
	if w.OldestSince != changes[2].UnownedSince.UTC().Format(time.RFC3339) {
		t.Errorf("oldest = %s, want change 3's", w.OldestSince)
	}

	var many []TargetChange
	for i := range 50 {
		many = append(many, unownedChange(int64(i+1), "no_intent", time.Minute))
	}
	if w := NewUnownedWarning(many, time.Now()); w.Count != 50 || len(w.ChangeIDs) != maxWarningIDs {
		t.Errorf("50 unowned changes: count %d with %d ids, want 50 with %d", w.Count, len(w.ChangeIDs), maxWarningIDs)
	}
}

func TestSummaryFlagsUnownedChange(t *testing.T) {
	m := unownedChange(2, "claim_lapsed", time.Minute).Summary()
	if m["unowned"] != "claim_lapsed" || m["unowned_since"] == nil || m["baseline"] != "sha256:orig" {
		t.Errorf("summary = %v, want the reason, since and baseline", m)
	}
	if m := unownedChange(1, "", 0).Summary(); m["unowned"] != nil {
		t.Errorf("owned change summary carries unowned: %v", m)
	}
}

func TestListChangesWarnsAboutUnownedChanges(t *testing.T) {
	m := &targetChangeMockStore{live: []TargetChange{unownedChange(4, "", 0), unownedChange(5, "claim_lapsed", time.Minute)}}
	_, out := runTool(t, makeListChanges(m, 1, 1), WithSessionID(context.Background(), "s"), `{"since":"2026-01-01T00:00:00Z"}`)
	raw, _ := json.Marshal(out["unowned_target_changes"])
	var w UnownedWarning
	if err := json.Unmarshal(raw, &w); err != nil || w.Count != 1 || !slices.Equal(w.ChangeIDs, []int64{5}) {
		t.Fatalf("unowned_target_changes = %s, want change 5", raw)
	}
	if !strings.Contains(w.Action, "adopt_changes") {
		t.Errorf("action does not say how to take ownership: %s", w.Action)
	}

	m.live = []TargetChange{unownedChange(4, "", 0)}
	if _, out := runTool(t, makeListChanges(m, 1, 1), WithSessionID(context.Background(), "s"), `{"since":"2026-01-01T00:00:00Z"}`); out["unowned_target_changes"] != nil {
		t.Errorf("warning still present once every change is owned: %v", out["unowned_target_changes"])
	}
}

func TestParseUnownedChangeThreshold(t *testing.T) {
	if d, err := ParseUnownedChangeThreshold(" 15m "); err != nil || d != 15*time.Minute {
		t.Errorf("15m = %v, %v", d, err)
	}
	for _, bad := range []string{"30s", "90s", "ten", "0m"} {
		if _, err := ParseUnownedChangeThreshold(bad); err == nil || !strings.Contains(err.Error(), "unowned change threshold") {
			t.Errorf("ParseUnownedChangeThreshold(%q) err = %v", bad, err)
		}
	}
}
