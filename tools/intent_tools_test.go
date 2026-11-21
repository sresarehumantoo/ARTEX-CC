package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type intentMockStore struct {
	mockStore
	nodes map[int64]*Node
}

func newIntentMockStore() *intentMockStore {
	return &intentMockStore{nodes: map[int64]*Node{}}
}

func (m *intentMockStore) NodeDetail(_ context.Context, id int64) (*Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return nil, nil
	}
	return n, nil
}

func (m *intentMockStore) UpdateNodeState(_ context.Context, id int64, state string) error {
	n, ok := m.nodes[id]
	if !ok {
		return fmt.Errorf("node %d not found", id)
	}
	n.State = state
	return nil
}

func (m *intentMockStore) UpdateNodeFields(_ context.Context, id int64, state string, _ map[string]any) error {
	n, ok := m.nodes[id]
	if !ok {
		return fmt.Errorf("node %d not found", id)
	}
	n.State = state
	return nil
}

func TestStartIntentHappyPath(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, ok := reg.Lookup("start_intent")
	if !ok {
		t.Fatal("start_intent not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "running") {
		t.Errorf("expected 'running' in result, got: %s", r.Content)
	}
	if ms.nodes[10].State != "running" {
		t.Errorf("expected state running, got %s", ms.nodes[10].State)
	}
}

func TestStartIntentWrongState(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "done"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for starting a done intent")
	}
	if !strings.Contains(r.Content, "must be open") {
		t.Errorf("expected 'must be open' in error, got: %s", r.Content)
	}
}

func TestStartIntentWrongKind(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindFact, State: "confirmed"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for non-intent node")
	}
	if !strings.Contains(r.Content, "not an intent") {
		t.Errorf("expected 'not an intent' in error, got: %s", r.Content)
	}
}

func TestStartIntentMissingNodeID(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing node_id")
	}
}

func TestStartIntentNotFound(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 999}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for nonexistent node")
	}
	if !strings.Contains(r.Content, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", r.Content)
	}
}

func TestCompleteIntentFromRunning(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("complete_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10, "result": "found XSS"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if ms.nodes[10].State != "done" {
		t.Errorf("expected state done, got %s", ms.nodes[10].State)
	}
}

func TestCompleteIntentFromOpen(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("complete_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success (open -> done allowed), got error: %s", r.Content)
	}
}

func TestCompleteIntentFromDone(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "done"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("complete_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for completing a done intent")
	}
}

func TestCancelIntentFromOpen(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10, "reason": "no longer needed"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if ms.nodes[10].State != "stopped" {
		t.Errorf("expected state stopped, got %s", ms.nodes[10].State)
	}
}

func TestCancelIntentFromRunning(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestCancelIntentAlreadyDone(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "done"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for cancelling a done intent")
	}
	if !strings.Contains(r.Content, "already") {
		t.Errorf("expected 'already' in error, got: %s", r.Content)
	}
}

func TestStartIntentAlreadyRunning(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for starting an already-running intent")
	}
	if !strings.Contains(r.Content, "must be open") {
		t.Errorf("expected 'must be open' in error, got: %s", r.Content)
	}
}

func TestStartIntentFromCancelled(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "stopped"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for starting a cancelled intent")
	}
	if !strings.Contains(r.Content, "must be open") {
		t.Errorf("expected 'must be open' in error, got: %s", r.Content)
	}
}

func TestCompleteIntentFromStopped(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "stopped"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("complete_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for completing a cancelled intent")
	}
	if !strings.Contains(r.Content, "must be running or open") {
		t.Errorf("expected 'must be running or open' in error, got: %s", r.Content)
	}
}

func TestCancelIntentAlreadyStopped(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "stopped"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for cancelling an already-stopped intent")
	}
	if !strings.Contains(r.Content, "already") {
		t.Errorf("expected 'already' in error, got: %s", r.Content)
	}
}

func TestIntentOpenToCancelledLifecycle(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	cancel, _ := reg.Lookup("cancel_intent")
	r, _ := cancel.Run(context.Background(), json.RawMessage(`{"node_id": 10, "reason": "obsolete"}`))
	if r.IsError {
		t.Fatalf("cancel from open failed: %s", r.Content)
	}
	if ms.nodes[10].State != "stopped" {
		t.Fatalf("expected stopped, got %s", ms.nodes[10].State)
	}

	start, _ := reg.Lookup("start_intent")
	r, _ = start.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if !r.IsError {
		t.Fatal("expected error for starting a cancelled intent")
	}
}

func TestIntentRunningToCancelledLifecycle(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	start, _ := reg.Lookup("start_intent")
	cancel, _ := reg.Lookup("cancel_intent")
	complete, _ := reg.Lookup("complete_intent")

	r, _ := start.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if r.IsError {
		t.Fatalf("start failed: %s", r.Content)
	}

	r, _ = cancel.Run(context.Background(), json.RawMessage(`{"node_id": 10, "reason": "pivoting"}`))
	if r.IsError {
		t.Fatalf("cancel from running failed: %s", r.Content)
	}
	if ms.nodes[10].State != "stopped" {
		t.Fatalf("expected stopped, got %s", ms.nodes[10].State)
	}

	r, _ = complete.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if !r.IsError {
		t.Fatal("expected error for completing a cancelled intent")
	}
}

func TestIntentFullLifecycle(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	start, _ := reg.Lookup("start_intent")
	complete, _ := reg.Lookup("complete_intent")

	r, _ := start.Run(context.Background(), json.RawMessage(`{"node_id": 10}`))
	if r.IsError {
		t.Fatalf("start failed: %s", r.Content)
	}
	if ms.nodes[10].State != "running" {
		t.Fatalf("expected running, got %s", ms.nodes[10].State)
	}

	r, _ = complete.Run(context.Background(), json.RawMessage(`{"node_id": 10, "result": "done"}`))
	if r.IsError {
		t.Fatalf("complete failed: %s", r.Content)
	}
	if ms.nodes[10].State != "done" {
		t.Fatalf("expected done, got %s", ms.nodes[10].State)
	}
}
