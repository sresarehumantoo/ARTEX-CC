package store

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type TaskScope struct {
	ID          int64  `json:"id"`
	TaskID      int64  `json:"task_id"`
	Kind        string `json:"kind"`
	CompanyID   *int64 `json:"company_id,omitempty"`
	CompanyName string `json:"company_name,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Net         string `json:"net,omitempty"`
	Value       string `json:"value,omitempty"`
	Source      string `json:"source"`
	Reason      string `json:"reason,omitempty"`
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

func (s *Store) findExistingScope(taskID int64, ts TaskScope) (TaskScope, error) {
	row := s.QueryRow(`
		SELECT ts.id, ts.task_id, ts.kind, ts.company_id, COALESCE(c.name, ''),
		       ts.domain, COALESCE(ts.net::text, ''), ts.value, ts.source,
		       COALESCE(ts.reason, ''), ts.created_at
		FROM task_scope ts
		LEFT JOIN companies c ON c.id = ts.company_id
		WHERE ts.task_id = $1
		  AND ts.kind = $2
		  AND COALESCE(ts.domain, '') = COALESCE($3, '')
		  AND COALESCE(ts.net::text, '') = COALESCE($4, '')
		  AND COALESCE(ts.company_id, 0) = COALESCE($5, 0)
		  AND COALESCE(ts.value, '') = COALESCE($6, '')`,
		taskID, ts.Kind, nilIfEmpty(ts.Domain), nilIfEmpty(ts.Net),
		ts.CompanyID, nilIfEmpty(ts.Value),
	)
	return scanTaskScope(row)
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

func (s *Store) listTaskScopeInternal(taskID int64) ([]TaskScope, error) {
	rows, err := s.Query(`
		SELECT ts.id, ts.task_id, ts.kind, ts.company_id, COALESCE(c.name, ''),
		       ts.domain, COALESCE(ts.net::text, ''), ts.value, ts.source,
		       COALESCE(ts.reason, ''), ts.created_at
		FROM task_scope ts
		LEFT JOIN companies c ON c.id = ts.company_id
		WHERE ts.task_id = $1
		ORDER BY ts.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskScope
	for rows.Next() {
		ts, err := scanTaskScope(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ts)
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

func scanTaskScope(row interface{ Scan(...any) error }) (TaskScope, error) {
	var ts TaskScope
	var companyID sql.NullInt64
	var domain, netStr, value, reason sql.NullString
	err := row.Scan(
		&ts.ID, &ts.TaskID, &ts.Kind, &companyID, &ts.CompanyName,
		&domain, &netStr, &value, &ts.Source, &reason, &ts.CreatedAt,
	)
	if err != nil {
		return ts, err
	}
	if companyID.Valid {
		ts.CompanyID = &companyID.Int64
	}
	ts.Domain = domain.String
	ts.Net = netStr.String
	ts.Value = value.String
	ts.Reason = reason.String
	return ts, nil
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
