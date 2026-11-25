package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type claimStore struct {
	tools.Store
	mu     sync.Mutex
	nodes  map[int64]*tools.Node
	claims map[int64]tools.IntentClaim
}

func (m *claimStore) NodeDetail(_ context.Context, id int64) (*tools.Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.nodes[id]; ok {
		c := *n
		return &c, nil
	}
	return nil, nil
}

func (m *claimStore) ClaimIntent(_ context.Context, id int64, owner string, lease time.Duration) (*tools.IntentClaim, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := m.nodes[id]
	prev, held := m.claims[id]
	live := n.State == "running" && held && prev.ExpiresAt.After(time.Now())
	if live && prev.Owner != owner {
		return nil, &tools.ClaimConflictError{NodeID: id, Owner: prev.Owner, ExpiresAt: prev.ExpiresAt}
	}
	n.State = "running"
	c := tools.IntentClaim{NodeID: id, Owner: owner, ExpiresAt: time.Now().Add(lease), Renewed: live}
	m.claims[id] = c
	return &c, nil
}

func (m *claimStore) UpdateNodeFields(_ context.Context, id int64, state string, _ map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[id].State = state
	delete(m.claims, id)
	return nil
}

func (m *claimStore) owner(id int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.claims[id].Owner
}

func callModern(t *testing.T, url, tool string, args map[string]any) (string, bool) {
	t.Helper()
	resp := postModern(t, url, nil, tool, args)
	defer resp.Body.Close()
	return decodeToolResult(t, tool, resp)
}

func postModern(t *testing.T, url string, header http.Header, tool string, args map[string]any) *http.Response {
	t.Helper()
	params := map[string]any{
		"name":      tool,
		"arguments": args,
		"_meta": map[string]any{
			mcp.MetaKeyProtocolVersion:    mcp.ProtocolVersion20260728,
			mcp.MetaKeyClientInfo:         map[string]any{"name": "test-client", "version": "1.0.0"},
			mcp.MetaKeyClientCapabilities: map[string]any{},
		},
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(mcp.HeaderProtocolVersion, mcp.ProtocolVersion20260728)
	req.Header.Set(mcp.HeaderMethod, "tools/call")
	req.Header.Set(mcp.HeaderName, tool)
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeToolResult(t *testing.T, tool string, resp *http.Response) (string, bool) {
	t.Helper()
	raw, _ := io.ReadAll(resp.Body)
	var rr struct {
		Result *mcp.CallToolResult `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &rr); err != nil {
		t.Fatalf("decode %s: %v\n%s", tool, err, raw)
	}
	if rr.Error != nil || rr.Result == nil || len(rr.Result.Content) == 0 {
		t.Fatalf("%s: unexpected response %s", tool, raw)
	}
	text, _ := mcp.AsTextContent(rr.Result.Content[0])
	return text.Text, rr.Result.IsError
}

func TestModernHTTPClientsHoldDistinctClaims(t *testing.T) {
	t.Parallel()
	st := &claimStore{
		nodes:  map[int64]*tools.Node{10: {ID: 10, Kind: tools.KindIntent, State: "open"}},
		claims: map[int64]tools.IntentClaim{},
	}
	reg := tools.NewRegistry()
	tools.RegisterIntentTools(reg, st, 1)
	srv := mcpserver.New(reg)
	ts := httptest.NewServer(server.NewStreamableHTTPServer(srv.MCPServer()))
	t.Cleanup(ts.Close)
	url := ts.URL + "/mcp"

	out, isErr := callModern(t, url, "start_intent", map[string]any{"node_id": 10})
	if !isErr || !strings.Contains(out, "owner") {
		t.Fatalf("start_intent without session or owner = %q, want refusal naming owner", out)
	}

	out, isErr = callModern(t, url, "start_intent", map[string]any{"node_id": 10, "owner": "client-a"})
	if isErr || !strings.Contains(out, "claimed by") {
		t.Fatalf("client A claim = %q", out)
	}
	if strings.Contains(out, "client-a") {
		t.Errorf("claim reply exposes the raw owner: %q", out)
	}

	out, isErr = callModern(t, url, "start_intent", map[string]any{"node_id": 10, "owner": "client-b"})
	if !isErr || !strings.Contains(out, "claimed by") {
		t.Fatalf("client B on A's intent = %q, want claim conflict", out)
	}
	if strings.Contains(out, "client-a") {
		t.Errorf("conflict exposes the holder's raw owner: %q", out)
	}

	out, isErr = callModern(t, url, "complete_intent", map[string]any{"node_id": 10, "owner": "client-b"})
	if !isErr || !strings.Contains(out, "claimed by") {
		t.Fatalf("client B completing A's intent = %q, want claim conflict", out)
	}

	out, isErr = callModern(t, url, "start_intent", map[string]any{"node_id": 10, "owner": "client-a"})
	if isErr || !strings.Contains(out, "renewed") {
		t.Fatalf("client A renewal = %q", out)
	}
	if got := st.owner(10); got != "owner:client-a" {
		t.Errorf("stored owner = %q, want owner:client-a", got)
	}
}

func TestLegacyHTTPSessionsHoldDistinctClaims(t *testing.T) {
	t.Parallel()
	st := &claimStore{
		nodes:  map[int64]*tools.Node{10: {ID: 10, Kind: tools.KindIntent, State: "open"}},
		claims: map[int64]tools.IntentClaim{},
	}
	reg := tools.NewRegistry()
	tools.RegisterIntentTools(reg, st, 1)
	srv := mcpserver.New(reg)
	ts := httptest.NewServer(server.NewStreamableHTTPServer(srv.MCPServer()))
	t.Cleanup(ts.Close)
	addr := strings.TrimPrefix(ts.URL, "http://")

	call := func(sid string) (string, bool) {
		_, rr := doRPC(t, addr, sid, 2, "tools/call", map[string]any{
			"name": "start_intent", "arguments": map[string]any{"node_id": 10},
		})
		var res mcp.CallToolResult
		if rr.Error != nil || json.Unmarshal(rr.Result, &res) != nil || len(res.Content) == 0 {
			t.Fatalf("start_intent: %+v", rr)
		}
		text, _ := mcp.AsTextContent(res.Content[0])
		return text.Text, res.IsError
	}
	a, b := initSession(t, addr), initSession(t, addr)

	if out, isErr := call(a); isErr || strings.Contains(out, a) {
		t.Fatalf("session A claim = %q", out)
	}
	if out, isErr := call(b); !isErr || strings.Contains(out, a) {
		t.Fatalf("session B on A's intent = %q, want conflict without A's session ID", out)
	}
	if got := st.owner(10); got != a {
		t.Errorf("stored owner = %q, want session %q", got, a)
	}
}
