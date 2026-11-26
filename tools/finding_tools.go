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
	reg.Register(makeMergeFindings(store))
	reg.Register(makeDismissFinding(store))
}

func makeReportFinding(store Store, taskID, explorationID int64) Tool {
	return Tool{
		Name:     "report_finding",
		Category: CategoryFindings,
		Description: "Record a confirmed finding, with evidence providing verifiable proof such as command output and logs. " +
			"Pass the current intent_id when in a task context; it also renews your claim on that intent (pass owner too if you claimed with one). " +
			"After reporting, the coordinator sees this in graph_overview and can use it as evidence for prove_goal.\n" +
			"Before reporting, especially when other agents work the same engagement, call list_findings (or list_changes) and check for " +
			"an existing finding with the same vulnclass on the same asset; if it is the same issue, add to it instead of filing a new one. " +
			"Pass asset_ids so duplicates can be detected. If the response has possible_duplicates, compare them and call merge_findings for " +
			"any that are the same issue rather than leaving both.",
		Schema: obj(map[string]any{
			"vulnclass": str("vulnerability class"),
			"name":      str("vulnerability name"),
			"severity":  enm([]string{"critical", "high", "medium", "low"}, "severity level"),
			"summary":   str("finding summary"),
			"intent_id": intp("the current task's intent id"),
			"asset_ids": arr(map[string]any{"type": "integer"}, "affected asset ids"),
			"evidence":  str("evidence/PoC text"),
			"owner":     str(renewOwnerArgDesc),
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
				Owner     string            `json:"owner"`
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

			dupes, _ := store.FindPotentialDuplicates(ctx, explorationID, input.VulnClass, input.Severity, input.AssetIDs)

			findingID, nodeID, err := store.AddFinding(ctx, input)
			if err != nil {
				return errResult(err.Error()), nil
			}
			renewHeldIntents(ctx, store, a.Owner, []int64{input.IntentID})

			resp := map[string]any{"finding_id": findingID, "node_id": nodeID}
			if len(dupes) > 0 {
				resp["possible_duplicates"] = dupes
				resp["duplicate_note"] = fmt.Sprintf("Open findings that may be the same issue: same vulnclass, or same severity on an overlapping asset. "+
					"If one is the same issue, call merge_findings with finding_ids including %d and primary_id set to the best-evidenced one; "+
					"if they are distinct issues, leave them.", findingID)
			}
			return jsonResult(resp)
		},
	}
}

func makeListFindings(store Store, explorationID int64) Tool {
	return Tool{
		Name:        "list_findings",
		Category:    CategoryFindings,
		Description: "List the confirmed findings of this task (compact: id+vulnclass+severity+summary+state+status+traffic_count).",
		ReadOnly:    true,
		Schema:      obj(map[string]any{}),
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
				if f.Dismissed {
					m["dismissed"] = true
					m["dismiss_reason"] = f.DismissReason
				}
				if f.MergedInto > 0 {
					m["merged_into"] = f.MergedInto
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

func makeBindFindingTraffic(store Store, snapshotExists func(id string) (bool, error)) Tool {
	return Tool{
		Name:     "bind_finding_traffic",
		Category: CategoryFindings,
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
			found, err := snapshotExists(snapshotID)
			if err != nil {
				return errResult(err.Error()), nil
			}
			if !found {
				return errResult(fmt.Sprintf("snapshot %q not found in captured traffic; use traffic_search to find a valid id", snapshotID)), nil
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

func makeMergeFindings(store Store) Tool {
	return Tool{
		Name:     "merge_findings",
		Category: CategoryFindings,
		Description: "Merge two or more findings into a single primary finding. " +
			"Evidence and assets from secondary findings are folded into the primary. " +
			"Secondary findings are marked as duplicates with a pointer to the primary. " +
			"If primary_id was already merged, the merge goes to the live finding it now points to: " +
			"the result's id is that finding and requested_primary_id is the one you passed. " +
			"Secondaries that are already merged or dismissed are left alone and listed in skipped with a reason.",
		Schema: obj(map[string]any{
			"finding_ids": arr(map[string]any{"type": "integer"}, "all finding ids to merge (must include primary_id)"),
			"primary_id":  intp("the finding to keep as the merged result"),
		}, "finding_ids", "primary_id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				FindingIDs []json.RawMessage `json:"finding_ids"`
				PrimaryID  json.RawMessage   `json:"primary_id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			primaryID := pid(a.PrimaryID)
			if primaryID <= 0 {
				return errResult("primary_id is required"), nil
			}
			ids := pidList(a.FindingIDs)
			if len(ids) < 2 {
				return errResult("finding_ids must contain at least two findings"), nil
			}
			hasPrimary := false
			var secondaryIDs []int64
			for _, id := range ids {
				if id == primaryID {
					hasPrimary = true
				} else {
					secondaryIDs = append(secondaryIDs, id)
				}
			}
			if !hasPrimary {
				return errResult("finding_ids must include primary_id"), nil
			}
			if len(secondaryIDs) == 0 {
				return errResult("finding_ids must contain at least one secondary finding"), nil
			}

			merged, err := store.MergeFindings(ctx, primaryID, secondaryIDs)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(merged)
		},
	}
}

func makeDismissFinding(store Store) Tool {
	return Tool{
		Name:     "dismiss_finding",
		Category: CategoryFindings,
		Description: "Mark a finding as dismissed (false positive). " +
			"Dismissed findings are excluded from generate_report output, and goals they alone proved reopen. " +
			"Do not dismiss a duplicate of another finding: use merge_findings so its evidence and goal proofs move to the kept finding.",
		Schema: obj(map[string]any{
			"finding_id": intp("the finding to dismiss"),
			"reason":     str("why this finding is a false positive"),
		}, "finding_id", "reason"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				FindingID json.RawMessage `json:"finding_id"`
				Reason    string          `json:"reason"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			findingID := pid(a.FindingID)
			if findingID <= 0 {
				return errResult("finding_id is required"), nil
			}
			reason := strings.TrimSpace(a.Reason)
			if reason == "" {
				return errResult("reason is required"), nil
			}
			if err := store.DismissFinding(ctx, findingID, reason); err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(fmt.Sprintf("finding %d dismissed: %s", findingID, reason)), nil
		},
	}
}

var vulnClassAliases = map[string]string{
	"insecuredirectobjectreference":  "idor",
	"insecuredirectobjectreferences": "idor",
	"brokenobjectlevelauthorization": "idor",
	"bola":                           "idor",
	"sqlinjection":                   "sqli",
	"crosssitescripting":             "xss",
	"crosssiterequestforgery":        "csrf",
	"serversiderequestforgery":       "ssrf",
	"remotecodeexecution":            "rce",
	"localfileinclusion":             "lfi",
	"xmlexternalentity":              "xxe",
	"xmlexternalentities":            "xxe",
	"servertemplateinjection":        "ssti",
	"serversidetemplateinjection":    "ssti",
}

// NormalizeVulnClass maps spelling variants of a vulnerability class to one
// key: case, spaces, punctuation and parenthesized parts are ignored and
// common long forms map to their acronym, so "IDOR" and "Insecure Direct
// Object Reference (IDOR)" match.
func NormalizeVulnClass(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
		case depth == 0 && ((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')):
			b.WriteRune(r)
		}
	}
	key := b.String()
	if alias, ok := vulnClassAliases[key]; ok {
		return alias
	}
	return key
}
