package store

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func doneIntent(t *testing.T, s *Store, expID int64, result string) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "test login"}, 5, "open", "agent", nil)
	if err != nil {
		t.Fatalf("AddNode intent: %v", err)
	}
	if _, err := s.ClaimIntent(ctx, id, "sess-a", time.Hour); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	if err := s.UpdateNodeFields(ctx, id, "done", map[string]any{"completed_by": "sess-a", "result": result}); err != nil {
		t.Fatalf("UpdateNodeFields: %v", err)
	}
	return id
}

func engagementStatus(t *testing.T, s *Store, taskID int64) (task, exploration string) {
	t.Helper()
	if err := s.QueryRowContext(context.Background(),
		`SELECT t.status, e.status FROM tasks t JOIN explorations e ON e.id = t.exploration_id WHERE t.id = $1`,
		taskID).Scan(&task, &exploration); err != nil {
		t.Fatalf("engagement status: %v", err)
	}
	return task, exploration
}

func TestReopenIntentMovesResultToHistory(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "reopen-intent")
	intent := doneIntent(t, s, expID, "no injection found")
	fact, err := s.AddFact(ctx, expID, intent, map[string]any{"text": "login form at /login"}, nil)
	if err != nil {
		t.Fatalf("AddFact: %v", err)
	}
	_, findingNode, err := s.AddFinding(ctx, tools.FindingInput{
		TaskID: taskID, ExplorationID: expID, IntentID: intent, VulnClass: "xss", Severity: "low", Summary: "xss",
	})
	if err != nil {
		t.Fatalf("AddFinding: %v", err)
	}
	if _, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET owner = 'stale', claim_expires_at = now() + interval '1 hour' WHERE id = $1`, intent); err != nil {
		t.Fatal(err)
	}

	res, err := s.ReopenNode(ctx, expID, intent, "web-ui", "the test missed the second form")
	if err != nil {
		t.Fatalf("ReopenNode: %v", err)
	}
	if want := []int64{fact, findingNode}; !slices.Equal(res.Superseded, want) {
		t.Errorf("superseded = %v, want %v", res.Superseded, want)
	}
	if len(res.ReopenedGoals) != 0 {
		t.Errorf("reopened goals = %v, want none", res.ReopenedGoals)
	}

	n, err := s.NodeDetail(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	if n.State != "open" {
		t.Errorf("state = %q, want open", n.State)
	}
	for _, k := range []string{"result", "completed_by"} {
		if _, ok := n.Payload[k]; ok {
			t.Errorf("payload still has %s: %v", k, n.Payload)
		}
	}
	if n.Payload["reopen_reason"] != "the test missed the second form" || n.Payload["reopened_by"] != "web-ui" || n.Payload["reopened_at"] == nil {
		t.Errorf("reopen fields = %v", n.Payload)
	}
	history, _ := n.Payload["history"].([]any)
	if len(history) != 1 {
		t.Fatalf("history = %v, want one entry", n.Payload["history"])
	}
	entry, _ := history[0].(map[string]any)
	if entry["state"] != "done" || entry["result"] != "no injection found" || entry["completed_by"] != "sess-a" ||
		entry["reopened_by"] != "web-ui" || entry["reason"] != "the test missed the second form" || entry["at"] == nil {
		t.Errorf("history entry = %v", entry)
	}
	if _, ok := entry["cancelled_by"]; ok {
		t.Errorf("history entry has empty cancelled_by: %v", entry)
	}

	var owner *string
	var expires *time.Time
	if err := s.QueryRowContext(ctx,
		`SELECT owner, claim_expires_at FROM exploration_nodes WHERE id = $1`, intent).Scan(&owner, &expires); err != nil {
		t.Fatal(err)
	}
	if owner != nil || expires != nil {
		t.Errorf("claim not cleared: owner %v, expires %v", owner, expires)
	}

	for _, id := range []int64{fact, findingNode} {
		y, err := s.NodeDetail(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if y.Payload["superseded_by_reopen"] != float64(intent) || y.Payload["superseded_at"] == nil {
			t.Errorf("node %d payload = %v, want superseded marker", id, y.Payload)
		}
		if y.State == "dismissed" {
			t.Errorf("node %d was dismissed", id)
		}
	}
	findings, err := s.ListFindings(ctx, expID)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].SupersededByReopen != intent {
		t.Errorf("findings = %+v, want one superseded by %d", findings, intent)
	}
	report, err := s.ListFindingsForReport(ctx, expID)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 1 || report[0].SupersededByReopen != intent {
		t.Errorf("report findings = %+v, want one superseded by %d", report, intent)
	}

	if _, err := s.ClaimIntent(ctx, intent, "sess-b", time.Hour); err != nil {
		t.Fatalf("reopened intent not claimable: %v", err)
	}
	if err := s.UpdateNodeFields(ctx, intent, "stopped", map[string]any{
		"cancelled_by": "sess-b", "reopen_reason": nil, "reopened_by": nil, "reopened_at": nil,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReopenNode(ctx, expID, intent, "web-ui", "again"); err != nil {
		t.Fatalf("second ReopenNode: %v", err)
	}
	n, _ = s.NodeDetail(ctx, intent)
	history, _ = n.Payload["history"].([]any)
	if len(history) != 2 {
		t.Fatalf("history after second reopen = %v, want two entries", n.Payload["history"])
	}
	if second, _ := history[1].(map[string]any); second["state"] != "stopped" || second["cancelled_by"] != "sess-b" {
		t.Errorf("second history entry = %v", second)
	}
}

func TestReopenIntentReopensGoalItAloneProved(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "reopen-intent-goal")
	intent := doneIntent(t, s, expID, "got admin")
	fact, err := s.AddFact(ctx, expID, intent, map[string]any{"text": "admin session"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, otherNode := addTestFinding(t, s, taskID, expID)
	alone := provedGoal(t, s, expID, fact)
	shared := provedGoal(t, s, expID, fact, otherNode)
	if err := s.CompleteEngagement(ctx, taskID, ""); err != nil {
		t.Fatal(err)
	}

	res, err := s.ReopenNode(ctx, expID, intent, "web-ui", "the session was a test account")
	if err != nil {
		t.Fatalf("ReopenNode: %v", err)
	}
	if !slices.Equal(res.ReopenedGoals, []int64{alone}) {
		t.Errorf("reopened goals = %v, want [%d]", res.ReopenedGoals, alone)
	}
	if got := goalState(t, s, alone); got != "open" {
		t.Errorf("sole-proof goal state = %q, want open", got)
	}
	if got := goalState(t, s, shared); got != "met" {
		t.Errorf("shared-proof goal state = %q, want met", got)
	}
	g, _ := s.NodeDetail(ctx, alone)
	if history, _ := g.Payload["history"].([]any); len(history) != 1 {
		t.Errorf("goal history = %v, want one entry", g.Payload["history"])
	}
	var edges int
	if err := s.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exploration_edges WHERE dst_id = $1 AND rel = $2`, alone, RelProves).Scan(&edges); err != nil {
		t.Fatal(err)
	}
	if edges != 1 {
		t.Errorf("proves edges on reopened goal = %d, want 1 kept", edges)
	}
	if task, exp := engagementStatus(t, s, taskID); task != "running" || exp != "open" {
		t.Errorf("engagement status = %s/%s, want running/open", task, exp)
	}

	if err := s.ProveGoal(ctx, alone, fact); err != nil {
		t.Fatal(err)
	}
	g, _ = s.NodeDetail(ctx, alone)
	if _, ok := g.Payload["reopen_reason"]; ok {
		t.Errorf("proved goal still carries reopen_reason: %v", g.Payload)
	}
}

func TestReopenIntentWithoutGoalKeepsEngagementDone(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "reopen-intent-nogoal")
	intent := doneIntent(t, s, expID, "nothing")
	if err := s.CompleteEngagement(ctx, taskID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReopenNode(ctx, expID, intent, "web-ui", "redo"); err != nil {
		t.Fatal(err)
	}
	if task, exp := engagementStatus(t, s, taskID); task != "done" || exp != "achieved" {
		t.Errorf("engagement status = %s/%s, want done/achieved", task, exp)
	}
}

func TestReopenGoalDirectly(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "reopen-goal")
	_, node := addTestFinding(t, s, taskID, expID)
	goal := provedGoal(t, s, expID, node)
	if err := s.CompleteEngagement(ctx, taskID, ""); err != nil {
		t.Fatal(err)
	}

	res, err := s.ReopenNode(ctx, expID, goal, "web-ui", "proof was weak")
	if err != nil {
		t.Fatalf("ReopenNode: %v", err)
	}
	if res.Kind != KindGoal {
		t.Errorf("kind = %q, want goal", res.Kind)
	}
	g, _ := s.NodeDetail(ctx, goal)
	if g.State != "open" || g.Payload["reopen_reason"] != "proof was weak" {
		t.Errorf("goal = %+v", g)
	}
	if task, exp := engagementStatus(t, s, taskID); task != "running" || exp != "open" {
		t.Errorf("engagement status = %s/%s, want running/open", task, exp)
	}
}

func TestReopenRefusesOtherStates(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "reopen-refused")
	_, otherExp := testEngagement(t, s, "reopen-refused-other")

	open, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "open"}, 0, "open", "agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "blocked"}, 0, "blocked", "agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	openGoal, err := s.AddNode(ctx, expID, KindGoal, map[string]any{"text": "g"}, 0, "open", "system", nil)
	if err != nil {
		t.Fatal(err)
	}
	fact, err := s.AddFact(ctx, expID, 0, map[string]any{"text": "f"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{open, blocked, openGoal, fact} {
		if _, err := s.ReopenNode(ctx, expID, id, "web-ui", "why"); !errors.Is(err, tools.ErrNotReopenable) {
			t.Errorf("node %d: err = %v, want ErrNotReopenable", id, err)
		}
	}
	if got := goalState(t, s, blocked); got != "blocked" {
		t.Errorf("blocked intent state = %q after refusal", got)
	}

	done := doneIntent(t, s, expID, "r")
	if _, err := s.ReopenNode(ctx, otherExp, done, "web-ui", "why"); !errors.Is(err, tools.ErrNodeNotFound) {
		t.Errorf("other exploration: err = %v, want ErrNodeNotFound", err)
	}
	if _, err := s.ReopenNode(ctx, expID, 1<<40, "web-ui", "why"); !errors.Is(err, tools.ErrNodeNotFound) {
		t.Errorf("missing node: err = %v, want ErrNodeNotFound", err)
	}
}
