package store

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type InterceptRule struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Enabled        bool      `json:"enabled"`
	Priority       int       `json:"priority"`
	MatchTarget    string    `json:"match_target"`
	MatchType      string    `json:"match_type"`
	Pattern        string    `json:"pattern"`
	Action         string    `json:"action"`
	Message        string    `json:"message"`
	TimeoutEnabled bool      `json:"timeout_enabled"`
	TimeoutSeconds int       `json:"timeout_seconds"`
	TimeoutAction  string    `json:"timeout_action"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type InterceptPending struct {
	ID        int64           `json:"id"`
	RuleID    *int64          `json:"rule_id"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Status    string          `json:"status"`
	Reason    string          `json:"reason"`
	DecidedAt *time.Time      `json:"decided_at"`
	CreatedAt time.Time       `json:"created_at"`
}

const interceptRuleCols = `id, name, enabled, priority, match_target, match_type, pattern, action, message, timeout_enabled, timeout_seconds, timeout_action, created_at, updated_at`

func scanInterceptRule(row interface{ Scan(...any) error }) (InterceptRule, error) {
	var r InterceptRule
	err := row.Scan(&r.ID, &r.Name, &r.Enabled, &r.Priority,
		&r.MatchTarget, &r.MatchType, &r.Pattern, &r.Action, &r.Message,
		&r.TimeoutEnabled, &r.TimeoutSeconds, &r.TimeoutAction,
		&r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func (s *Store) ListInterceptRules() ([]InterceptRule, error) {
	rows, err := s.Query(`SELECT ` + interceptRuleCols + ` FROM intercept_rules ORDER BY priority DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InterceptRule
	for rows.Next() {
		r, err := scanInterceptRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddInterceptRule(r *InterceptRule) error {
	return s.QueryRow(`
INSERT INTO intercept_rules(name, enabled, priority, match_target, match_type, pattern, action, message, timeout_enabled, timeout_seconds, timeout_action)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, created_at, updated_at`,
		r.Name, r.Enabled, r.Priority, r.MatchTarget, r.MatchType, r.Pattern, r.Action, r.Message,
		r.TimeoutEnabled, r.TimeoutSeconds, r.TimeoutAction).
		Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
}

func (s *Store) DeleteInterceptRule(id int64) error {
	_, err := s.Exec(`DELETE FROM intercept_rules WHERE id=$1`, id)
	return err
}

func (s *Store) CheckIntercept(toolName string, toolInput json.RawMessage) (*InterceptPending, error) {
	rules, err := s.ListInterceptRules()
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if !matchesRule(r, toolName, toolInput) {
			continue
		}
		if r.Action == "allow" {
			return nil, nil
		}
		status := "pending"
		if r.Action == "deny" {
			status = "denied"
		}
		raw := toolInput
		if len(raw) == 0 {
			raw = json.RawMessage("{}")
		}
		var p InterceptPending
		err := s.QueryRow(`
INSERT INTO intercept_pending(rule_id, tool_name, tool_input, status, reason)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, rule_id, tool_name, tool_input, status, reason, decided_at, created_at`,
			r.ID, toolName, raw, status, r.Message).
			Scan(&p.ID, &p.RuleID, &p.ToolName, &p.ToolInput, &p.Status, &p.Reason, &p.DecidedAt, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		if status == "denied" {
			now := time.Now()
			p.DecidedAt = &now
			_, _ = s.Exec(`UPDATE intercept_pending SET decided_at=now() WHERE id=$1`, p.ID)
		}
		return &p, nil
	}
	return nil, nil
}

func (s *Store) DecideIntercept(id int64, status string) error {
	_, err := s.Exec(`UPDATE intercept_pending SET status=$2, decided_at=now() WHERE id=$1`, id, status)
	return err
}

func matchesRule(r InterceptRule, toolName string, toolInput json.RawMessage) bool {
	var subject string
	switch r.MatchTarget {
	case "tool_name":
		subject = toolName
	case "tool_input":
		subject = string(toolInput)
	default:
		return false
	}
	switch r.MatchType {
	case "string":
		return strings.Contains(strings.ToLower(subject), strings.ToLower(r.Pattern))
	case "regex":
		matched, err := regexp.MatchString(r.Pattern, subject)
		return err == nil && matched
	}
	return false
}
