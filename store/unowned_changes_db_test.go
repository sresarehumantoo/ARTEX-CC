package store

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func addIntent(t *testing.T, s *Store, expID int64, summary string, anchors ...int64) int64 {
	t.Helper()
	id, err := s.AddNode(context.Background(), expID, KindIntent, map[string]any{"summary": summary}, 5, "open", "planner", anchors)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	return id
}

func recordOn(t *testing.T, s *Store, expID, intentID int64, resource string) tools.TargetChange {
	t.Helper()
	rec, err := s.RecordTargetChange(context.Background(), expID, tools.TargetChangeInput{
		Target: "WIN01", Resource: resource, Kind: "modify", OriginalFingerprint: "sha256:orig",
		Backup: "C:\\backup\\" + resource, IntentID: intentID, RecordedBy: "h-1",
	})
	if err != nil {
		t.Fatalf("RecordTargetChange: %v", err)
	}
	return rec.Change
}

// lapseClaim moves the intent's claim expiry to ago in the past, and its
// changes' recording an hour back so they predate the lapse.
func lapseClaim(t *testing.T, s *Store, intentID int64, ago time.Duration) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET claim_expires_at = now() - make_interval(secs => $2) WHERE id = $1`,
		intentID, ago.Seconds()); err != nil {
		t.Fatalf("lapse claim: %v", err)
	}
	if _, err := s.ExecContext(ctx,
		`UPDATE target_changes SET created_at = now() - interval '1 hour' WHERE intent_id = $1`, intentID); err != nil {
		t.Fatalf("backdate changes: %v", err)
	}
}

func changeByID(t *testing.T, s *Store, expID, id int64) tools.TargetChange {
	t.Helper()
	all, err := s.ListTargetChanges(context.Background(), expID, false)
	if err != nil {
		t.Fatalf("ListTargetChanges: %v", err)
	}
	for _, c := range all {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("change %d not listed", id)
	return tools.TargetChange{}
}

func TestTargetChangeOwnership(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "unowned-reasons")

	live := addIntent(t, s, expID, "live")
	lapsed := addIntent(t, s, expID, "lapsed")
	done := addIntent(t, s, expID, "done")
	open := addIntent(t, s, expID, "never started")
	for _, id := range []int64{live, lapsed, done} {
		if _, err := s.ClaimIntent(ctx, id, "sess-a", time.Hour); err != nil {
			t.Fatalf("ClaimIntent: %v", err)
		}
	}
	cases := map[int64]string{
		recordOn(t, s, expID, live, "a").ID:   "",
		recordOn(t, s, expID, lapsed, "b").ID: "claim_lapsed",
		recordOn(t, s, expID, done, "c").ID:   "intent_done",
		recordOn(t, s, expID, open, "d").ID:   "intent_open",
		recordOn(t, s, expID, 0, "e").ID:      "no_intent",
	}
	lapseClaim(t, s, lapsed, 5*time.Minute)
	if err := s.UpdateNodeFields(ctx, done, "done", nil); err != nil {
		t.Fatalf("UpdateNodeFields: %v", err)
	}

	for id, want := range cases {
		c := changeByID(t, s, expID, id)
		if c.Unowned != want {
			t.Errorf("change %d unowned = %q, want %q", id, c.Unowned, want)
		}
		if (c.UnownedSince != nil) != (want != "") {
			t.Errorf("change %d unowned_since = %v with reason %q", id, c.UnownedSince, want)
		}
		if want == "claim_lapsed" {
			if d := c.UnownedFor(); d < 4*time.Minute || d > 6*time.Minute {
				t.Errorf("lapsed change unowned for %v, want about 5m", d)
			}
		}
	}
}

func TestTakeoverClosesUnownedWindow(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "unowned-takeover")
	intent := addIntent(t, s, expID, "payload")
	if _, err := s.ClaimIntent(ctx, intent, "sess-dead", time.Hour); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	change := recordOn(t, s, expID, intent, "task.ps1")
	lapseClaim(t, s, intent, 7*time.Minute)
	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatal(err)
	}
	if w := ov.UnownedTargetChanges; w == nil || w.Count != 1 || w.ChangeIDs[0] != change.ID {
		t.Errorf("graph_overview warning = %+v, want change %d", w, change.ID)
	}

	claim, err := s.ClaimIntent(ctx, intent, "sess-live", time.Hour)
	if err != nil {
		t.Fatalf("takeover: %v", err)
	}
	if !claim.TookOver || claim.PreviousOwner != "sess-dead" || claim.Renewed {
		t.Errorf("claim = %+v, want a takeover from sess-dead", claim)
	}
	c := changeByID(t, s, expID, change.ID)
	if c.Unowned != "" {
		t.Errorf("change still unowned (%s) after takeover", c.Unowned)
	}
	if c.MaxUnownedSeconds < 6*60 || c.MaxUnownedSeconds > 8*60 {
		t.Errorf("max_unowned_seconds = %d, want about 420", c.MaxUnownedSeconds)
	}
	if ov, err := s.GraphOverview(ctx, expID); err != nil || ov.UnownedTargetChanges != nil {
		t.Errorf("graph_overview still warns after the takeover: %+v, %v", ov.UnownedTargetChanges, err)
	}

	again, err := s.ClaimIntent(ctx, intent, "sess-live", time.Hour)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if again.TookOver || !again.Renewed {
		t.Errorf("renewal = %+v, want renewed and not a takeover", again)
	}
	if after := changeByID(t, s, expID, change.ID); after.MaxUnownedSeconds != c.MaxUnownedSeconds || after.Unowned != "" {
		t.Errorf("renewal changed the window: %+v", after)
	}
}

func TestRenewAfterLapseRecordsWindow(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "unowned-renew")
	intent := addIntent(t, s, expID, "slow shell work")
	if _, err := s.ClaimIntent(ctx, intent, "sess-a", time.Hour); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	change := recordOn(t, s, expID, intent, "svc")
	lapseClaim(t, s, intent, 3*time.Minute)

	if err := s.RenewIntentClaims(ctx, []int64{intent}, "sess-b", time.Hour); err != nil {
		t.Fatalf("RenewIntentClaims: %v", err)
	}
	if c := changeByID(t, s, expID, change.ID); c.Unowned != "claim_lapsed" || c.MaxUnownedSeconds != 0 {
		t.Errorf("another holder's renewal touched the change: %+v", c)
	}
	if err := s.RenewIntentClaims(ctx, []int64{intent}, "sess-a", time.Hour); err != nil {
		t.Fatalf("RenewIntentClaims: %v", err)
	}
	c := changeByID(t, s, expID, change.ID)
	if c.Unowned != "" || c.MaxUnownedSeconds < 150 || c.MaxUnownedSeconds > 210 {
		t.Errorf("after the holder renewed: unowned %q, max %ds, want owned and about 180s", c.Unowned, c.MaxUnownedSeconds)
	}
}

func TestRestoreClosesUnownedWindow(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "unowned-restore")
	change := recordOn(t, s, expID, 0, "orphan")
	if _, err := s.ExecContext(ctx,
		`UPDATE target_changes SET created_at = now() - interval '12 minutes' WHERE id = $1`, change.ID); err != nil {
		t.Fatal(err)
	}

	out, err := s.RecordTargetRestore(ctx, expID, []int64{change.ID}, "sha256:wrong", "h-2")
	if err != nil {
		t.Fatalf("RecordTargetRestore: %v", err)
	}
	if o := out[0]; o.BaselineBackup != change.Backup || o.RecordedBy != "h-1" || o.RestoreAttempts != 1 {
		t.Errorf("failed restore outcome lacks the retry context: %+v", o)
	}
	out, err = s.RecordTargetRestore(ctx, expID, []int64{change.ID}, "sha256:wrong2", "h-2")
	if err != nil {
		t.Fatalf("RecordTargetRestore: %v", err)
	}
	if o := out[0]; o.PreviousObserved != "sha256:wrong" || o.RestoreAttempts != 2 {
		t.Errorf("second failed restore = %+v, want the first attempt's fingerprint", o)
	}

	if _, err := s.RecordTargetRestore(ctx, expID, []int64{change.ID}, "sha256:orig", "h-2"); err != nil {
		t.Fatalf("RecordTargetRestore: %v", err)
	}
	c := changeByID(t, s, expID, change.ID)
	if c.RestoredAt == nil || c.Unowned != "" {
		t.Fatalf("change not restored: %+v", c)
	}
	if c.MaxUnownedSeconds < 11*60 || c.MaxUnownedSeconds > 13*60 {
		t.Errorf("max_unowned_seconds = %d, want about 720", c.MaxUnownedSeconds)
	}
}

func TestAdoptTargetChanges(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "unowned-adopt")
	mine := addIntent(t, s, expID, "cleanup")
	other := addIntent(t, s, expID, "busy")
	closed := addIntent(t, s, expID, "finished")
	for id, owner := range map[int64]string{mine: "sess-a", other: "sess-b", closed: "sess-c"} {
		if _, err := s.ClaimIntent(ctx, id, owner, time.Hour); err != nil {
			t.Fatalf("ClaimIntent: %v", err)
		}
	}
	owned := recordOn(t, s, expID, other, "owned")
	orphan := recordOn(t, s, expID, closed, "orphan")
	if err := s.UpdateNodeFields(ctx, closed, "done", nil); err != nil {
		t.Fatal(err)
	}

	if _, err := s.AdoptTargetChanges(ctx, expID, mine, "sess-b", []int64{orphan.ID}); err == nil {
		t.Error("adopting under someone else's claim succeeded")
	}
	res, err := s.AdoptTargetChanges(ctx, expID, mine, "sess-a", []int64{owned.ID, orphan.ID, 999999})
	if err != nil {
		t.Fatalf("AdoptTargetChanges: %v", err)
	}
	if len(res.Adopted) != 1 || res.Adopted[0].ID != orphan.ID || res.Adopted[0].IntentID != mine || res.Adopted[0].Unowned != "" {
		t.Errorf("adopted = %+v, want only the orphan, now owned by intent %d", res.Adopted, mine)
	}
	if _, ok := res.Refused[owned.ID]; !ok {
		t.Errorf("a change with a live owner was not refused: %v", res.Refused)
	}
	if _, ok := res.Refused[999999]; !ok {
		t.Errorf("an unknown change was not refused: %v", res.Refused)
	}
	if c := changeByID(t, s, expID, owned.ID); c.IntentID != other {
		t.Errorf("refused change moved to intent %d", c.IntentID)
	}
}
func insertAsset(t *testing.T, s *Store, column, value string) int64 {
	t.Helper()
	typ := map[string]string{"domain": "subdomain", "ip": "ip", "url": "endpoint"}[column]
	var id int64
	if err := s.QueryRowContext(context.Background(),
		`INSERT INTO assets (type, `+column+`) VALUES ($1, $2) RETURNING id`, typ, value).Scan(&id); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	return id
}

func claimUntil(t *testing.T, s *Store, id int64) time.Time {
	t.Helper()
	_, exp := claimExpiry(t, s, id)
	return exp.Time
}

func TestRenewIntentClaimsForHost(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "traffic-renew")
	const lease = 30 * time.Minute
	// Assets are global, so each run uses its own hosts.
	run := fmt.Sprintf("r%d", time.Now().UnixNano())
	webHost, sharedHost := "app."+run+".example.test", "shared."+run+".example.test"
	ipHost := fmt.Sprintf("10.%d.%d.%d", time.Now().UnixNano()%250, time.Now().UnixNano()/7%250, time.Now().UnixNano()/13%250)
	web := insertAsset(t, s, "url", "https://user@"+strings.ToUpper(webHost)+":8443/login")
	shared := insertAsset(t, s, "domain", sharedHost)
	ip := insertAsset(t, s, "ip", ipHost)

	solo := addIntent(t, s, expID, "web app", web)
	viaIP := addIntent(t, s, expID, "by ip", ip)
	first := addIntent(t, s, expID, "shared a", shared)
	second := addIntent(t, s, expID, "shared b", shared)
	for id, owner := range map[int64]string{solo: "sess-a", viaIP: "sess-a", first: "sess-a", second: "sess-b"} {
		if _, err := s.ClaimIntent(ctx, id, owner, lease); err != nil {
			t.Fatalf("ClaimIntent: %v", err)
		}
	}
	for _, id := range []int64{solo, viaIP, first, second} {
		lapseClaim(t, s, id, -5*time.Minute)
	}

	ids, err := s.RenewIntentClaimsForHost(ctx, expID, webHost, lease)
	if err != nil {
		t.Fatalf("RenewIntentClaimsForHost: %v", err)
	}
	if !slices.Equal(ids, []int64{solo}) || time.Until(claimUntil(t, s, solo)) < 25*time.Minute {
		t.Errorf("renewed %v until %v, want intent %d about a lease out", ids, claimUntil(t, s, solo), solo)
	}
	if ids, _ := s.RenewIntentClaimsForHost(ctx, expID, ipHost, lease); !slices.Equal(ids, []int64{viaIP}) {
		t.Errorf("ip host renewed %v, want %d", ids, viaIP)
	}
	if ids, _ := s.RenewIntentClaimsForHost(ctx, expID, sharedHost, lease); len(ids) != 0 {
		t.Errorf("a host two holders work on renewed %v", ids)
	}

	lapseClaim(t, s, solo, time.Minute)
	if ids, _ := s.RenewIntentClaimsForHost(ctx, expID, webHost, lease); len(ids) != 0 {
		t.Errorf("traffic revived a lapsed claim: %v", ids)
	}

	if _, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET claim_renewed_at = now() - interval '55 minutes', claim_expires_at = now() + interval '2 minutes'
		 WHERE id = $1`, solo); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RenewIntentClaimsForHost(ctx, expID, webHost, lease); err != nil {
		t.Fatal(err)
	}
	if until := time.Until(claimUntil(t, s, solo)); until > 6*time.Minute {
		t.Errorf("traffic extended a claim %v out, past two leases after the holder's last renewal", until)
	}

	// A claim from before migration 0010 has no renewal time to cap against.
	if _, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET claim_renewed_at = NULL, claim_expires_at = now() + interval '2 minutes' WHERE id = $1`, solo); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if ids, err := s.RenewIntentClaimsForHost(ctx, expID, webHost+".", lease); err != nil || len(ids) != 0 {
			t.Fatalf("a claim without claim_renewed_at was renewed: %v, %v", ids, err)
		}
	}
	if until := time.Until(claimUntil(t, s, solo)); until > 3*time.Minute {
		t.Errorf("claim without claim_renewed_at extended to %v out", until)
	}

	if _, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET claim_renewed_at = now(), claim_expires_at = now() + interval '2 minutes' WHERE id = $1`, solo); err != nil {
		t.Fatal(err)
	}
	if ids, _ := s.RenewIntentClaimsForHost(ctx, expID, strings.ToUpper(webHost)+".", lease); !slices.Equal(ids, []int64{solo}) {
		t.Errorf("host with a trailing dot renewed %v, want %d", ids, solo)
	}
}

func TestReleaseHolderClaims(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "release-holder")
	a := addIntent(t, s, expID, "a")
	b := addIntent(t, s, expID, "b")
	c := addIntent(t, s, expID, "c")
	for id, owner := range map[int64]string{a: "id:chat/1", b: "id:chat/1/owner:x", c: "id:chat/10"} {
		if _, err := s.ClaimIntent(ctx, id, owner, time.Hour); err != nil {
			t.Fatalf("ClaimIntent: %v", err)
		}
	}
	n, err := s.ReleaseHolderClaims(ctx, "id:chat/1")
	if err != nil || n != 2 {
		t.Fatalf("ReleaseHolderClaims = %d, %v, want 2", n, err)
	}
	for _, id := range []int64{a, b} {
		if owner, exp := claimExpiry(t, s, id); owner == "" || exp.Time.After(time.Now()) {
			t.Errorf("intent %d still held (%s until %v)", id, owner, exp.Time)
		}
	}
	if time.Until(claimUntil(t, s, c)) < 50*time.Minute {
		t.Error("another identity's claim was released")
	}
	claim, err := s.ClaimIntent(ctx, a, "id:chat/2", time.Hour)
	if err != nil || !claim.TookOver {
		t.Errorf("released intent not taken over at once: %+v, %v", claim, err)
	}
}
