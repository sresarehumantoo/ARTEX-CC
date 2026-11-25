package mcpserver_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestSessionManagerCreate(t *testing.T) {
	t.Parallel()
	m := mcpserver.NewSessionManager()
	s := m.Create()
	if s == nil {
		t.Fatal("Create returned nil")
	}
	if s.ID == "" {
		t.Fatal("session has empty ID")
	}
	if m.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", m.Count())
	}
	if m.Get(s.ID) != s {
		t.Fatal("Get did not return the same session")
	}
}

func TestSessionManagerGetOrCreate(t *testing.T) {
	t.Parallel()
	m := mcpserver.NewSessionManager()
	s1 := m.GetOrCreate("abc")
	s2 := m.GetOrCreate("abc")
	if s1 != s2 {
		t.Fatal("GetOrCreate returned different sessions for the same ID")
	}
	if m.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", m.Count())
	}
	if s1.ID != "abc" {
		t.Fatalf("expected ID abc, got %q", s1.ID)
	}
}

func TestSessionManagerRemove(t *testing.T) {
	t.Parallel()
	m := mcpserver.NewSessionManager()
	s := m.Create()
	if !m.Remove(s.ID) {
		t.Fatal("Remove returned false for existing session")
	}
	if m.Count() != 0 {
		t.Fatal("session not removed")
	}
	if m.Remove(s.ID) {
		t.Fatal("Remove returned true for already-removed session")
	}
}

func TestSessionManagerList(t *testing.T) {
	t.Parallel()
	m := mcpserver.NewSessionManager()
	s1 := m.Create()
	s2 := m.Create()

	infos := m.List()
	if len(infos) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(infos))
	}

	ids := map[string]bool{}
	for _, info := range infos {
		ids[info.ID] = true
	}
	if !ids[s1.ID] || !ids[s2.ID] {
		t.Fatal("List did not return both session IDs")
	}
}

func TestSessionContext(t *testing.T) {
	t.Parallel()
	m := mcpserver.NewSessionManager()
	s := m.Create()

	if v := s.Get("key"); v != "" {
		t.Fatalf("expected empty value, got %q", v)
	}
	s.Set("key", "value")
	if v := s.Get("key"); v != "value" {
		t.Fatalf("expected value, got %q", v)
	}
}

func TestSessionIsolation(t *testing.T) {
	t.Parallel()
	m := mcpserver.NewSessionManager()
	s1 := m.Create()
	s2 := m.Create()

	s1.Set("name", "alpha")
	s2.Set("name", "beta")

	if s1.Get("name") != "alpha" {
		t.Fatal("session 1 context leaked")
	}
	if s2.Get("name") != "beta" {
		t.Fatal("session 2 context leaked")
	}
}

func TestSessionManagerConcurrency(t *testing.T) {
	t.Parallel()
	m := mcpserver.NewSessionManager()
	const n = 50
	var wg sync.WaitGroup
	errs := make(chan string, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := m.Create()
			s.Set("idx", fmt.Sprintf("%d", idx))
			if v := s.Get("idx"); v != fmt.Sprintf("%d", idx) {
				errs <- fmt.Sprintf("session %s: expected %d, got %s", s.ID, idx, v)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if m.Count() != n {
		t.Fatalf("expected %d sessions, got %d", n, m.Count())
	}
}

func TestSessionSnapshotIsolated(t *testing.T) {
	t.Parallel()
	m := mcpserver.NewSessionManager()
	s := m.Create()
	s.Set("k", "v1")

	infos := m.List()
	if len(infos) != 1 {
		t.Fatal("expected 1 session in list")
	}
	if infos[0].Context["k"] != "v1" {
		t.Fatal("snapshot missing context value")
	}

	s.Set("k", "v2")
	if infos[0].Context["k"] != "v1" {
		t.Fatal("snapshot was mutated by later Set call")
	}
}

func TestListSessionsToolReturnsAllClients(t *testing.T) {
	t.Parallel()
	reg := tools.NewRegistry()
	reg.Register(pingTool())
	addr := startTestServer(t, reg)

	sid1 := initSession(t, addr)
	sid2 := initSession(t, addr)

	_, rr := doRPC(t, addr, sid1, 10, "tools/call", map[string]any{
		"name":      "list_sessions",
		"arguments": map[string]any{},
	})
	if rr.Error != nil {
		t.Fatalf("tools/call error: %s", rr.Error.Message)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("empty content")
	}
	text := result.Content[0].Text
	if strings.Contains(text, sid1) || strings.Contains(text, sid2) {
		t.Fatalf("list_sessions exposes raw session IDs: %s", text)
	}

	var payload struct {
		Count    int `json:"count"`
		Sessions []struct {
			Label string `json:"label"`
			Self  bool   `json:"self"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Count != 2 || len(payload.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %s", text)
	}
	labels := map[string]bool{}
	for _, s := range payload.Sessions {
		labels[s.Label] = s.Self
	}
	if self, ok := labels[tools.OwnerLabel(sid1)]; !ok || !self {
		t.Errorf("caller's own label missing or not marked self: %s", text)
	}
	if self, ok := labels[tools.OwnerLabel(sid2)]; !ok || self {
		t.Errorf("other session's label missing or marked self: %s", text)
	}
}

func TestListSessionsToolInToolsList(t *testing.T) {
	t.Parallel()
	reg := tools.NewRegistry()
	addr := startTestServer(t, reg)
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
		if tool.Name == "list_sessions" {
			found = true
			break
		}
	}
	if !found {
		t.Error("list_sessions not found in tools/list")
	}
}

func TestSessionsAccessor(t *testing.T) {
	t.Parallel()
	reg := tools.NewRegistry()
	srv := mcpserver.New(reg)
	if srv.Sessions() == nil {
		t.Fatal("Sessions() returned nil")
	}
	if srv.Sessions().Count() != 0 {
		t.Fatalf("expected 0 sessions, got %d", srv.Sessions().Count())
	}
}

func TestMultiClientSessionIsolation(t *testing.T) {
	t.Parallel()
	reg := tools.NewRegistry()
	addr := startTestServer(t, reg)

	const clients = 10
	var wg sync.WaitGroup
	errs := make(chan string, clients)

	for i := range clients {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sid := initSession(t, addr)
			if sid == "" {
				errs <- fmt.Sprintf("client %d: empty session id", n)
				return
			}
			_, rr := doRPC(t, addr, sid, 2, "tools/call", map[string]any{
				"name":      "list_sessions",
				"arguments": map[string]any{},
			})
			if rr.Error != nil {
				errs <- fmt.Sprintf("client %d: %s", n, rr.Error.Message)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
