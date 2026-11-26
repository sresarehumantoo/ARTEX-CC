package store

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type Company struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	NKey      string    `json:"nkey"`
	Logo      string    `json:"logo"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScopeRule struct {
	ID        int64     `json:"id"`
	CompanyID int64     `json:"company_id"`
	Kind      string    `json:"kind"`
	Domain    string    `json:"domain,omitempty"`
	Net       string    `json:"net,omitempty"`
	Value     string    `json:"value,omitempty"`
	Raw       string    `json:"raw"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func companyNKey(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(name))), " ")
}

func (s *Store) ListCompanies(ctx context.Context) ([]tools.Company, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT c.id, c.name,
			COALESCE((SELECT COUNT(*) FROM assets a WHERE a.company_id = c.id), 0),
			COALESCE(
				(SELECT array_agg(COALESCE(cs.domain, cs.net::text, cs.value, cs.raw))
				 FROM company_scope cs WHERE cs.company_id = c.id),
				'{}'::text[]
			)
		FROM companies c ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.Company
	for rows.Next() {
		var c tools.Company
		var scope stringArray
		if err := rows.Scan(&c.ID, &c.Name, &c.AssetCount, &scope); err != nil {
			return nil, err
		}
		c.Scope = []string(scope)
		if c.Scope == nil {
			c.Scope = []string{}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetOrCreateCompany(ctx context.Context, name, logo string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("company name is required")
	}
	nkey := companyNKey(name)
	if _, err := s.ExecContext(ctx,
		`INSERT INTO companies(name, nkey, logo) VALUES ($1, $2, NULLIF($3,'')) ON CONFLICT (nkey) DO UPDATE SET logo = COALESCE(NULLIF(EXCLUDED.logo,''), companies.logo)`,
		name, nkey, logo); err != nil {
		return 0, err
	}
	var id int64
	err := s.QueryRowContext(ctx, `SELECT id FROM companies WHERE nkey = $1`, nkey).Scan(&id)
	return id, err
}

func (s *Store) AddCompanyScope(ctx context.Context, companyID int64, lines []string, reason string) (added, skipped, invalid int, errors []string, err error) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kind := classifyScopeLine(line)
		parsed, parseErr := ParseScopeLine(kind, line)
		if parseErr != nil {
			invalid++
			errors = append(errors, fmt.Sprintf("%s: %v", line, parseErr))
			continue
		}
		parsed.CompanyID = companyID
		parsed.Raw = line
		parsed.Reason = reason

		var insertErr error
		switch parsed.Kind {
		case "domain":
			_, insertErr = s.ExecContext(ctx, `
				INSERT INTO company_scope(company_id, kind, domain, raw, reason)
				VALUES ($1, $2, $3, $4, NULLIF($5,''))
				ON CONFLICT (company_id, domain) DO NOTHING`,
				companyID, parsed.Kind, parsed.Domain, line, reason)
		case "ip", "cidr":
			_, insertErr = s.ExecContext(ctx, `
				INSERT INTO company_scope(company_id, kind, net, raw, reason)
				VALUES ($1, $2, $3::cidr, $4, NULLIF($5,''))
				ON CONFLICT (company_id, net) DO NOTHING`,
				companyID, parsed.Kind, parsed.Net, line, reason)
		case "keyword":
			_, insertErr = s.ExecContext(ctx, `
				INSERT INTO company_scope(company_id, kind, value, raw, reason)
				VALUES ($1, $2, $3, $4, NULLIF($5,''))
				ON CONFLICT (company_id, kind, value) WHERE kind IN ('icp','keyword') DO NOTHING`,
				companyID, parsed.Kind, parsed.Value, line, reason)
		default:
			invalid++
			errors = append(errors, fmt.Sprintf("%s: unsupported kind %s", line, parsed.Kind))
			continue
		}
		if insertErr != nil {
			return added, skipped, invalid, errors, insertErr
		}
		added++
	}
	return
}

func classifyScopeLine(line string) string {
	line = strings.TrimSpace(line)
	if _, _, err := net.ParseCIDR(line); err == nil {
		return "cidr"
	}
	if net.ParseIP(line) != nil {
		return "ip"
	}
	if !strings.ContainsAny(line, " \t") && strings.Contains(line, ".") {
		return "domain"
	}
	return "keyword"
}

func (s *Store) ListCompanyScope(companyID int64) ([]ScopeRule, error) {
	rows, err := s.Query(`
		SELECT id, company_id, kind, COALESCE(domain,''), COALESCE(net::text,''), COALESCE(value,''), raw, COALESCE(reason,''), created_at
		FROM company_scope WHERE company_id = $1 ORDER BY id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScopeRule
	for rows.Next() {
		var r ScopeRule
		if err := rows.Scan(&r.ID, &r.CompanyID, &r.Kind, &r.Domain, &r.Net, &r.Value, &r.Raw, &r.Reason, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func ParseScopeLine(kind, value string) (*ScopeRule, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("scope value is required")
	}
	r := &ScopeRule{Kind: kind, Raw: value}
	switch kind {
	case "domain":
		d := strings.ToLower(strings.TrimSuffix(value, "."))
		if base := strings.TrimPrefix(d, "*."); base == "" || strings.ContainsAny(base, " \t/*") {
			return nil, fmt.Errorf("invalid domain: %s", value)
		}
		r.Domain = d
	case "ip":
		ip := net.ParseIP(value)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP: %s", value)
		}
		if ip.To4() != nil {
			r.Net = value + "/32"
		} else {
			r.Net = value + "/128"
		}
	case "cidr":
		_, _, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR: %s", value)
		}
		r.Net = value
	case "icp", "keyword":
		r.Value = value
	default:
		return nil, fmt.Errorf("unsupported scope kind: %s", kind)
	}
	return r, nil
}
