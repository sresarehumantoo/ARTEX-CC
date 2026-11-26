package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func intentSummaries(t *testing.T, s *Store, expID int64) map[int64]string {
	t.Helper()
	nodes, err := s.ListNodes(context.Background(), expID, KindIntent, 0)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	out := map[int64]string{}
	for _, n := range nodes {
		out[n.ID], _ = n.Payload["summary"].(string)
	}
	return out
}

func TestAddIntentsSkipsDuplicates(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "add-intents")
	asset, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: "add-intents.example"})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}

	res, err := s.AddIntents(ctx, expID, []tools.IntentInput{
		{Summary: "Test the /login form for SQL injection.", Priority: 5},
		{Summary: "test the login form, for SQL-injection", Priority: 5},
		{Summary: "Enumerate open ports", Priority: 5, AssetIDs: []int64{asset}},
		{Summary: "Enumerate open ports", Priority: 5},
		{Summary: "Enumerate open ports", Priority: 5, AssetIDs: []int64{asset + 100000}},
	})
	if err != nil {
		t.Fatalf("AddIntents: %v", err)
	}
	if res.IDs[0] == 0 || res.IDs[1] != 0 || res.IDs[2] == 0 || res.IDs[3] == 0 || res.IDs[4] != 0 {
		t.Fatalf("ids = %v, want 2nd skipped and 5th failed", res.IDs)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Index != 1 || res.Skipped[0].ExistingID != res.IDs[0] || res.Skipped[0].State != "open" {
		t.Errorf("skipped = %+v, want index 1 matching %d", res.Skipped, res.IDs[0])
	}
	if len(res.Errors) != 1 || res.Errors[4] == "" {
		t.Errorf("errors = %v, want one for index 4", res.Errors)
	}
	if len(res.Open) != 0 {
		t.Errorf("open = %+v, want none on an empty board", res.Open)
	}

	origin, err := s.OriginFactID(ctx, expID)
	if err != nil {
		t.Fatalf("OriginFactID: %v", err)
	}
	edges, err := s.Edges(ctx, expID, 0)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	derived := 0
	for _, e := range edges {
		if e.From == origin && e.Rel == RelDerivedFrom && (e.To == res.IDs[0] || e.To == res.IDs[2] || e.To == res.IDs[3]) {
			derived++
		}
	}
	if derived != 3 {
		t.Errorf("got %d derived_from edges from the origin fact, want 3", derived)
	}
	n, err := s.NodeDetail(ctx, res.IDs[2])
	if err != nil {
		t.Fatalf("NodeDetail: %v", err)
	}
	if n.Payload["asset_ids"] == nil {
		t.Errorf("payload = %v, want asset_ids", n.Payload)
	}

	if _, err := s.ClaimIntent(ctx, res.IDs[0], "sess-a", time.Hour); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	if err := s.UpdateNodeFields(ctx, res.IDs[3], "done", nil); err != nil {
		t.Fatalf("UpdateNodeFields: %v", err)
	}
	fact, err := s.AddNode(ctx, expID, KindFact, map[string]any{"summary": "login form found"}, 5, "confirmed", "worker", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	res, err = s.AddIntents(ctx, expID, []tools.IntentInput{
		{Summary: "TEST THE LOGIN FORM FOR SQL INJECTION"},
		{Summary: "Enumerate open ports"},
		{Summary: "Brute force the admin password", ParentIDs: []int64{fact}},
	})
	if err != nil {
		t.Fatalf("second AddIntents: %v", err)
	}
	if res.IDs[0] != 0 || res.IDs[1] == 0 || res.IDs[2] == 0 {
		t.Fatalf("ids = %v, want running duplicate skipped and done one re-added", res.IDs)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].State != "running" {
		t.Errorf("skipped = %+v, want the running intent", res.Skipped)
	}
	edges, err = s.Edges(ctx, expID, 0)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	var fromFact bool
	for _, e := range edges {
		if e.From == fact && e.To == res.IDs[2] && e.Rel == RelDerivedFrom {
			fromFact = true
		}
	}
	if !fromFact {
		t.Error("missing derived_from edge from the parent fact")
	}
}

func TestAddIntentsListsOpenIntents(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "add-intents-open")

	first, err := s.AddIntents(ctx, expID, []tools.IntentInput{
		{Summary: "low", Priority: 2},
		{Summary: "held", Priority: 5},
		{Summary: "lapsed", Priority: 5},
		{Summary: "high", Priority: 9},
	})
	if err != nil {
		t.Fatalf("AddIntents: %v", err)
	}
	low, held, lapsed, high := first.IDs[0], first.IDs[1], first.IDs[2], first.IDs[3]
	if _, err := s.ClaimIntent(ctx, held, "sess-a", time.Hour); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	if _, err := s.ClaimIntent(ctx, lapsed, "sess-a", time.Hour); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	if _, err := s.ExecContext(ctx, `UPDATE exploration_nodes SET claim_expires_at = now() - interval '1 minute' WHERE id = $1`, lapsed); err != nil {
		t.Fatalf("expire claim: %v", err)
	}

	res, err := s.AddIntents(ctx, expID, []tools.IntentInput{{Summary: "new"}, {Summary: "Low."}})
	if err != nil {
		t.Fatalf("AddIntents: %v", err)
	}
	var got []int64
	for _, o := range res.Open {
		got = append(got, o.ID)
	}
	if fmt.Sprint(got) != fmt.Sprint([]int64{high, lapsed}) {
		t.Errorf("open = %v, want [%d %d] (high first, held and duplicate left out)", got, high, lapsed)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].ExistingID != low {
		t.Errorf("skipped = %+v, want match on %d", res.Skipped, low)
	}
}

func TestAddIntentsConcurrentPlannersDoNotDuplicate(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "add-intents-race")

	plans := [][]tools.IntentInput{
		{{Summary: "Enumerate SMB shares on DC01"}, {Summary: "Kerberoast service accounts"}, {Summary: "Test the web login for SQL injection"}, {Summary: "Check for AS-REP roastable users"}},
		{{Summary: "enumerate smb shares on dc01."}, {Summary: "Kerberoast service accounts!"}, {Summary: "Spray common passwords"}},
		{{Summary: "Test the web login for SQL-injection"}, {Summary: "Check for AS-REP roastable users"}, {Summary: "Spray common passwords"}},
	}
	unique := map[string]bool{}
	for _, p := range plans {
		for _, it := range p {
			unique[tools.IntentKey(it.Summary, nil)] = true
		}
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]*tools.IntentsResult, len(plans))
	errs := make([]error, len(plans))
	for i := range plans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = s.AddIntents(ctx, expID, plans[i])
		}()
	}
	close(start)
	wg.Wait()

	created, skipped := 0, 0
	for i, r := range results {
		if errs[i] != nil {
			t.Fatalf("AddIntents[%d]: %v", i, errs[i])
		}
		for _, id := range r.IDs {
			if id > 0 {
				created++
			}
		}
		skipped += len(r.Skipped)
	}
	if created != len(unique) || created+skipped != 10 {
		t.Errorf("created %d and skipped %d, want %d created of 10", created, skipped, len(unique))
	}
	keys := map[string]int64{}
	for id, summary := range intentSummaries(t, s, expID) {
		k := tools.IntentKey(summary, nil)
		if prev, dup := keys[k]; dup {
			t.Errorf("intents %d and %d duplicate %q", prev, id, summary)
		}
		keys[k] = id
	}
	if len(keys) != len(unique) {
		t.Errorf("got %d intents, want %d", len(keys), len(unique))
	}
}
