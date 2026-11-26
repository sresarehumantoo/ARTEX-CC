package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestAddFindingMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO findings`).
		WithArgs(int64(1), "xss", "Reflected XSS", "high", "XSS in search", "evidence", "worker1", "[]", FindingStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectQuery(`INSERT INTO exploration_nodes`).
		WithArgs(int64(7), KindFinding, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(500))
	mock.ExpectExec(`INSERT INTO exploration_edges`).
		WithArgs(int64(7), int64(3), RelYields, int64(500)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE findings SET node_id`).
		WithArgs(int64(500), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	id, _, err := s.AddFinding(ctx, tools.FindingInput{
		TaskID:        1,
		ExplorationID: 7,
		IntentID:      3,
		VulnClass:     "xss",
		Name:          "Reflected XSS",
		Severity:      "high",
		Summary:       "XSS in search",
		Evidence:      "evidence",
		Worker:        "worker1",
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

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO findings`).
		WithArgs(int64(1), "sqli", "SQL Injection", "critical", "SQLi found", "", "", "[10,20]", FindingStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))
	mock.ExpectQuery(`INSERT INTO exploration_nodes`).
		WithArgs(int64(7), KindFinding, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(501))
	for _, asset := range []int64{10, 20} {
		mock.ExpectExec(`INSERT INTO exploration_anchors`).
			WithArgs(int64(501), asset).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec(`UPDATE findings SET node_id`).
		WithArgs(int64(501), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	id, _, err := s.AddFinding(ctx, tools.FindingInput{
		TaskID:        1,
		ExplorationID: 7,
		VulnClass:     "sqli",
		Name:          "SQL Injection",
		Severity:      "critical",
		Summary:       "SQLi found",
		AssetIDs:      []int64{10, 20},
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
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE findings SET dismissed = true`).
			WithArgs("false positive", FindingStatusFalsePositive, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE exploration_nodes SET state = 'dismissed'`).
			WithArgs(int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`SELECT g.id FROM exploration_nodes g`).
			WithArgs(int64(1), RelProves, KindGoal).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(`WITH revoked AS`).
			WithArgs(int64(1), RelProves, KindGoal).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		err := s.DismissFinding(context.Background(), 1, "false positive")
		if err != nil {
			t.Fatalf("DismissFinding: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		s, mock := newRegexpMockStore(t)
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE findings SET dismissed = true`).
			WithArgs("reason", FindingStatusFalsePositive, int64(999)).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		err := s.DismissFinding(context.Background(), 999, "reason")
		if err == nil {
			t.Fatal("expected error for not-found finding")
		}
	})
}

func TestFindPotentialDuplicatesMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	rows := sqlmock.NewRows([]string{"id", "vulnclass", "severity", "summary", "asset_ids"}).
		AddRow(int64(30), "Insecure Direct Object Reference", "medium", "IDOR on /api/orders", []byte("[1]")).
		AddRow(int64(20), "xss", "high", "XSS on asset 2", []byte("[2]")).
		AddRow(int64(10), "sqli", "low", "SQLi elsewhere", []byte("[9]"))
	mock.ExpectQuery(`SELECT .+ FROM findings f\s+JOIN tasks t`).
		WithArgs(int64(5)).
		WillReturnRows(rows)

	got, err := s.FindPotentialDuplicates(context.Background(), 5, "IDOR", "high", []int64{1, 2})
	if err != nil {
		t.Fatalf("FindPotentialDuplicates: %v", err)
	}
	if len(got) != 2 || got[0].ID != 30 || got[1].ID != 20 {
		t.Fatalf("got %+v, want findings 30 (same class) and 20 (same severity, shared asset)", got)
	}
	if got[0].Summary != "IDOR on /api/orders" {
		t.Errorf("summary not carried: %+v", got[0])
	}
}

func TestPossibleDuplicate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		vc, sev           string
		assets            []int64
		otherVC, otherSev string
		otherAssets       []int64
		want              bool
	}{
		{"same class, case and spacing differ", "IDOR", "high", []int64{1}, " idor ", "medium", []int64{1}, true},
		{"class alias", "idor", "high", []int64{1}, "Insecure Direct Object Reference", "high", []int64{1}, true},
		{"same class, different assets", "idor", "high", []int64{1}, "idor", "high", []int64{2}, false},
		{"same class, new has no assets", "idor", "high", nil, "idor", "medium", []int64{2}, true},
		{"same class, existing has no assets", "idor", "high", []int64{1}, "IDOR", "low", nil, true},
		{"different class, same severity, shared asset", "xss", "high", []int64{1, 3}, "sqli", "high", []int64{3}, true},
		{"different class, different severity, shared asset", "xss", "high", []int64{1}, "sqli", "low", []int64{1}, false},
		{"different class, no assets", "xss", "high", nil, "sqli", "high", nil, false},
		{"empty class never matches on class", "", "high", nil, "", "high", nil, false},
	}
	for _, c := range cases {
		if got := possibleDuplicate(c.vc, c.sev, c.assets, c.otherVC, c.otherSev, c.otherAssets); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMergeFindingsMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()
	now := time.Now()
	taskID := int64(1)

	mock.ExpectBegin()

	mock.ExpectQuery(`SELECT merged_into FROM findings WHERE id`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"merged_into"}).AddRow(nil))

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

	mock.ExpectExec(`SELECT n.id FROM exploration_nodes n`).
		WithArgs([]int64{20}).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT g.id FROM exploration_nodes g`).
		WithArgs([]int64{20}, RelProves, KindGoal).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`UPDATE findings SET merged_into`).
		WithArgs(int64(10), FindingStatusDuplicate, int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE exploration_nodes SET state = 'dismissed'`).
		WithArgs(int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`SELECT g.id FROM exploration_nodes g`).
		WithArgs(int64(20), RelProves, KindGoal).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`WITH revoked AS`).
		WithArgs(int64(20), RelProves, KindGoal).
		WillReturnResult(sqlmock.NewResult(0, 0))

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
