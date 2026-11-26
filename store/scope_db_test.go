package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestListScopeRulesDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectQuery(`FROM task_scope ts(.|\n)*UNION ALL(.|\n)*FROM company_scope cs`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "value", "reason"}).
			AddRow("root_domain", "example.com", "contract").
			AddRow("cidr", "10.0.0.0/24", ""))

	got, err := s.ListScopeRules(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListScopeRules: %v", err)
	}
	want := []tools.ScopeRule{
		{Kind: "root_domain", Value: "example.com", Reason: "contract"},
		{Kind: "cidr", Value: "10.0.0.0/24"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rules, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rule %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestListTaskScopeDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(int64(1)).
		AddRow(int64(2)).
		AddRow(int64(3))

	mock.ExpectQuery("SELECT id FROM task_scope").
		WithArgs(int64(10)).
		WillReturnRows(rows)

	ids, err := s.ListTaskScope(ctx, 10)
	if err != nil {
		t.Fatalf("ListTaskScope: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("got %d ids, want 3", len(ids))
	}
	if ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("ids = %v, want [1 2 3]", ids)
	}
}

func TestListTaskScopeEmpty(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id FROM task_scope").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ids, err := s.ListTaskScope(ctx, 10)
	if err != nil {
		t.Fatalf("ListTaskScope: %v", err)
	}
	if ids != nil {
		t.Errorf("expected nil, got %v", ids)
	}
}

func TestDeleteTaskScopeDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectExec("DELETE FROM task_scope").
		WithArgs(int64(5), int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.DeleteTaskScope(5, 10); err != nil {
		t.Fatalf("DeleteTaskScope: %v", err)
	}
}

func TestDeleteTaskScopeNotFound(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectExec("DELETE FROM task_scope").
		WithArgs(int64(999), int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := s.DeleteTaskScope(999, 10)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestIsInScopeNoTaskID(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	inScope, reason, _, err := s.IsInScope(ctx, 0, "example.com")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if !inScope {
		t.Error("expected in scope (permissive default)")
	}
	if reason != "no scope rules defined (permissive default)" {
		t.Errorf("reason = %q", reason)
	}
}

func TestIsInScopeEmptyTargetMock(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	inScope, reason, _, err := s.IsInScope(ctx, 0, "")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if inScope {
		t.Error("expected not in scope for empty target")
	}
	if reason != "could not parse target" {
		t.Errorf("reason = %q", reason)
	}
}

func TestIsInScopeDomainMatch(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT kind").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}).
			AddRow("root_domain", "example.com", ""))

	mock.ExpectQuery("FROM company_scope").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}))

	inScope, reason, _, err := s.IsInScope(ctx, 1, "sub.example.com")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if !inScope {
		t.Error("expected in scope for matching domain")
	}
	if reason != "domain matches scope rule" {
		t.Errorf("reason = %q", reason)
	}
}

func TestIsInScopeIPMatch(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT kind").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}).
			AddRow("cidr", "", "10.0.0.0/24"))

	mock.ExpectQuery("FROM company_scope").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}))

	inScope, reason, _, err := s.IsInScope(ctx, 1, "10.0.0.5")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if !inScope {
		t.Error("expected in scope for matching IP")
	}
	if reason != "IP matches CIDR rule" {
		t.Errorf("reason = %q", reason)
	}
}

func TestIsInScopeNoMatch(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT kind").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}).
			AddRow("root_domain", "example.com", ""))

	mock.ExpectQuery("FROM company_scope").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}))

	mock.ExpectQuery("SELECT c.name FROM companies").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"name"}))

	inScope, reason, _, err := s.IsInScope(ctx, 1, "other.org")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if inScope {
		t.Error("expected not in scope")
	}
	if reason != "target does not match any scope rule" {
		t.Errorf("reason = %q", reason)
	}
}

func TestLoadTaskScopeRulesDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT kind").
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}).
			AddRow("root_domain", "example.com", "").
			AddRow("cidr", "", "10.0.0.0/8"))

	rules, err := s.loadTaskScopeRules(ctx, 5)
	if err != nil {
		t.Fatalf("loadTaskScopeRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].domain != "example.com" {
		t.Errorf("domain = %q, want %q", rules[0].domain, "example.com")
	}
	if rules[1].net == nil {
		t.Fatal("net is nil, expected non-nil")
	}
}

func TestIsInScopeCompanyWithoutHostScopeFailsClosed(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("FROM task_scope WHERE task_id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}).
			AddRow("company", "", ""))

	mock.ExpectQuery("FROM company_scope").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}).
			AddRow("keyword", "", ""))

	mock.ExpectQuery("SELECT c.name FROM companies").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Acme"))

	inScope, reason, _, err := s.IsInScope(ctx, 1, "anything.example")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if inScope {
		t.Error("expected not in scope when a company entry has no host scope")
	}
	if want := "target does not match any scope rule; company Acme has no domain/IP/CIDR scope, add it with add_company_scope"; reason != want {
		t.Errorf("reason = %q, want %q", reason, want)
	}
}

func TestLoadCompanyScopeRulesDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM company_scope WHERE company_id IN \((.|\n)*kind = 'company'(.|\n)*FROM tasks`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "domain", "net"}).
			AddRow("domain", "corp.example.com", "").
			AddRow("keyword", "", ""))

	rules, err := s.loadCompanyScopeRules(ctx, 5)
	if err != nil {
		t.Fatalf("loadCompanyScopeRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	if rules[0].domain != "corp.example.com" {
		t.Errorf("domain = %q", rules[0].domain)
	}
}

func TestAddTaskScopeValidationErrors(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	if err := s.AddTaskScope(ctx, 0, "root_domain", "example.com", "", ""); err == nil {
		t.Error("expected error for taskID 0")
	}

	if err := s.AddTaskScope(ctx, 1, "root_domain", "", "", ""); err == nil {
		t.Error("expected error for empty value")
	}

	if err := s.AddTaskScope(ctx, 1, "unknown_kind", "value", "", ""); err == nil {
		t.Error("expected error for unknown kind")
	}

	if err := s.AddTaskScope(ctx, 1, "ip", "not-an-ip", "", ""); err == nil {
		t.Error("expected error for invalid IP")
	}

	if err := s.AddTaskScope(ctx, 1, "cidr", "not-cidr", "", ""); err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestAddTaskScopeRootDomain(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO task_scope").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.AddTaskScope(ctx, 1, "root_domain", "example.com", "recon", "agent"); err != nil {
		t.Fatalf("AddTaskScope: %v", err)
	}
}

func TestAddTaskScopeSubdomain(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO task_scope").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.AddTaskScope(ctx, 1, "subdomain", "sub.example.com", "", "agent"); err != nil {
		t.Fatalf("AddTaskScope: %v", err)
	}
}

func TestAddTaskScopeIP(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO task_scope").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.AddTaskScope(ctx, 1, "ip", "10.0.0.1", "", "agent"); err != nil {
		t.Fatalf("AddTaskScope: %v", err)
	}
}

func TestAddTaskScopeCIDR(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO task_scope").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.AddTaskScope(ctx, 1, "cidr", "10.0.0.0/24", "", "agent"); err != nil {
		t.Fatalf("AddTaskScope: %v", err)
	}
}

func TestAddTaskScopeKeyword(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO task_scope").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.AddTaskScope(ctx, 1, "keyword", "some keyword", "", "manual"); err != nil {
		t.Fatalf("AddTaskScope: %v", err)
	}
}

func TestAddTaskScopeCompanyByID(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id FROM companies WHERE id").
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(5)))

	mock.ExpectExec("INSERT INTO task_scope").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.AddTaskScope(ctx, 1, "company", "5", "", "agent"); err != nil {
		t.Fatalf("AddTaskScope: %v", err)
	}
}

func TestAddTaskScopeCompanyByName(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id FROM companies WHERE nkey").
		WithArgs("acme corp").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

	mock.ExpectExec("INSERT INTO task_scope").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.AddTaskScope(ctx, 1, "company", "Acme Corp", "", "agent"); err != nil {
		t.Fatalf("AddTaskScope: %v", err)
	}
}

func TestAddTaskScopeCompanyNotFound(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id FROM companies WHERE id").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	err := s.AddTaskScope(ctx, 1, "company", "999", "", "agent")
	if err == nil {
		t.Fatal("expected error for company not found")
	}
}
