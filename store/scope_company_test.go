package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func addScopedCompany(t *testing.T, s *Store, name string, lines ...string) int64 {
	t.Helper()
	ctx := context.Background()
	companyID, err := s.GetOrCreateCompany(ctx, name, "")
	if err != nil {
		t.Fatalf("GetOrCreateCompany: %v", err)
	}
	if _, _, _, _, err := s.AddCompanyScope(ctx, companyID, lines, "test"); err != nil {
		t.Fatalf("AddCompanyScope: %v", err)
	}
	return companyID
}

func assertInScope(t *testing.T, s *Store, taskID int64, target string, want bool) string {
	t.Helper()
	got, reason, _, err := s.IsInScope(context.Background(), taskID, target)
	if err != nil {
		t.Fatalf("IsInScope(%q): %v", target, err)
	}
	if got != want {
		t.Errorf("IsInScope(%q) = %v (%s), want %v", target, got, reason, want)
	}
	return reason
}

func TestIsInScopeTaskCompanyEntry(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "scope-company-entry")
	name := fmt.Sprintf("scope-company-entry-%d", taskID)
	addScopedCompany(t, s, name, "company-entry.example", "10.20.0.0/16", "acme keyword")

	if err := s.AddTaskScope(ctx, taskID, "company", name, "", "agent"); err != nil {
		t.Fatalf("AddTaskScope company: %v", err)
	}

	assertInScope(t, s, taskID, "api.company-entry.example", true)
	assertInScope(t, s, taskID, "https://company-entry.example/login", true)
	assertInScope(t, s, taskID, "10.20.3.4", true)
	assertInScope(t, s, taskID, "other.example", false)
	assertInScope(t, s, taskID, "10.21.0.1", false)
}

func TestIsInScopeTaskCompanyEntryWithoutScopeFailsClosed(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "scope-company-empty")
	name := fmt.Sprintf("scope-company-empty-%d", taskID)
	addScopedCompany(t, s, name, "only a keyword")

	if err := s.AddTaskScope(ctx, taskID, "company", name, "", "agent"); err != nil {
		t.Fatalf("AddTaskScope company: %v", err)
	}

	reason := assertInScope(t, s, taskID, "anything.example", false)
	if !strings.Contains(reason, name) || !strings.Contains(reason, "add_company_scope") {
		t.Errorf("reason %q should name the company and add_company_scope", reason)
	}

	if err := s.AddTaskScope(ctx, taskID, "root_domain", "company-empty-extra.example", "", "agent"); err != nil {
		t.Fatalf("AddTaskScope root_domain: %v", err)
	}
	assertInScope(t, s, taskID, "www.company-empty-extra.example", true)
	assertInScope(t, s, taskID, "anything.example", false)
}

func TestIsInScopeTaskCompanyID(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	companyID := addScopedCompany(t, s, fmt.Sprintf("scope-task-company-%d", time.Now().UnixNano()), "task-company.example")
	task, err := s.CreateTask(ctx, "scope task company", "desc", "goal", &companyID)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	assertInScope(t, s, task.ID, "a.task-company.example", true)
	assertInScope(t, s, task.ID, "elsewhere.example", false)
}

func TestListScopeRulesExpandsCompanyEntries(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "scope-rules-company")
	name := fmt.Sprintf("scope-rules-company-%d", taskID)
	addScopedCompany(t, s, name, "rules-company.example", "10.30.0.0/24")

	if err := s.AddTaskScope(ctx, taskID, "root_domain", "rules-own.example", "", "agent"); err != nil {
		t.Fatalf("AddTaskScope root_domain: %v", err)
	}
	if err := s.AddTaskScope(ctx, taskID, "company", name, "", "agent"); err != nil {
		t.Fatalf("AddTaskScope company: %v", err)
	}

	rules, err := s.ListScopeRules(ctx, taskID)
	if err != nil {
		t.Fatalf("ListScopeRules: %v", err)
	}
	want := []tools.ScopeRule{
		{Kind: "root_domain", Value: "rules-own.example"},
		{Kind: "company", Value: name},
		{Kind: "domain", Value: "rules-company.example", Reason: "test"},
		{Kind: "cidr", Value: "10.30.0.0/24", Reason: "test"},
	}
	if len(rules) != len(want) {
		t.Fatalf("rules = %+v, want %+v", rules, want)
	}
	for i := range want {
		if rules[i] != want[i] {
			t.Errorf("rule %d = %+v, want %+v", i, rules[i], want[i])
		}
	}
}
