package tools

import (
	"context"
	"encoding/json"
	"strings"
)

func RegisterScopeTools(reg *Registry, store Store, taskID int64) {
	reg.Register(makeRecordCoverage(store, taskID))
	reg.Register(makeCheckScope(store, taskID))
}

func makeCheckScope(store Store, taskID int64) Tool {
	return Tool{
		Name:        "check_scope",
		Category:    CategoryAssets,
		Description: "Check whether a target (domain, IP, or URL) is within the engagement's scope rules.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"target": str("domain, IP address, or URL to check"),
		}, "target"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Target string `json:"target"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Target) == "" {
				return errResult("target is required"), nil
			}
			ok, reason, rule, err := store.IsInScope(ctx, taskID, a.Target)
			if err != nil {
				return errResult("scope check failed: " + err.Error()), nil
			}
			return jsonResult(map[string]any{
				"in_scope":      ok,
				"reason":        reason,
				"matching_rule": rule,
			})
		},
	}
}

func makeRecordCoverage(store Store, taskID int64) Tool {
	return Tool{
		Name:     "record_coverage",
		Category: CategoryAssets,
		Description: "Record what you examined on a surface for one risk class (use the goal's vulnclass as risk_area when it applies), " +
			"including negative results, so an absent finding is never read as a clean target.",
		Schema: obj(map[string]any{
			"surface":   str("what was examined: a URL, endpoint, parameter, host or feature"),
			"risk_area": str("risk class, e.g. sqli, xss, idor"),
			"outcome":   enm([]string{"reported", "no_issue_found", "ruled_out", "not_applicable", "needs_follow_up"}, "result for this surface and risk class"),
			"evidence":  str("required for no_issue_found: a fact id, intent id or traffic id that shows the test"),
			"reason":    str("required for ruled_out and not_applicable: why"),
			"intent_id": intp("the intent you are working; renews your claim on it"),
			"owner":     str(renewOwnerArgDesc),
		}, "surface", "risk_area", "outcome"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			if taskID <= 0 {
				return errResult("record_coverage requires a task context"), nil
			}
			var a struct {
				CoverageEntry
				Owner string `json:"owner"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			e := a.CoverageEntry
			e.Outcome = strings.ToLower(strings.TrimSpace(e.Outcome))
			if err := store.RecordCoverage(ctx, taskID, e); err != nil {
				return errResult(err.Error()), nil
			}
			renewHeldIntents(ctx, store, a.Owner, []int64{e.IntentID})
			return textResult("recorded"), nil
		},
	}
}
