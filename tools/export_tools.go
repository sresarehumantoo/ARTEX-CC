package tools

import (
	"context"
	"encoding/json"
)

func RegisterExportTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeExportEngagement(store, taskID))
	reg.Register(makeImportEngagement(store))
}

func makeExportEngagement(store Store, defaultTaskID int64) Tool {
	return Tool{
		Name: "export_engagement",
		Description: "Export the current engagement (or a specific one) as JSON. " +
			"Includes assets, exploration graph, findings, credentials, coverage, and scope. " +
			"Use this to back up engagements or share them.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"task_id": intp("task id to export (defaults to current engagement)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				TaskID json.RawMessage `json:"task_id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			taskID := pid(a.TaskID)
			if taskID <= 0 {
				taskID = defaultTaskID
			}
			if taskID <= 0 {
				return errResult("task_id is required"), nil
			}

			export, err := store.ExportEngagementData(ctx, taskID)
			if err != nil {
				return errResult(err.Error()), nil
			}

			return jsonResult(export)
		},
	}
}

func makeImportEngagement(store Store) Tool {
	return Tool{
		Name: "import_engagement",
		Description: "Import a previously exported engagement from JSON. " +
			"Creates a new engagement with all the imported data (assets, graph, findings, credentials, coverage, scope). " +
			"Returns the new task_id and exploration_id.",
		Schema: obj(map[string]any{
			"data": str("the exported engagement JSON string"),
		}, "data"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if a.Data == "" {
				return errResult("data is required"), nil
			}

			var export EngagementExport
			if err := json.Unmarshal([]byte(a.Data), &export); err != nil {
				return errResult("invalid export data: " + err.Error()), nil
			}

			taskID, explorationID, err := store.ImportEngagementData(ctx, &export)
			if err != nil {
				return errResult(err.Error()), nil
			}

			return jsonResult(map[string]any{
				"task_id":        taskID,
				"exploration_id": explorationID,
				"message":        "engagement imported successfully",
			})
		},
	}
}
