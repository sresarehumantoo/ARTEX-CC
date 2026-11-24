package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func newLLMTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()), WithClaudeBin("/nonexistent/claude"))
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func TestLLMSettingsDefaults(t *testing.T) {
	srv := newLLMTestServer(t)
	code, body := agentsDo(t, srv, "GET", "/api/llm/settings", "")
	var s LLMSettings
	if err := json.Unmarshal(body, &s); err != nil || code != 200 {
		t.Fatalf("code=%d err=%v body=%s", code, err, body)
	}
	if s != defaultLLMSettings() {
		t.Fatalf("settings = %+v", s)
	}
}

func TestLLMSettingsUpdate(t *testing.T) {
	srv := newLLMTestServer(t)
	in := `{"backend":"claude","default_model":"haiku","master_model":"opus","subagent_model":"haiku","subagent_strategy":"sequential","max_parallel_subagents":2,"max_turns":80,"max_conversations":6,"thinking":"high","global_system_prompt":"be careful"}`
	code, body := agentsDo(t, srv, "PUT", "/api/llm/settings", in)
	if code != 200 {
		t.Fatalf("put: code=%d body=%s", code, body)
	}
	code, body = agentsDo(t, srv, "GET", "/api/llm/settings", "")
	var s LLMSettings
	json.Unmarshal(body, &s)
	if code != 200 || s.MaxTurns != 80 || s.SubagentStrategy != "sequential" || s.Thinking != "high" || s.GlobalSystemPrompt != "be careful" || s.DefaultModel != "haiku" {
		t.Fatalf("get: code=%d %+v", code, s)
	}
}

func TestLLMSettingsPartialMerge(t *testing.T) {
	srv := newLLMTestServer(t)
	code, body := agentsDo(t, srv, "PUT", "/api/llm/settings", `{"max_turns":10}`)
	var s LLMSettings
	json.Unmarshal(body, &s)
	want := defaultLLMSettings()
	want.MaxTurns = 10
	if code != 200 || s != want {
		t.Fatalf("code=%d %+v", code, s)
	}
}

func TestLLMSettingsInvalid(t *testing.T) {
	srv := newLLMTestServer(t)
	for name, body := range map[string]string{
		"backend":         `{"backend":"codex"}`,
		"model":           `{"master_model":"gpt"}`,
		"max_turns":       `{"max_turns":500}`,
		"max_turns_low":   `{"max_turns":1}`,
		"strategy":        `{"subagent_strategy":"random"}`,
		"thinking":        `{"thinking":"extreme"}`,
		"max_conversions": `{"max_conversations":17}`,
		"json":            `{`,
	} {
		if code, _ := agentsDo(t, srv, "PUT", "/api/llm/settings", body); code != 400 {
			t.Errorf("%s: code=%d", name, code)
		}
	}
	_, got := agentsDo(t, srv, "GET", "/api/llm/settings", "")
	var s LLMSettings
	json.Unmarshal(got, &s)
	if s != defaultLLMSettings() {
		t.Fatalf("invalid PUT persisted: %+v", s)
	}
}

func TestLLMStatus(t *testing.T) {
	srv := newLLMTestServer(t)
	code, body := agentsDo(t, srv, "GET", "/api/llm/status", "")
	var st LLMStatus
	if err := json.Unmarshal(body, &st); err != nil || code != 200 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if len(st.Backends) != 1 || st.Backends[0].ID != "claude" || st.Backends[0].Available || !st.Backends[0].Enabled {
		t.Fatalf("backends = %+v", st.Backends)
	}
	if len(st.Models) != len(modelCatalog) || st.Models[0].ID != "opus" {
		t.Fatalf("models = %+v", st.Models)
	}
}

func TestValidAgentModelVersioned(t *testing.T) {
	for _, m := range []string{"", "opus", "fable", "claude-opus-5-5", "claude-opus-5", "claude-opus-4-8", "claude-sonnet-4-6"} {
		if !validAgentModel(m) {
			t.Errorf("expected %q valid", m)
		}
	}
	if validAgentModel("claude-bogus-1-0") {
		t.Error("expected unknown model invalid")
	}
}
