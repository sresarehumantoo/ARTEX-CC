package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestClosingIntentWithUnrestoredChangesWarns(t *testing.T) {
	t.Parallel()
	for _, tool := range []string{"complete_intent", "cancel_intent"} {
		ms := newIntentMockStore()
		ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
		ms.nodes[11] = &Node{ID: 11, Kind: KindIntent, State: "open"}
		change := unownedChange(5, "", 0)
		change.IntentID = 10
		ms.changes = []TargetChange{change}
		reg := NewRegistry()
		RegisterIntentTools(reg, ms, 1)
		finish, _ := reg.Lookup(tool)

		r, _ := finish.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
		if r.IsError {
			t.Fatalf("%s refused: %s", tool, r.Content)
		}
		if ms.nodes[10].State != "done" && ms.nodes[10].State != "stopped" {
			t.Errorf("%s left intent 10 %q", tool, ms.nodes[10].State)
		}
		if !strings.Contains(r.Content, "WARNING: intent 10 closed with 1 unrestored target change(s) (#5)") {
			t.Errorf("%s result = %q, want a warning naming #5", tool, r.Content)
		}
		if r, _ := finish.Run(testSessionCtx, json.RawMessage(`{"node_id": 11}`)); strings.Contains(r.Content, "WARNING") {
			t.Errorf("%s warned for an intent without changes: %q", tool, r.Content)
		}
	}
}

type goalMetChangesStore struct {
	mockStore
	live []TargetChange
}

func (m *goalMetChangesStore) ListTargetChanges(context.Context, int64, bool) ([]TargetChange, error) {
	return m.live, nil
}

func TestGoalMetWarnsAboutUnrestoredChanges(t *testing.T) {
	t.Parallel()
	ms := &goalMetChangesStore{live: []TargetChange{unownedChange(3, "no_intent", time.Minute)}}
	r, _ := makeGoalMet(ms, 1, 1).Run(context.Background(), json.RawMessage(`{}`))
	if r.IsError || !strings.Contains(r.Content, "WARNING: 1 target change(s) are still unrestored (#3)") {
		t.Errorf("goal_met = %q, want a warning naming #3", r.Content)
	}
	ms.live = nil
	if r, _ := makeGoalMet(ms, 1, 1).Run(context.Background(), json.RawMessage(`{}`)); strings.Contains(r.Content, "WARNING") {
		t.Errorf("goal_met warned with nothing unrestored: %q", r.Content)
	}
}
