package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type scopeMockStore struct {
	mockStore
	inScope bool
	reason  string
}

func (m *scopeMockStore) IsInScope(_ context.Context, _ int64, target string) (bool, string, string, error) {
	return m.inScope, m.reason, "", nil
}

func TestCheckScopeInScope(t *testing.T) {
	t.Parallel()
	ms := &scopeMockStore{inScope: true, reason: "domain matches scope rule"}
	reg := NewRegistry()
	RegisterScopeTools(reg, ms, 1)

	tool, ok := reg.Lookup("check_scope")
	if !ok {
		t.Fatal("check_scope not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"target": "example.com"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"in_scope":true`) {
		t.Errorf("expected in_scope:true, got: %s", r.Content)
	}
}

func TestCheckScopeOutOfScope(t *testing.T) {
	t.Parallel()
	ms := &scopeMockStore{inScope: false, reason: "target does not match any scope rule"}
	reg := NewRegistry()
	RegisterScopeTools(reg, ms, 1)

	tool, _ := reg.Lookup("check_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"target": "evil.com"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success (scope check returns JSON, not error), got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"in_scope":false`) {
		t.Errorf("expected in_scope:false, got: %s", r.Content)
	}
}

func TestCheckScopeEmptyTarget(t *testing.T) {
	t.Parallel()
	ms := &scopeMockStore{}
	reg := NewRegistry()
	RegisterScopeTools(reg, ms, 1)

	tool, _ := reg.Lookup("check_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"target": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty target")
	}
	if !strings.Contains(r.Content, "target is required") {
		t.Errorf("expected 'target is required', got: %s", r.Content)
	}
}

func TestCheckScopeInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &scopeMockStore{}
	reg := NewRegistry()
	RegisterScopeTools(reg, ms, 1)

	tool, _ := reg.Lookup("check_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(r.Content, "invalid input:") {
		t.Errorf("expected 'invalid input:' prefix, got: %s", r.Content)
	}
}

func TestRecordCoverageNoTask(t *testing.T) {
	t.Parallel()
	ms := &scopeMockStore{}
	reg := NewRegistry()
	RegisterScopeTools(reg, ms, 0)

	tool, ok := reg.Lookup("record_coverage")
	if !ok {
		t.Fatal("record_coverage not found")
	}
	r, err := tool.Run(context.Background(), json.RawMessage(`{"surface":"x","risk_area":"xss","outcome":"no_issue_found"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when taskID is 0")
	}
	if !strings.Contains(r.Content, "requires a task context") {
		t.Errorf("expected 'requires a task context', got: %s", r.Content)
	}
}

func TestRecordCoverageHappyPath(t *testing.T) {
	t.Parallel()
	ms := &scopeMockStore{}
	reg := NewRegistry()
	RegisterScopeTools(reg, ms, 1)

	tool, _ := reg.Lookup("record_coverage")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"surface":"login form","risk_area":"sqli","outcome":"no_issue_found","evidence":"fact-42"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if r.Content != "recorded" {
		t.Errorf("expected 'recorded', got: %s", r.Content)
	}
}
