package store

import (
	"context"
	"testing"
	"time"
)

func TestMigrationBackfillsClaimRenewedAt(t *testing.T) {
	s, migrate := migrationDB(t)
	ctx := context.Background()
	migrate("0010")
	_, expID := testEngagement(t, s, "renewed-at-backfill")
	var live, lapsed int64
	for _, p := range []struct {
		id      *int64
		expires string
	}{{&live, "20 minutes"}, {&lapsed, "-5 minutes"}} {
		if err := s.QueryRowContext(ctx,
			`INSERT INTO exploration_nodes (exploration_id, kind, state, owner, claim_expires_at)
			 VALUES ($1, 'intent', 'running', 'sess-old', now() + $2::interval) RETURNING id`,
			expID, p.expires).Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	migrate("9999")

	var renewed time.Time
	if err := s.QueryRowContext(ctx, `SELECT claim_renewed_at FROM exploration_nodes WHERE id = $1`, live).Scan(&renewed); err != nil {
		t.Fatalf("live claim not backfilled: %v", err)
	}
	if ago := time.Since(renewed); ago < 9*time.Minute || ago > 11*time.Minute {
		t.Errorf("claim_renewed_at %v ago, want about 10m (expiry minus the default lease)", ago)
	}
	if err := s.QueryRowContext(ctx, `SELECT claim_renewed_at FROM exploration_nodes WHERE id = $1`, lapsed).Scan(&renewed); err != nil {
		t.Fatalf("lapsed claim not backfilled: %v", err)
	}
}
