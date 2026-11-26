package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/report"
)

func RegisterReportTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeGenerateReport(store, taskID, explorationID))
}

func makeGenerateReport(store Store, taskID, explorationID int64) Tool {
	return Tool{
		Name:        "generate_report",
		Category:    CategoryReporting,
		Description: "Generate a Markdown penetration test report from the current engagement's findings, assets, and coverage, with a Target changes section listing every recorded change to a target and whether it was restored to its baseline.",
		ReadOnly:    true,
		Schema:      obj(map[string]any{}),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			_, goal, err := store.Root(ctx, explorationID)
			if err != nil {
				return errResult(err.Error()), nil
			}

			findings, err := store.ListFindingsForReport(ctx, explorationID)
			if err != nil {
				return errResult(err.Error()), nil
			}

			assets, _, err := store.ListAssets(ctx, AssetFilter{TaskID: taskID, Limit: 10000})
			if err != nil {
				return errResult(err.Error()), nil
			}
			assetCounts := map[string]int{}
			for _, a := range assets {
				assetCounts[a.Type]++
			}

			reportFindings := make([]report.ReportFinding, len(findings))
			for i, f := range findings {
				reportFindings[i] = report.ReportFinding{
					ID:        f.ID,
					VulnClass: f.VulnClass,
					Name:      f.Name,
					Severity:  f.Severity,
					Summary:   f.Summary,
					Evidence:  f.Evidence,
				}
			}

			changes, err := store.ListTargetChanges(ctx, explorationID, false)
			if err != nil {
				return errResult(err.Error()), nil
			}
			reportChanges := make([]report.TargetChange, len(changes))
			for i, c := range changes {
				rc := report.TargetChange{
					ID:              c.ID,
					Target:          c.Target,
					Resource:        c.Resource,
					Kind:            c.Kind,
					Baseline:        c.Baseline,
					RecordedBy:      c.RecordedBy,
					RestoredBy:      c.RestoredBy,
					LastObserved:    c.LastObserved,
					RestoreAttempts: c.RestoreAttempts,
				}
				if c.RestoredAt != nil {
					rc.Restored = true
					rc.RestoredAt = *c.RestoredAt
				}
				reportChanges[i] = rc
			}

			in := report.Input{
				Title:         goal,
				Goal:          goal,
				GeneratedAt:   time.Now(),
				AssetCounts:   assetCounts,
				Findings:      reportFindings,
				TargetChanges: reportChanges,
			}

			return textResult(report.Markdown(in)), nil
		},
	}
}
