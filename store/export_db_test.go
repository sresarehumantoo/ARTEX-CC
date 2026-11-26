package store

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestExportEngagementData(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	now := time.Now()

	// GetTask
	mock.ExpectQuery("SELECT id, name, description, goal, exploration_id, status, company_id, created_at, updated_at FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "goal", "exploration_id", "status", "company_id", "created_at", "updated_at"}).
			AddRow(1, "task1", "desc", "goal", int64(10), "active", nil, now, now))

	// exportNodes
	mock.ExpectQuery("SELECT n.id, n.kind, n.payload, n.priority, n.state").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "kind", "payload", "priority", "state", "origin", "owner", "anchors"}).
			AddRow(int64(1), "fact", json.RawMessage(`{"text":"root"}`), 0, "origin", "system", "", "[]"))

	// exportEdges
	mock.ExpectQuery("SELECT src_id, rel, dst_id FROM exploration_edges").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"src_id", "rel", "dst_id"}))

	// exportConstraints
	mock.ExpectQuery("SELECT kind, text, COALESCE\\(origin").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "text", "origin"}))

	// exportAssets
	mock.ExpectQuery("SELECT id, type, domain, root_domain, ip, port, url, service_type, service_name").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "type", "domain", "root_domain", "ip", "port",
			"url", "service_type", "service_name", "bundle_id",
			"app_name", "method", "technologies", "extra",
			"company_id", "company_source",
		}).AddRow(
			int64(100), "root_domain",
			sql.NullString{String: "example.com", Valid: true},
			sql.NullString{String: "example.com", Valid: true},
			sql.NullString{}, sql.NullInt32{},
			sql.NullString{}, sql.NullString{}, sql.NullString{},
			sql.NullString{}, sql.NullString{}, sql.NullString{},
			"{}", []byte("{}"),
			nil, "explicit",
		))

	// exportFindings
	mock.ExpectQuery("SELECT id, node_id, vulnclass, name, severity, summary, evidence, worker, asset_ids, status").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "node_id", "vulnclass", "name", "severity", "summary",
			"evidence", "worker", "asset_ids", "status",
			"merged_into", "dismissed", "dismiss_reason",
		}))

	// exportCredentials
	mock.ExpectQuery("SELECT domain, username, password, source, note FROM task_credentials").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"domain", "username", "password", "source", "note"}).
			AddRow("example.com", "admin", "pass123", "agent", "test cred"))

	// exportCoverage
	mock.ExpectQuery("SELECT surface, risk_area, outcome, evidence, reason, worker FROM coverage_ledger").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"surface", "risk_area", "outcome", "evidence", "reason", "worker"}))

	// exportScope
	mock.ExpectQuery("FROM task_scope ts").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net", "value", "company_id", "company_name", "source", "reason"}).
			AddRow("root_domain", "example.com", "", "", nil, "", "agent", ""))

	// exportCompanies
	mock.ExpectQuery("SELECT id, name, COALESCE\\(logo").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "logo"}))

	mock.ExpectQuery("FROM target_changes").
		WithArgs(int64(10), false).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "target", "resource", "kind", "intent_id", "original_fingerprint", "baseline",
			"baseline_change_id", "backup", "note", "recorded_by", "created_at", "restored_at", "restored_by",
			"last_observed", "restore_attempts",
		}))

	export, err := s.ExportEngagementData(ctx(), 1)
	if err != nil {
		t.Fatalf("ExportEngagementData: %v", err)
	}
	if export.Goal != "goal" {
		t.Errorf("Goal = %q, want %q", export.Goal, "goal")
	}
	if len(export.Nodes) != 1 {
		t.Errorf("Nodes = %d, want 1", len(export.Nodes))
	}
	if len(export.Assets) != 1 {
		t.Errorf("Assets = %d, want 1", len(export.Assets))
	}
	if len(export.Credentials) != 1 {
		t.Errorf("Credentials = %d, want 1", len(export.Credentials))
	}
	if len(export.Scope) != 1 {
		t.Errorf("Scope = %d, want 1", len(export.Scope))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExportEngagementData_NotFound(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectQuery("SELECT id, name, description, goal").
		WithArgs(int64(999)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "goal", "exploration_id", "status", "company_id", "created_at", "updated_at"}))

	_, err := s.ExportEngagementData(ctx(), 999)
	if err == nil {
		t.Fatal("expected error for nonexistent engagement")
	}
}

func TestImportEngagementData(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	data := &tools.EngagementExport{
		Version:     1,
		Goal:        "find bugs",
		Description: "test import",
		Nodes: []map[string]any{
			{"id": float64(1), "kind": "fact", "state": "origin", "priority": float64(0), "origin": "system", "payload": map[string]any{"text": "root"}},
		},
		Edges:       []map[string]any{},
		Constraints: []map[string]any{},
		Assets:      []map[string]any{},
		Findings:    []map[string]any{},
		Credentials: []map[string]any{},
		Coverage:    []map[string]any{},
		Scope:       []map[string]any{},
	}

	mock.ExpectBegin()

	// create exploration
	mock.ExpectQuery("INSERT INTO explorations").
		WithArgs("test import", "find bugs").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(100)))

	// create task
	mock.ExpectQuery("INSERT INTO tasks").
		WithArgs("find bugs", "test import", "find bugs", int64(100), nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(50)))

	// import node
	mock.ExpectQuery("INSERT INTO exploration_nodes").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(200)))

	mock.ExpectCommit()

	taskID, expID, err := s.ImportEngagementData(ctx(), data)
	if err != nil {
		t.Fatalf("ImportEngagementData: %v", err)
	}
	if taskID != 50 {
		t.Errorf("taskID = %d, want 50", taskID)
	}
	if expID != 100 {
		t.Errorf("explorationID = %d, want 100", expID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestImportEngagementData_BadVersion(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)

	data := &tools.EngagementExport{Version: 0}
	_, _, err := s.ImportEngagementData(ctx(), data)
	if err == nil {
		t.Fatal("expected error for version 0")
	}
}
