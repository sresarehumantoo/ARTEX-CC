package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func RegisterFindingTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeReportFinding(store, taskID, explorationID))
	reg.Register(makeListFindings(store, explorationID))
	reg.Register(makeBindFindingTraffic(store))
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
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.VulnClass) == "" {
				return errResult("vulnclass is required"), nil
			}
			if err := validateTextLen("vulnclass", a.VulnClass); err != nil {
				return errResult(err.Error()), nil
			}
			sev := strings.ToLower(strings.TrimSpace(a.Severity))
			if !validSeverities[sev] {
				return errResult(fmt.Sprintf("severity %q is not valid (use critical/high/medium/low)", a.Severity)), nil
			}
			a.Severity = sev
			if strings.TrimSpace(a.Summary) == "" {
				return errResult("summary is required"), nil
			}
			if err := validateTextLen("summary", a.Summary); err != nil {
				return errResult(err.Error()), nil
			}
			if err := validateTextLen("evidence", a.Evidence); err != nil {
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
		Description: "List the confirmed findings of this task (compact: id+vulnclass+severity+summary+state+status+traffic_count).",
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
				if tc, err := store.FindingTrafficCount(ctx, f.ID); err == nil && tc > 0 {
					m["traffic_count"] = tc
				}
				out = append(out, m)
			}
			return jsonResult(out)
		},
	}
}

func makeBindFindingTraffic(store Store) Tool {
	return Tool{
		Name: "bind_finding_traffic",
		Description: "Link a traffic exchange (snapshot) to a finding as evidence. " +
			"Use traffic_search to find the snapshot_id, then bind it here with a role describing how it supports the finding.",
		Schema: obj(map[string]any{
			"finding_id":  intp("finding id"),
			"snapshot_id": str("traffic snapshot id from traffic_search/traffic_get"),
			"role":        enm([]string{"baseline", "proof", "verification", "supporting"}, "how this traffic supports the finding (default: supporting)"),
			"note":        str("optional note about this traffic evidence"),
		}, "finding_id", "snapshot_id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				FindingID  json.RawMessage `json:"finding_id"`
				SnapshotID string          `json:"snapshot_id"`
				Role       string          `json:"role"`
				Note       string          `json:"note"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			findingID := pid(a.FindingID)
			if findingID <= 0 {
				return errResult("finding_id is required"), nil
			}
			snapshotID := strings.TrimSpace(a.SnapshotID)
			if snapshotID == "" {
				return errResult("snapshot_id is required"), nil
			}
			role := strings.ToLower(strings.TrimSpace(a.Role))
			if role == "" {
				role = "supporting"
			}
			validRoles := map[string]bool{"baseline": true, "proof": true, "verification": true, "supporting": true}
			if !validRoles[role] {
				return errResult(fmt.Sprintf("role %q is not valid (use baseline/proof/verification/supporting)", a.Role)), nil
			}

			existing, err := store.ListFindingTraffic(ctx, findingID)
			if err != nil {
				return errResult(err.Error()), nil
			}
			position := len(existing)

			if err := store.BindFindingTraffic(ctx, findingID, snapshotID, role, a.Note, position); err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(fmt.Sprintf("traffic %s bound to finding %d as %s", snapshotID, findingID, role)), nil
		},
	}
}
