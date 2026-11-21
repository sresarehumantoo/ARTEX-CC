package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/report"
)

func RegisterReportTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeGenerateReport(store, taskID, explorationID))
}

func makeGenerateReport(store Store, taskID, explorationID int64) Tool {
	return Tool{
		Name:        "generate_report",
		Description: "Generate a Markdown penetration test report from the current engagement's findings, assets, and coverage.",
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

			in := report.Input{
				Title:       goal,
				Goal:        goal,
				GeneratedAt: time.Now(),
				AssetCounts: assetCounts,
				Findings:    reportFindings,
			}

			return textResult(report.Markdown(in)), nil
		},
	}
}
