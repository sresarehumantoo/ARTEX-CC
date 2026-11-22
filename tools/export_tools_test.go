package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type exportMockStore struct {
	mockStore
	exportData *EngagementExport
	exportErr  error
	importTask int64
	importExpl int64
	importErr  error
}

func (m *exportMockStore) ExportEngagementData(_ context.Context, _ int64) (*EngagementExport, error) {
	return m.exportData, m.exportErr
}

func (m *exportMockStore) ImportEngagementData(_ context.Context, _ *EngagementExport) (int64, int64, error) {
	return m.importTask, m.importExpl, m.importErr
}

func TestExportEngagementJSONHappy(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{exportData: &EngagementExport{
		Version: 1,
		Goal:    "test goal",
		Assets:  []map[string]any{{"id": float64(1), "type": "ip"}},
	}}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("export_engagement")
	if !ok {
		t.Fatal("export_engagement not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"version":1`) {
		t.Errorf("expected version 1, got: %s", r.Content)
	}
}

func TestExportEngagementCSVHappy(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{exportData: &EngagementExport{
		Version: 1,
		Goal:    "csv test",
		Assets:  []map[string]any{{"id": float64(1), "type": "ip", "ip": "10.0.0.1"}},
		Findings: []map[string]any{{
			"id": float64(1), "name": "XSS", "severity": "high",
			"status": "confirmed", "summary": "reflected xss",
		}},
		Nodes: []map[string]any{{
			"id": float64(1), "kind": "fact", "state": "confirmed",
			"payload": map[string]any{"text": "some fact"},
		}},
	}}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("export_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"format": "csv"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"format":"csv"`) {
		t.Errorf("expected csv format, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "assets_csv") {
		t.Errorf("expected assets_csv, got: %s", r.Content)
	}
}

func TestExportEngagementUnsupportedFormat(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{exportData: &EngagementExport{Version: 1}}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("export_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"format": "xml"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(r.Content, "unsupported format") {
		t.Errorf("expected unsupported format error, got: %s", r.Content)
	}
}

func TestExportEngagementExplicitTaskID(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{exportData: &EngagementExport{Version: 1, Goal: "explicit"}}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("export_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"task_id": 5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestExportEngagementNoTaskID(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 0, 0)

	tool, _ := reg.Lookup("export_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when no task_id available")
	}
	if !strings.Contains(r.Content, "task_id is required") {
		t.Errorf("expected task_id required, got: %s", r.Content)
	}
}

func TestExportEngagementStoreError(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{exportErr: fmt.Errorf("not found")}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("export_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestExportEngagementInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("export_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestImportEngagementHappy(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{importTask: 10, importExpl: 20}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("import_engagement")
	if !ok {
		t.Fatal("import_engagement not found")
	}

	exportJSON := `{"version":1,"goal":"imported","description":"test","nodes":[],"edges":[],"constraints":[],"assets":[],"findings":[],"credentials":[],"coverage":[],"scope":[]}`
	input := fmt.Sprintf(`{"data": %q}`, exportJSON)
	r, err := tool.Run(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"task_id":10`) {
		t.Errorf("expected task_id 10, got: %s", r.Content)
	}
}

func TestImportEngagementEmptyData(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("import_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"data": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty data")
	}
	if !strings.Contains(r.Content, "data is required") {
		t.Errorf("expected data required, got: %s", r.Content)
	}
}

func TestImportEngagementInvalidExportData(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("import_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"data": "not valid json"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid export data")
	}
	if !strings.Contains(r.Content, "invalid export data") {
		t.Errorf("expected invalid export data, got: %s", r.Content)
	}
}

func TestImportEngagementStoreError(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{importErr: fmt.Errorf("duplicate key")}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("import_engagement")
	exportJSON := `{"version":1,"goal":"test","description":"","nodes":[],"edges":[],"constraints":[],"assets":[],"findings":[],"credentials":[],"coverage":[],"scope":[]}`
	input := fmt.Sprintf(`{"data": %q}`, exportJSON)
	r, err := tool.Run(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestImportEngagementInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("import_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExportCSVEmptyExport(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{exportData: &EngagementExport{
		Version: 1,
		Goal:    "empty",
	}}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("export_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"format": "csv"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success for empty CSV export, got error: %s", r.Content)
	}
}

func TestExportCSVWithAssetIDs(t *testing.T) {
	t.Parallel()
	ms := &exportMockStore{exportData: &EngagementExport{
		Version: 1,
		Findings: []map[string]any{{
			"id": float64(1), "name": "test", "severity": "high",
			"status": "open", "summary": "s",
			"asset_ids": []any{float64(1), float64(2)},
		}},
	}}
	reg := NewRegistry()
	RegisterExportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("export_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"format": "csv"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestCsvMapStr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{"string", map[string]any{"k": "hello"}, "k", "hello"},
		{"float_int", map[string]any{"k": float64(42)}, "k", "42"},
		{"float_decimal", map[string]any{"k": float64(3.14)}, "k", "3.14"},
		{"int", map[string]any{"k": 7}, "k", "7"},
		{"int64", map[string]any{"k": int64(99)}, "k", "99"},
		{"missing", map[string]any{}, "k", ""},
		{"json_number", map[string]any{"k": json.Number("123")}, "k", "123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := csvMapStr(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("csvMapStr(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
			}
		})
	}
}
