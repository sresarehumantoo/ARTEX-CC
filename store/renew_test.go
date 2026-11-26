package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func claimExpiry(t *testing.T, s *Store, id int64) (string, sql.NullTime) {
	t.Helper()
	var owner sql.NullString
	var expires sql.NullTime
	if err := s.QueryRowContext(context.Background(),
		`SELECT owner, claim_expires_at FROM exploration_nodes WHERE id = $1`, id).Scan(&owner, &expires); err != nil {
		t.Fatalf("read claim: %v", err)
	}
	return owner.String, expires
}

func TestRenewIntentClaimsOnlyForHolder(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	expID, own := claimIntentNode(t, s, "renew-holder")
	sub, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "sub"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	other, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "other"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	for id, owner := range map[int64]string{own: "sess-a", sub: "sess-a/owner:x", other: "sess-ab"} {
		if _, err := s.ClaimIntent(ctx, id, owner, time.Minute); err != nil {
			t.Fatalf("ClaimIntent(%d, %s): %v", id, owner, err)
		}
	}
	_, ownBefore := claimExpiry(t, s, own)
	_, otherBefore := claimExpiry(t, s, other)

	if err := s.RenewIntentClaims(ctx, []int64{own, sub, other}, "sess-a", time.Hour); err != nil {
		t.Fatalf("RenewIntentClaims: %v", err)
	}
	for _, id := range []int64{own, sub} {
		if _, exp := claimExpiry(t, s, id); !exp.Time.After(ownBefore.Time.Add(50 * time.Minute)) {
			t.Errorf("intent %d expires %v, want about an hour out", id, exp.Time)
		}
	}
	if owner, exp := claimExpiry(t, s, other); owner != "sess-ab" || !exp.Time.Equal(otherBefore.Time) {
		t.Errorf("another holder's claim changed: %s until %v", owner, exp.Time)
	}

	sibling, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "sibling"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if _, err := s.ClaimIntent(ctx, sibling, "sess-a/owner:y", time.Minute); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	_, siblingBefore := claimExpiry(t, s, sibling)
	if err := s.RenewIntentClaims(ctx, []int64{sibling}, "sess-a/owner:x", time.Hour); err != nil {
		t.Fatalf("RenewIntentClaims: %v", err)
	}
	if _, exp := claimExpiry(t, s, sibling); !exp.Time.Equal(siblingBefore.Time) {
		t.Errorf("owner x renewed sibling owner y's claim: %v -> %v", siblingBefore.Time, exp.Time)
	}

	_, renewed := claimExpiry(t, s, own)
	if err := s.RenewIntentClaims(ctx, []int64{own}, "sess-a", time.Hour); err != nil {
		t.Fatalf("RenewIntentClaims: %v", err)
	}
	if _, exp := claimExpiry(t, s, own); !exp.Time.Equal(renewed.Time) {
		t.Errorf("claim with most of its lease left was rewritten: %v -> %v", renewed.Time, exp.Time)
	}
}

func TestRenewIntentClaimsLapsedAndTakenOver(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	expID, lapsed := claimIntentNode(t, s, "renew-lapsed")
	taken, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "taken"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	done, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "done"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	for _, id := range []int64{lapsed, taken, done} {
		if _, err := s.ClaimIntent(ctx, id, "sess-a", time.Minute); err != nil {
			t.Fatalf("ClaimIntent: %v", err)
		}
	}
	if _, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET claim_expires_at = now() - interval '1 minute' WHERE id IN ($1, $2)`, lapsed, taken); err != nil {
		t.Fatalf("expire claims: %v", err)
	}
	if _, err := s.ClaimIntent(ctx, taken, "sess-b", time.Minute); err != nil {
		t.Fatalf("takeover: %v", err)
	}
	if err := s.UpdateNodeFields(ctx, done, "done", nil); err != nil {
		t.Fatalf("UpdateNodeFields: %v", err)
	}
	_, takenBefore := claimExpiry(t, s, taken)

	if err := s.RenewIntentClaims(ctx, []int64{lapsed, taken, done}, "sess-a", time.Hour); err != nil {
		t.Fatalf("RenewIntentClaims: %v", err)
	}
	if owner, exp := claimExpiry(t, s, lapsed); owner != "sess-a" || !exp.Time.After(time.Now().Add(50*time.Minute)) {
		t.Errorf("lapsed claim not revived: %s until %v", owner, exp.Time)
	}
	if owner, exp := claimExpiry(t, s, taken); owner != "sess-b" || !exp.Time.Equal(takenBefore.Time) {
		t.Errorf("taken-over claim changed: %s until %v", owner, exp.Time)
	}
	if n, err := s.NodeDetail(ctx, done); err != nil || n.State != "done" {
		t.Errorf("done intent = %+v, %v", n, err)
	}
	if _, exp := claimExpiry(t, s, done); exp.Valid {
		t.Errorf("done intent got a lease: %v", exp.Time)
	}
}
