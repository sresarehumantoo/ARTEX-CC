package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	interceptRulesPath = "intercept_rules.json"
	approvalsPath      = "approvals.json"
	approvalsTopic     = "approvals"
)

var interceptMu sync.Mutex

type InterceptRule struct {
	ID          string    `json:"id"`
	Pattern     string    `json:"pattern"`
	Scope       string    `json:"scope"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

type ruleInput struct {
	Pattern     *string `json:"pattern"`
	Scope       *string `json:"scope"`
	Action      *string `json:"action"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
}

type Approval struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversation_id"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	RuleID         string          `json:"rule_id"`
	Status         string          `json:"status"`
	DecidedBy      string          `json:"decided_by"`
	DecidedAt      string          `json:"decided_at"`
	CreatedAt      time.Time       `json:"created_at"`
}

func (a *API) registerInterceptRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/intercept/rules", a.handleListRules)
	mux.HandleFunc("POST /api/intercept/rules", a.handleCreateRule)
	mux.HandleFunc("PATCH /api/intercept/rules/{id}", a.handleUpdateRule)
	mux.HandleFunc("DELETE /api/intercept/rules/{id}", a.handleDeleteRule)
	mux.HandleFunc("GET /api/approvals", a.handleListApprovals)
	mux.HandleFunc("POST /api/approvals/{id}/decide", a.handleDecideApproval)
}

func newInterceptID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func seedRules() []InterceptRule {
	now := time.Now().UTC()
	seeds := []InterceptRule{
		{Pattern: "Bash", Scope: "tool_name", Action: "ask", Description: "Require approval for shell commands"},
		{Pattern: "mcp__artex__dismiss_finding", Scope: "tool_name", Action: "deny", Description: "Block auto-dismissing findings"},
		{Pattern: "mcp__artex__*", Scope: "tool_name", Action: "allow", Description: "Allow all ARTEX tools by default"},
	}
	for i := range seeds {
		seeds[i].ID = newInterceptID("r")
		seeds[i].Enabled = true
		seeds[i].CreatedAt = now.Add(time.Duration(i) * time.Millisecond)
	}
	return seeds
}

func (a *API) loadRules() ([]InterceptRule, error) {
	var rules []InterceptRule
	err := a.files.ReadJSON(interceptRulesPath, &rules)
	if errors.Is(err, os.ErrNotExist) {
		rules = seedRules()
		return rules, a.files.WriteJSON(interceptRulesPath, rules)
	}
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []InterceptRule{}
	}
	return rules, nil
}

func (a *API) handleListRules(w http.ResponseWriter, r *http.Request) {
	interceptMu.Lock()
	defer interceptMu.Unlock()
	rules, err := a.loadRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func applyRuleInput(w http.ResponseWriter, rule *InterceptRule, in ruleInput) bool {
	if in.Pattern != nil {
		rule.Pattern = strings.TrimSpace(*in.Pattern)
		if rule.Pattern == "" {
			writeError(w, http.StatusBadRequest, "pattern must not be empty")
			return false
		}
	}
	if in.Scope != nil {
		if *in.Scope != "tool_name" && *in.Scope != "input" {
			writeError(w, http.StatusBadRequest, "scope must be tool_name or input")
			return false
		}
		rule.Scope = *in.Scope
	}
	if in.Action != nil {
		if *in.Action != "allow" && *in.Action != "ask" && *in.Action != "deny" {
			writeError(w, http.StatusBadRequest, "action must be allow, ask, or deny")
			return false
		}
		rule.Action = *in.Action
	}
	if in.Description != nil {
		rule.Description = *in.Description
	}
	if in.Enabled != nil {
		rule.Enabled = *in.Enabled
	}
	return true
}

func (a *API) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var in ruleInput
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	rule := InterceptRule{ID: newInterceptID("r"), Scope: "tool_name", Enabled: true, CreatedAt: time.Now().UTC()}
	if !applyRuleInput(w, &rule, in) {
		return
	}
	if rule.Pattern == "" || rule.Action == "" {
		writeError(w, http.StatusBadRequest, "pattern and action are required")
		return
	}
	interceptMu.Lock()
	defer interceptMu.Unlock()
	rules, err := a.loadRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rules = append(rules, rule)
	if err := a.files.WriteJSON(interceptRulesPath, rules); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (a *API) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	var in ruleInput
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id := r.PathValue("id")
	interceptMu.Lock()
	defer interceptMu.Unlock()
	rules, err := a.loadRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range rules {
		if rules[i].ID != id {
			continue
		}
		if !applyRuleInput(w, &rules[i], in) {
			return
		}
		if err := a.files.WriteJSON(interceptRulesPath, rules); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rules[i])
		return
	}
	writeError(w, http.StatusNotFound, "rule not found")
}

func (a *API) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	interceptMu.Lock()
	defer interceptMu.Unlock()
	rules, err := a.loadRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range rules {
		if rules[i].ID != id {
			continue
		}
		rules = append(rules[:i], rules[i+1:]...)
		if err := a.files.WriteJSON(interceptRulesPath, rules); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, "rule not found")
}

func (a *API) loadApprovals() ([]Approval, error) {
	var list []Approval
	err := a.files.ReadJSON(approvalsPath, &list)
	if errors.Is(err, os.ErrNotExist) || (err == nil && list == nil) {
		return []Approval{}, nil
	}
	return list, err
}

func (a *API) handleListApprovals(w http.ResponseWriter, r *http.Request) {
	interceptMu.Lock()
	defer interceptMu.Unlock()
	list, err := a.loadApprovals()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filtered := []Approval{}
		for _, ap := range list {
			if ap.Status == status {
				filtered = append(filtered, ap)
			}
		}
		list = filtered
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleDecideApproval(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action string `json:"action"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var status string
	switch body.Action {
	case "approve":
		status = "approved"
	case "deny":
		status = "denied"
	default:
		writeError(w, http.StatusBadRequest, "action must be approve or deny")
		return
	}
	id := r.PathValue("id")
	interceptMu.Lock()
	defer interceptMu.Unlock()
	list, err := a.loadApprovals()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range list {
		if list[i].ID != id {
			continue
		}
		if list[i].Status != "pending" {
			writeError(w, http.StatusConflict, "approval already decided")
			return
		}
		list[i].Status = status
		list[i].DecidedBy = "web-ui"
		list[i].DecidedAt = time.Now().UTC().Format(time.RFC3339)
		if err := a.files.WriteJSON(approvalsPath, list); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if payload, err := json.Marshal(list[i]); err == nil {
			a.hub.Publish(approvalsTopic, payload)
		}
		writeJSON(w, http.StatusOK, list[i])
		return
	}
	writeError(w, http.StatusNotFound, "approval not found")
}
