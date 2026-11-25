package mcpserver_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/plugin"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type switchStore struct {
	tools.Store
	mu       sync.Mutex
	scope    map[int64]string
	inserted map[int64][]string
}

func (s *switchStore) GetEngagement(_ context.Context, id int64) (*tools.EngagementDetail, error) {
	if _, ok := s.scope[id]; !ok {
		return nil, nil
	}
	return &tools.EngagementDetail{EngagementSummary: tools.EngagementSummary{ID: id, ExplorationID: id * 10, Goal: s.scope[id]}}, nil
}

func (s *switchStore) ListScopeRules(_ context.Context, taskID int64) ([]tools.ScopeRule, error) {
	return []tools.ScopeRule{{Kind: "subdomain", Value: s.scope[taskID]}}, nil
}

func (s *switchStore) IsInScope(_ context.Context, taskID int64, target string) (bool, string, string, error) {
	if target == s.scope[taskID] {
		return true, "matched", target, nil
	}
	return false, "no matching rule", "", nil
}

func (s *switchStore) InsertAsset(_ context.Context, taskID int64, item tools.AssetInput) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inserted[taskID] = append(s.inserted[taskID], item.Domain)
	return 1, nil
}

func (s *switchStore) Root(context.Context, int64) (string, string, error) { return "", "", nil }

func TestPluginPersistsToSwitchedEngagement(t *testing.T) {
	store := &switchStore{
		scope:    map[int64]string{1: "a.one.test", 2: "a.two.test"},
		inserted: map[int64][]string{},
	}

	dir := t.TempDir()
	capture := filepath.Join(t.TempDir(), "invocation.json")
	script := `#!/bin/sh
if [ "$1" = "--manifest" ]; then
  echo '{"name":"rec","version":"1.0.0","tools":[{"name":"run","description":"run","schema":{"type":"object"}}]}'
  exit 0
fi
cat > '` + capture + `'
echo '{"content":[{"type":"text","text":"ok"}],"assets":[{"type":"subdomain","domain":"a.two.test"}]}'
`
	if err := os.WriteFile(filepath.Join(dir, "rec"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	reg := tools.NewRegistry()
	tools.RegisterAll(reg, store, 1, 10)
	var srv *mcpserver.Server
	pctx := plugin.EngagementContext(store, func() (int64, int64, *tools.Registry) { return srv.ActiveRegistry() }, "", nil)
	if err := plugin.DiscoverAndRegister(reg, dir, 10*time.Second, pctx); err != nil {
		t.Fatal(err)
	}
	srv = mcpserver.New(reg, mcpserver.WithEngagement(store, 1, 10))

	ctx := context.Background()
	srv.MCPServer().HandleMessage(ctx, json.RawMessage(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"switch_engagement","arguments":{"task_id":2}}}`))
	resp := srv.MCPServer().HandleMessage(ctx, json.RawMessage(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"rec_run","arguments":{}}}`))
	out, _ := json.Marshal(resp)

	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var inv plugin.Invocation
	if err := json.Unmarshal(data, &inv); err != nil {
		t.Fatal(err)
	}
	if inv.Config.Engagement.ID != 2 {
		t.Errorf("plugin was told engagement %d, want 2", inv.Config.Engagement.ID)
	}
	if !strings.Contains(string(out), "inserted 1 asset") {
		t.Errorf("unexpected tool response: %s", out)
	}
	if got := store.inserted[2]; len(got) != 1 || got[0] != "a.two.test" {
		t.Errorf("engagement 2 assets = %v, want [a.two.test]", got)
	}
	if got := store.inserted[1]; len(got) != 0 {
		t.Errorf("engagement 1 assets = %v, want none", got)
	}
}
