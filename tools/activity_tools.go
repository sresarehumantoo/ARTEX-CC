package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func RegisterActivityTools(reg *Registry, store Store, explorationID int64) {
	reg.Register(makeListActivity(store, explorationID))
	reg.Register(makeLogActivity(store, explorationID))
}

func makeListActivity(store Store, explorationID int64) Tool {
	return Tool{
		Name: "list_activity",
		Description: "List recent activity log entries for this engagement: intents started/completed, facts recorded, findings reported, tool calls. " +
			"Optionally filter by node_id or kind. Returns newest first.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"limit":   intp("number of entries to return, default 50, max 500"),
			"node_id": intp("filter to activity on a specific exploration node"),
			"kind":    str("filter by activity kind (e.g. tool_use, tool_result, intent_start, intent_done, finding)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Limit  int             `json:"limit"`
				NodeID json.RawMessage `json:"node_id"`
				Kind   string          `json:"kind"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}

			filter := ActivityFilter{
				NodeID: pid(a.NodeID),
				Kind:   strings.TrimSpace(a.Kind),
				Limit:  a.Limit,
			}

			entries, err := store.ListActivity(ctx, explorationID, filter)
			if err != nil {
				return errResult(err.Error()), nil
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
		Name: "log_activity",
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
