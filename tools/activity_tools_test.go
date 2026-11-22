package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type activityMockStore struct {
	mockStore
	activities []Activity
	listErr    error
	loggedID   int64
	logErr     error
}

func (m *activityMockStore) ListActivity(_ context.Context, _ int64, _ ActivityFilter) ([]Activity, error) {
	return m.activities, m.listErr
}

func (m *activityMockStore) LogActivity(_ context.Context, _ int64, _ ActivityEntry) (int64, error) {
	return m.loggedID, m.logErr
}

func TestListActivityHappyPath(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{
		activities: []Activity{
			{ID: 1, Kind: "tool_use", Summary: "ran nmap"},
			{ID: 2, Kind: "finding", Summary: "XSS found"},
		},
	}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, ok := reg.Lookup("list_activity")
	if !ok {
		t.Fatal("list_activity not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"count":2`) {
		t.Errorf("expected count 2, got: %s", r.Content)
	}
}

func TestListActivityWithFilters(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{activities: []Activity{}}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"limit": 10, "kind": "finding", "node_id": 5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestListActivityStoreError(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{listErr: fmt.Errorf("connection lost")}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestListActivityInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLogActivityHappyPath(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{loggedID: 42}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, ok := reg.Lookup("log_activity")
	if !ok {
		t.Fatal("log_activity not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"action": "tool_use",
		"summary": "ran nmap scan",
		"detail": "nmap -sV 10.0.0.1"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "activity logged: 42") {
		t.Errorf("expected logged id, got: %s", r.Content)
	}
}

func TestLogActivityMissingAction(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("log_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"summary": "test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing action")
	}
	if !strings.Contains(r.Content, "action is required") {
		t.Errorf("expected action required, got: %s", r.Content)
	}
}

func TestLogActivityStoreError(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{logErr: fmt.Errorf("insert failed")}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("log_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"action": "test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestLogActivityInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("log_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}
