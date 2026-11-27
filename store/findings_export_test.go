package store

import (
	"context"
	"fmt"
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

func TestImportRemapsFindingNodePayload(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "finding-node-payload-export")

	oldID, _, err := s.AddFinding(ctx, tools.FindingInput{
		TaskID: taskID, ExplorationID: expID,
		VulnClass: "xss", Name: "A", Severity: SeverityHigh, Summary: "a", Evidence: "a evidence",
	})
	if err != nil {
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

	newA := importedFindings(t, s, newTaskID)["A"]
	if newA == nil || newA.NodeID == nil {
		t.Fatalf("imported finding A = %+v, want one with a node", newA)
	}
	if newA.ID == oldID {
		t.Fatalf("imported A kept old id %d", oldID)
	}
	var payloadID int64
	if err := s.QueryRowContext(ctx,
		`SELECT (payload->>'finding_id')::bigint FROM exploration_nodes WHERE id = $1`, *newA.NodeID,
	).Scan(&payloadID); err != nil {
		t.Fatal(err)
	}
	if payloadID != newA.ID {
		t.Errorf("imported node payload finding_id = %d, want %d (old %d)", payloadID, newA.ID, oldID)
	}
}

func TestImportRemapsIntentAssetIDs(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "intent-asset-ids-export")

	oldAssetID, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: fmt.Sprintf("intent-assets-%d.test", taskID)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIntents(ctx, expID, []tools.IntentInput{{Summary: "probe login", Priority: 5, AssetIDs: []int64{oldAssetID}}}); err != nil {
		t.Fatal(err)
	}

	export, err := s.ExportEngagementData(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	// Assets dedupe across tasks in one database, so shift the exported ids to
	// stand in for a different exporting database.
	const foreignAssetID, unexportedAssetID = 1 << 40, 1<<40 + 1
	if len(export.Assets) != 1 {
		t.Fatalf("exported assets = %d, want 1", len(export.Assets))
	}
	export.Assets[0]["id"] = float64(foreignAssetID)
	for _, n := range export.Nodes {
		if p, _ := n["payload"].(map[string]any); p["summary"] == "probe login" {
			p["asset_ids"] = []any{float64(foreignAssetID), float64(unexportedAssetID)}
			n["anchors"] = []any{float64(foreignAssetID)}
		}
	}
	newTaskID, newExpID, err := s.ImportEngagementData(ctx, export)
	if err != nil {
		t.Fatal(err)
	}

	var newAssetID int64
	if err := s.QueryRowContext(ctx, `SELECT id FROM assets WHERE $1 = ANY(task_ids)`, newTaskID).Scan(&newAssetID); err != nil {
		t.Fatal(err)
	}
	var payloadIDs string
	if err := s.QueryRowContext(ctx,
		`SELECT payload->>'asset_ids' FROM exploration_nodes WHERE exploration_id = $1 AND kind = $2`, newExpID, KindIntent,
	).Scan(&payloadIDs); err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("[%d]", newAssetID); payloadIDs != want {
		t.Errorf("imported intent payload asset_ids = %s, want %s (exported %d)", payloadIDs, want, foreignAssetID)
	}
}
