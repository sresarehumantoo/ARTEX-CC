package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestListCompaniesDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "name", "asset_count", "scope"}).
		AddRow(int64(1), "Acme Corp", 42, `{example.com,"10.0.0.0/24"}`).
		AddRow(int64(2), "Beta Inc", 0, "{}")

	mock.ExpectQuery("SELECT .+ FROM companies").WillReturnRows(rows)

	companies, err := s.ListCompanies(ctx)
	if err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	if len(companies) != 2 {
		t.Fatalf("got %d companies, want 2", len(companies))
	}
	if companies[0].Name != "Acme Corp" {
		t.Errorf("Name = %q, want %q", companies[0].Name, "Acme Corp")
	}
	if companies[0].AssetCount != 42 {
		t.Errorf("AssetCount = %d, want 42", companies[0].AssetCount)
	}
	if len(companies[0].Scope) != 2 {
		t.Errorf("Scope len = %d, want 2", len(companies[0].Scope))
	}
	if len(companies[1].Scope) != 0 {
		t.Errorf("Scope len = %d, want 0", len(companies[1].Scope))
	}
}

func TestListCompaniesEmpty(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .+ FROM companies").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "asset_count", "scope"}))

	companies, err := s.ListCompanies(ctx)
	if err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	if companies != nil {
		t.Errorf("got %d companies, want nil", len(companies))
	}
}

func TestGetOrCreateCompanyDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO companies").
		WithArgs("Test Co", "test co", "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id FROM companies").
		WithArgs("test co").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

	id, err := s.GetOrCreateCompany(ctx, "Test Co", "")
	if err != nil {
		t.Fatalf("GetOrCreateCompany: %v", err)
	}
	if id != 7 {
		t.Errorf("id = %d, want 7", id)
	}
}

func TestGetOrCreateCompanyEmptyName(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	_, err := s.GetOrCreateCompany(ctx, "", "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetOrCreateCompanyWhitespace(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	_, err := s.GetOrCreateCompany(ctx, "   ", "")
	if err == nil {
		t.Fatal("expected error for whitespace-only name")
	}
}

func TestAddCompanyScopeDomain(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO company_scope").
		WithArgs(int64(1), "domain", "example.com", "example.com", "recon").
		WillReturnResult(sqlmock.NewResult(0, 1))

	added, _, invalid, errs, err := s.AddCompanyScope(ctx, 1, []string{"example.com"}, "recon")
	if err != nil {
		t.Fatalf("AddCompanyScope: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	if invalid != 0 {
		t.Errorf("invalid = %d, want 0", invalid)
	}
	if len(errs) != 0 {
		t.Errorf("errors = %v", errs)
	}
}

func TestAddCompanyScopeIP(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO company_scope").
		WithArgs(int64(1), "ip", "10.0.0.1/32", "10.0.0.1", "recon").
		WillReturnResult(sqlmock.NewResult(0, 1))

	added, _, invalid, _, err := s.AddCompanyScope(ctx, 1, []string{"10.0.0.1"}, "recon")
	if err != nil {
		t.Fatalf("AddCompanyScope: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	if invalid != 0 {
		t.Errorf("invalid = %d, want 0", invalid)
	}
}

func TestAddCompanyScopeCIDR(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO company_scope").
		WithArgs(int64(1), "cidr", "10.0.0.0/24", "10.0.0.0/24", "net").
		WillReturnResult(sqlmock.NewResult(0, 1))

	added, _, _, _, err := s.AddCompanyScope(ctx, 1, []string{"10.0.0.0/24"}, "net")
	if err != nil {
		t.Fatalf("AddCompanyScope: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
}

func TestAddCompanyScopeSkipsEmptyLines(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	added, _, _, _, err := s.AddCompanyScope(ctx, 1, []string{"", "  ", ""}, "test")
	if err != nil {
		t.Fatalf("AddCompanyScope: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0", added)
	}
}

func TestListCompanyScopeDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "company_id", "kind", "domain", "net", "value", "raw", "reason", "created_at"}).
		AddRow(int64(1), int64(5), "domain", "example.com", "", "", "example.com", "recon", now).
		AddRow(int64(2), int64(5), "ip", "", "10.0.0.1/32", "", "10.0.0.1", "", now)

	mock.ExpectQuery("SELECT .+ FROM company_scope").
		WithArgs(int64(5)).
		WillReturnRows(rows)

	rules, err := s.ListCompanyScope(5)
	if err != nil {
		t.Fatalf("ListCompanyScope: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].Kind != "domain" {
		t.Errorf("Kind = %q, want %q", rules[0].Kind, "domain")
	}
	if rules[0].Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", rules[0].Domain, "example.com")
	}
	if rules[1].Kind != "ip" {
		t.Errorf("Kind = %q, want %q", rules[1].Kind, "ip")
	}
	if rules[1].Net != "10.0.0.1/32" {
		t.Errorf("Net = %q, want %q", rules[1].Net, "10.0.0.1/32")
	}
}
