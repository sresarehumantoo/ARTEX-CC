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

func TestParseSince_Empty(t *testing.T) {
	t.Parallel()
	v, err := parseSince("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Errorf("expected empty, got %q", v)
	}
}

func TestParseSince_RFC3339(t *testing.T) {
	t.Parallel()
	v, err := parseSince("2024-01-15T10:30:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "2024-01-15 10:30:00" {
		t.Errorf("got %q, want 2024-01-15 10:30:00", v)
	}
}

func TestParseSince_Duration(t *testing.T) {
	t.Parallel()
	v, err := parseSince("1h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == "" {
		t.Fatal("expected non-empty result for duration")
	}
}

func TestParseSince_Invalid(t *testing.T) {
	t.Parallel()
	_, err := parseSince("not-a-time")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestActivityToCSV(t *testing.T) {
	t.Parallel()
	entries := []Activity{
		{ID: 1, ExplorationID: 10, Kind: "tool_use", Summary: "ran nmap", CreatedAt: "2024-01-01 00:00:00"},
		{ID: 2, ExplorationID: 10, Kind: "finding", Summary: "XSS, \"quoted\"", CreatedAt: "2024-01-01 01:00:00"},
	}
	csv := activityToCSV(entries)
	if !strings.HasPrefix(csv, "id,exploration_id,") {
		t.Errorf("expected CSV header, got: %s", csv[:40])
	}
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d", len(lines))
	}
	if !strings.Contains(lines[2], `"XSS, ""quoted"""`) {
		t.Errorf("expected quoted CSV field, got line: %s", lines[2])
	}
}

func TestListActivityCSVFormat(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{
		activities: []Activity{
			{ID: 1, Kind: "tool_use", Summary: "test"},
		},
	}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"format": "csv"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.HasPrefix(r.Content, "id,exploration_id,") {
		t.Errorf("expected CSV header, got: %s", r.Content[:30])
	}
}

func TestListActivityWithSince(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{activities: []Activity{}}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"since": "1h"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestListActivityInvalidSince(t *testing.T) {
	t.Parallel()
	ms := &activityMockStore{activities: []Activity{}}
	reg := NewRegistry()
	RegisterActivityTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"since": "not-a-time"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid since")
	}
}
