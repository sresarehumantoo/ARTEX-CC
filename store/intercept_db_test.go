package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func interceptRuleCols14() []string {
	return []string{"id", "name", "enabled", "priority", "match_target", "match_type", "pattern", "action", "message", "timeout_enabled", "timeout_seconds", "timeout_action", "created_at", "updated_at"}
}

func TestListInterceptRulesDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	rows := sqlmock.NewRows(interceptRuleCols14()).
		AddRow(int64(1), "block shell", true, 100, "tool_name", "string", "shell", "deny", "blocked", true, 30, "deny", now, now).
		AddRow(int64(2), "allow read", true, 50, "tool_name", "string", "read", "allow", "", false, 0, "", now, now)

	mock.ExpectQuery("SELECT .+ FROM intercept_rules").WillReturnRows(rows)

	rules, err := s.ListInterceptRules()
	if err != nil {
		t.Fatalf("ListInterceptRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].Name != "block shell" {
		t.Errorf("Name = %q, want %q", rules[0].Name, "block shell")
	}
	if rules[0].Action != "deny" {
		t.Errorf("Action = %q, want %q", rules[0].Action, "deny")
	}
	if !rules[0].TimeoutEnabled {
		t.Error("TimeoutEnabled = false, want true")
	}
}

func TestListInterceptRulesEmpty(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectQuery("SELECT .+ FROM intercept_rules").
		WillReturnRows(sqlmock.NewRows(interceptRuleCols14()))

	rules, err := s.ListInterceptRules()
	if err != nil {
		t.Fatalf("ListInterceptRules: %v", err)
	}
	if rules != nil {
		t.Errorf("expected nil, got %d rules", len(rules))
	}
}

func TestAddInterceptRuleDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	mock.ExpectQuery("INSERT INTO intercept_rules").
		WithArgs("test rule", true, 10, "tool_name", "string", "shell", "deny", "blocked",
			false, 0, "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(int64(5), now, now))

	r := &InterceptRule{
		Name:        "test rule",
		Enabled:     true,
		Priority:    10,
		MatchTarget: "tool_name",
		MatchType:   "string",
		Pattern:     "shell",
		Action:      "deny",
		Message:     "blocked",
	}
	if err := s.AddInterceptRule(r); err != nil {
		t.Fatalf("AddInterceptRule: %v", err)
	}
	if r.ID != 5 {
		t.Errorf("ID = %d, want 5", r.ID)
	}
}

func TestDeleteInterceptRuleDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectExec("DELETE FROM intercept_rules").
		WithArgs(int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.DeleteInterceptRule(3); err != nil {
		t.Fatalf("DeleteInterceptRule: %v", err)
	}
}

func TestCheckInterceptNoRules(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectQuery("SELECT .+ FROM intercept_rules").
		WillReturnRows(sqlmock.NewRows(interceptRuleCols14()))

	p, err := s.CheckIntercept("some_tool", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CheckIntercept: %v", err)
	}
	if p != nil {
		t.Error("expected nil pending, got non-nil")
	}
}

func TestCheckInterceptAllowRule(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM intercept_rules").
		WillReturnRows(sqlmock.NewRows(interceptRuleCols14()).
			AddRow(int64(1), "allow", true, 100, "tool_name", "string", "read", "allow", "", false, 0, "", now, now))

	p, err := s.CheckIntercept("read_file", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CheckIntercept: %v", err)
	}
	if p != nil {
		t.Error("expected nil for allow rule")
	}
}

func TestCheckInterceptDenyRule(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	ruleID := int64(1)
	mock.ExpectQuery("SELECT .+ FROM intercept_rules").
		WillReturnRows(sqlmock.NewRows(interceptRuleCols14()).
			AddRow(ruleID, "deny shell", true, 100, "tool_name", "string", "shell", "deny", "not allowed", false, 0, "", now, now))

	mock.ExpectQuery("INSERT INTO intercept_pending").
		WithArgs(ruleID, "run_shell", json.RawMessage(`{"cmd":"ls"}`), "denied", "not allowed").
		WillReturnRows(sqlmock.NewRows([]string{"id", "rule_id", "tool_name", "tool_input", "status", "reason", "decided_at", "created_at"}).
			AddRow(int64(10), &ruleID, "run_shell", json.RawMessage(`{"cmd":"ls"}`), "denied", "not allowed", (*time.Time)(nil), now))

	mock.ExpectExec("UPDATE intercept_pending").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	p, err := s.CheckIntercept("run_shell", json.RawMessage(`{"cmd":"ls"}`))
	if err != nil {
		t.Fatalf("CheckIntercept: %v", err)
	}
	if p == nil {
		t.Fatal("expected pending, got nil")
	}
	if p.Status != "denied" {
		t.Errorf("Status = %q, want %q", p.Status, "denied")
	}
}

func TestCheckInterceptPauseRule(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	ruleID := int64(1)
	mock.ExpectQuery("SELECT .+ FROM intercept_rules").
		WillReturnRows(sqlmock.NewRows(interceptRuleCols14()).
			AddRow(ruleID, "pause shell", true, 100, "tool_name", "string", "shell", "pause", "needs review", false, 0, "", now, now))

	mock.ExpectQuery("INSERT INTO intercept_pending").
		WithArgs(ruleID, "run_shell", json.RawMessage(`{}`), "pending", "needs review").
		WillReturnRows(sqlmock.NewRows([]string{"id", "rule_id", "tool_name", "tool_input", "status", "reason", "decided_at", "created_at"}).
			AddRow(int64(11), &ruleID, "run_shell", json.RawMessage(`{}`), "pending", "needs review", (*time.Time)(nil), now))

	p, err := s.CheckIntercept("run_shell", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CheckIntercept: %v", err)
	}
	if p == nil {
		t.Fatal("expected pending, got nil")
	}
	if p.Status != "pending" {
		t.Errorf("Status = %q, want %q", p.Status, "pending")
	}
}

func TestCheckInterceptDisabledRule(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM intercept_rules").
		WillReturnRows(sqlmock.NewRows(interceptRuleCols14()).
			AddRow(int64(1), "disabled", false, 100, "tool_name", "string", "shell", "deny", "blocked", false, 0, "", now, now))

	p, err := s.CheckIntercept("run_shell", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CheckIntercept: %v", err)
	}
	if p != nil {
		t.Error("expected nil for disabled rule")
	}
}

func TestCheckInterceptEmptyInput(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	ruleID := int64(1)
	mock.ExpectQuery("SELECT .+ FROM intercept_rules").
		WillReturnRows(sqlmock.NewRows(interceptRuleCols14()).
			AddRow(ruleID, "pause all", true, 100, "tool_name", "string", "tool", "pause", "review", false, 0, "", now, now))

	mock.ExpectQuery("INSERT INTO intercept_pending").
		WithArgs(ruleID, "my_tool", json.RawMessage("{}"), "pending", "review").
		WillReturnRows(sqlmock.NewRows([]string{"id", "rule_id", "tool_name", "tool_input", "status", "reason", "decided_at", "created_at"}).
			AddRow(int64(20), &ruleID, "my_tool", json.RawMessage("{}"), "pending", "review", (*time.Time)(nil), now))

	p, err := s.CheckIntercept("my_tool", nil)
	if err != nil {
		t.Fatalf("CheckIntercept: %v", err)
	}
	if p == nil {
		t.Fatal("expected pending")
	}
}

func TestDecideInterceptDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectExec("UPDATE intercept_pending").
		WithArgs(int64(5), "approved").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.DecideIntercept(5, "approved"); err != nil {
		t.Fatalf("DecideIntercept: %v", err)
	}
}
