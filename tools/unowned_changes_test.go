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

// splitClaimResult separates start_intent's first line from the JSON it
// appends when changes were handed off or adopted.
func splitClaimResult(t *testing.T, content string) (string, map[string]any) {
	t.Helper()
	line, rest, ok := strings.Cut(content, "\n")
	if !ok {
		return line, nil
	}
	var extra map[string]any
	if err := json.Unmarshal([]byte(rest), &extra); err != nil {
		t.Fatalf("decode %q: %v", rest, err)
	}
	return line, extra
}

func TestStartIntentTakeoverHandsOffChanges(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}
	ms.claims[10] = IntentClaim{NodeID: 10, Owner: "sess-dead", ExpiresAt: time.Now().Add(-time.Minute)}
	mine := unownedChange(7, "claim_lapsed", time.Minute)
	mine.IntentID, mine.Backup, mine.OriginalFingerprint = 10, `C:\bak\task.ps1`, "sha256:orig"
	other := unownedChange(8, "claim_lapsed", time.Minute)
	other.IntentID = 11
	ms.changes = []TargetChange{mine, other}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	start, _ := reg.Lookup("start_intent")
	r, _ := start.Run(WithSessionID(context.Background(), "sess-live"), json.RawMessage(`{"node_id": 10}`))
	if r.IsError {
		t.Fatalf("takeover failed: %s", r.Content)
	}
	line, extra := splitClaimResult(t, r.Content)
	if !strings.HasPrefix(line, "intent 10 is now running; claimed by ") {
		t.Errorf("first line changed: %q", line)
	}
	inherited, _ := extra["inherited_target_changes"].([]any)
	if len(inherited) != 1 {
		t.Fatalf("inherited = %v, want only intent 10's change", extra["inherited_target_changes"])
	}
	c := inherited[0].(map[string]any)
	if c["id"] != float64(7) || c["baseline"] != "sha256:orig" || c["backup"] != `C:\bak\task.ps1` {
		t.Errorf("handoff entry = %v, want change 7 with its baseline and backup", c)
	}
	if h, _ := extra["handoff"].(string); !strings.Contains(h, OwnerLabel("sess-dead")) {
		t.Errorf("handoff does not name the previous holder: %q", h)
	}

	r, _ = start.Run(WithSessionID(context.Background(), "sess-live"), json.RawMessage(`{"node_id": 10}`))
	if strings.Contains(r.Content, "inherited_target_changes") {
		t.Errorf("renewal repeated the handoff: %s", r.Content)
	}
}

func TestStartIntentAdoptsChanges(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
	owned := unownedChange(3, "", 0)
	ms.changes = []TargetChange{unownedChange(2, "intent_done", time.Minute), owned}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	start, _ := reg.Lookup("start_intent")
	r, _ := start.Run(WithSessionID(context.Background(), "sess-a"), json.RawMessage(`{"node_id": 10, "adopt_changes": [2, 3]}`))
	if r.IsError {
		t.Fatalf("start_intent failed: %s", r.Content)
	}
	_, extra := splitClaimResult(t, r.Content)
	adopted, _ := extra["adopted_changes"].([]any)
	if len(adopted) != 1 || adopted[0].(map[string]any)["id"] != float64(2) {
		t.Errorf("adopted = %v, want change 2", extra["adopted_changes"])
	}
	if refused, _ := extra["not_adopted"].(map[string]any); refused["3"] == nil {
		t.Errorf("not_adopted = %v, want change 3", extra["not_adopted"])
	}
	if ms.changes[0].IntentID != 10 {
		t.Errorf("change 2 is on intent %d, want 10", ms.changes[0].IntentID)
	}
}
