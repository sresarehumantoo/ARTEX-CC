package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func RegisterEngagementTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeCreateEngagement(store))
	reg.Register(makeListEngagements(store, taskID))
	reg.Register(makeEngagementStatus(store, taskID, explorationID))
	reg.Register(makeGoalMet(store, taskID))
	reg.Register(makeUpdateEngagement(store, taskID))
	reg.Register(makeDeleteEngagement(store))
	reg.Register(makeEngagementActivity(store, explorationID))
	reg.Register(makeListCoverageGaps(store, taskID, explorationID))
}

func makeCreateEngagement(store Store) Tool {
	return Tool{
		Name:        "create_engagement",
		Category:    CategoryEngagement,
		Description: "Create a new pentest engagement (task + exploration graph).",
		Schema: obj(map[string]any{
			"goal":        str("high-level goal for the engagement"),
			"description": str("detailed description of the engagement scope"),
		}, "goal"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Goal        string `json:"goal"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Goal) == "" {
				return errResult("goal is required"), nil
			}
			if err := validateTextLen("goal", a.Goal); err != nil {
				return errResult(err.Error()), nil
			}
			if err := validateTextLen("description", a.Description); err != nil {
				return errResult(err.Error()), nil
			}
			desc := a.Description
			if desc == "" {
				desc = a.Goal
			}
			tid, eid, err := store.CreateEngagement(ctx, a.Goal, desc)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"task_id":        tid,
				"exploration_id": eid,
			})
		},
	}
}

func makeListEngagements(store Store, activeTaskID int64) Tool {
	return Tool{
		Name:        "list_engagements",
		Category:    CategoryEngagement,
		Description: "List all pentest engagements with summary stats (goal, status, node/finding/asset counts).",
		ReadOnly:    true,
		Schema:      obj(map[string]any{}),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			engagements, err := store.ListEngagements(ctx)
			if err != nil {
				return errResult(err.Error()), nil
			}
			out := make([]map[string]any, 0, len(engagements))
			for _, e := range engagements {
				m := map[string]any{
					"id":             e.ID,
					"goal":           e.Goal,
					"description":    e.Description,
					"status":         e.Status,
					"exploration_id": e.ExplorationID,
					"created_at":     e.CreatedAt,
					"node_count":     e.NodeCount,
					"finding_count":  e.FindingCount,
					"asset_count":    e.AssetCount,
					"goals_met":      e.GoalsMet,
					"active":         e.ID == activeTaskID,
				}
				out = append(out, m)
			}
			return jsonResult(out)
		},
	}
}

func makeGoalMet(store Store, taskID int64) Tool {
	return Tool{
		Name:     "goal_met",
		Category: CategoryEngagement,
		Description: "Mark the entire engagement as complete. Call this when all goals have been proved and the engagement is finished. " +
			"Sets the task status to done. Provide an optional summary of what was accomplished.",
		Schema: obj(map[string]any{
			"summary": str("optional summary of what was accomplished"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			if taskID <= 0 {
				return errResult("goal_met requires a task context"), nil
			}
			var a struct {
				Summary string `json:"summary"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if err := store.CompleteEngagement(ctx, taskID, a.Summary); err != nil {
				return errResult(err.Error()), nil
			}
			msg := fmt.Sprintf("engagement %d marked complete", taskID)
			if a.Summary != "" {
				msg += ": " + firstLine(a.Summary, 120)
			}
			return textResult(msg), nil
		},
	}
}

func makeUpdateEngagement(store Store, taskID int64) Tool {
	return Tool{
		Name:        "update_engagement",
		Category:    CategoryEngagement,
		Description: "Update the current engagement's goal or description.",
		Schema: obj(map[string]any{
			"goal":        str("new goal for the engagement"),
			"description": str("new description for the engagement"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			if taskID <= 0 {
				return errResult("update_engagement requires a task context"), nil
			}
			var a struct {
				Goal        *string `json:"goal"`
				Description *string `json:"description"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if a.Goal == nil && a.Description == nil {
				return errResult("at least one of goal or description is required"), nil
			}
			if a.Goal != nil {
				if strings.TrimSpace(*a.Goal) == "" {
					return errResult("goal cannot be empty"), nil
				}
				if err := validateTextLen("goal", *a.Goal); err != nil {
					return errResult(err.Error()), nil
				}
			}
			if a.Description != nil {
				if err := validateTextLen("description", *a.Description); err != nil {
					return errResult(err.Error()), nil
				}
			}
			if err := store.UpdateEngagement(ctx, taskID, a.Goal, a.Description); err != nil {
				return errResult(err.Error()), nil
			}
			parts := make([]string, 0, 2)
			if a.Goal != nil {
				parts = append(parts, "goal")
			}
			if a.Description != nil {
				parts = append(parts, "description")
			}
			return textResult(fmt.Sprintf("engagement %d updated: %s", taskID, strings.Join(parts, ", "))), nil
		},
	}
}

func makeDeleteEngagement(store Store) Tool {
	return Tool{
		Name:     "delete_engagement",
		Category: CategoryEngagement,
		Description: "Delete an engagement and all its data (nodes, edges, findings, credentials, coverage, assets, constraints). " +
			"This is irreversible. Both task_id and confirm=true are required.",
		Schema: obj(map[string]any{
			"task_id": intp("task id of the engagement to delete"),
			"confirm": map[string]any{"type": "boolean", "description": "must be true to confirm deletion"},
		}, "task_id", "confirm"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				TaskID  json.RawMessage `json:"task_id"`
				Confirm bool            `json:"confirm"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			taskID := pid(a.TaskID)
			if taskID <= 0 {
				return errResult("task_id is required"), nil
			}
			if !a.Confirm {
				return errResult("confirm must be true to delete an engagement"), nil
			}
			if err := store.DeleteEngagement(ctx, taskID); err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(fmt.Sprintf("engagement %d deleted", taskID)), nil
		},
	}
}

func makeEngagementActivity(store Store, explorationID int64) Tool {
	return Tool{
		Name:        "engagement_activity",
		Category:    CategoryEngagement,
		Description: "Show recent activity in the current engagement as a timeline of findings, assets, and graph nodes.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"limit": intp("maximum number of entries to return (default 20)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			if explorationID <= 0 {
				return errResult("engagement_activity requires an engagement context"), nil
			}
			var a struct {
				Limit json.RawMessage `json:"limit"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			limit := int(pid(a.Limit))
			if limit <= 0 {
				limit = 20
			}
			entries, err := store.EngagementActivity(ctx, explorationID, limit)
			if err != nil {
				return errResult(err.Error()), nil
			}
			if len(entries) == 0 {
				return textResult("no activity yet"), nil
			}
			var lines []string
			for _, e := range entries {
				lines = append(lines, fmt.Sprintf("%s - %s: %s", e.Timestamp, e.Kind, e.Summary))
			}
			return textResult(strings.Join(lines, "\n")), nil
		},
	}
}

func makeEngagementStatus(store Store, defaultTaskID, defaultExplorationID int64) Tool {
	return Tool{
		Name:        "engagement_status",
		Category:    CategoryEngagement,
		Description: "Get the status of an engagement: goal, status, finding count, and coverage summary.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"task_id": intp("task id (defaults to current engagement)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				TaskID json.RawMessage `json:"task_id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}

			taskID := pid(a.TaskID)
			explorationID := defaultExplorationID

			if taskID > 0 && taskID != defaultTaskID {
				detail, err := store.GetEngagement(ctx, taskID)
				if err != nil {
					return errResult(err.Error()), nil
				}
				if detail == nil {
					return errResult(fmt.Sprintf("engagement %d not found", taskID)), nil
				}
				explorationID = detail.ExplorationID
			} else {
				taskID = defaultTaskID
			}

			ov, err := store.GraphOverview(ctx, explorationID)
			if err != nil {
				return errResult(err.Error()), nil
			}

			_, goal, _ := store.Root(ctx, explorationID)

			result := map[string]any{
				"task_id":        taskID,
				"exploration_id": explorationID,
				"goal":           goal,
				"status":         ov.Task["status"],
				"findings_total": ov.FindingsTotal,
				"facts_total":    ov.FactsTotal,
				"frontier_open":  ov.FrontierOpen,
				"done_intents":   ov.DoneIntentsTotal,
			}
			if ov.Coverage != nil {
				result["coverage"] = ov.Coverage
			}
			return jsonResult(result)
		},
	}
}

func makeListCoverageGaps(store Store, taskID, explorationID int64) Tool {
	return Tool{
		Name:     "list_coverage_gaps",
		Category: CategoryEngagement,
		Description: "Identify testing gaps in the current engagement: assets never targeted by an intent, " +
			"intents with no recorded facts, goals not yet proven, and service ports discovered but not enriched.",
		ReadOnly: true,
		Schema:   obj(map[string]any{}),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			if taskID <= 0 || explorationID <= 0 {
				return errResult("list_coverage_gaps requires an active engagement context"), nil
			}
			gaps, err := store.ListCoverageGaps(ctx, taskID, explorationID)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(gaps)
		},
	}
}
