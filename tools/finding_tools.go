package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

func RegisterFindingTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeReportFinding(store, taskID, explorationID))
	reg.Register(makeListFindings(store, explorationID))
}

func makeReportFinding(store Store, taskID, explorationID int64) Tool {
	return Tool{
		Name: "report_finding",
		Description: "Record a confirmed finding, with evidence providing verifiable proof such as command output and logs. " +
			"Pass the current intent_id when in a task context. " +
			"After reporting, the coordinator sees this in graph_overview and can use it as evidence for prove_goal.",
		Schema: obj(map[string]any{
			"vulnclass": str("vulnerability class"),
			"name":      str("vulnerability name"),
			"severity":  enm([]string{"critical", "high", "medium", "low"}, "severity level"),
			"summary":   str("finding summary"),
			"intent_id": intp("the current task's intent id"),
			"asset_ids": arr(map[string]any{"type": "integer"}, "affected asset ids"),
			"evidence":  str("evidence/PoC text"),
		}, "vulnclass", "severity", "summary"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				VulnClass string            `json:"vulnclass"`
				Name      string            `json:"name"`
				Severity  string            `json:"severity"`
				Summary   string            `json:"summary"`
				Evidence  string            `json:"evidence"`
				IntentID  json.RawMessage   `json:"intent_id"`
				AssetIDs  []json.RawMessage `json:"asset_ids"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult(err.Error()), nil
			}

			input := FindingInput{
				TaskID:        taskID,
				ExplorationID: explorationID,
				IntentID:      pid(a.IntentID),
				VulnClass:     a.VulnClass,
				Name:          a.Name,
				Severity:      a.Severity,
				Summary:       a.Summary,
				Evidence:      a.Evidence,
				AssetIDs:      pidList(a.AssetIDs),
			}

			findingID, err := store.AddFinding(ctx, input)
			if err != nil {
				return errResult(err.Error()), nil
			}

			return textResult(fmt.Sprintf("finding recorded: %d", findingID)), nil
		},
	}
}

func makeListFindings(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "list_findings",
		Description: "List the confirmed findings of this task (compact: id+vulnclass+severity+summary+state+status).",
		ReadOnly: true,
		Schema:   obj(map[string]any{}),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			findings, err := store.ListFindings(ctx, explorationID)
			if err != nil {
				return errResult(err.Error()), nil
			}
			out := make([]map[string]any, 0, len(findings))
			for _, f := range findings {
				m := map[string]any{
					"id":      f.ID,
					"summary": f.Summary,
				}
				if f.VulnClass != "" {
					m["vulnclass"] = f.VulnClass
				}
				if f.Severity != "" {
					m["severity"] = f.Severity
				}
				if f.State != "" {
					m["state"] = f.State
				}
				if f.Status != "" {
					m["status"] = f.Status
				}
				out = append(out, m)
			}
			return jsonResult(out)
		},
	}
}
