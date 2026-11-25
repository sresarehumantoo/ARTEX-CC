package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestFindPotentialDuplicatesDB(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "dupes")
	otherTask, otherExp := testEngagement(t, s, "dupes-other")

	asset, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: fmt.Sprintf("dupes-%d.test", taskID)})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	add := func(task, exp int64, vc, sev string, assets []int64) int64 {
		t.Helper()
		id, _, err := s.AddFinding(ctx, tools.FindingInput{
			TaskID: task, ExplorationID: exp, VulnClass: vc, Severity: sev,
			Summary: vc + " finding", AssetIDs: assets,
		})
		if err != nil {
			t.Fatalf("AddFinding: %v", err)
		}
		return id
	}
	idor := add(taskID, expID, "Insecure Direct Object Reference", "medium", []int64{asset})
	dismissed := add(taskID, expID, "idor", "high", []int64{asset})
	if err := s.DismissFinding(ctx, dismissed, "false positive"); err != nil {
		t.Fatalf("DismissFinding: %v", err)
	}
	add(otherTask, otherExp, "IDOR", "high", []int64{asset})
	add(taskID, expID, "xss", "low", nil)

	got, err := s.FindPotentialDuplicates(ctx, expID, "IDOR", "high", []int64{asset})
	if err != nil {
		t.Fatalf("FindPotentialDuplicates: %v", err)
	}
	if len(got) != 1 || got[0].ID != idor {
		t.Fatalf("got %+v, want only finding %d", got, idor)
	}
}
