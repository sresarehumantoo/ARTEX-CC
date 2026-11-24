package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
)

const llmSettingsPath = "settings/llm.json"

var llmSettingsMu sync.Mutex

type LLMSettings struct {
	Backend              string `json:"backend"`
	DefaultModel         string `json:"default_model"`
	MasterModel          string `json:"master_model"`
	SubagentModel        string `json:"subagent_model"`
	SubagentStrategy     string `json:"subagent_strategy"`
	MaxParallelSubagents int    `json:"max_parallel_subagents"`
	MaxTurns             int    `json:"max_turns"`
	MaxConversations     int    `json:"max_conversations"`
	Thinking             string `json:"thinking"`
	GlobalSystemPrompt   string `json:"global_system_prompt"`
}

type LLMStatus struct {
	Backends []BackendStatus `json:"backends"`
	Models   []ModelOption   `json:"models"`
}

type BackendStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
	Enabled   bool   `json:"enabled"`
}

type ModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group"`
}

var modelCatalog = []ModelOption{
	{ID: "opus", Label: "Opus (latest)", Group: "Aliases"},
	{ID: "sonnet", Label: "Sonnet (latest)", Group: "Aliases"},
	{ID: "haiku", Label: "Haiku (latest)", Group: "Aliases"},
	{ID: "fable", Label: "Fable (latest)", Group: "Aliases"},

	{ID: "claude-mythos-5-2", Label: "Mythos 5.2", Group: "Mythos"},
	{ID: "claude-mythos-5-1", Label: "Mythos 5.1", Group: "Mythos"},

	{ID: "claude-fable-5-2", Label: "Fable 5.2", Group: "Fable"},
	{ID: "claude-fable-5-1", Label: "Fable 5.1", Group: "Fable"},
	{ID: "claude-fable-5", Label: "Fable 5", Group: "Fable"},

	{ID: "claude-opus-5-5", Label: "Opus 5.5", Group: "Claude 5.5"},
	{ID: "claude-sonnet-5-5", Label: "Sonnet 5.5", Group: "Claude 5.5"},
	{ID: "claude-haiku-5-5", Label: "Haiku 5.5", Group: "Claude 5.5"},

	{ID: "claude-opus-5", Label: "Opus 5", Group: "Claude 5"},
	{ID: "claude-sonnet-5", Label: "Sonnet 5", Group: "Claude 5"},

	{ID: "claude-opus-4-8", Label: "Opus 4.8", Group: "Claude 4"},
	{ID: "claude-opus-4-7", Label: "Opus 4.7", Group: "Claude 4"},
	{ID: "claude-opus-4-6", Label: "Opus 4.6", Group: "Claude 4"},
	{ID: "claude-sonnet-4-6", Label: "Sonnet 4.6", Group: "Claude 4"},
	{ID: "claude-haiku-4-5", Label: "Haiku 4.5", Group: "Claude 4"},
}

var validModels = func() map[string]bool {
	m := map[string]bool{"": true}
	for _, o := range modelCatalog {
		m[o.ID] = true
	}
	return m
}()

func (a *API) registerLLMSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/llm/settings", a.handleGetLLMSettings)
	mux.HandleFunc("PUT /api/llm/settings", a.handleUpdateLLMSettings)
	mux.HandleFunc("GET /api/llm/status", a.handleGetLLMStatus)
}

func defaultLLMSettings() LLMSettings {
	return LLMSettings{
		Backend:              "claude",
		MasterModel:          "opus",
		SubagentModel:        "sonnet",
		SubagentStrategy:     "parallel",
		MaxParallelSubagents: 3,
		MaxTurns:             50,
		MaxConversations:     4,
		Thinking:             "default",
		GlobalSystemPrompt:   "You are an autonomous pentesting assistant driving an ARTEX engagement through ARTEX-CC. Use the MCP tools (graph_overview, insert_asset, insert_finding, etc.) to track assets, findings, and coverage. When a conversation starts, even with a greeting, orient to the engagement by calling graph_overview, then ask what target or technique to focus on. Be direct and task-focused, and skip small talk.",
	}
}

func validateLLMSettings(s *LLMSettings) error {
	if s.Backend != "claude" {
		return fmt.Errorf("unsupported backend %q", s.Backend)
	}
	for name, m := range map[string]string{
		"default_model":  s.DefaultModel,
		"master_model":   s.MasterModel,
		"subagent_model": s.SubagentModel,
	} {
		if !validAgentModel(m) {
			return fmt.Errorf("invalid %s %q", name, m)
		}
	}
	switch s.SubagentStrategy {
	case "parallel", "sequential", "per_agent":
	default:
		return fmt.Errorf("invalid subagent_strategy %q", s.SubagentStrategy)
	}
	if s.MaxParallelSubagents < 1 || s.MaxParallelSubagents > 10 {
		return errors.New("max_parallel_subagents must be between 1 and 10")
	}
	if s.MaxTurns < 5 || s.MaxTurns > 200 {
		return errors.New("max_turns must be between 5 and 200")
	}
	if s.MaxConversations < 1 || s.MaxConversations > 16 {
		return errors.New("max_conversations must be between 1 and 16")
	}
	switch s.Thinking {
	case "default", "off", "low", "medium", "high":
	default:
		return fmt.Errorf("invalid thinking %q", s.Thinking)
	}
	return nil
}

func (a *API) loadLLMSettings() (LLMSettings, error) {
	s := defaultLLMSettings()
	if err := a.files.ReadJSON(llmSettingsPath, &s); err != nil && !errors.Is(err, os.ErrNotExist) {
		return defaultLLMSettings(), err
	}
	return s, nil
}

func (a *API) llmSettings() LLMSettings {
	s, _ := a.loadLLMSettings()
	return s
}

func (a *API) handleGetLLMSettings(w http.ResponseWriter, r *http.Request) {
	s, err := a.loadLLMSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *API) handleUpdateLLMSettings(w http.ResponseWriter, r *http.Request) {
	s := defaultLLMSettings()
	if err := decodeBody(r, &s); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateLLMSettings(&s); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	llmSettingsMu.Lock()
	defer llmSettingsMu.Unlock()
	if err := a.files.WriteJSON(llmSettingsPath, s); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *API) handleGetLLMStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, LLMStatus{
		Backends: []BackendStatus{a.claudeBackendStatus(r.Context())},
		Models:   modelCatalog,
	})
}

func (a *API) claudeBackendStatus(ctx context.Context) BackendStatus {
	st := a.claudeStatus(ctx)
	b := BackendStatus{ID: "claude", Name: "Claude Code", Enabled: true}
	b.Available, _ = st["available"].(bool)
	b.Path, _ = st["path"].(string)
	b.Version, _ = st["version"].(string)
	b.Error, _ = st["error"].(string)
	return b
}
