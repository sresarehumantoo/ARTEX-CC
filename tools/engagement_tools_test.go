package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type engagementMockStore struct {
	mockStore
	createErr    error
	engagements  []EngagementSummary
	listErr      error
	detail       *EngagementDetail
	getErr       error
	completeErr  error
	updateErr    error
	deleteErr    error
	activityList []TimelineEntry
	activityErr  error
	coverageGaps *CoverageGaps
	gapsErr      error
}

func (m *engagementMockStore) CreateEngagement(_ context.Context, _, _ string) (int64, int64, error) {
	if m.createErr != nil {
		return 0, 0, m.createErr
	}
	return 10, 20, nil
}

func (m *engagementMockStore) ListEngagements(_ context.Context) ([]EngagementSummary, error) {
	return m.engagements, m.listErr
}

func (m *engagementMockStore) GetEngagement(_ context.Context, _ int64) (*EngagementDetail, error) {
	return m.detail, m.getErr
}

func (m *engagementMockStore) CompleteEngagement(_ context.Context, _ int64, _ string) error {
	return m.completeErr
}

func (m *engagementMockStore) UpdateEngagement(_ context.Context, _ int64, _, _ *string) error {
	return m.updateErr
}

func (m *engagementMockStore) DeleteEngagement(_ context.Context, _ int64) error {
	return m.deleteErr
}

func (m *engagementMockStore) EngagementActivity(_ context.Context, _ int64, _ int) ([]TimelineEntry, error) {
	return m.activityList, m.activityErr
}

func (m *engagementMockStore) ListCoverageGaps(_ context.Context, _, _ int64) (*CoverageGaps, error) {
	return m.coverageGaps, m.gapsErr
}

func TestCreateEngagementHappyPath(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("create_engagement")
	if !ok {
		t.Fatal("create_engagement not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal": "test webapp", "description": "full scope"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"task_id":10`) {
		t.Errorf("expected task_id 10, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"exploration_id":20`) {
		t.Errorf("expected exploration_id 20, got: %s", r.Content)
	}
}

func TestCreateEngagementEmptyGoal(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("create_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty goal")
	}
	if !strings.Contains(r.Content, "goal is required") {
		t.Errorf("expected goal required, got: %s", r.Content)
	}
}

func TestCreateEngagementStoreError(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{createErr: fmt.Errorf("db full")}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("create_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal": "test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestCreateEngagementInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("create_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestListEngagementsHappyPath(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{
		engagements: []EngagementSummary{
			{ID: 1, Goal: "web pentest", Status: "active", ExplorationID: 10},
			{ID: 2, Goal: "api review", Status: "done", ExplorationID: 20},
		},
	}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("list_engagements")
	if !ok {
		t.Fatal("list_engagements not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"web pentest"`) {
		t.Errorf("expected engagement goal, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"active":true`) {
		t.Errorf("expected active flag for current task, got: %s", r.Content)
	}
}

func TestListEngagementsStoreError(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{listErr: fmt.Errorf("timeout")}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("list_engagements")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestGoalMetHappyPath(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("goal_met")
	if !ok {
		t.Fatal("goal_met not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"summary": "all vulns found"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "marked complete") {
		t.Errorf("expected completion message, got: %s", r.Content)
	}
}

func TestGoalMetNoTask(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 0, 1)

	tool, _ := reg.Lookup("goal_met")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when taskID is 0")
	}
	if !strings.Contains(r.Content, "requires a task context") {
		t.Errorf("expected task context error, got: %s", r.Content)
	}
}

func TestGoalMetStoreError(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{completeErr: fmt.Errorf("already complete")}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("goal_met")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestUpdateEngagementHappyPath(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("update_engagement")
	if !ok {
		t.Fatal("update_engagement not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal": "updated goal"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "updated") {
		t.Errorf("expected update confirmation, got: %s", r.Content)
	}
}

func TestUpdateEngagementBothFields(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("update_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal": "new goal", "description": "new desc"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "goal") || !strings.Contains(r.Content, "description") {
		t.Errorf("expected both fields in confirmation, got: %s", r.Content)
	}
}

func TestUpdateEngagementNoFields(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("update_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when no fields provided")
	}
	if !strings.Contains(r.Content, "at least one") {
		t.Errorf("expected validation error, got: %s", r.Content)
	}
}

func TestUpdateEngagementEmptyGoal(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("update_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty goal")
	}
	if !strings.Contains(r.Content, "goal cannot be empty") {
		t.Errorf("expected empty goal error, got: %s", r.Content)
	}
}

func TestUpdateEngagementNoTask(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 0, 1)

	tool, _ := reg.Lookup("update_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal": "test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when taskID is 0")
	}
}

func TestUpdateEngagementStoreError(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{updateErr: fmt.Errorf("not found")}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("update_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal": "test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestDeleteEngagementHappyPath(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("delete_engagement")
	if !ok {
		t.Fatal("delete_engagement not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"task_id": 5, "confirm": true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "deleted") {
		t.Errorf("expected deletion confirmation, got: %s", r.Content)
	}
}

func TestDeleteEngagementNoConfirm(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("delete_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"task_id": 5, "confirm": false}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error without confirm")
	}
	if !strings.Contains(r.Content, "confirm must be true") {
		t.Errorf("expected confirm error, got: %s", r.Content)
	}
}

func TestDeleteEngagementMissingTaskID(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("delete_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"confirm": true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing task_id")
	}
	if !strings.Contains(r.Content, "task_id is required") {
		t.Errorf("expected task_id required, got: %s", r.Content)
	}
}

func TestDeleteEngagementStoreError(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{deleteErr: fmt.Errorf("foreign key")}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("delete_engagement")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"task_id": 5, "confirm": true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestEngagementActivityHappyPath(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{
		activityList: []TimelineEntry{
			{Timestamp: "2024-01-01T00:00:00Z", Kind: "finding", Summary: "XSS found"},
		},
	}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("engagement_activity")
	if !ok {
		t.Fatal("engagement_activity not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "XSS found") {
		t.Errorf("expected activity entry, got: %s", r.Content)
	}
}

func TestEngagementActivityEmpty(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{activityList: []TimelineEntry{}}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("engagement_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if r.Content != "no activity yet" {
		t.Errorf("expected 'no activity yet', got: %s", r.Content)
	}
}

func TestEngagementActivityNoContext(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 0)

	tool, _ := reg.Lookup("engagement_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when explorationID is 0")
	}
}

func TestEngagementActivityStoreError(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{activityErr: fmt.Errorf("timeout")}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("engagement_activity")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestEngagementStatusHappyPath(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("engagement_status")
	if !ok {
		t.Fatal("engagement_status not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"task_id":1`) {
		t.Errorf("expected task_id, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"findings_total"`) {
		t.Errorf("expected findings_total, got: %s", r.Content)
	}
}

func TestListCoverageGapsHappyPath(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{
		coverageGaps: &CoverageGaps{
			UntargetedAssets: []Asset{{ID: 1, Label: "test.com"}},
		},
	}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("list_coverage_gaps")
	if !ok {
		t.Fatal("list_coverage_gaps not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"test.com"`) {
		t.Errorf("expected untargeted asset, got: %s", r.Content)
	}
}

func TestListCoverageGapsNoContext(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 0, 0)

	tool, _ := reg.Lookup("list_coverage_gaps")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when no engagement context")
	}
}

func TestListCoverageGapsStoreError(t *testing.T) {
	t.Parallel()
	ms := &engagementMockStore{gapsErr: fmt.Errorf("query failed")}
	reg := NewRegistry()
	RegisterEngagementTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("list_coverage_gaps")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}
