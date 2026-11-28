package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStartIntentOnOpenIntentHandsOffItsChanges(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
	for i := range 25 {
		c := unownedChange(int64(i+1), "intent_open", time.Minute)
		c.IntentID, c.Backup = 10, strings.Repeat("x", 1000)
		ms.changes = append(ms.changes, c)
	}
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	start, _ := reg.Lookup("start_intent")
	r, _ := start.Run(WithSessionID(context.Background(), "sess-a"), json.RawMessage(`{"node_id": 10}`))
	_, extra := splitClaimResult(t, r.Content)
	inherited, _ := extra["inherited_target_changes"].([]any)
	if len(inherited) != maxWarningIDs || extra["inherited_total"] != float64(25) {
		t.Fatalf("inherited %d entries, total %v; want %d of 25", len(inherited), extra["inherited_total"], maxWarningIDs)
	}
	if b := inherited[0].(map[string]any)["backup"].(string); len([]rune(b)) > maxHandoffText {
		t.Errorf("backup not truncated: %d runes", len([]rune(b)))
	}
	if h, _ := extra["handoff"].(string); !strings.HasPrefix(h, "This intent already had 25 unrestored") {
		t.Errorf("handoff = %q", h)
	}
}

func TestClosingWarningCapsIDs(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
	for i := range 25 {
		c := unownedChange(int64(i+1), "", 0)
		c.IntentID = 10
		ms.changes = append(ms.changes, c)
	}
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	complete, _ := reg.Lookup("complete_intent")
	r, _ := complete.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if !strings.Contains(r.Content, "#20, and 5 more)") || strings.Contains(r.Content, "#21") {
		t.Errorf("warning does not cap the ids: %q", r.Content)
	}
}
