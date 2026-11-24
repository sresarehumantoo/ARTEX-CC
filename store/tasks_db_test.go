package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

var taskCols = []string{"id", "name", "description", "goal", "exploration_id", "status", "company_id", "created_at", "updated_at"}

func TestGetTask_Found(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(1), "task1", "desc", "goal", int64(10), "open", nil, now, now))

	task, err := s.GetTask(1)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task == nil {
		t.Fatal("expected non-nil task")
	}
	if task.ID != 1 || task.Name != "task1" || task.Goal != "goal" || task.ExplorationID != 10 {
		t.Errorf("unexpected task: %+v", task)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(99)).
		WillReturnError(sql.ErrNoRows)

	task, err := s.GetTask(99)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task != nil {
		t.Errorf("expected nil task, got %+v", task)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTasks_NonEmpty(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks ORDER BY created_at DESC").
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(2), "t2", "d2", "g2", int64(20), "open", nil, now, now).
			AddRow(int64(1), "t1", "d1", "g1", int64(10), "done", nil, now.Add(-time.Hour), now))

	tasks, err := s.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].ID != 2 || tasks[1].ID != 1 {
		t.Errorf("task IDs = [%d, %d], want [2, 1]", tasks[0].ID, tasks[1].ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTasks_Empty(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)

	mock.ExpectQuery("SELECT .+ FROM tasks ORDER BY created_at DESC").
		WillReturnRows(sqlmock.NewRows(taskCols))

	tasks, err := s.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("got %d tasks, want 0", len(tasks))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSetTaskStatus(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)

	mock.ExpectExec("UPDATE tasks SET status").
		WithArgs("done", int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.SetTaskStatus(5, "done"); err != nil {
		t.Fatalf("SetTaskStatus: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteEngagement_Success(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(1), "t1", "d1", "g1", int64(10), "open", nil, now, now))

	mock.ExpectExec("UPDATE tasks SET status = 'done' WHERE id").
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("UPDATE explorations SET status = 'achieved' WHERE id").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("INSERT INTO activity").
		WithArgs(int64(10), "done summary").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := s.CompleteEngagement(ctx, 1, "done summary"); err != nil {
		t.Fatalf("CompleteEngagement: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteEngagement_NoSummary(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(1), "t1", "d1", "g1", int64(10), "open", nil, now, now))

	mock.ExpectExec("UPDATE tasks SET status = 'done' WHERE id").
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("UPDATE explorations SET status = 'achieved' WHERE id").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.CompleteEngagement(ctx, 1, ""); err != nil {
		t.Fatalf("CompleteEngagement: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteEngagement_TaskNotFound(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	err := s.CompleteEngagement(ctx, 999, "summary")
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListEngagementsMock(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	cols := []string{"id", "goal", "description", "status", "exploration_id", "created_at", "node_count", "finding_count", "asset_count", "goals_met"}
	mock.ExpectQuery("SELECT .+ FROM tasks t ORDER BY").
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(int64(1), "g1", "d1", "open", int64(10), now, 5, 2, 3, false).
			AddRow(int64(2), "g2", "d2", "done", int64(20), now.Add(-time.Hour), 10, 4, 6, true))

	engagements, err := s.ListEngagements(ctx)
	if err != nil {
		t.Fatalf("ListEngagements: %v", err)
	}
	if len(engagements) != 2 {
		t.Fatalf("got %d engagements, want 2", len(engagements))
	}
	if engagements[0].ID != 1 || engagements[0].Goal != "g1" {
		t.Errorf("first engagement: %+v", engagements[0])
	}
	if engagements[1].GoalsMet != true {
		t.Errorf("second engagement GoalsMet = false, want true")
	}
	if engagements[0].CreatedAt == "" {
		t.Error("CreatedAt should be formatted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListEngagementsMock_Empty(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()

	cols := []string{"id", "goal", "description", "status", "exploration_id", "created_at", "node_count", "finding_count", "asset_count", "goals_met"}
	mock.ExpectQuery("SELECT .+ FROM tasks t ORDER BY").
		WillReturnRows(sqlmock.NewRows(cols))

	engagements, err := s.ListEngagements(ctx)
	if err != nil {
		t.Fatalf("ListEngagements: %v", err)
	}
	if len(engagements) != 0 {
		t.Errorf("got %d engagements, want 0", len(engagements))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateEngagement_Goal(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(1), "t1", "d1", "g1", int64(10), "open", nil, now, now))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE tasks SET goal .+ WHERE id").
		WithArgs("new goal", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE explorations SET goal .+ WHERE id").
		WithArgs("new goal", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	newGoal := "new goal"
	if err := s.UpdateEngagement(ctx, 1, &newGoal, nil); err != nil {
		t.Fatalf("UpdateEngagement: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateEngagement_Description(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(1), "t1", "d1", "g1", int64(10), "open", nil, now, now))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE tasks SET description .+ WHERE id").
		WithArgs("new desc", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE explorations SET description .+ WHERE id").
		WithArgs("new desc", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	newDesc := "new desc"
	if err := s.UpdateEngagement(ctx, 1, nil, &newDesc); err != nil {
		t.Fatalf("UpdateEngagement: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateEngagement_TaskNotFound(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	goal := "x"
	err := s.UpdateEngagement(ctx, 999, &goal, nil)
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteEngagement_Success(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(1), "t1", "d1", "g1", int64(10), "open", nil, now, now))

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM findings WHERE task_id").
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE assets SET task_ids").
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM explorations WHERE id").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := s.DeleteEngagement(ctx, 1); err != nil {
		t.Fatalf("DeleteEngagement: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteEngagement_TaskNotFound(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	err := s.DeleteEngagement(ctx, 999)
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEngagementActivityMock(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	cols := []string{"ts", "kind", "summary"}
	mock.ExpectQuery("SELECT ts, kind, summary FROM").
		WithArgs(int64(10), 20).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(now, "finding", "high: XSS found").
			AddRow(now.Add(-time.Minute), "node:intent", "open: check login form").
			AddRow(now.Add(-2*time.Minute), "asset", "service: https://example.com"))

	entries, err := s.EngagementActivity(ctx, 10, 0)
	if err != nil {
		t.Fatalf("EngagementActivity: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[0].Kind != "finding" {
		t.Errorf("first entry kind = %q, want finding", entries[0].Kind)
	}
	if entries[0].Timestamp == "" {
		t.Error("Timestamp should be set")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEngagementActivity_ClampLimit(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()

	cols := []string{"ts", "kind", "summary"}
	mock.ExpectQuery("SELECT ts, kind, summary FROM").
		WithArgs(int64(10), 200).
		WillReturnRows(sqlmock.NewRows(cols))

	_, err := s.EngagementActivity(ctx, 10, 999)
	if err != nil {
		t.Fatalf("EngagementActivity: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetEngagement_Found(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(1), "t1", "the desc", "the goal", int64(10), "open", nil, now, now))

	detail, err := s.GetEngagement(ctx, 1)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	if detail == nil {
		t.Fatal("expected non-nil detail")
	}
	if detail.ID != 1 || detail.Goal != "the goal" || detail.Description != "the desc" {
		t.Errorf("unexpected detail: %+v", detail)
	}
	if detail.ExplorationID != 10 {
		t.Errorf("ExplorationID = %d, want 10", detail.ExplorationID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetEngagement_NotFound(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(99)).
		WillReturnError(sql.ErrNoRows)

	detail, err := s.GetEngagement(ctx, 99)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	if detail != nil {
		t.Errorf("expected nil detail, got %+v", detail)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteEngagement_UpdateTaskFails(t *testing.T) {
	t.Parallel()
	s, mock := newMockStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM tasks WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(taskCols).
			AddRow(int64(1), "t1", "d1", "g1", int64(10), "open", nil, now, now))

	mock.ExpectExec("UPDATE tasks SET status = 'done' WHERE id").
		WithArgs(int64(1)).
		WillReturnError(fmt.Errorf("db write error"))

	err := s.CompleteEngagement(ctx, 1, "summary")
	if err == nil {
		t.Fatal("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
