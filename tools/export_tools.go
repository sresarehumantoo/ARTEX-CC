package tools

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func RegisterExportTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeExportEngagement(store, taskID))
	reg.Register(makeImportEngagement(store))
}

func makeExportEngagement(store Store, defaultTaskID int64) Tool {
	return Tool{
		Name:     "export_engagement",
		Category: CategoryReporting,
		Description: "Export the current engagement (or a specific one). " +
			"Includes assets, exploration graph, findings, credentials, coverage, and scope. " +
			"Format: json (default) returns the full export object; csv returns per-entity CSV tables (assets, findings, nodes).",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"task_id": intp("task id to export (defaults to current engagement)"),
			"format":  enm([]string{"json", "csv"}, "export format: json (default) or csv"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				TaskID json.RawMessage `json:"task_id"`
				Format string          `json:"format"`
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

			format := strings.ToLower(strings.TrimSpace(a.Format))
			if format == "" {
				format = "json"
			}

			export, err := store.ExportEngagementData(ctx, taskID)
			if err != nil {
				return errResult(err.Error()), nil
			}

			switch format {
			case "json":
				return jsonResult(export)
			case "csv":
				return exportCSV(export)
			default:
				return errResult(fmt.Sprintf("unsupported format: %s (use json or csv)", format)), nil
			}
		},
	}
}

func makeImportEngagement(store Store) Tool {
	return Tool{
		Name:     "import_engagement",
		Category: CategoryReporting,
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

func exportCSV(export *EngagementExport) (Result, error) {
	assetsCSV, err := exportAssetsCSV(export.Assets)
	if err != nil {
		return errResult(fmt.Sprintf("assets csv: %v", err)), nil
	}
	findingsCSV, err := exportFindingsCSV(export.Findings)
	if err != nil {
		return errResult(fmt.Sprintf("findings csv: %v", err)), nil
	}
	nodesCSV, err := exportNodesCSV(export.Nodes)
	if err != nil {
		return errResult(fmt.Sprintf("nodes csv: %v", err)), nil
	}

	out := map[string]any{
		"format":       "csv",
		"exported_at":  time.Now().UTC().Format(time.RFC3339),
		"assets_csv":   assetsCSV,
		"findings_csv": findingsCSV,
		"nodes_csv":    nodesCSV,
	}
	return jsonResult(out)
}

func exportAssetsCSV(assets []map[string]any) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := []string{"id", "type", "domain", "ip", "url", "service_name", "port", "status_code", "page_title"}
	if err := w.Write(header); err != nil {
		return "", err
	}
	for _, a := range assets {
		row := []string{
			csvMapStr(a, "id"),
			csvMapStr(a, "type"),
			csvMapStr(a, "domain"),
			csvMapStr(a, "ip"),
			csvMapStr(a, "url"),
			csvMapStr(a, "service_name"),
			csvMapStr(a, "port"),
			csvMapStr(a, "status_code"),
			csvMapStr(a, "page_title"),
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}
	w.Flush()
	return buf.String(), w.Error()
}

func exportFindingsCSV(findings []map[string]any) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := []string{"id", "name", "severity", "status", "summary", "asset_ids"}
	if err := w.Write(header); err != nil {
		return "", err
	}
	for _, f := range findings {
		assetIDs := ""
		if ids, ok := f["asset_ids"]; ok {
			switch v := ids.(type) {
			case []any:
				parts := make([]string, 0, len(v))
				for _, id := range v {
					parts = append(parts, fmt.Sprintf("%v", id))
				}
				assetIDs = strings.Join(parts, ";")
			case []int64:
				parts := make([]string, 0, len(v))
				for _, id := range v {
					parts = append(parts, strconv.FormatInt(id, 10))
				}
				assetIDs = strings.Join(parts, ";")
			}
		}
		row := []string{
			csvMapStr(f, "id"),
			csvMapStr(f, "name"),
			csvMapStr(f, "severity"),
			csvMapStr(f, "status"),
			csvMapStr(f, "summary"),
			assetIDs,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}
	w.Flush()
	return buf.String(), w.Error()
}

func exportNodesCSV(nodes []map[string]any) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := []string{"id", "kind", "state", "text", "created_at"}
	if err := w.Write(header); err != nil {
		return "", err
	}
	for _, n := range nodes {
		text := ""
		if p, ok := n["payload"]; ok {
			if pm, ok := p.(map[string]any); ok {
				if t, ok := pm["text"]; ok {
					text = fmt.Sprintf("%v", t)
				}
			}
		}
		row := []string{
			csvMapStr(n, "id"),
			csvMapStr(n, "kind"),
			csvMapStr(n, "state"),
			text,
			csvMapStr(n, "created_at"),
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}
	w.Flush()
	return buf.String(), w.Error()
}

func csvMapStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
