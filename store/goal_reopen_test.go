package store

import (
	"context"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func provedGoal(t *testing.T, s *Store, expID int64, proofs ...int64) int64 {
	t.Helper()
	ctx := context.Background()
	goal, err := s.AddNode(ctx, expID, KindGoal, map[string]any{"text": "g"}, 0, "open", "system", nil)
	if err != nil {
		t.Fatalf("AddNode goal: %v", err)
	}
	for _, p := range proofs {
		if err := s.AddEdge(ctx, p, RelProves, goal); err != nil {
			t.Fatalf("AddEdge proves: %v", err)
		}
	}
	if err := s.UpdateNodeState(ctx, goal, "met"); err != nil {
		t.Fatalf("UpdateNodeState: %v", err)
	}
	return goal
}

func addTestFinding(t *testing.T, s *Store, taskID, expID int64) (int64, int64) {
	t.Helper()
	id, node, err := s.AddFinding(context.Background(), tools.FindingInput{
		TaskID: taskID, ExplorationID: expID, VulnClass: "sqli", Severity: "high", Summary: "sqli",
	})
	if err != nil {
		t.Fatalf("AddFinding: %v", err)
	}
	return id, node
}

func goalState(t *testing.T, s *Store, goal int64) string {
	t.Helper()
	n, err := s.NodeDetail(context.Background(), goal)
	if err != nil || n == nil {
		t.Fatalf("NodeDetail(%d): %v", goal, err)
	}
	return n.State
}

func TestDismissSoleProofReopensGoal(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	taskID, expID := testEngagement(t, s, "reopen-sole")
	f, node := addTestFinding(t, s, taskID, expID)
	goal := provedGoal(t, s, expID, node)

	if err := s.DismissFinding(context.Background(), f, "false positive"); err != nil {
		t.Fatalf("DismissFinding: %v", err)
	}
	if got := goalState(t, s, goal); got != "open" {
		t.Errorf("goal state = %q, want open", got)
	}
}

func TestDismissWithOtherValidProofKeepsGoalMet(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	taskID, expID := testEngagement(t, s, "reopen-other")
	f1, n1 := addTestFinding(t, s, taskID, expID)
	_, n2 := addTestFinding(t, s, taskID, expID)
	goal := provedGoal(t, s, expID, n1, n2)

	if err := s.DismissFinding(context.Background(), f1, "false positive"); err != nil {
		t.Fatalf("DismissFinding: %v", err)
	}
	if got := goalState(t, s, goal); got != "met" {
		t.Errorf("goal state = %q, want met", got)
	}
}

func TestDismissAfterEarlierDismissalReopensGoal(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "reopen-both")
	f1, n1 := addTestFinding(t, s, taskID, expID)
	f2, n2 := addTestFinding(t, s, taskID, expID)
	goal := provedGoal(t, s, expID, n1, n2)

	for _, f := range []int64{f1, f2} {
		if err := s.DismissFinding(ctx, f, "false positive"); err != nil {
			t.Fatalf("DismissFinding(%d): %v", f, err)
		}
	}
	if got := goalState(t, s, goal); got != "open" {
		t.Errorf("goal state = %q, want open", got)
	}
}

func TestMergeMovesProofToPrimary(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "reopen-merge")
	primary, primaryNode := addTestFinding(t, s, taskID, expID)
	secondary, secondaryNode := addTestFinding(t, s, taskID, expID)
	goal := provedGoal(t, s, expID, secondaryNode)

	if _, err := s.MergeFindings(ctx, primary, []int64{secondary}); err != nil {
		t.Fatalf("MergeFindings: %v", err)
	}
	if got := goalState(t, s, goal); got != "met" {
		t.Fatalf("goal state after merge = %q, want met", got)
	}
	var src int64
	if err := s.QueryRowContext(ctx,
		`SELECT src_id FROM exploration_edges WHERE dst_id = $1 AND rel = $2`, goal, RelProves,
	).Scan(&src); err != nil {
		t.Fatalf("proves edge: %v", err)
	}
	if src != primaryNode {
		t.Errorf("proves edge source = %d, want primary node %d", src, primaryNode)
	}

	if err := s.DismissFinding(ctx, primary, "false positive"); err != nil {
		t.Fatalf("DismissFinding: %v", err)
	}
	if got := goalState(t, s, goal); got != "open" {
		t.Errorf("goal state after dismissing primary = %q, want open", got)
	}
}
