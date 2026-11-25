package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func RegisterActivityTools(reg *Registry, store Store, explorationID int64) {
	reg.Register(makeListActivity(store, explorationID))
	reg.Register(makeLogActivity(store, explorationID))
}

func makeListActivity(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "list_activity",
		Category: CategoryActivity,
		Description: "List recent activity log entries for this engagement: intents started/completed, facts recorded, findings reported, tool calls. " +
			"Optionally filter by node_id, kind, or since (time). Returns newest first.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"limit":   intp("number of entries to return, default 50, max 500"),
			"node_id": intp("filter to activity on a specific exploration node"),
			"kind":    str("filter by activity kind (e.g. tool_use, tool_result, intent_start, intent_done, finding)"),
			"since":   str("only return entries created after this time (RFC 3339 or duration like 1h, 30m)"),
			"format":  enm([]string{"json", "csv"}, "output format (default json)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Limit  int             `json:"limit"`
				NodeID json.RawMessage `json:"node_id"`
				Kind   string          `json:"kind"`
				Since  string          `json:"since"`
				Format string          `json:"format"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}

			since, err := parseSince(a.Since)
			if err != nil {
				return errResult("invalid since: " + err.Error()), nil
			}

			filter := ActivityFilter{
				NodeID: pid(a.NodeID),
				Kind:   strings.TrimSpace(a.Kind),
				Limit:  a.Limit,
				Since:  since,
			}

			entries, err := store.ListActivity(ctx, explorationID, filter)
			if err != nil {
				return errResult(err.Error()), nil
			}

			if strings.EqualFold(strings.TrimSpace(a.Format), "csv") {
				return textResult(activityToCSV(entries)), nil
			}

			return jsonResult(map[string]any{
				"count":    len(entries),
				"activity": entries,
			})
		},
	}
}

func makeLogActivity(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "log_activity",
		Category: CategoryActivity,
		Description: "Record an activity log entry for this engagement. Use this to log significant actions, " +
			"decisions, or observations that should be visible in the activity timeline.",
		Schema: obj(map[string]any{
			"action":  str("what happened (used as the activity kind)"),
			"node_id": intp("related exploration node id (optional)"),
			"detail":  str("detailed description of the activity"),
			"summary": str("one-line summary"),
		}, "action"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Action  string          `json:"action"`
				NodeID  json.RawMessage `json:"node_id"`
				Detail  string          `json:"detail"`
				Summary string          `json:"summary"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			action := strings.TrimSpace(a.Action)
			if action == "" {
				return errResult("action is required"), nil
			}

			entry := ActivityEntry{
				NodeID:  pid(a.NodeID),
				Kind:    action,
				Summary: a.Summary,
				Detail:  a.Detail,
			}

			id, err := store.LogActivity(ctx, explorationID, entry)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(fmt.Sprintf("activity logged: %d", id)), nil
		},
	}
}

func parseSince(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format("2006-01-02 15:04:05"), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return "", fmt.Errorf("expected RFC 3339 timestamp or duration (e.g. 1h, 30m): %s", s)
	}
	return time.Now().Add(-d).UTC().Format("2006-01-02 15:04:05"), nil
}

func activityToCSV(entries []Activity) string {
	var b strings.Builder
	b.WriteString("id,exploration_id,node_id,worker,kind,tool,is_error,summary,detail,created_at\n")
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("%d,%d,%d,%s,%s,%s,%t,%s,%s,%s\n",
			e.ID, e.ExplorationID, e.NodeID,
			csvQuote(e.Worker), csvQuote(e.Kind), csvQuote(e.Tool),
			e.IsError, csvQuote(e.Summary), csvQuote(e.Detail), csvQuote(e.CreatedAt)))
	}
	return b.String()
}

func csvQuote(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
