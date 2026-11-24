package api

import (
	"errors"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const agentsDir = "agents"

var (
	agentIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	slugStrip      = regexp.MustCompile(`[^a-z0-9]+`)
	agentsMu       sync.Mutex
)

type AgentPreset struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"system_prompt"`
	Model        string    `json:"model"`
	AllowedTools []string  `json:"allowed_tools"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type agentInput struct {
	Name         *string   `json:"name"`
	Description  *string   `json:"description"`
	SystemPrompt *string   `json:"system_prompt"`
	Model        *string   `json:"model"`
	AllowedTools *[]string `json:"allowed_tools"`
}

func (a *API) registerAgentsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agents", a.handleListAgents)
	mux.HandleFunc("POST /api/agents", a.handleCreateAgent)
	mux.HandleFunc("GET /api/agents/{id}", a.handleGetAgent)
	mux.HandleFunc("PATCH /api/agents/{id}", a.handleUpdateAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", a.handleDeleteAgent)
}

func slugify(name string) string {
	return strings.Trim(slugStrip.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func agentPath(id string) string {
	return agentsDir + "/" + id + ".json"
}

func cleanTools(tools []string) []string {
	out := make([]string, 0, len(tools))
	seen := map[string]bool{}
	for _, t := range tools {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func validAgentModel(m string) bool {
	return validModels[m]
}

func defaultAgents() []AgentPreset {
	return []AgentPreset{
		{
			Name:         "Pentest Lead",
			Description:  "Orchestrates the engagement, tracks progress, and dispatches work to specialists",
			SystemPrompt: "You are the lead pentester for this ARTEX engagement. When a conversation starts, call graph_overview to understand the current state, then ask the user which target or technique to focus on. Be direct and task-oriented: do not make small talk. Assess coverage, prioritize open intents, and dispatch focused work for them. Use the MCP tools to track assets, findings, and evidence, and prove goals when evidence exists. Stay within the engagement constraints at all times.",
			Model:        "opus",
			AllowedTools: []string{"mcp__artex__*", "Bash", "Read"},
		},
		{
			Name:         "Recon Specialist",
			Description:  "Reconnaissance, asset discovery, and port scanning",
			SystemPrompt: "You are a reconnaissance specialist. Enumerate the target's attack surface: subdomains, hosts, open ports, services, and technologies. Record every discovery with insert_asset and keep notes on how it was found. Prefer passive techniques first, and stay within scope.",
			Model:        "sonnet",
			AllowedTools: []string{"mcp__artex__insert_asset", "mcp__artex__graph_overview", "Bash", "Read"},
		},
		{
			Name:         "Web Tester",
			Description:  "Web application testing: XSS, SQL injection, and authentication bypass",
			SystemPrompt: "You are a web application security tester. Test discovered web surfaces for XSS, SQL injection, authentication and authorization bypass, and other OWASP Top 10 issues. Confirm each issue with a minimal proof of concept, capture evidence, and record findings with clear reproduction steps.",
			Model:        "sonnet",
			AllowedTools: []string{"mcp__artex__*", "Bash", "Read", "WebFetch"},
		},
	}
}

func (a *API) seedAgents() error {
	names, err := a.files.ListDir(agentsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(names) > 0 {
		return nil
	}
	now := time.Now().UTC()
	for i, p := range defaultAgents() {
		p.ID = slugify(p.Name)
		p.CreatedAt = now.Add(time.Duration(i) * time.Millisecond)
		p.UpdatedAt = p.CreatedAt
		if err := a.files.WriteJSON(agentPath(p.ID), p); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) listAgents() ([]AgentPreset, error) {
	if err := a.seedAgents(); err != nil {
		return nil, err
	}
	names, err := a.files.ListDir(agentsDir)
	if err != nil {
		return nil, err
	}
	out := []AgentPreset{}
	for _, n := range names {
		id, ok := strings.CutSuffix(n, ".json")
		if !ok {
			continue
		}
		var p AgentPreset
		if err := a.files.ReadJSON(agentPath(id), &p); err != nil {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (a *API) loadAgent(w http.ResponseWriter, id string) (AgentPreset, bool) {
	var p AgentPreset
	if !agentIDPattern.MatchString(id) {
		writeError(w, http.StatusNotFound, "agent not found")
		return p, false
	}
	if err := a.files.ReadJSON(agentPath(id), &p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "agent not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return p, false
	}
	return p, true
}

func (a *API) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agentsMu.Lock()
	defer agentsMu.Unlock()
	ps, err := a.listAgents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (a *API) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	if p, ok := a.loadAgent(w, r.PathValue("id")); ok {
		writeJSON(w, http.StatusOK, p)
	}
}

func (a *API) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var in agentInput
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if in.Name == nil || strings.TrimSpace(*in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	base := slugify(*in.Name)
	if base == "" {
		writeError(w, http.StatusBadRequest, "name must contain letters or digits")
		return
	}
	p := AgentPreset{AllowedTools: []string{}}
	if !applyAgentInput(w, &p, in) {
		return
	}

	agentsMu.Lock()
	defer agentsMu.Unlock()
	if err := a.seedAgents(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	id := base
	for n := 2; ; n++ {
		err := a.files.ReadJSON(agentPath(id), &AgentPreset{})
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		id = base + "-" + strconv.Itoa(n)
	}
	now := time.Now().UTC()
	p.ID, p.CreatedAt, p.UpdatedAt = id, now, now
	if err := a.files.WriteJSON(agentPath(id), p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *API) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	var in agentInput
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name cannot be empty")
		return
	}
	agentsMu.Lock()
	defer agentsMu.Unlock()
	p, ok := a.loadAgent(w, r.PathValue("id"))
	if !ok {
		return
	}
	if !applyAgentInput(w, &p, in) {
		return
	}
	p.UpdatedAt = time.Now().UTC()
	if err := a.files.WriteJSON(agentPath(p.ID), p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	agentsMu.Lock()
	defer agentsMu.Unlock()
	p, ok := a.loadAgent(w, r.PathValue("id"))
	if !ok {
		return
	}
	if err := a.files.Delete(agentPath(p.ID)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func applyAgentInput(w http.ResponseWriter, p *AgentPreset, in agentInput) bool {
	if in.Name != nil {
		p.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if in.SystemPrompt != nil {
		p.SystemPrompt = *in.SystemPrompt
	}
	if in.Model != nil {
		if !validAgentModel(*in.Model) {
			writeError(w, http.StatusBadRequest, "invalid model")
			return false
		}
		p.Model = *in.Model
	}
	if in.AllowedTools != nil {
		p.AllowedTools = cleanTools(*in.AllowedTools)
	}
	return true
}
