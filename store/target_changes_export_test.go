package store

import (
	"context"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestTargetChangesExportImport(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "target-change-export")

	intentID, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "x"}, 5, "open", "agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.RecordTargetChange(ctx, expID, tools.TargetChangeInput{
		Target: "WIN01", Resource: script, Kind: "modify", OriginalFingerprint: "sha256:orig",
		Backup: `C:\Temp\r.bak`, Note: "append", IntentID: intentID, RecordedBy: "agent-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	recordChange(t, s, expID, "WIN01", "sha256:payload", "agent-b")
	other := recordChange(t, s, expID, "WIN02", "absent", "agent-a")
	if _, err := s.RecordTargetRestore(ctx, expID, []int64{other.Change.ID}, "absent", "agent-a"); err != nil {
		t.Fatal(err)
	}

	export, err := s.ExportEngagementData(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(export.TargetChanges) != 3 {
		t.Fatalf("exported %d target changes, want 3", len(export.TargetChanges))
	}
	_, newExp, err := s.ImportEngagementData(ctx, export)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ListTargetChanges(ctx, newExp, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("imported %d target changes, want 3", len(got))
	}
	if got[0].BaselineChangeID != got[0].ID || got[1].BaselineChangeID != got[0].ID || got[2].BaselineChangeID != got[2].ID {
		t.Errorf("baseline ids not remapped: %d %d %d / %d %d %d",
			got[0].ID, got[1].ID, got[2].ID, got[0].BaselineChangeID, got[1].BaselineChangeID, got[2].BaselineChangeID)
	}
	if got[0].Backup != first.Change.Backup || got[0].Note != "append" || got[0].RecordedBy != "agent-a" || got[0].IntentID == 0 || got[0].IntentID == intentID {
		t.Errorf("first imported change = %+v", got[0])
	}
	if got[1].Baseline != "sha256:orig" || got[1].RestoredAt != nil || got[2].RestoredAt == nil {
		t.Errorf("imported states = %+v / %+v", got[1], got[2])
	}
	live, _ := s.ListTargetChanges(ctx, newExp, true)
	if len(live) != 2 {
		t.Errorf("imported live = %d, want 2", len(live))
	}
}
