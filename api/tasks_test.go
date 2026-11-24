package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

var errTestTask = errors.New("test error")

type mockTaskStore struct {
	mockCoverageStore
	task    *store.Task
	taskErr error
}

func (m *mockTaskStore) GetTask(id int64) (*store.Task, error) {
	return m.task, m.taskErr
}

func TestHandleTasks_OK(t *testing.T) {
	ms := &mockStore{
		graphOverview:   &tools.Overview{FindingsTotal: 5},
		rootGoal:        "hack the planet",
		listAssets:      []tools.Asset{{ID: 1}},
		listAssetsTotal: 1,
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var tasks []taskSummary
	json.NewDecoder(rec.Body).Decode(&tasks)
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "hack the planet" || tasks[0].Stats.Findings != 5 || tasks[0].Stats.Assets != 1 {
		t.Fatalf("task: %+v", tasks[0])
	}
	if tasks[0].Status != "running" {
		t.Errorf("status: want running, got %s", tasks[0].Status)
	}
}

func TestHandleTasks_GraphOverviewError(t *testing.T) {
	ms := &mockStore{graphOverviewErr: errTestTask}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleTasks_AssetsError(t *testing.T) {
	ms := &mockStore{
		graphOverview: &tools.Overview{},
		listAssetsErr: errTestTask,
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleTasks_WithTaskGetter(t *testing.T) {
	ms := &mockTaskStore{
		mockCoverageStore: mockCoverageStore{
			mockStore: mockStore{
				graphOverview:   &tools.Overview{FindingsTotal: 2},
				rootGoal:        "",
				listAssetsTotal: 0,
			},
			stats: &store.CoverageStats{
				Entries:    []store.CoverageEntry{{Surface: "web", RiskArea: "xss", Outcome: "reported"}},
				Unexamined: []string{"api"},
			},
		},
		task: &store.Task{
			ID:        1,
			Name:      "task from db",
			Status:    "complete",
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	a := New(ms, nil, 1, 10)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var tasks []taskSummary
	json.NewDecoder(rec.Body).Decode(&tasks)
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "task from db" || tasks[0].Status != "complete" || tasks[0].CreatedAt == "" {
		t.Fatalf("task: %+v", tasks[0])
	}
	if tasks[0].Stats.CoveragePct != 50 {
		t.Errorf("coverage: want 50, got %d", tasks[0].Stats.CoveragePct)
	}
}

func TestCoveragePct_NoInterface(t *testing.T) {
	a := newTestAPI(&mockStore{})
	if pct := a.coveragePct(); pct != 0 {
		t.Fatalf("want 0, got %d", pct)
	}
}

func TestCoveragePct_Error(t *testing.T) {
	ms := &mockCoverageStore{statsErr: errTestTask}
	a := New(ms, nil, 1, 10)
	if pct := a.coveragePct(); pct != 0 {
		t.Fatalf("want 0, got %d", pct)
	}
}

func TestCoveragePct_Empty(t *testing.T) {
	ms := &mockCoverageStore{stats: &store.CoverageStats{}}
	a := New(ms, nil, 1, 10)
	if pct := a.coveragePct(); pct != 0 {
		t.Fatalf("want 0, got %d", pct)
	}
}

func TestHandleTaskGoals_OK(t *testing.T) {
	ms := &mockStore{
		graphOverview: &tools.Overview{
			Goals: []map[string]any{
				{"id": 1, "label": "root access", "state": "open"},
				{"id": 2, "label": "data exfil", "state": "done"},
			},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/1/goals", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var goals []map[string]any
	json.NewDecoder(rec.Body).Decode(&goals)
	if len(goals) != 2 {
		t.Fatalf("want 2 goals, got %d", len(goals))
	}
}

func TestHandleTaskGoals_FallbackToRoot(t *testing.T) {
	ms := &mockStore{
		graphOverview: &tools.Overview{Goals: nil},
		rootGoal:      "find all vulns",
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/1/goals", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var goals []map[string]any
	json.NewDecoder(rec.Body).Decode(&goals)
	if len(goals) != 1 || goals[0]["label"] != "find all vulns" {
		t.Fatalf("goals: %+v", goals)
	}
}

func TestHandleTaskGoals_WrongTaskID(t *testing.T) {
	a := newTestAPI(&mockStore{graphOverview: &tools.Overview{}})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/999/goals", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", rec.Code)
	}
}

func TestHandleTaskGoals_BadID(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/abc/goals", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", rec.Code)
	}
}

func TestHandleTaskGoals_OverviewError(t *testing.T) {
	a := newTestAPI(&mockStore{graphOverviewErr: errTestTask})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/1/goals", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}
