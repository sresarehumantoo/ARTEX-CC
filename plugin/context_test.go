package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type contextStore struct {
	tools.Store
	engagements map[int64]string
	scope       map[int64][]tools.ScopeRule
}

func (m *contextStore) GetEngagement(_ context.Context, id int64) (*tools.EngagementDetail, error) {
	goal, ok := m.engagements[id]
	if !ok {
		return nil, nil
	}
	return &tools.EngagementDetail{EngagementSummary: tools.EngagementSummary{ID: id, Goal: goal}}, nil
}

func (m *contextStore) ListScopeRules(_ context.Context, taskID int64) ([]tools.ScopeRule, error) {
	return m.scope[taskID], nil
}

// registerCapture registers a plugin that writes the invocation it receives on
// stdin to a file, and returns the tool plus a function that reads that file.
func registerCapture(t *testing.T, contextFn ContextFunc) (tools.Tool, func() string) {
	t.Helper()
	dir := t.TempDir()
	capture := filepath.Join(t.TempDir(), "invocation.json")
	writePlugin(t, dir, "cap", `#!/bin/sh
if [ "$1" = "--manifest" ]; then
  echo '{"name":"cap","version":"1.0.0","tools":[{"name":"run","description":"capture","schema":{"type":"object"}}]}'
  exit 0
fi
cat > '`+capture+`'
echo '{"content":[{"type":"text","text":"ok"}]}'
`)
	reg := tools.NewRegistry()
	if err := DiscoverAndRegister(reg, dir, 10*time.Second, contextFn); err != nil {
		t.Fatal(err)
	}
	tool, ok := reg.Lookup("cap_run")
	if !ok {
		t.Fatal("tool cap_run not found")
	}
	read := func() string {
		t.Helper()
		data, err := os.ReadFile(capture)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return tool, read
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("invalid JSON from plugin stdin: %v\n%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("invalid expected JSON: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("invocation mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestInvocation_EngagementContext(t *testing.T) {
	store := &contextStore{
		engagements: map[int64]string{1: "Pentest example.com", 2: "Pentest corp.test"},
		scope: map[int64][]tools.ScopeRule{
			1: {
				{Kind: "root_domain", Value: "example.com", Reason: "contract"},
				{Kind: "cidr", Value: "10.0.0.0/24"},
			},
		},
	}
	taskID, explorationID := int64(1), int64(11)
	active := func() (int64, int64, *tools.Registry) { return taskID, explorationID, tools.NewRegistry() }
	proxy := &Proxy{URL: "http://127.0.0.1:8788", CACert: "/data/traffic/_ca/mitmproxy-ca-cert.pem"}
	tool, read := registerCapture(t, EngagementContext(store, active, "/data", proxy))

	if _, err := tool.Run(t.Context(), json.RawMessage(`{"target":"example.com"}`)); err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, read(), `{
		"tool": "run",
		"input": {"target": "example.com"},
		"config": {
			"engagement": {"id": 1, "exploration_id": 11, "name": "Pentest example.com"},
			"in_scope": [
				{"kind": "root_domain", "value": "example.com", "reason": "contract"},
				{"kind": "cidr", "value": "10.0.0.0/24"}
			],
			"data_dir": "/data",
			"proxy": {"url": "http://127.0.0.1:8788", "ca_cert": "/data/traffic/_ca/mitmproxy-ca-cert.pem"}
		}
	}`)

	taskID, explorationID = 2, 22
	if _, err := tool.Run(t.Context(), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, read(), `{
		"tool": "run",
		"input": {},
		"config": {
			"engagement": {"id": 2, "exploration_id": 22, "name": "Pentest corp.test"},
			"in_scope": [],
			"data_dir": "/data",
			"proxy": {"url": "http://127.0.0.1:8788", "ca_cert": "/data/traffic/_ca/mitmproxy-ca-cert.pem"}
		}
	}`)
}

func TestInvocation_NoEngagementNoProxy(t *testing.T) {
	active := func() (int64, int64, *tools.Registry) { return 0, 0, tools.NewRegistry() }
	tool, read := registerCapture(t, EngagementContext(&contextStore{}, active, "/data", nil))

	if _, err := tool.Run(t.Context(), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, read(), `{
		"tool": "run",
		"input": {},
		"config": {
			"engagement": {"id": 0, "exploration_id": 0, "name": ""},
			"in_scope": [],
			"data_dir": "/data"
		}
	}`)
}

func TestInvocation_ContextError(t *testing.T) {
	failing := func(context.Context) (PluginConfig, *tools.Registry, error) {
		return PluginConfig{}, nil, errors.New("db down")
	}
	tool, _ := registerCapture(t, failing)

	_, err := tool.Run(t.Context(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error when the plugin context cannot be built")
	}
}
