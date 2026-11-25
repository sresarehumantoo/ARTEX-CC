package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func overviewCursor(t *testing.T, s *Store, expID int64) tools.ChangeCursor {
	t.Helper()
	ov, err := s.GraphOverview(context.Background(), expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	c, err := tools.ParseChangeCursor(ov.ChangesCursor)
	if err != nil {
		t.Fatalf("parse changes_cursor %q: %v", ov.ChangesCursor, err)
	}
	return c
}

func changesByKind(changes []tools.Change) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, c := range changes {
		out[c.Kind] = append(out[c.Kind], c.Fields)
	}
	return out
}

func TestListChanges(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "list-changes")
	otherTask, otherExp := testEngagement(t, s, "list-changes-other")

	start := overviewCursor(t, s, expID)

	factID, err := s.AddNode(ctx, expID, KindFact, map[string]any{"summary": "admin panel at /admin"}, 5, "confirmed", "worker", nil)
	if err != nil {
		t.Fatal(err)
	}
	intentID, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"description": "brute /admin"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatal(err)
	}
	findingID, _, err := s.AddFinding(ctx, tools.FindingInput{TaskID: taskID, ExplorationID: expID, VulnClass: "xss", Severity: "high", Summary: "reflected xss"})
	if err != nil {
		t.Fatal(err)
	}
	assetID, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: fmt.Sprintf("changes-%d.example", taskID)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNode(ctx, otherExp, KindFact, map[string]any{"summary": "other engagement"}, 5, "confirmed", "worker", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertAsset(ctx, otherTask, tools.AssetInput{Type: "root_domain", Domain: fmt.Sprintf("changes-other-%d.example", otherTask)}); err != nil {
		t.Fatal(err)
	}

	changes, err := s.ListChanges(ctx, expID, taskID, start, 100)
	if err != nil {
		t.Fatalf("ListChanges: %v", err)
	}
	got := changesByKind(changes)
	want := map[string]int64{"fact": factID, "intent": intentID, "finding": findingID, "asset": assetID}
	if len(changes) != len(want) {
		t.Fatalf("changes = %+v, want exactly %v", got, want)
	}
	for kind, id := range want {
		rows := got[kind]
		if len(rows) != 1 || rows[0]["id"] != id || rows[0]["change"] != "new" {
			t.Errorf("%s changes = %v, want new id %d", kind, rows, id)
		}
	}
	for i := 1; i < len(changes); i++ {
		if changes[i].Cursor.At.Before(changes[i-1].Cursor.At) {
			t.Errorf("changes not oldest first at %d", i)
		}
	}

	cursor := changes[len(changes)-1].Cursor
	again, err := s.ListChanges(ctx, expID, taskID, cursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("re-poll at cursor returned %+v, want nothing", again)
	}

	if _, err := s.ClaimIntent(ctx, intentID, "sess-a", tools.IntentLease); err != nil {
		t.Fatal(err)
	}
	if err := s.DismissFinding(ctx, findingID, "false positive"); err != nil {
		t.Fatal(err)
	}
	updates, err := s.ListChanges(ctx, expID, taskID, cursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	got = changesByKind(updates)
	if len(updates) != 2 {
		t.Fatalf("updates = %+v, want intent and finding", got)
	}
	in := got["intent"]
	if len(in) != 1 || in[0]["change"] != "updated" || in[0]["state"] != "running" || in[0]["claimed_by"] != tools.OwnerLabel("sess-a") {
		t.Errorf("intent update = %v", in)
	}
	fi := got["finding"]
	if len(fi) != 1 || fi[0]["change"] != "updated" || fi[0]["dismissed"] != true {
		t.Errorf("finding update = %v", fi)
	}

	ts, err := tools.ParseChangeCursor(start.At.UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	byTime, err := s.ListChanges(ctx, expID, taskID, ts, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(byTime) != len(want) {
		t.Errorf("RFC 3339 since returned %d changes, want %d", len(byTime), len(want))
	}
}

func TestListChangesPagesThroughTies(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "list-changes-ties")
	cursor := overviewCursor(t, s, expID)

	if _, err := s.ExecContext(ctx, `
		INSERT INTO exploration_nodes (exploration_id, kind, payload, state, origin)
		SELECT $1, 'fact', jsonb_build_object('summary', 'bulk ' || g), 'confirmed', 'worker'
		FROM generate_series(1, 5) g`, expID); err != nil {
		t.Fatal(err)
	}

	seen := map[int64]int{}
	for range 10 {
		page, err := s.ListChanges(ctx, expID, taskID, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, c := range page {
			seen[c.Fields["id"].(int64)]++
		}
		cursor = page[len(page)-1].Cursor
	}
	if len(seen) != 5 {
		t.Errorf("paged %d distinct facts, want 5: %v", len(seen), seen)
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("fact %d returned %d times", id, n)
		}
	}
}

func TestListClaims(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	expID, held := claimIntentNode(t, s, "list-claims")
	expired, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"description": "lapsed"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"description": "unclaimed"}, 5, "open", "planner", nil); err != nil {
		t.Fatal(err)
	}

	if _, err := s.ClaimIntent(ctx, held, "sess-a", tools.IntentLease); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimIntent(ctx, expired, "sess-a", time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	claims, err := s.ListClaims(ctx, expID, "sess-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 2 {
		t.Fatalf("claims = %+v, want 2", claims)
	}
	if claims[0].NodeID != expired || !claims[0].Expired {
		t.Errorf("first claim = %+v, want lapsed intent %d marked expired", claims[0], expired)
	}
	if claims[1].NodeID != held || claims[1].Expired || time.Until(claims[1].ExpiresAt) < 25*time.Minute {
		t.Errorf("second claim = %+v, want live claim on %d", claims[1], held)
	}
	if claims[1].Payload["summary"] != "list-claims" {
		t.Errorf("claim payload missing: %+v", claims[1])
	}

	other, err := s.ListClaims(ctx, expID, "sess-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Errorf("sess-b claims = %+v, want none", other)
	}
}
