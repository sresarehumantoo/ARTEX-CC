package store

import (
	"context"
	"testing"
	"time"
)

func TestImportedIntentDoesNotCountTimeBeforeImport(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "unowned-import")
	open := addIntent(t, s, expID, "imported open")
	running := addIntent(t, s, expID, "imported running")
	// Import inserts intents as they were exported, with no claim.
	if _, err := s.ExecContext(ctx, `UPDATE exploration_nodes SET state = 'running' WHERE id = $1`, running); err != nil {
		t.Fatal(err)
	}
	ids := []int64{recordOn(t, s, expID, open, "a").ID, recordOn(t, s, expID, running, "b").ID}
	if _, err := s.ExecContext(ctx,
		`UPDATE target_changes SET created_at = now() - interval '2 hours' WHERE id = ANY($1)`, ids); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		c := changeByID(t, s, expID, id)
		if c.Unowned == "" || c.AsOf.IsZero() {
			t.Fatalf("change %d: unowned %q as of %v", id, c.Unowned, c.AsOf)
		}
		if d := c.UnownedFor(); d > time.Minute {
			t.Errorf("change %d (%s) unowned for %v, counting time before its intent existed", id, c.Unowned, d)
		}
	}
}
