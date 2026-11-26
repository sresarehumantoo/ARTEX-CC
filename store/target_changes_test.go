package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

const script = `C:\ProgramData\WinforgeMaint\rotate-logs.ps1`

func recordChange(t *testing.T, s *Store, expID int64, target, fp, by string) *tools.TargetChangeRecord {
	t.Helper()
	rec, err := s.RecordTargetChange(context.Background(), expID, tools.TargetChangeInput{
		Target: target, Resource: script, Kind: "modify", OriginalFingerprint: fp, RecordedBy: by,
	})
	if err != nil {
		t.Fatalf("RecordTargetChange: %v", err)
	}
	return rec
}

func TestTargetChangeFirstBaselineWinsConcurrently(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	_, expID := testEngagement(t, s, "target-change-race")

	const n = 8
	recs := make([]*tools.TargetChangeRecord, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			recs[i], errs[i] = s.RecordTargetChange(context.Background(), expID, tools.TargetChangeInput{
				Target: "WIN01", Resource: script, Kind: "modify",
				OriginalFingerprint: fmt.Sprintf("sha256:%02d", i), RecordedBy: fmt.Sprintf("agent-%d", i),
			})
		}()
	}
	close(start)
	wg.Wait()

	var first *tools.TargetChangeRecord
	for i, err := range errs {
		if err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		if len(recs[i].Existing) == 0 {
			if first != nil {
				t.Fatalf("changes %d and %d both saw no live change", first.Change.ID, recs[i].Change.ID)
			}
			first = recs[i]
		}
	}
	if first == nil {
		t.Fatal("no change became the baseline")
	}
	if first.Change.Baseline != first.Change.OriginalFingerprint || first.Change.BaselineChangeID != first.Change.ID {
		t.Errorf("first change = %+v", first.Change)
	}
	for _, r := range recs {
		if r.Change.Baseline != first.Change.Baseline || r.Change.BaselineChangeID != first.Change.ID {
			t.Errorf("change %d baseline %q (#%d), want %q (#%d)", r.Change.ID, r.Change.Baseline, r.Change.BaselineChangeID, first.Change.Baseline, first.Change.ID)
		}
	}
}

func TestTargetChangeExistingAndMatching(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "target-change-existing")
	_, otherExp := testEngagement(t, s, "target-change-other")

	a := recordChange(t, s, expID, "WIN01", "sha256:orig", "agent-a")
	b := recordChange(t, s, expID, " win01 ", "sha256:payload", "agent-b")
	if len(b.Existing) != 1 || b.Existing[0].ID != a.Change.ID || b.Existing[0].RecordedBy != "agent-a" {
		t.Fatalf("existing = %+v", b.Existing)
	}
	if b.Change.Baseline != "sha256:orig" || b.Change.OriginalFingerprint != "sha256:payload" {
		t.Errorf("second change = %+v", b.Change)
	}
	if r := recordChange(t, s, expID, "WIN02", "sha256:x", "agent-a"); len(r.Existing) != 0 {
		t.Errorf("different target sees %+v", r.Existing)
	}
	if r := recordChange(t, s, otherExp, "WIN01", "sha256:x", "agent-a"); len(r.Existing) != 0 {
		t.Errorf("different engagement sees %+v", r.Existing)
	}

	if _, err := s.RecordTargetChange(ctx, expID, tools.TargetChangeInput{
		Target: "WIN01", Resource: "x", Kind: "modify", OriginalFingerprint: "y", IntentID: a.Change.ID + 1_000_000,
	}); err == nil || !strings.Contains(err.Error(), "not an intent") {
		t.Errorf("bad intent_id: %v", err)
	}
	intentID, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "abuse task"}, 5, "open", "agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	withIntent, err := s.RecordTargetChange(ctx, expID, tools.TargetChangeInput{
		Target: "WIN03", Resource: "svc", Kind: "other", OriginalFingerprint: "StartName=LocalSystem", IntentID: intentID,
	})
	if err != nil || withIntent.Change.IntentID != intentID {
		t.Fatalf("record with intent: %+v, %v", withIntent, err)
	}
}

func TestTargetRestoreRequiresBaselineMatch(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "target-restore")

	a := recordChange(t, s, expID, "WIN01", "sha256:ORIG", "agent-a")
	b := recordChange(t, s, expID, "WIN01", "sha256:payload", "agent-b")

	out, err := s.RecordTargetRestore(ctx, expID, []int64{b.Change.ID}, "sha256:payload", "agent-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Status != tools.RestoreStatusNotRestored ||
		!strings.Contains(out[0].Message, "observed sha256:payload, baseline sha256:ORIG") {
		t.Fatalf("mismatch outcome = %+v", out)
	}
	live, err := s.ListTargetChanges(ctx, expID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 2 {
		t.Fatalf("live after mismatch = %d, want 2", len(live))
	}
	for _, c := range live {
		if c.ID == b.Change.ID && (c.RestoreAttempts != 1 || c.LastObserved != "sha256:payload") {
			t.Errorf("attempt not recorded: %+v", c)
		}
	}

	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.UnrestoredTargetChanges) != 2 {
		t.Errorf("overview unrestored = %v", ov.UnrestoredTargetChanges)
	}

	out, err = s.RecordTargetRestore(ctx, expID, []int64{b.Change.ID, a.Change.ID}, " sha256:orig ", "agent-c")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Status != tools.RestoreStatusRestored || out[1].Status != tools.RestoreStatusRestored {
		t.Fatalf("match outcomes = %+v", out)
	}
	if len(out[0].AlsoRestored) != 1 || out[0].AlsoRestored[0] != a.Change.ID {
		t.Errorf("also_restored = %v", out[0].AlsoRestored)
	}
	if live, _ := s.ListTargetChanges(ctx, expID, true); len(live) != 0 {
		t.Errorf("live after restore = %+v", live)
	}
	all, _ := s.ListTargetChanges(ctx, expID, false)
	for _, c := range all {
		if c.RestoredAt == nil || c.RestoredBy != "agent-c" {
			t.Errorf("change not restored: %+v", c)
		}
	}
	if ov, _ := s.GraphOverview(ctx, expID); len(ov.UnrestoredTargetChanges) != 0 {
		t.Errorf("overview still lists %v", ov.UnrestoredTargetChanges)
	}

	out, err = s.RecordTargetRestore(ctx, expID, []int64{a.Change.ID}, "sha256:other", "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != tools.RestoreStatusAlreadyRestored || !strings.Contains(out[0].Message, "changed again") {
		t.Errorf("already restored outcome = %+v", out[0])
	}

	c := recordChange(t, s, expID, "WIN01", "sha256:orig", "agent-a")
	if len(c.Existing) != 0 || c.Change.BaselineChangeID != c.Change.ID {
		t.Errorf("change after full restore = %+v", c)
	}

	if _, err := s.RecordTargetRestore(ctx, expID, []int64{a.Change.ID + 1_000_000}, "x", "y"); err == nil {
		t.Error("unknown change id accepted")
	}
}
