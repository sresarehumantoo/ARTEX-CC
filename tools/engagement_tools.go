package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func RegisterEngagementTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeCreateEngagement(store))
	reg.Register(makeListEngagements(store))
	reg.Register(makeEngagementStatus(store, taskID, explorationID))
}

func makeCreateEngagement(store Store) Tool {
	return Tool{
		Name:        "create_engagement",
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
				return errResult(err.Error()), nil
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

func makeListEngagements(store Store) Tool {
	return Tool{
		Name:        "list_engagements",
		Description: "List all pentest engagements (id, goal, status).",
		ReadOnly:    true,
		Schema:      obj(map[string]any{}),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			engagements, err := store.ListEngagements(ctx)
			if err != nil {
				return errResult(err.Error()), nil
			}
			out := make([]map[string]any, 0, len(engagements))
			for _, e := range engagements {
				out = append(out, map[string]any{
					"id":             e.ID,
					"goal":           e.Goal,
					"status":         e.Status,
					"exploration_id": e.ExplorationID,
				})
			}
			return jsonResult(out)
		},
	}
}

func makeEngagementStatus(store Store, defaultTaskID, defaultExplorationID int64) Tool {
	return Tool{
		Name:        "engagement_status",
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

			status := "active"
			if s, ok := ov.Task["status"].(string); ok && s != "" {
				status = s
			}

			result := map[string]any{
				"task_id":        taskID,
				"exploration_id": explorationID,
				"goal":           goal,
				"status":         status,
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
