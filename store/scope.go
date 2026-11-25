package store

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type TaskScope struct {
	ID          int64     `json:"id"`
	TaskID      int64     `json:"task_id"`
	Kind        string    `json:"kind"`
	CompanyID   *int64    `json:"company_id,omitempty"`
	CompanyName string    `json:"company_name,omitempty"`
	Domain      string    `json:"domain,omitempty"`
	Net         string    `json:"net,omitempty"`
	Value       string    `json:"value,omitempty"`
	Source      string    `json:"source"`
	Reason      string    `json:"reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Store) AddTaskScope(ctx context.Context, taskID int64, kind, value, reason, source string) error {
	if source == "" {
		source = "agent"
	}
	ts := TaskScope{TaskID: taskID, Kind: kind, Source: source, Reason: reason}
	if taskID <= 0 {
		return fmt.Errorf("task_id is required")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value must not be empty")
	}

	switch kind {
	case "company":
		var companyID int64
		if id, err := strconv.ParseInt(value, 10, 64); err == nil {
			err = s.QueryRowContext(ctx, `SELECT id FROM companies WHERE id = $1`, id).Scan(&companyID)
			if err == sql.ErrNoRows {
				return fmt.Errorf("company not found: %s", value)
			} else if err != nil {
				return err
			}
		} else {
			nkey := companyNKey(value)
			err := s.QueryRowContext(ctx, `SELECT id FROM companies WHERE nkey = $1`, nkey).Scan(&companyID)
			if err == sql.ErrNoRows {
				return fmt.Errorf("company not found: %s", value)
			} else if err != nil {
				return err
			}
		}
		ts.CompanyID = &companyID
	case "root_domain":
		d := DomainKey(stripHostPort(value))
		root, _ := RootDomain(d)
		if root == "" {
			root = d
		}
		if root == "" {
			return fmt.Errorf("invalid root domain: %s", value)
		}
		ts.Domain = root
	case "subdomain":
		d := DomainKey(stripHostPort(value))
		if d == "" {
			return fmt.Errorf("invalid subdomain: %s", value)
		}
		ts.Domain = d
	case "ip":
		cidr := ipToHostCIDR(stripHostPort(value))
		if cidr == "" {
			return fmt.Errorf("invalid IP address: %s", value)
		}
		ts.Net = cidr
		ts.Kind = "ip"
	case "cidr":
		_, _, err := net.ParseCIDR(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("invalid CIDR: %s", value)
		}
		ts.Net = strings.TrimSpace(value)
	case "icp", "keyword":
		ts.Value = value
	default:
		return fmt.Errorf("unknown scope kind: %s", kind)
	}

	var domainVal, netVal, companyVal, valueVal any
	if ts.Domain != "" {
		domainVal = ts.Domain
	}
	if ts.Net != "" {
		netVal = ts.Net
	}
	if ts.CompanyID != nil && *ts.CompanyID > 0 {
		companyVal = *ts.CompanyID
	}
	if ts.Value != "" {
		valueVal = ts.Value
	}

	_, err := s.ExecContext(ctx, `
		INSERT INTO task_scope (task_id, kind, company_id, domain, net, value, source, reason)
		VALUES ($1, $2, $3, $4, $5::cidr, $6, $7, NULLIF($8, ''))
		ON CONFLICT DO NOTHING`,
		taskID, ts.Kind, companyVal, domainVal, netVal, valueVal, ts.Source, ts.Reason,
	)
	return err
}

func (s *Store) ListTaskScope(ctx context.Context, taskID int64) ([]int64, error) {
	rows, err := s.QueryContext(ctx, `SELECT id FROM task_scope WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ListScopeRules returns the task's own scope entries followed by the scope of
// the company the task belongs to, the same two sources IsInScope consults.
func (s *Store) ListScopeRules(ctx context.Context, taskID int64) ([]tools.ScopeRule, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT kind, value, reason FROM (
			SELECT 0 AS src, ts.id, ts.kind,
				COALESCE(ts.domain, ts.net::text, ts.value, c.name, '') AS value,
				COALESCE(ts.reason, '') AS reason
			FROM task_scope ts LEFT JOIN companies c ON c.id = ts.company_id
			WHERE ts.task_id = $1
			UNION ALL
			SELECT 1, cs.id, cs.kind,
				COALESCE(cs.domain, cs.net::text, cs.value, cs.raw),
				COALESCE(cs.reason, '')
			FROM company_scope cs JOIN tasks t ON t.company_id = cs.company_id
			WHERE t.id = $1
		) r ORDER BY src, id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []tools.ScopeRule{}
	for rows.Next() {
		var r tools.ScopeRule
		if err := rows.Scan(&r.Kind, &r.Value, &r.Reason); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteTaskScope(id, taskID int64) error {
	res, err := s.Exec(`DELETE FROM task_scope WHERE id = $1 AND task_id = $2`, id, taskID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("scope entry not found")
	}
	return nil
}

func stripHostPort(v string) string {
	v = strings.TrimSpace(v)
	if host, _, err := net.SplitHostPort(v); err == nil {
		return host
	}
	return v
}

func ipToHostCIDR(ip string) string {
	ip = strings.TrimSpace(ip)
	p := net.ParseIP(ip)
	if p == nil {
		return ""
	}
	if p.To4() != nil {
		return ip + "/32"
	}
	return ip + "/128"
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) IsInScope(ctx context.Context, taskID int64, target string) (bool, string, string, error) {
	host := extractScopeTarget(target)
	if host == "" {
		return false, "could not parse target", "", nil
	}

	ip := net.ParseIP(host)

	var rules []scopeCheckRule
	if taskID > 0 {
		taskRules, err := s.loadTaskScopeRules(ctx, taskID)
		if err != nil {
			return false, "", "", err
		}
		rules = append(rules, taskRules...)

		companyRules, err := s.loadCompanyScopeForTask(ctx, taskID)
		if err != nil {
			return false, "", "", err
		}
		rules = append(rules, companyRules...)
	}

	if len(rules) == 0 {
		return true, "no scope rules defined (permissive default)", "", nil
	}

	for _, r := range rules {
		if ip != nil && r.net != nil {
			if r.net.Contains(ip) {
				return true, "IP matches CIDR rule", r.raw, nil
			}
			continue
		}
		if ip == nil && r.domain != "" {
			if matchesDomain(host, r.domain) {
				return true, "domain matches scope rule", r.raw, nil
			}
		}
	}

	return false, "target does not match any scope rule", "", nil
}

type scopeCheckRule struct {
	domain string
	net    *net.IPNet
	raw    string
}

func (s *Store) loadTaskScopeRules(ctx context.Context, taskID int64) ([]scopeCheckRule, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT kind, COALESCE(domain,''), COALESCE(net::text,'')
		FROM task_scope WHERE task_id = $1`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scopeCheckRule
	for rows.Next() {
		var kind, domain, netStr string
		if err := rows.Scan(&kind, &domain, &netStr); err != nil {
			return nil, err
		}
		r := parseScopeCheckRule(kind, domain, netStr)
		if r != nil {
			out = append(out, *r)
		}
	}
	return out, rows.Err()
}

func (s *Store) loadCompanyScopeForTask(ctx context.Context, taskID int64) ([]scopeCheckRule, error) {
	var companyID sql.NullInt64
	err := s.QueryRowContext(ctx, `SELECT company_id FROM tasks WHERE id = $1`, taskID).Scan(&companyID)
	if err != nil || !companyID.Valid {
		return nil, err
	}

	rows, err := s.QueryContext(ctx, `
		SELECT kind, COALESCE(domain,''), COALESCE(net::text,'')
		FROM company_scope WHERE company_id = $1`, companyID.Int64)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scopeCheckRule
	for rows.Next() {
		var kind, domain, netStr string
		if err := rows.Scan(&kind, &domain, &netStr); err != nil {
			return nil, err
		}
		r := parseScopeCheckRule(kind, domain, netStr)
		if r != nil {
			out = append(out, *r)
		}
	}
	return out, rows.Err()
}

func parseScopeCheckRule(kind, domain, netStr string) *scopeCheckRule {
	switch kind {
	case "root_domain", "subdomain", "domain":
		if domain != "" {
			return &scopeCheckRule{domain: domain, raw: domain}
		}
	case "ip", "cidr":
		if netStr != "" {
			_, n, err := net.ParseCIDR(netStr)
			if err == nil {
				return &scopeCheckRule{net: n, raw: netStr}
			}
		}
	}
	return nil
}

func matchesDomain(target, rule string) bool {
	target = strings.ToLower(target)
	rule = strings.ToLower(rule)
	if target == rule {
		return true
	}
	return strings.HasSuffix(target, "."+rule)
}

func extractScopeTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if strings.Contains(target, "://") {
		if u, err := url.Parse(target); err == nil && u.Hostname() != "" {
			return strings.ToLower(u.Hostname())
		}
	}
	host, _, err := net.SplitHostPort(target)
	if err == nil {
		return strings.ToLower(host)
	}
	return strings.ToLower(target)
}
