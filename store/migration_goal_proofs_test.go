package store

import (
	"context"
	"io/fs"
	"testing"
)

func provesEdge(t *testing.T, s *Store, src, goal int64) bool {
	t.Helper()
	var ok bool
	if err := s.QueryRowContext(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM exploration_edges WHERE src_id = $1 AND dst_id = $2 AND rel = 'proves')`,
		src, goal).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

// Not parallel: the migration rewrites proves edges across the whole database.
func TestMigrationBackfillDeadGoalProofs(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "backfill-dead-proofs")
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	dismiss := func(f int64) {
		exec(`UPDATE findings SET dismissed = true, status = $2 WHERE id = $1`, f, FindingStatusFalsePositive)
		exec(`UPDATE exploration_nodes SET state = 'dismissed' WHERE id = (SELECT node_id FROM findings WHERE id = $1)`, f)
	}
	merge := func(f, into int64) {
		exec(`UPDATE findings SET merged_into = $2, status = $3 WHERE id = $1`, f, into, FindingStatusDuplicate)
		exec(`UPDATE exploration_nodes SET state = 'dismissed' WHERE id = (SELECT node_id FROM findings WHERE id = $1)`, f)
	}

	fDismissed, nDismissed := addTestFinding(t, s, taskID, expID)
	soleGoal := provedGoal(t, s, expID, nDismissed)
	dismiss(fDismissed)

	fSec, nSec := addTestFinding(t, s, taskID, expID)
	fMid, _ := addTestFinding(t, s, taskID, expID)
	fRoot, nRoot := addTestFinding(t, s, taskID, expID)
	mergedGoal := provedGoal(t, s, expID, nSec)
	merge(fMid, fRoot)
	merge(fSec, fMid)

	fDead, nDead := addTestFinding(t, s, taskID, expID)
	_, nLive := addTestFinding(t, s, taskID, expID)
	mixedGoal := provedGoal(t, s, expID, nDead, nLive)
	dismiss(fDead)

	sqlText, err := fs.ReadFile(migrationsFS(), "0006_backfill_dead_goal_proofs.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		exec(string(sqlText))
	}

	if got := goalState(t, s, soleGoal); got != "open" {
		t.Errorf("goal proved only by a dismissed finding: state = %q, want open", got)
	}
	if provesEdge(t, s, nDismissed, soleGoal) {
		t.Error("dismissed finding still proves its goal")
	}

	if got := goalState(t, s, mergedGoal); got != "met" {
		t.Errorf("goal proved by a merged secondary: state = %q, want met", got)
	}
	if !provesEdge(t, s, nRoot, mergedGoal) {
		t.Error("proof was not moved to the live root of the merge chain")
	}
	if provesEdge(t, s, nSec, mergedGoal) {
		t.Error("merged secondary still proves its goal")
	}

	if got := goalState(t, s, mixedGoal); got != "met" {
		t.Errorf("goal with a live proof: state = %q, want met", got)
	}
	if provesEdge(t, s, nDead, mixedGoal) {
		t.Error("dead proof edge was not removed")
	}
	if !provesEdge(t, s, nLive, mixedGoal) {
		t.Error("live proof edge was removed")
	}
}
