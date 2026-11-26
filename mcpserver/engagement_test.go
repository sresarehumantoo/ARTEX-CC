package mcpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/evidence"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/SREsAreHumanToo/ARTEX-CC/traffic"
)

type mockStore struct {
	tools.Store
	rootDesc    string
	rootGoal    string
	constraints []tools.Constraint
	engagement  *tools.EngagementDetail
	engErr      error
}

func (m *mockStore) Root(_ context.Context, _ int64) (string, string, error) {
	return m.rootDesc, m.rootGoal, nil
}

func (m *mockStore) ListConstraints(_ context.Context, _ int64) ([]tools.Constraint, error) {
	return m.constraints, nil
}

func (m *mockStore) GetEngagement(_ context.Context, id int64) (*tools.EngagementDetail, error) {
	if m.engErr != nil {
		return nil, m.engErr
	}
	if m.engagement != nil && m.engagement.ID == id {
		return m.engagement, nil
	}
	return nil, nil
}

func startTestServerWithOpts(t *testing.T, reg *tools.Registry, opts ...mcpserver.Option) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := mcpserver.New(reg, opts...)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Serve(ctx, ln)

	return ln.Addr().String()
}

func TestMCPServerAccessor(t *testing.T) {
	t.Parallel()
	reg := tools.NewRegistry()
	srv := mcpserver.New(reg)
	if srv.MCPServer() == nil {
		t.Fatal("MCPServer() returned nil")
	}
}

func TestWithEngagementRegistersResource(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		rootDesc:    "Test engagement description",
		rootGoal:    "Test goal",
		constraints: []tools.Constraint{{ID: 1, Kind: "scope", Text: "Only test systems"}},
	}

	reg := tools.NewRegistry()
	reg.Register(pingTool())
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 2, "resources/read", map[string]any{
		"uri": "artex://engagement",
	})
	if rr.Error != nil {
		t.Fatalf("resources/read error: %s", rr.Error.Message)
	}
	if rr.Result == nil {
		t.Fatal("resources/read returned nil result")
	}

	var result struct {
		Contents []struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, rr.Result)
	}
	if len(result.Contents) == 0 {
		t.Fatal("no resource contents returned")
	}
	text := result.Contents[0].Text
	if !strings.Contains(text, "Test goal") {
		t.Errorf("resource text missing goal; got:\n%s", text)
	}
	if !strings.Contains(text, "Test engagement description") {
		t.Errorf("resource text missing description; got:\n%s", text)
	}
	if !strings.Contains(text, "[scope] Only test systems") {
		t.Errorf("resource text missing constraint; got:\n%s", text)
	}
	if !strings.Contains(text, "Task ID: 1") {
		t.Errorf("resource text missing task ID; got:\n%s", text)
	}
	if !strings.Contains(text, "Exploration ID: 100") {
		t.Errorf("resource text missing exploration ID; got:\n%s", text)
	}
}

func TestWithEngagementNoConstraints(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		rootDesc: "No constraints",
		rootGoal: "Goal",
	}

	reg := tools.NewRegistry()
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 2, "resources/read", map[string]any{
		"uri": "artex://engagement",
	})
	if rr.Error != nil {
		t.Fatalf("resources/read error: %s", rr.Error.Message)
	}
	var result struct {
		Contents []struct {
			Text string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Contents) == 0 {
		t.Fatal("no resource contents")
	}
	if !strings.Contains(result.Contents[0].Text, "None set.") {
		t.Error("expected 'None set.' in constraints section")
	}
}

func TestWithEngagementRegistersSwitchTool(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		rootDesc: "desc",
		rootGoal: "goal",
	}

	reg := tools.NewRegistry()
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 2, "tools/list", map[string]any{})
	if rr.Error != nil {
		t.Fatalf("tools/list error: %s", rr.Error.Message)
	}
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	found := false
	for _, tool := range result.Tools {
		if tool.Name == "switch_engagement" {
			found = true
			break
		}
	}
	if !found {
		t.Error("switch_engagement tool not registered")
	}
}

func TestSwitchEngagementInvalidInput(t *testing.T) {
	t.Parallel()
	store := &mockStore{rootDesc: "d", rootGoal: "g"}
	reg := tools.NewRegistry()
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "switch_engagement",
		"arguments": map[string]any{"task_id": "not-a-number"},
	})
	if rr.Error != nil {
		t.Fatalf("unexpected RPC error: %s", rr.Error.Message)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError for invalid input")
	}
}

func TestSwitchEngagementZeroTaskID(t *testing.T) {
	t.Parallel()
	store := &mockStore{rootDesc: "d", rootGoal: "g"}
	reg := tools.NewRegistry()
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "switch_engagement",
		"arguments": map[string]any{"task_id": 0},
	})
	if rr.Error != nil {
		t.Fatalf("unexpected RPC error: %s", rr.Error.Message)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError for zero task_id")
	}
	if len(result.Content) > 0 && !strings.Contains(result.Content[0].Text, "task_id is required") {
		t.Errorf("expected 'task_id is required', got %q", result.Content[0].Text)
	}
}

func TestSwitchEngagementStoreError(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		rootDesc: "d",
		rootGoal: "g",
		engErr:   errors.New("db connection lost"),
	}
	reg := tools.NewRegistry()
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "switch_engagement",
		"arguments": map[string]any{"task_id": 999},
	})
	if rr.Error != nil {
		t.Fatalf("unexpected RPC error: %s", rr.Error.Message)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError for store error")
	}
	if len(result.Content) > 0 && !strings.Contains(result.Content[0].Text, "db connection lost") {
		t.Errorf("expected store error message, got %q", result.Content[0].Text)
	}
}

func TestSwitchEngagementNotFound(t *testing.T) {
	t.Parallel()
	store := &mockStore{rootDesc: "d", rootGoal: "g"}
	reg := tools.NewRegistry()
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "switch_engagement",
		"arguments": map[string]any{"task_id": 999},
	})
	if rr.Error != nil {
		t.Fatalf("unexpected RPC error: %s", rr.Error.Message)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError for not found engagement")
	}
	if len(result.Content) > 0 && !strings.Contains(result.Content[0].Text, "not found") {
		t.Errorf("expected 'not found', got %q", result.Content[0].Text)
	}
}

func TestSwitchEngagementSuccess(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		rootDesc: "d",
		rootGoal: "g",
		engagement: &tools.EngagementDetail{
			EngagementSummary: tools.EngagementSummary{
				ID:            42,
				Goal:          "new goal",
				ExplorationID: 200,
			},
		},
	}
	reg := tools.NewRegistry()
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "switch_engagement",
		"arguments": map[string]any{"task_id": 42},
	})
	if rr.Error != nil {
		t.Fatalf("unexpected RPC error: %s", rr.Error.Message)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("empty content")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "switched to engagement 42") {
		t.Errorf("unexpected response: %s", text)
	}
	if !strings.Contains(text, "exploration 200") {
		t.Errorf("missing exploration ID in response: %s", text)
	}
	if !strings.Contains(text, "new goal") {
		t.Errorf("missing goal in response: %s", text)
	}
}

func TestActiveEngagementFollowsSwitch(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		engagement: &tools.EngagementDetail{
			EngagementSummary: tools.EngagementSummary{ID: 42, ExplorationID: 200},
		},
	}
	srv := mcpserver.New(tools.NewRegistry(), mcpserver.WithEngagement(store, 1, 100))
	if task, expl := srv.ActiveEngagement(); task != 1 || expl != 100 {
		t.Fatalf("ActiveEngagement = (%d, %d), want (1, 100)", task, expl)
	}

	srv.MCPServer().HandleMessage(context.Background(), json.RawMessage(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"switch_engagement","arguments":{"task_id":42}}}`))
	if task, expl := srv.ActiveEngagement(); task != 42 || expl != 200 {
		t.Fatalf("ActiveEngagement after switch = (%d, %d), want (42, 200)", task, expl)
	}
}

func TestSwitchEngagementConcurrentReads(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		rootDesc: "d",
		rootGoal: "g",
		engagement: &tools.EngagementDetail{
			EngagementSummary: tools.EngagementSummary{ID: 42, ExplorationID: 200},
		},
	}
	srv := mcpserver.New(tools.NewRegistry(), mcpserver.WithEngagement(store, 1, 100))
	ctx := context.Background()
	switchMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"switch_engagement","arguments":{"task_id":42}}}`)
	readMsg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"artex://engagement"}}`)

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			srv.MCPServer().HandleMessage(ctx, switchMsg)
		}()
		go func() {
			defer wg.Done()
			srv.MCPServer().HandleMessage(ctx, readMsg)
		}()
		go func() {
			defer wg.Done()
			srv.ActiveEngagement()
		}()
	}
	wg.Wait()

	if task, expl := srv.ActiveEngagement(); task != 42 || expl != 200 {
		t.Fatalf("ActiveEngagement = (%d, %d), want (42, 200)", task, expl)
	}
}

func TestSwitchEngagementWithEvidenceAndTraffic(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		rootDesc: "d",
		rootGoal: "g",
		engagement: &tools.EngagementDetail{
			EngagementSummary: tools.EngagementSummary{
				ID:            42,
				Goal:          "goal with extras",
				ExplorationID: 200,
			},
		},
	}
	evidenceStore := evidence.NewStore(t.TempDir())
	reg := tools.NewRegistry()
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithEngagement(store, 1, 100),
		mcpserver.WithEvidence(evidenceStore),
		mcpserver.WithTraffic(&traffic.Traffic{}),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "switch_engagement",
		"arguments": map[string]any{"task_id": 42},
	})
	if rr.Error != nil {
		t.Fatalf("unexpected RPC error: %s", rr.Error.Message)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content[0].Text)
	}
	if len(result.Content) > 0 && !strings.Contains(result.Content[0].Text, "switched to engagement 42") {
		t.Errorf("unexpected response: %s", result.Content[0].Text)
	}
}

func TestNotifierCalledOnMutatingTool(t *testing.T) {
	t.Parallel()
	var called atomic.Int32

	mutatingTool := tools.Tool{
		Name:        "mutate",
		Description: "mutating tool",
		ReadOnly:    false,
		Schema:      map[string]any{"type": "object", "properties": map[string]any{}},
		Run: func(_ context.Context, _ json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: "mutated"}, nil
		},
	}
	reg := tools.NewRegistry()
	reg.Register(mutatingTool)
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithNotifier(func(name string, _ json.RawMessage) {
			if name == "mutate" {
				called.Add(1)
			}
		}),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "mutate",
		"arguments": map[string]any{},
	})
	if rr.Error != nil {
		t.Fatalf("tools/call error: %s", rr.Error.Message)
	}
	if called.Load() != 1 {
		t.Errorf("notifier called %d times, want 1", called.Load())
	}
}

func TestNotifierNotCalledOnReadOnlyTool(t *testing.T) {
	t.Parallel()
	var called atomic.Int32

	readOnlyTool := tools.Tool{
		Name:        "reader",
		Description: "read-only tool",
		ReadOnly:    true,
		Schema:      map[string]any{"type": "object", "properties": map[string]any{}},
		Run: func(_ context.Context, _ json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: "read"}, nil
		},
	}
	reg := tools.NewRegistry()
	reg.Register(readOnlyTool)
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithNotifier(func(_ string, _ json.RawMessage) {
			called.Add(1)
		}),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "reader",
		"arguments": map[string]any{},
	})
	if rr.Error != nil {
		t.Fatalf("tools/call error: %s", rr.Error.Message)
	}
	if called.Load() != 0 {
		t.Errorf("notifier called %d times on read-only tool, want 0", called.Load())
	}
}

func TestNotifierNotCalledOnToolError(t *testing.T) {
	t.Parallel()
	var called atomic.Int32

	errorTool := tools.Tool{
		Name:        "fail_soft",
		Description: "tool that returns an error result",
		ReadOnly:    false,
		Schema:      map[string]any{"type": "object", "properties": map[string]any{}},
		Run: func(_ context.Context, _ json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: "something went wrong", IsError: true}, nil
		},
	}
	reg := tools.NewRegistry()
	reg.Register(errorTool)
	addr := startTestServerWithOpts(t, reg,
		mcpserver.WithNotifier(func(_ string, _ json.RawMessage) {
			called.Add(1)
		}),
	)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "fail_soft",
		"arguments": map[string]any{},
	})
	if rr.Error != nil {
		t.Fatalf("unexpected RPC error: %s", rr.Error.Message)
	}
	if called.Load() != 0 {
		t.Errorf("notifier called %d times on error result, want 0", called.Load())
	}
}

func TestToolRunHardError(t *testing.T) {
	t.Parallel()

	hardErrorTool := tools.Tool{
		Name:        "crash",
		Description: "tool that returns a Go error",
		Schema:      map[string]any{"type": "object", "properties": map[string]any{}},
		Run: func(_ context.Context, _ json.RawMessage) (tools.Result, error) {
			return tools.Result{}, fmt.Errorf("unexpected panic")
		},
	}
	reg := tools.NewRegistry()
	reg.Register(hardErrorTool)
	addr := startTestServerWithOpts(t, reg)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "crash",
		"arguments": map[string]any{},
	})
	if rr.Error != nil {
		t.Fatalf("unexpected RPC error: %s", rr.Error.Message)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError for hard error")
	}
	if len(result.Content) > 0 && !strings.Contains(result.Content[0].Text, "unexpected panic") {
		t.Errorf("expected error message, got %q", result.Content[0].Text)
	}
}

func TestNewWithoutEngagement(t *testing.T) {
	t.Parallel()
	store := &mockStore{}
	reg := tools.NewRegistry()
	reg.Register(pingTool())
	srv := mcpserver.New(reg, mcpserver.WithEngagement(store, 1, 0))
	if srv.MCPServer() == nil {
		t.Fatal("MCPServer() returned nil")
	}
}

func TestReadOnlyAnnotation(t *testing.T) {
	t.Parallel()
	readOnlyTool := tools.Tool{
		Name:        "ro_tool",
		Description: "read-only annotated tool",
		ReadOnly:    true,
		Schema:      map[string]any{"type": "object", "properties": map[string]any{}},
		Run: func(_ context.Context, _ json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: "ok"}, nil
		},
	}
	reg := tools.NewRegistry()
	reg.Register(readOnlyTool)
	addr := startTestServerWithOpts(t, reg)
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 2, "tools/list", map[string]any{})
	if rr.Error != nil {
		t.Fatalf("tools/list error: %s", rr.Error.Message)
	}
	var result struct {
		Tools []struct {
			Name        string `json:"name"`
			Annotations struct {
				ReadOnlyHint *bool `json:"readOnlyHint"`
			} `json:"annotations"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, tool := range result.Tools {
		if tool.Name == "ro_tool" {
			if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
				t.Error("expected readOnlyHint=true for ro_tool")
			}
			return
		}
	}
	t.Error("ro_tool not found in tools list")
}
