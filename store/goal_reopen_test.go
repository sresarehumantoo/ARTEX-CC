package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
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

func TestConcurrentDismissOfBothProofsReopensGoal(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "reopen-concurrent")

	for round := range 40 {
		f1, n1 := addTestFinding(t, s, taskID, expID)
		f2, n2 := addTestFinding(t, s, taskID, expID)
		goal := provedGoal(t, s, expID, n1, n2)

		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for _, f := range []int64{f1, f2} {
			wg.Go(func() {
				<-start
				errs <- s.DismissFinding(ctx, f, "false positive")
			})
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: DismissFinding: %v", round, err)
			}
		}
		if got := goalState(t, s, goal); got != "open" {
			t.Fatalf("round %d: goal state = %q, want open", round, got)
		}
	}
}

func TestProveGoalRefusesDeadFinding(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "prove-dead")
	dismissed, dismissedNode := addTestFinding(t, s, taskID, expID)
	primary, primaryNode := addTestFinding(t, s, taskID, expID)
	merged, mergedNode := addTestFinding(t, s, taskID, expID)
	goal := provedGoal(t, s, expID)
	if err := s.UpdateNodeState(ctx, goal, "open"); err != nil {
		t.Fatalf("UpdateNodeState: %v", err)
	}

	if err := s.DismissFinding(ctx, dismissed, "false positive"); err != nil {
		t.Fatalf("DismissFinding: %v", err)
	}
	if _, err := s.MergeFindings(ctx, primary, []int64{merged}); err != nil {
		t.Fatalf("MergeFindings: %v", err)
	}
	for _, node := range []int64{dismissedNode, mergedNode} {
		if err := s.ProveGoal(ctx, goal, node); !errors.Is(err, tools.ErrDeadEvidence) {
			t.Errorf("ProveGoal(node %d) = %v, want ErrDeadEvidence", node, err)
		}
	}
	if got := goalState(t, s, goal); got != "open" {
		t.Fatalf("goal state after refused proofs = %q, want open", got)
	}

	if err := s.ProveGoal(ctx, goal, primaryNode); err != nil {
		t.Fatalf("ProveGoal(live): %v", err)
	}
	if got := goalState(t, s, goal); got != "met" {
		t.Errorf("goal state = %q, want met", got)
	}
}

func findingState(t *testing.T, s *Store, id int64) (mergedInto *int64, dismissed bool) {
	t.Helper()
	if err := s.QueryRowContext(context.Background(),
		`SELECT merged_into, dismissed FROM findings WHERE id = $1`, id,
	).Scan(&mergedInto, &dismissed); err != nil {
		t.Fatalf("finding %d: %v", id, err)
	}
	return mergedInto, dismissed
}

func TestConcurrentOppositeMergesLeaveOneLive(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	for i := range 10 {
		taskID, expID := testEngagement(t, s, fmt.Sprintf("merge-race-%d", i))
		a, aNode := addTestFinding(t, s, taskID, expID)
		b, bNode := addTestFinding(t, s, taskID, expID)
		goal := provedGoal(t, s, expID, aNode, bNode)

		var wg sync.WaitGroup
		errs := make([]error, 2)
		for j, pair := range [][2]int64{{a, b}, {b, a}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[j] = s.MergeFindings(ctx, pair[0], []int64{pair[1]})
			}()
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("round %d: MergeFindings: %v", i, err)
			}
		}

		live := 0
		for _, f := range []int64{a, b} {
			if merged, dismissed := findingState(t, s, f); merged == nil && !dismissed {
				live++
			}
		}
		if live != 1 {
			t.Fatalf("round %d: %d live findings, want 1", i, live)
		}
		if got := goalState(t, s, goal); got != "met" {
			t.Fatalf("round %d: goal state = %q, want met", i, got)
		}
	}
}

func TestMergeIntoMergedPrimaryResolvesRoot(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-resolve")
	a, _ := addTestFinding(t, s, taskID, expID)
	b, _ := addTestFinding(t, s, taskID, expID)
	c, cNode := addTestFinding(t, s, taskID, expID)
	goal := provedGoal(t, s, expID, cNode)

	if _, err := s.MergeFindings(ctx, b, []int64{a}); err != nil {
		t.Fatalf("merge a into b: %v", err)
	}
	merged, err := s.MergeFindings(ctx, a, []int64{b, c})
	if err != nil {
		t.Fatalf("merge into merged a: %v", err)
	}
	if merged.ID != b || merged.RequestedPrimaryID != a {
		t.Errorf("result id = %d requested = %d, want %d and %d", merged.ID, merged.RequestedPrimaryID, b, a)
	}
	if len(merged.Skipped) != 1 || merged.Skipped[0].ID != b {
		t.Errorf("skipped = %+v, want only %d", merged.Skipped, b)
	}
	if into, _ := findingState(t, s, c); into == nil || *into != b {
		t.Errorf("c merged_into = %v, want %d", into, b)
	}
	if into, dismissed := findingState(t, s, b); into != nil || dismissed {
		t.Errorf("b is not live: merged_into = %v dismissed = %v", into, dismissed)
	}
	if got := goalState(t, s, goal); got != "met" {
		t.Errorf("goal state = %q, want met", got)
	}
}

func TestMergeSkipsDeadSecondaries(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-skip")
	p, _ := addTestFinding(t, s, taskID, expID)
	other, _ := addTestFinding(t, s, taskID, expID)
	gone, _ := addTestFinding(t, s, taskID, expID)
	fp, _ := addTestFinding(t, s, taskID, expID)

	if _, err := s.MergeFindings(ctx, other, []int64{gone}); err != nil {
		t.Fatalf("merge gone into other: %v", err)
	}
	if err := s.DismissFinding(ctx, fp, "false positive"); err != nil {
		t.Fatalf("DismissFinding: %v", err)
	}
	merged, err := s.MergeFindings(ctx, p, []int64{gone, fp, p})
	if err != nil {
		t.Fatalf("MergeFindings: %v", err)
	}
	want := []tools.SkippedFinding{
		{ID: gone, Reason: fmt.Sprintf("already merged into finding %d", other)},
		{ID: fp, Reason: "dismissed"},
	}
	if !slices.Equal(merged.Skipped, want) {
		t.Fatalf("skipped = %+v, want %+v", merged.Skipped, want)
	}
	if into, _ := findingState(t, s, gone); into == nil || *into != other {
		t.Errorf("gone merged_into = %v, want %d", into, other)
	}
	if into, _ := findingState(t, s, fp); into != nil {
		t.Errorf("dismissed finding was merged into %d", *into)
	}
}

func TestMergeIntoDismissedRootFails(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-dismissed-root")
	a, _ := addTestFinding(t, s, taskID, expID)
	b, _ := addTestFinding(t, s, taskID, expID)
	c, _ := addTestFinding(t, s, taskID, expID)

	if _, err := s.MergeFindings(ctx, b, []int64{a}); err != nil {
		t.Fatalf("merge a into b: %v", err)
	}
	if err := s.DismissFinding(ctx, b, "false positive"); err != nil {
		t.Fatalf("DismissFinding: %v", err)
	}
	_, err := s.MergeFindings(ctx, a, []int64{c})
	if err == nil || !strings.Contains(err.Error(), "dismissed") {
		t.Fatalf("err = %v, want dismissed primary error", err)
	}
	if into, dismissed := findingState(t, s, c); into != nil || dismissed {
		t.Errorf("c changed: merged_into = %v dismissed = %v", into, dismissed)
	}
}

func TestMergeSkipsResolvedPrimary(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-skip-root")
	a, _ := addTestFinding(t, s, taskID, expID)
	b, _ := addTestFinding(t, s, taskID, expID)

	if _, err := s.MergeFindings(ctx, b, []int64{a}); err != nil {
		t.Fatalf("merge a into b: %v", err)
	}
	merged, err := s.MergeFindings(ctx, a, []int64{b})
	if err != nil {
		t.Fatalf("MergeFindings: %v", err)
	}
	want := []tools.SkippedFinding{{ID: b, Reason: "is the resolved primary"}}
	if !slices.Equal(merged.Skipped, want) {
		t.Errorf("skipped = %+v, want %+v", merged.Skipped, want)
	}
}

func TestMergeFollowsChainToRoot(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-chain")
	a, _ := addTestFinding(t, s, taskID, expID)
	b, _ := addTestFinding(t, s, taskID, expID)
	c, _ := addTestFinding(t, s, taskID, expID)
	d, _ := addTestFinding(t, s, taskID, expID)

	if _, err := s.MergeFindings(ctx, b, []int64{a}); err != nil {
		t.Fatalf("merge a into b: %v", err)
	}
	if _, err := s.MergeFindings(ctx, c, []int64{b}); err != nil {
		t.Fatalf("merge b into c: %v", err)
	}
	merged, err := s.MergeFindings(ctx, a, []int64{d})
	if err != nil {
		t.Fatalf("MergeFindings: %v", err)
	}
	if merged.ID != c || merged.RequestedPrimaryID != a {
		t.Errorf("result id = %d requested = %d, want %d and %d", merged.ID, merged.RequestedPrimaryID, c, a)
	}
	if into, _ := findingState(t, s, d); into == nil || *into != c {
		t.Errorf("d merged_into = %v, want %d", into, c)
	}
}

func TestMergeChainCycleFails(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-cycle")
	a, _ := addTestFinding(t, s, taskID, expID)
	b, _ := addTestFinding(t, s, taskID, expID)
	c, _ := addTestFinding(t, s, taskID, expID)
	if _, err := s.ExecContext(ctx,
		`UPDATE findings SET merged_into = CASE id WHEN $1 THEN $2 ELSE $1 END WHERE id IN ($1, $2)`, a, b,
	); err != nil {
		t.Fatalf("create cycle: %v", err)
	}

	_, err := s.MergeFindings(ctx, a, []int64{c})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("err = %v, want cycle error", err)
	}
	if into, _ := findingState(t, s, c); into != nil {
		t.Errorf("c merged_into = %d, want unchanged", *into)
	}
}

func TestConcurrentMergesAlongChainDoNotDeadlock(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-chain-race")

	for round := range 40 {
		low, _ := addTestFinding(t, s, taskID, expID)
		mid, _ := addTestFinding(t, s, taskID, expID)
		high, _ := addTestFinding(t, s, taskID, expID)
		if _, err := s.MergeFindings(ctx, low, []int64{mid}); err != nil {
			t.Fatalf("round %d: merge mid into low: %v", round, err)
		}

		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for _, m := range [][2]int64{{mid, high}, {low, mid}} {
			wg.Go(func() {
				<-start
				_, err := s.MergeFindings(ctx, m[0], []int64{m[1]})
				errs <- err
			})
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: MergeFindings: %v", round, err)
			}
		}
		if into, _ := findingState(t, s, high); into == nil || *into != low {
			t.Fatalf("round %d: high merged_into = %v, want %d", round, into, low)
		}
	}
}

func TestConcurrentMergesOverSharedGoalsDoNotDeadlock(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-goal-race")

	for round := range 100 {
		p1, _ := addTestFinding(t, s, taskID, expID)
		p2, _ := addTestFinding(t, s, taskID, expID)
		s1, n1 := addTestFinding(t, s, taskID, expID)
		s2, n2 := addTestFinding(t, s, taskID, expID)
		s3, n3 := addTestFinding(t, s, taskID, expID)
		s4, n4 := addTestFinding(t, s, taskID, expID)
		g1 := provedGoal(t, s, expID, n1, n4)
		g2 := provedGoal(t, s, expID, n2, n3)

		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for _, m := range [][3]int64{{p1, s1, s2}, {p2, s3, s4}} {
			wg.Go(func() {
				<-start
				_, err := s.MergeFindings(ctx, m[0], m[1:])
				errs <- err
			})
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: MergeFindings: %v", round, err)
			}
		}
		for _, g := range []int64{g1, g2} {
			if got := goalState(t, s, g); got != "met" {
				t.Fatalf("round %d: goal %d state = %q, want met", round, g, got)
			}
		}
	}
}

func TestConcurrentMergeAndProveGoalDoNotDeadlock(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-prove-race")

	for round := range 100 {
		primary, _ := addTestFinding(t, s, taskID, expID)
		s1, n1 := addTestFinding(t, s, taskID, expID)
		s2, n2 := addTestFinding(t, s, taskID, expID)
		goal := provedGoal(t, s, expID, n1)

		start := make(chan struct{})
		var mergeErr, proveErr error
		var wg sync.WaitGroup
		wg.Go(func() {
			<-start
			_, mergeErr = s.MergeFindings(ctx, primary, []int64{s1, s2})
		})
		wg.Go(func() {
			<-start
			proveErr = s.ProveGoal(ctx, goal, n2)
		})
		close(start)
		wg.Wait()
		if mergeErr != nil {
			t.Fatalf("round %d: MergeFindings: %v", round, mergeErr)
		}
		if proveErr != nil && !errors.Is(proveErr, tools.ErrDeadEvidence) {
			t.Fatalf("round %d: ProveGoal: %v", round, proveErr)
		}
		if got := goalState(t, s, goal); got != "met" {
			t.Fatalf("round %d: goal state = %q, want met", round, got)
		}
	}
}
