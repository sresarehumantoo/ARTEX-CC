package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SREsAreHumanToo/cc-artex/tools"
)

func TestAddFindingMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`INSERT INTO findings`).
		WithArgs(int64(1), nil, "xss", "Reflected XSS", "high", "XSS in search", "evidence", "worker1", "[]", FindingStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))

	id, err := s.AddFinding(ctx, tools.FindingInput{
		TaskID:    1,
		VulnClass: "xss",
		Name:      "Reflected XSS",
		Severity:  "high",
		Summary:   "XSS in search",
		Evidence:  "evidence",
		Worker:    "worker1",
	})
	if err != nil {
		t.Fatalf("AddFinding: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAddFindingWithAssetIDsMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`INSERT INTO findings`).
		WithArgs(int64(1), nil, "sqli", "SQL Injection", "critical", "SQLi found", "", "", "[10,20]", FindingStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))

	id, err := s.AddFinding(ctx, tools.FindingInput{
		TaskID:    1,
		VulnClass: "sqli",
		Name:      "SQL Injection",
		Severity:  "critical",
		Summary:   "SQLi found",
		AssetIDs:  []int64{10, 20},
	})
	if err != nil {
		t.Fatalf("AddFinding: %v", err)
	}
	if id != 99 {
		t.Errorf("id = %d, want 99", id)
	}
}

func TestListFindingsMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "node_id", "vulnclass", "severity", "summary", "status", "merged_into", "dismissed", "dismiss_reason"}).
		AddRow(int64(1), sql.NullInt64{Int64: 5, Valid: true}, "xss", "high", "XSS found", "pending", sql.NullInt64{}, false, "").
		AddRow(int64(2), sql.NullInt64{}, "sqli", "critical", "SQLi found", "confirmed", sql.NullInt64{Int64: 1, Valid: true}, false, "")

	mock.ExpectQuery(`SELECT f\.id.*FROM findings f.*JOIN tasks t`).
		WithArgs(int64(10)).
		WillReturnRows(rows)

	findings, err := s.ListFindings(ctx, 10)
	if err != nil {
		t.Fatalf("ListFindings: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
	}
	if findings[0].ID != 1 || findings[0].VulnClass != "xss" || findings[0].NodeID != 5 {
		t.Errorf("findings[0] = %+v", findings[0])
	}
	if findings[1].MergedInto != 1 {
		t.Errorf("findings[1].MergedInto = %d, want 1", findings[1].MergedInto)
	}
}

func TestListFindingsInternalMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	taskID := int64(5)
	nodeID := int64(10)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM findings WHERE task_id`).
		WithArgs(taskID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT .+ FROM findings WHERE task_id.+ORDER BY created_at DESC LIMIT`).
		WithArgs(taskID, 50, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "node_id", "vulnclass", "name", "severity",
			"summary", "evidence", "worker", "asset_ids", "status", "report",
			"merged_into", "dismissed", "dismiss_reason", "created_at",
		}).AddRow(
			int64(1), &taskID, &nodeID, "xss", "XSS", "high",
			"summary", "evidence", "worker", json.RawMessage("[]"), "pending", "",
			nil, false, "", now,
		))

	findings, total, err := s.listFindingsInternal(FindingFilter{TaskID: taskID})
	if err != nil {
		t.Fatalf("listFindingsInternal: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].VulnClass != "xss" {
		t.Errorf("VulnClass = %q, want xss", findings[0].VulnClass)
	}
}

func TestListFindingsInternalNoFilterMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM findings$`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT .+ FROM findings ORDER BY created_at DESC LIMIT`).
		WithArgs(50, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "node_id", "vulnclass", "name", "severity",
			"summary", "evidence", "worker", "asset_ids", "status", "report",
			"merged_into", "dismissed", "dismiss_reason", "created_at",
		}))

	findings, total, err := s.listFindingsInternal(FindingFilter{})
	if err != nil {
		t.Fatalf("listFindingsInternal: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0", len(findings))
	}
}

func TestFindingExistsMock(t *testing.T) {
	t.Parallel()

	t.Run("exists", func(t *testing.T) {
		s, mock := newRegexpMockStore(t)
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := s.FindingExists(context.Background(), 1)
		if err != nil {
			t.Fatalf("FindingExists: %v", err)
		}
		if !exists {
			t.Error("expected true")
		}
	})

	t.Run("not exists", func(t *testing.T) {
		s, mock := newRegexpMockStore(t)
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs(int64(999)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		exists, err := s.FindingExists(context.Background(), 999)
		if err != nil {
			t.Fatalf("FindingExists: %v", err)
		}
		if exists {
			t.Error("expected false")
		}
	})
}

func TestGetFindingMock(t *testing.T) {
	t.Parallel()

	t.Run("found", func(t *testing.T) {
		s, mock := newRegexpMockStore(t)
		now := time.Now()
		taskID := int64(5)

		mock.ExpectQuery(`SELECT .+ FROM findings WHERE id`).
			WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "task_id", "node_id", "vulnclass", "name", "severity",
				"summary", "evidence", "worker", "asset_ids", "status", "report",
				"merged_into", "dismissed", "dismiss_reason", "created_at",
			}).AddRow(
				int64(1), &taskID, nil, "xss", "Reflected XSS", "high",
				"summary", "evidence", "worker", json.RawMessage("[]"), "pending", "",
				nil, false, "", now,
			))

		f, err := s.GetFinding(1)
		if err != nil {
			t.Fatalf("GetFinding: %v", err)
		}
		if f == nil {
			t.Fatal("expected non-nil finding")
		}
		if f.VulnClass != "xss" {
			t.Errorf("VulnClass = %q, want xss", f.VulnClass)
		}
		if f.TaskID == nil || *f.TaskID != 5 {
			t.Errorf("TaskID = %v, want 5", f.TaskID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		s, mock := newRegexpMockStore(t)
		mock.ExpectQuery(`SELECT .+ FROM findings WHERE id`).
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		f, err := s.GetFinding(999)
		if err != nil {
			t.Fatalf("GetFinding: %v", err)
		}
		if f != nil {
			t.Error("expected nil for not-found")
		}
	})
}

func TestSetFindingStatusMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectExec(`UPDATE findings SET status`).
		WithArgs("confirmed", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.SetFindingStatus(1, "confirmed"); err != nil {
		t.Fatalf("SetFindingStatus: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListFindingsForReportMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "vulnclass", "name", "severity", "summary", "evidence", "status"}).
		AddRow(int64(1), "xss", "Reflected XSS", "high", "XSS found", "script tag", "pending").
		AddRow(int64(2), "sqli", "SQL Injection", "critical", "SQLi found", "", "confirmed")

	mock.ExpectQuery(`SELECT f\.id.*FROM findings f.*JOIN tasks t.*WHERE t\.exploration_id`).
		WithArgs(int64(10)).
		WillReturnRows(rows)

	findings, err := s.ListFindingsForReport(ctx, 10)
	if err != nil {
		t.Fatalf("ListFindingsForReport: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
	}
	if findings[0].VulnClass != "xss" || findings[0].Name != "Reflected XSS" {
		t.Errorf("findings[0] = %+v", findings[0])
	}
	if findings[1].Evidence != "" {
		t.Errorf("findings[1].Evidence = %q, want empty", findings[1].Evidence)
	}
}

func TestBindFindingTrafficMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec(`INSERT INTO finding_traffic_bindings`).
		WithArgs(int64(1), "snap-123", "request", "login attempt", 0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.BindFindingTraffic(ctx, 1, "snap-123", "request", "login attempt", 0)
	if err != nil {
		t.Fatalf("BindFindingTraffic: %v", err)
	}
}

func TestListFindingTrafficMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "finding_id", "snapshot_id", "role", "note", "position"}).
		AddRow(int64(1), int64(5), "snap-a", "request", "initial", 0).
		AddRow(int64(2), int64(5), "snap-b", "response", "reflected", 1)

	mock.ExpectQuery(`SELECT id, finding_id, snapshot_id.*FROM finding_traffic_bindings`).
		WithArgs(int64(5)).
		WillReturnRows(rows)

	bindings, err := s.ListFindingTraffic(ctx, 5)
	if err != nil {
		t.Fatalf("ListFindingTraffic: %v", err)
	}
	if len(bindings) != 2 {
		t.Fatalf("got %d bindings, want 2", len(bindings))
	}
	if bindings[0].SnapshotID != "snap-a" || bindings[0].Role != "request" {
		t.Errorf("bindings[0] = %+v", bindings[0])
	}
	if bindings[1].Position != 1 {
		t.Errorf("bindings[1].Position = %d, want 1", bindings[1].Position)
	}
}

func TestFindingTrafficCountMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM finding_traffic_bindings`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	count, err := s.FindingTrafficCount(ctx, 5)
	if err != nil {
		t.Fatalf("FindingTrafficCount: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestDismissFindingMock(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		s, mock := newRegexpMockStore(t)
		mock.ExpectExec(`UPDATE findings SET dismissed = true`).
			WithArgs("false positive", FindingStatusFalsePositive, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := s.DismissFinding(context.Background(), 1, "false positive")
		if err != nil {
			t.Fatalf("DismissFinding: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		s, mock := newRegexpMockStore(t)
		mock.ExpectExec(`UPDATE findings SET dismissed = true`).
			WithArgs("reason", FindingStatusFalsePositive, int64(999)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := s.DismissFinding(context.Background(), 999, "reason")
		if err == nil {
			t.Fatal("expected error for not-found finding")
		}
	})
}

func TestFindPotentialDuplicatesMock(t *testing.T) {
	t.Parallel()

	t.Run("empty input", func(t *testing.T) {
		s, _ := newRegexpMockStore(t)
		ids, err := s.FindPotentialDuplicates(context.Background(), "high", nil)
		if err != nil {
			t.Fatalf("FindPotentialDuplicates: %v", err)
		}
		if ids != nil {
			t.Errorf("expected nil, got %v", ids)
		}
	})

	t.Run("with results", func(t *testing.T) {
		s, mock := newRegexpMockStore(t)
		rows := sqlmock.NewRows([]string{"id"}).AddRow(int64(10)).AddRow(int64(20))

		mock.ExpectQuery(`SELECT id FROM findings`).
			WithArgs("high", "[1,2]").
			WillReturnRows(rows)

		ids, err := s.FindPotentialDuplicates(context.Background(), "high", []int64{1, 2})
		if err != nil {
			t.Fatalf("FindPotentialDuplicates: %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("got %d ids, want 2", len(ids))
		}
		if ids[0] != 10 || ids[1] != 20 {
			t.Errorf("ids = %v, want [10 20]", ids)
		}
	})
}

func TestMergeFindingsMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()
	now := time.Now()
	taskID := int64(1)

	mock.ExpectBegin()

	mock.ExpectQuery(`SELECT .+ FROM findings WHERE id = .+ FOR UPDATE`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "node_id", "vulnclass", "name", "severity",
			"summary", "evidence", "worker", "asset_ids", "status", "report",
			"merged_into", "dismissed", "dismiss_reason", "created_at",
		}).AddRow(
			int64(10), &taskID, nil, "xss", "Primary XSS", "high",
			"summary", "primary evidence", "worker", json.RawMessage("[1]"), "pending", "",
			nil, false, "", now,
		))

	mock.ExpectQuery(`SELECT .+ FROM findings WHERE id = .+ FOR UPDATE`).
		WithArgs(int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "node_id", "vulnclass", "name", "severity",
			"summary", "evidence", "worker", "asset_ids", "status", "report",
			"merged_into", "dismissed", "dismiss_reason", "created_at",
		}).AddRow(
			int64(20), &taskID, nil, "xss", "Secondary XSS", "high",
			"summary2", "secondary evidence", "worker", json.RawMessage("[2]"), "pending", "",
			nil, false, "", now,
		))

	mock.ExpectExec(`UPDATE findings SET merged_into`).
		WithArgs(int64(10), FindingStatusDuplicate, int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec(`UPDATE findings SET asset_ids`).
		WithArgs("[1,2]", "primary evidence\n\n[merged from #20]\nsecondary evidence", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	result, err := s.MergeFindings(ctx, 10, []int64{20})
	if err != nil {
		t.Fatalf("MergeFindings: %v", err)
	}
	if result.ID != 10 {
		t.Errorf("ID = %d, want 10", result.ID)
	}
	if result.VulnClass != "xss" {
		t.Errorf("VulnClass = %q, want xss", result.VulnClass)
	}
	if len(result.AssetIDs) != 2 {
		t.Errorf("AssetIDs = %v, want [1 2]", result.AssetIDs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
