package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestRecordCoverageValidationErrors(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		taskID  int64
		entry   tools.CoverageEntry
		wantErr string
	}{
		{"zero task_id", 0, tools.CoverageEntry{Surface: "s", RiskArea: "r", Outcome: OutcomeReported}, "task_id"},
		{"empty surface", 1, tools.CoverageEntry{RiskArea: "r", Outcome: OutcomeReported}, "surface"},
		{"empty risk_area", 1, tools.CoverageEntry{Surface: "s", Outcome: OutcomeReported}, "risk_area"},
		{"bad outcome", 1, tools.CoverageEntry{Surface: "s", RiskArea: "r", Outcome: "invalid"}, "outcome"},
		{"surface too long", 1, tools.CoverageEntry{Surface: strings.Repeat("x", 201), RiskArea: "r", Outcome: OutcomeReported}, "too long"},
		{"risk_area too long", 1, tools.CoverageEntry{Surface: "s", RiskArea: strings.Repeat("x", 65), Outcome: OutcomeReported}, "too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.RecordCoverage(ctx, tt.taskID, tt.entry)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRecordCoverageSuccess(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("SELECT 1 FROM tasks").
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(int64(1), "login form", "authentication").
		WillReturnRows(sqlmock.NewRows([]string{"exists", "count"}).AddRow(false, 0))
	mock.ExpectExec("INSERT INTO coverage_ledger_history").
		WithArgs(int64(1), "login form", "authentication", OutcomeReported, "brute force", "no lockout", "worker1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO coverage_ledger").
		WithArgs(int64(1), "login form", "authentication", OutcomeReported, "brute force", "no lockout", "worker1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
		Surface:  "login form",
		RiskArea: "Authentication",
		Outcome:  OutcomeReported,
		Evidence: "brute force",
		Reason:   "no lockout",
		Worker:   "worker1",
	})
	if err != nil {
		t.Fatalf("RecordCoverage: %v", err)
	}
}

func TestRecordCoverageLedgerFull(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("SELECT 1 FROM tasks").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT EXISTS").
		WillReturnRows(sqlmock.NewRows([]string{"exists", "count"}).AddRow(false, 2000))
	mock.ExpectRollback()

	err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
		Surface:  "new surface",
		RiskArea: "new area",
		Outcome:  OutcomeReported,
	})
	if err == nil {
		t.Fatal("expected error for full ledger")
	}
	if !strings.Contains(err.Error(), "full") {
		t.Errorf("error = %q, want to contain 'full'", err.Error())
	}
}

func TestCoverageStatsDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	ledgerRows := sqlmock.NewRows([]string{"surface", "risk_area", "outcome", "evidence", "reason", "worker", "updated_at"}).
		AddRow("login", "authentication", OutcomeReported, "brute force", "no lockout", "w1", now).
		AddRow("api", "injection", OutcomeNeedsFollowUp, "", "", "w2", now).
		AddRow("upload", "file_upload", OutcomeNoIssueFound, "", "", "w3", now)

	mock.ExpectQuery("SELECT surface, risk_area, outcome").
		WithArgs(int64(1)).
		WillReturnRows(ledgerRows)

	classRows := sqlmock.NewRows([]string{"vulnclass"}).
		AddRow("xss").
		AddRow("authentication")

	mock.ExpectQuery("SELECT DISTINCT").
		WithArgs(int64(1)).
		WillReturnRows(classRows)

	stats, err := s.CoverageStats(1)
	if err != nil {
		t.Fatalf("CoverageStats: %v", err)
	}
	if len(stats.Entries) != 3 {
		t.Errorf("Entries = %d, want 3", len(stats.Entries))
	}
	if len(stats.Unexamined) != 1 {
		t.Errorf("Unexamined = %v, want [xss]", stats.Unexamined)
	} else if stats.Unexamined[0] != "xss" {
		t.Errorf("Unexamined[0] = %q, want %q", stats.Unexamined[0], "xss")
	}

	gapCount := 0
	for _, g := range stats.Gaps {
		if g.Kind == GapNeedsFollowUp {
			gapCount++
		}
		if g.Kind == GapMissingEvidence {
			gapCount++
		}
	}
	if gapCount < 2 {
		t.Errorf("expected at least 2 gaps (needs_follow_up + missing_evidence), got gaps: %v", stats.Gaps)
	}
}

func TestCoverageStatsEmpty(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectQuery("SELECT surface, risk_area, outcome").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"surface", "risk_area", "outcome", "evidence", "reason", "worker", "updated_at"}))

	mock.ExpectQuery("SELECT DISTINCT").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"vulnclass"}))

	stats, err := s.CoverageStats(1)
	if err != nil {
		t.Fatalf("CoverageStats: %v", err)
	}
	if len(stats.Entries) != 0 {
		t.Errorf("Entries = %d, want 0", len(stats.Entries))
	}
}

func TestCoverageStatsMissingReason(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	ledgerRows := sqlmock.NewRows([]string{"surface", "risk_area", "outcome", "evidence", "reason", "worker", "updated_at"}).
		AddRow("test", "area", OutcomeRuledOut, "some evidence", "", "w1", now)

	mock.ExpectQuery("SELECT surface, risk_area, outcome").
		WithArgs(int64(1)).
		WillReturnRows(ledgerRows)

	mock.ExpectQuery("SELECT DISTINCT").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"vulnclass"}))

	stats, err := s.CoverageStats(1)
	if err != nil {
		t.Fatalf("CoverageStats: %v", err)
	}
	found := false
	for _, g := range stats.Gaps {
		if g.Kind == GapMissingReason {
			found = true
		}
	}
	if !found {
		t.Error("expected missing_reason gap")
	}
}
