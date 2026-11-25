package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// newRegexpMockStore is defined in mock_helpers_test.go

func nodeRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "kind", "payload", "priority", "state",
		"origin", "owner", "blocked_reason", "delete_reason",
		"anchors_json", "created_at", "updated_at", "claim_expires_at",
	})
}

func TestCreateExploration(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	mock.ExpectQuery(`INSERT INTO explorations`).
		WithArgs("desc", "goal").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

	id, err := s.CreateExploration("desc", "goal")
	if err != nil {
		t.Fatalf("CreateExploration: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAddNode_NoAnchors(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO exploration_nodes`).
		WithArgs(int64(1), "intent", `{"text":"hello"}`, 5, "open", "planner").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(10)))
	mock.ExpectCommit()

	id, err := s.AddNode(ctx, 1, "intent", map[string]any{"text": "hello"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if id != 10 {
		t.Errorf("id = %d, want 10", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAddNode_WithAnchors(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO exploration_nodes`).
		WithArgs(int64(1), "fact", `{"text":"found"}`, 3, "confirmed", "worker").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(20)))
	mock.ExpectExec(`INSERT INTO exploration_anchors`).
		WithArgs(int64(20), int64(100)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO exploration_anchors`).
		WithArgs(int64(20), int64(200)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	id, err := s.AddNode(ctx, 1, "fact", map[string]any{"text": "found"}, 3, "confirmed", "worker", []int64{100, 200})
	if err != nil {
		t.Fatalf("AddNode with anchors: %v", err)
	}
	if id != 20 {
		t.Errorf("id = %d, want 20", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestLink(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectExec(`INSERT INTO exploration_edges`).
		WithArgs(int64(1), int64(10), "spawns", int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.Link(1, 10, "spawns", 20)
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdateNodeState(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE exploration_nodes SET state`).
		WithArgs("done", int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.UpdateNodeState(ctx, 5, "done")
	if err != nil {
		t.Fatalf("UpdateNodeState: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdateNodeFields_StateOnly(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE exploration_nodes SET state = \$1, owner = NULL, claim_expires_at = NULL WHERE id = \$2`).
		WithArgs("running", int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.UpdateNodeFields(ctx, 7, "running", nil)
	if err != nil {
		t.Fatalf("UpdateNodeFields state-only: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdateNodeFields_PayloadOnly(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE exploration_nodes SET payload = payload`).
		WithArgs(`{"key":"val"}`, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.UpdateNodeFields(ctx, 7, "", map[string]any{"key": "val"})
	if err != nil {
		t.Fatalf("UpdateNodeFields payload-only: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdateNodeFields_Both(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE exploration_nodes SET state = \$1, owner = NULL, claim_expires_at = NULL WHERE id = \$2`).
		WithArgs("done", int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE exploration_nodes SET payload = payload`).
		WithArgs(`{"result":"ok"}`, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.UpdateNodeFields(ctx, 7, "done", map[string]any{"result": "ok"})
	if err != nil {
		t.Fatalf("UpdateNodeFields both: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdateNodeFields_NeitherStateNorPayload(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	err := s.UpdateNodeFields(ctx, 7, "", nil)
	if err != nil {
		t.Fatalf("UpdateNodeFields no-op: %v", err)
	}
}

func TestAddEdge(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT exploration_id FROM exploration_nodes`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"exploration_id"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT INTO exploration_edges`).
		WithArgs(int64(1), int64(10), "yields", int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.AddEdge(ctx, 10, "yields", 20)
	if err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAddEdge_NodeNotFound(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT exploration_id FROM exploration_nodes`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	err := s.AddEdge(ctx, 999, "yields", 20)
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestMockListNodes_NoFilter(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()
	now := time.Now()

	rows := nodeRows().
		AddRow(int64(1), "goal", []byte(`{"text":"find xss"}`), 5, "open", "system", "", "", "", "[]", now, now, nil).
		AddRow(int64(2), "intent", []byte(`{"summary":"test login"}`), 3, "open", "planner", "", "", "", "[10]", now, now, nil)

	mock.ExpectQuery(`SELECT .+ FROM exploration_nodes`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	nodes, err := s.ListNodes(ctx, 1, "", 0)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	if nodes[0].Kind != "goal" {
		t.Errorf("nodes[0].Kind = %q, want goal", nodes[0].Kind)
	}
	if nodes[1].Kind != "intent" {
		t.Errorf("nodes[1].Kind = %q, want intent", nodes[1].Kind)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockListNodes_WithKindAndLimit(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()
	now := time.Now()

	rows := nodeRows().
		AddRow(int64(2), "intent", []byte(`{}`), 5, "open", "planner", "", "", "", "[]", now, now, nil)

	mock.ExpectQuery(`SELECT .+ FROM exploration_nodes`).
		WithArgs(int64(1), "intent", 10).
		WillReturnRows(rows)

	nodes, err := s.ListNodes(ctx, 1, "intent", 10)
	if err != nil {
		t.Fatalf("ListNodes with filter: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockAddEdges_Empty(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	err := s.AddEdges(ctx, 1, nil)
	if err != nil {
		t.Fatalf("AddEdges empty: %v", err)
	}
}

func TestMockAddEdges_Multiple(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO exploration_edges`).
		WithArgs(int64(1), int64(10), "spawns", int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO exploration_edges`).
		WithArgs(int64(1), int64(20), "yields", int64(30)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	edges := []Edge{
		{From: 10, Rel: "spawns", To: 20},
		{From: 20, Rel: "yields", To: 30},
	}
	err := s.AddEdges(ctx, 1, edges)
	if err != nil {
		t.Fatalf("AddEdges: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockNodeDetail_Found(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery(`SELECT .+ FROM exploration_nodes n WHERE n.id`).
		WithArgs(int64(5)).
		WillReturnRows(nodeRows().
			AddRow(int64(5), "fact", []byte(`{"text":"found vuln"}`), 3, "confirmed", "worker", "", "", "", "[]", now, now, nil))

	node, err := s.NodeDetail(ctx, 5)
	if err != nil {
		t.Fatalf("NodeDetail: %v", err)
	}
	if node.ID != 5 {
		t.Errorf("ID = %d, want 5", node.ID)
	}
	if node.Kind != "fact" {
		t.Errorf("Kind = %q, want fact", node.Kind)
	}
	if node.State != "confirmed" {
		t.Errorf("State = %q, want confirmed", node.State)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockNodeDetail_NotFound(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT .+ FROM exploration_nodes n WHERE n.id`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	_, err := s.NodeDetail(ctx, 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
	want := fmt.Sprintf("node %d not found", 999)
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestMockFrontier(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()
	now := time.Now()

	rows := nodeRows().
		AddRow(int64(3), "intent", []byte(`{"summary":"high prio"}`), 10, "open", "planner", "", "", "", "[]", now, now, nil).
		AddRow(int64(4), "intent", []byte(`{"summary":"low prio"}`), 1, "open", "planner", "", "", "", "[]", now, now, nil)

	mock.ExpectQuery(`SELECT .+ FROM exploration_nodes n.*WHERE.*intent.*open.*ORDER BY.*priority`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	nodes, err := s.Frontier(ctx, 1, 0)
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockFrontier_WithLimit(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()
	now := time.Now()

	rows := nodeRows().
		AddRow(int64(3), "intent", []byte(`{}`), 10, "open", "planner", "", "", "", "[]", now, now, nil)

	mock.ExpectQuery(`SELECT .+ FROM exploration_nodes n`).
		WithArgs(int64(1), 5).
		WillReturnRows(rows)

	nodes, err := s.Frontier(ctx, 1, 5)
	if err != nil {
		t.Fatalf("Frontier with limit: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockCountFinishedIntents(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT COUNT`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	count, err := s.CountFinishedIntents(ctx, 1)
	if err != nil {
		t.Fatalf("CountFinishedIntents: %v", err)
	}
	if count != 7 {
		t.Errorf("count = %d, want 7", count)
	}
}

func TestMockCountOpenIntents(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT COUNT`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	count, err := s.CountOpenIntents(ctx, 1)
	if err != nil {
		t.Fatalf("CountOpenIntents: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestMockOriginFactID(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT id FROM exploration_nodes`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))

	id, err := s.OriginFactID(ctx, 1)
	if err != nil {
		t.Fatalf("OriginFactID: %v", err)
	}
	if id != 99 {
		t.Errorf("id = %d, want 99", id)
	}
}

func TestMockEdges_NoLimit(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT src_id, rel, dst_id FROM exploration_edges`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"src_id", "rel", "dst_id"}).
			AddRow(int64(10), "spawns", int64(20)).
			AddRow(int64(20), "yields", int64(30)))

	edges, err := s.Edges(ctx, 1, 0)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2", len(edges))
	}
	if edges[0].Rel != "spawns" {
		t.Errorf("edges[0].Rel = %q, want spawns", edges[0].Rel)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockEdges_WithLimit(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT src_id, rel, dst_id FROM exploration_edges`).
		WithArgs(int64(1), 5).
		WillReturnRows(sqlmock.NewRows([]string{"src_id", "rel", "dst_id"}).
			AddRow(int64(10), "spawns", int64(20)))

	edges, err := s.Edges(ctx, 1, 5)
	if err != nil {
		t.Fatalf("Edges with limit: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockRoot(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT .+ FROM explorations WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"description", "goal"}).
			AddRow("my desc", "my goal"))

	desc, goal, err := s.Root(ctx, 1)
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if desc != "my desc" {
		t.Errorf("desc = %q, want %q", desc, "my desc")
	}
	if goal != "my goal" {
		t.Errorf("goal = %q, want %q", goal, "my goal")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockAddConstraint(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`INSERT INTO task_constraints`).
		WithArgs(int64(1), "rule", "no brute force", "operator").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(55)))

	id, err := s.AddConstraint(ctx, 1, "rule", "no brute force", "operator")
	if err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	if id != 55 {
		t.Errorf("id = %d, want 55", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMockListConstraints(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT id, kind, text FROM task_constraints`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "kind", "text"}).
			AddRow(int64(1), "rule", "no brute force").
			AddRow(int64(2), "rule", "stay in scope"))

	constraints, err := s.ListConstraints(ctx, 1)
	if err != nil {
		t.Fatalf("ListConstraints: %v", err)
	}
	if len(constraints) != 2 {
		t.Fatalf("got %d constraints, want 2", len(constraints))
	}
	if constraints[0].Text != "no brute force" {
		t.Errorf("constraints[0].Text = %q", constraints[0].Text)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
