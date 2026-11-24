package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestLogActivity(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO activity").
		WithArgs(int64(10), int64(5), "worker1", "tool_call", "nmap", false, "ran nmap", "detail text").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

	id, err := s.LogActivity(ctx, 10, tools.ActivityEntry{
		NodeID:  5,
		Worker:  "worker1",
		Kind:    "tool_call",
		Tool:    "nmap",
		IsError: false,
		Summary: "ran nmap",
		Detail:  "detail text",
	})
	if err != nil {
		t.Fatalf("LogActivity: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestLogActivityZeroNodeID(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO activity").
		WithArgs(int64(10), int64(0), "w", "k", "t", false, "s", "d").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	id, err := s.LogActivity(ctx, 10, tools.ActivityEntry{
		Worker:  "w",
		Kind:    "k",
		Tool:    "t",
		Summary: "s",
		Detail:  "d",
	})
	if err != nil {
		t.Fatalf("LogActivity: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
}

func TestListActivityNoFilters(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "exploration_id", "node_id", "worker", "kind", "tool", "is_error", "summary", "detail", "created_at"}).
		AddRow(int64(1), int64(10), sql.NullInt64{Int64: 5, Valid: true}, "w1", "tool_call", "nmap", false, "summary1", "detail1", "2024-01-01 00:00:00").
		AddRow(int64(2), int64(10), sql.NullInt64{Valid: false}, "w2", "note", "", false, "summary2", "", "2024-01-01 00:01:00")

	mock.ExpectQuery("SELECT .+ FROM activity").
		WithArgs(int64(10), 50).
		WillReturnRows(rows)

	activities, err := s.ListActivity(ctx, 10, tools.ActivityFilter{})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(activities) != 2 {
		t.Fatalf("got %d activities, want 2", len(activities))
	}
	if activities[0].NodeID != 5 {
		t.Errorf("activities[0].NodeID = %d, want 5", activities[0].NodeID)
	}
	if activities[1].NodeID != 0 {
		t.Errorf("activities[1].NodeID = %d, want 0", activities[1].NodeID)
	}
}

func TestListActivityWithFilters(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "exploration_id", "node_id", "worker", "kind", "tool", "is_error", "summary", "detail", "created_at"}).
		AddRow(int64(1), int64(10), sql.NullInt64{Int64: 7, Valid: true}, "w1", "tool_call", "nmap", false, "s", "d", "2024-01-01")

	mock.ExpectQuery("SELECT .+ FROM activity").
		WithArgs(int64(10), int64(7), "tool_call", "2024-01-01T00:00:00Z", 20).
		WillReturnRows(rows)

	activities, err := s.ListActivity(ctx, 10, tools.ActivityFilter{
		NodeID: 7,
		Kind:   "tool_call",
		Since:  "2024-01-01T00:00:00Z",
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("got %d activities, want 1", len(activities))
	}
	if activities[0].Kind != "tool_call" {
		t.Errorf("Kind = %q, want %q", activities[0].Kind, "tool_call")
	}
}

func TestListActivityLimitCap(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .+ FROM activity").
		WithArgs(int64(10), 500).
		WillReturnRows(sqlmock.NewRows([]string{"id", "exploration_id", "node_id", "worker", "kind", "tool", "is_error", "summary", "detail", "created_at"}))

	_, err := s.ListActivity(ctx, 10, tools.ActivityFilter{Limit: 9999})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
}
