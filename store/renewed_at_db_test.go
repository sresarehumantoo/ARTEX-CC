package store

import (
	"context"
	"testing"
	"time"
)

func renewedAt(t *testing.T, s *Store, id int64) (renewed, updated time.Time) {
	t.Helper()
	if err := s.QueryRowContext(context.Background(),
		`SELECT claim_renewed_at, updated_at FROM exploration_nodes WHERE id = $1`, id).Scan(&renewed, &updated); err != nil {
		t.Fatal(err)
	}
	return renewed, updated
}

func TestRenewIntentClaimsRefreshesRenewedAt(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "renewed-at")
	stale := addIntent(t, s, expID, "renewed long ago")
	fresh := addIntent(t, s, expID, "renewed just now")
	for _, id := range []int64{stale, fresh} {
		if _, err := s.ClaimIntent(ctx, id, "sess-a", time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	// Traffic kept stale's expiry far out, but its holder last renewed 40
	// minutes ago, so a write from the holder must move claim_renewed_at.
	if _, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET claim_renewed_at = now() - interval '40 minutes', claim_expires_at = now() + interval '50 minutes'
		 WHERE id = $1`, stale); err != nil {
		t.Fatal(err)
	}
	_, freshUpdated := renewedAt(t, s, fresh)

	if err := s.RenewIntentClaims(ctx, []int64{stale, fresh}, "sess-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	if renewed, _ := renewedAt(t, s, stale); time.Since(renewed) > time.Minute {
		t.Errorf("claim_renewed_at still %v ago after the holder's write", time.Since(renewed))
	}
	if _, updated := renewedAt(t, s, fresh); !updated.Equal(freshUpdated) {
		t.Error("a claim renewed moments ago with most of its lease left was rewritten")
	}
}
