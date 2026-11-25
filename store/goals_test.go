package store

import (
	"context"
	"sync"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func goalTexts(t *testing.T, s *Store, expID int64) []string {
	t.Helper()
	nodes, err := s.ListNodes(context.Background(), expID, KindGoal, 0)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	var out []string
	for _, n := range nodes {
		text, _ := n.Payload["text"].(string)
		out = append(out, text)
	}
	return out
}

func TestAddGoalsSkipsDuplicatesAndRefusesSecondSet(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "add-goals")

	res, err := s.AddGoals(ctx, expID, []tools.GoalInput{
		{Text: "Dump the database", VulnClass: "sqli"},
		{Text: "dump  the DATABASE."},
		{Text: "Get RCE"},
	}, false)
	if err != nil {
		t.Fatalf("AddGoals: %v", err)
	}
	if len(res.Created) != 2 || len(res.Skipped) != 1 || len(res.Existing) != 0 {
		t.Fatalf("first set: %+v", res)
	}
	if sk := res.Skipped[0]; sk.Reason != tools.GoalSkipDuplicate || sk.ExistingID != res.Created[0].ID {
		t.Errorf("skipped = %+v, want duplicate of %d", sk, res.Created[0].ID)
	}
	edges, err := s.Edges(ctx, expID, 0)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	spawned := 0
	for _, e := range edges {
		if e.Rel == RelSpawns && (e.To == res.Created[0].ID || e.To == res.Created[1].ID) {
			spawned++
		}
	}
	if spawned != 2 {
		t.Errorf("got %d spawns edges to the new goals, want 2", spawned)
	}

	res, err = s.AddGoals(ctx, expID, []tools.GoalInput{{Text: "Read /etc/passwd"}}, false)
	if err != nil {
		t.Fatalf("second AddGoals: %v", err)
	}
	if len(res.Created) != 0 || len(res.Existing) != 2 || len(res.Skipped) != 1 || res.Skipped[0].Reason != tools.GoalSkipAlreadySet {
		t.Fatalf("second set without append: %+v", res)
	}

	res, err = s.AddGoals(ctx, expID, []tools.GoalInput{{Text: "Read /etc/passwd"}, {Text: "get rce"}}, true)
	if err != nil {
		t.Fatalf("append AddGoals: %v", err)
	}
	if len(res.Created) != 1 || res.Created[0].Text != "Read /etc/passwd" || len(res.Skipped) != 1 || len(res.Existing) != 0 {
		t.Fatalf("append: %+v", res)
	}
	if got := goalTexts(t, s, expID); len(got) != 3 {
		t.Errorf("goals = %q, want 3", got)
	}
}

func TestAddGoalsConcurrentCallsDoNotDuplicate(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "add-goals-race")

	sets := [][]tools.GoalInput{
		{{Text: "Dump the database"}, {Text: "Get RCE"}, {Text: "Read admin notes"}},
		{{Text: "Achieve remote code execution"}, {Text: "Exfiltrate user records"}},
		{{Text: "dump the database"}, {Text: "Take over an admin account"}},
	}
	var wg sync.WaitGroup
	results := make([]*tools.GoalsResult, len(sets))
	errs := make([]error, len(sets))
	for i := range sets {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = s.AddGoals(ctx, expID, sets[i], false)
		}(i)
	}
	wg.Wait()

	winner := -1
	for i, r := range results {
		if errs[i] != nil {
			t.Fatalf("AddGoals[%d]: %v", i, errs[i])
		}
		if len(r.Created) == 0 {
			if len(r.Existing) == 0 {
				t.Errorf("call %d created nothing and got no existing_goals: %+v", i, r)
			}
			continue
		}
		if winner >= 0 {
			t.Fatalf("calls %d and %d both created goals", winner, i)
		}
		winner = i
	}
	if winner < 0 {
		t.Fatal("no call created goals")
	}
	if got := goalTexts(t, s, expID); len(got) != len(sets[winner]) {
		t.Errorf("goals = %q, want only call %d's %d goals", got, winner, len(sets[winner]))
	}
}
