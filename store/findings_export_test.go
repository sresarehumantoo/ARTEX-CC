package store

import (
	"context"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func importedFindings(t *testing.T, s *Store, taskID int64) map[string]*Finding {
	t.Helper()
	rows, err := s.QueryContext(context.Background(), `SELECT id FROM findings WHERE task_id = $1`, taskID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	out := map[string]*Finding{}
	for _, id := range ids {
		f, err := s.GetFinding(id)
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = f
	}
	return out
}

func TestFindingMergeStateExportImport(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "finding-merge-export")

	add := func(name string) int64 {
		id, _, err := s.AddFinding(ctx, tools.FindingInput{
			TaskID: taskID, ExplorationID: expID,
			VulnClass: "xss", Name: name, Severity: SeverityHigh, Summary: name, Evidence: name + " evidence",
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	a, b, c := add("A"), add("B"), add("C")
	if _, err := s.MergeFindings(ctx, a, []int64{b}); err != nil {
		t.Fatal(err)
	}
	if err := s.DismissFinding(ctx, c, "false positive"); err != nil {
		t.Fatal(err)
	}

	export, err := s.ExportEngagementData(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	newTaskID, _, err := s.ImportEngagementData(ctx, export)
	if err != nil {
		t.Fatal(err)
	}

	got := importedFindings(t, s, newTaskID)
	newA, newB, newC := got["A"], got["B"], got["C"]
	if newA == nil || newB == nil || newC == nil {
		t.Fatalf("imported findings = %v", got)
	}
	if newA.ID == a {
		t.Fatalf("imported A kept old id %d", a)
	}
	if newB.MergedInto == nil || *newB.MergedInto != newA.ID {
		t.Errorf("imported B merged_into = %v, want %d", newB.MergedInto, newA.ID)
	}
	if newB.Status != FindingStatusDuplicate {
		t.Errorf("imported B status = %q, want %q", newB.Status, FindingStatusDuplicate)
	}
	if newA.MergedInto != nil || newC.MergedInto != nil {
		t.Errorf("imported A, C merged_into = %v, %v, want nil", newA.MergedInto, newC.MergedInto)
	}
	if !newC.Dismissed || newC.DismissReason != "false positive" || newC.Status != FindingStatusFalsePositive {
		t.Errorf("imported C = dismissed %v reason %q status %q", newC.Dismissed, newC.DismissReason, newC.Status)
	}

	merged, err := s.MergeFindings(ctx, newB.ID, []int64{newC.ID})
	if err != nil {
		t.Fatal(err)
	}
	if merged.ID != newA.ID || merged.RequestedPrimaryID != newB.ID {
		t.Errorf("merge into imported B resolved to %d (requested %d), want %d", merged.ID, merged.RequestedPrimaryID, newA.ID)
	}
	if len(merged.Skipped) != 1 || merged.Skipped[0].ID != newC.ID || merged.Skipped[0].Reason != "dismissed" {
		t.Errorf("skipped = %+v, want imported C as dismissed", merged.Skipped)
	}
}

func TestImportFindingsWithoutMergedInto(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()

	taskID, _, err := s.ImportEngagementData(ctx, &tools.EngagementExport{
		Version: 1,
		Goal:    "old export",
		Findings: []map[string]any{
			{"id": float64(1), "vulnclass": "xss", "name": "A", "severity": "high", "summary": "a", "status": "pending"},
			{"id": float64(2), "vulnclass": "xss", "name": "B", "severity": "high", "summary": "b", "status": "duplicate"},
			{"vulnclass": "sqli", "name": "C", "severity": "low", "summary": "c", "status": "pending"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := importedFindings(t, s, taskID)
	if len(got) != 3 {
		t.Fatalf("imported findings = %v, want 3", got)
	}
	for name, f := range got {
		if f.MergedInto != nil || f.Dismissed {
			t.Errorf("%s: merged_into %v dismissed %v, want live", name, f.MergedInto, f.Dismissed)
		}
	}
	if got["B"].Status != "duplicate" {
		t.Errorf("B status = %q, want duplicate", got["B"].Status)
	}
}
