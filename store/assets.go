package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"golang.org/x/net/publicsuffix"
)

const (
	AssetRootDomain = "root_domain"
	AssetIP         = "ip"
	AssetSubdomain  = "subdomain"
	AssetApp        = "app"
	AssetService    = "service"
	AssetEndpoint   = "endpoint"
)

type Asset struct {
	ID             int64           `json:"id"`
	Type           string          `json:"type"`
	CompanyID      *int64          `json:"company_id,omitempty"`
	TaskIDs        []int64         `json:"task_ids"`
	Domain         string          `json:"domain,omitempty"`
	RootDomain     string          `json:"root_domain,omitempty"`
	IP             string          `json:"ip,omitempty"`
	CSegment       *string         `json:"c_segment,omitempty"`
	Port           *int            `json:"port,omitempty"`
	ICP            string          `json:"icp,omitempty"`
	BoundDomains   []string        `json:"bound_domains,omitempty"`
	RecordType     string          `json:"record_type,omitempty"`
	RecordValue    []string        `json:"record_value,omitempty"`
	BundleID       string          `json:"bundle_id,omitempty"`
	AppName        string          `json:"app_name,omitempty"`
	Category       string          `json:"category,omitempty"`
	AppDescription string          `json:"app_description,omitempty"`
	AppICP         string          `json:"app_icp,omitempty"`
	URL            string          `json:"url,omitempty"`
	ServiceType    string          `json:"service_type,omitempty"`
	ServiceName    string          `json:"service_name,omitempty"`
	FaviconMMH3    string          `json:"favicon_mmh3,omitempty"`
	StatusCode     *int            `json:"status_code,omitempty"`
	ContentLength  *int64          `json:"content_length,omitempty"`
	PageTitle      string          `json:"page_title,omitempty"`
	Technologies   []string        `json:"technologies,omitempty"`
	Method         string          `json:"method,omitempty"`
	Extra          json.RawMessage `json:"extra,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	LastSeen       time.Time       `json:"last_seen"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type AssetInput struct {
	Type         string          `json:"type"`
	Domain       string          `json:"domain,omitempty"`
	IP           string          `json:"ip,omitempty"`
	Port         *int            `json:"port,omitempty"`
	URL          string          `json:"url,omitempty"`
	ServiceName  string          `json:"service_name,omitempty"`
	ServiceType  string          `json:"service_type,omitempty"`
	Method       string          `json:"method,omitempty"`
	BundleID     string          `json:"bundle_id,omitempty"`
	AppName      string          `json:"app_name,omitempty"`
	Technologies []string        `json:"technologies,omitempty"`
	Extra        json.RawMessage `json:"extra,omitempty"`
}

func DomainKey(fqdn string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(fqdn)), ".")
}

// NormalizeAssetURL is the canonical form asset URLs are stored and looked up
// by: userinfo removed, scheme and host lowercased, default ports dropped.
// Path, query and fragment are kept as given, and so is a Unicode host.
func NormalizeAssetURL(raw string) string {
	raw = tools.RedactURLUserinfo(strings.TrimSpace(raw))
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	start, end := tools.URLAuthority(raw)
	if start < 0 {
		return raw
	}
	prefix := "//"
	if scheme != "" {
		prefix = scheme + "://"
	}
	return prefix + host + raw[end:]
}

func RootDomain(host string) (root string, isApex bool) {
	h := DomainKey(host)
	if h == "" || net.ParseIP(h) != nil {
		return h, true
	}
	etld1, err := publicsuffix.EffectiveTLDPlusOne(h)
	if err != nil || etld1 == "" {
		return h, true
	}
	return etld1, h == etld1
}

func calcCSegment(ip string) *string {
	p := net.ParseIP(strings.TrimSpace(ip))
	if p == nil {
		return nil
	}
	p4 := p.To4()
	if p4 == nil {
		return nil
	}
	cidr := fmt.Sprintf("%d.%d.%d.0/24", p4[0], p4[1], p4[2])
	return &cidr
}

const assetSelectCols = `id, type, company_id, task_ids,
	COALESCE(domain,''), COALESCE(root_domain,''), COALESCE(ip,''), c_segment, port,
	COALESCE(icp,''), bound_domains,
	COALESCE(record_type,''), record_value,
	COALESCE(bundle_id,''), COALESCE(app_name,''), COALESCE(category,''), COALESCE(app_description,''), COALESCE(app_icp,''),
	COALESCE(url,''), COALESCE(service_type,''), COALESCE(service_name,''),
	COALESCE(favicon_mmh3,''), status_code, content_length, COALESCE(page_title,''),
	technologies, COALESCE(method,''), extra,
	created_at, last_seen, updated_at`

func scanAssets(rows *sql.Rows) ([]Asset, error) {
	var out []Asset
	for rows.Next() {
		var a Asset
		var taskIDs int64Array
		var boundDomains, recordValue, technologies stringArray
		var extra []byte
		if err := rows.Scan(
			&a.ID, &a.Type, &a.CompanyID, &taskIDs,
			&a.Domain, &a.RootDomain, &a.IP, &a.CSegment, &a.Port,
			&a.ICP, &boundDomains,
			&a.RecordType, &recordValue,
			&a.BundleID, &a.AppName, &a.Category, &a.AppDescription, &a.AppICP,
			&a.URL, &a.ServiceType, &a.ServiceName,
			&a.FaviconMMH3, &a.StatusCode, &a.ContentLength, &a.PageTitle,
			&technologies, &a.Method, &extra,
			&a.CreatedAt, &a.LastSeen, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		a.TaskIDs = []int64(taskIDs)
		a.BoundDomains = []string(boundDomains)
		a.RecordValue = []string(recordValue)
		a.Technologies = []string(technologies)
		if len(extra) > 0 {
			a.Extra = extra
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) InsertAsset(ctx context.Context, taskID int64, item tools.AssetInput) (int64, error) {
	internal := AssetInput{
		Type:         item.Type,
		Domain:       item.Domain,
		IP:           item.IP,
		URL:          item.URL,
		ServiceName:  item.ServiceName,
		Method:       item.Method,
		BundleID:     item.BundleID,
		AppName:      item.AppName,
		Technologies: []string{},
	}
	if item.Port > 0 {
		p := item.Port
		internal.Port = &p
	}
	if item.ServiceName != "" && item.URL == "" {
		internal.ServiceType = "other"
	} else if item.URL != "" && item.Type == AssetService {
		internal.ServiceType = "http"
	}
	extra := map[string]any{}
	if item.ICP != "" {
		extra["icp"] = item.ICP
	}
	if item.RecordType != "" {
		extra["record_type"] = item.RecordType
	}
	if len(item.RecordValue) > 0 {
		extra["record_value"] = item.RecordValue
	}
	if len(item.BoundDomains) > 0 {
		extra["bound_domains"] = item.BoundDomains
	}
	if len(item.OpenPorts) > 0 {
		extra["open_ports"] = item.OpenPorts
	}
	if item.Category != "" {
		extra["category"] = item.Category
	}
	if item.Description != "" {
		extra["description"] = item.Description
	}
	if item.AppICP != "" {
		extra["app_icp"] = item.AppICP
	}
	if item.CompanyID != nil {
		extra["company_id"] = *item.CompanyID
	}
	if len(item.Technologies) > 0 {
		internal.Technologies = item.Technologies
	}
	if item.StatusCode != nil {
		extra["status_code"] = *item.StatusCode
	}
	if item.ContentLength != nil {
		extra["content_length"] = *item.ContentLength
	}
	if item.PageTitle != "" {
		extra["page_title"] = item.PageTitle
	}
	if item.FaviconMMH3 != "" {
		extra["favicon_mmh3"] = item.FaviconMMH3
	}
	if len(item.Auth) > 0 {
		extra["auth"] = item.Auth
	}
	if item.ServiceIP != "" {
		if internal.IP == "" {
			internal.IP = item.ServiceIP
		}
	}
	if item.Proto != "" {
		extra["proto"] = item.Proto
	}
	if len(item.Params) > 0 {
		extra["params"] = item.Params
	}
	if len(extra) > 0 {
		raw, _ := json.Marshal(extra)
		internal.Extra = raw
	}

	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	id, err := insertAssetReturningID(tx, taskID, internal)
	if err != nil {
		return 0, fmt.Errorf("insert %s: %w", item.Type, err)
	}
	if err := s.saveAssetURLCredential(ctx, tx, taskID, internal.URL); err != nil {
		return 0, fmt.Errorf("save url credential: %w", err)
	}
	return id, tx.Commit()
}

func insertAssetReturningID(tx *sql.Tx, taskID int64, item AssetInput) (int64, error) {
	extra := json.RawMessage("{}")
	if len(item.Extra) > 0 {
		extra = item.Extra
	}
	item.URL = NormalizeAssetURL(item.URL)
	mergeTasks := `array(SELECT DISTINCT unnest(assets.task_ids || ARRAY[$1::bigint]))`

	var id int64
	switch item.Type {
	case AssetRootDomain:
		d := DomainKey(item.Domain)
		if d == "" {
			return 0, fmt.Errorf("root_domain requires domain")
		}
		err := tx.QueryRow(`
			INSERT INTO assets (type, domain, root_domain, task_ids, extra)
			VALUES ('root_domain', $2, $2, ARRAY[$1::bigint], $3)
			ON CONFLICT (domain) WHERE type='root_domain' DO UPDATE
			SET task_ids = `+mergeTasks+`, last_seen = now()
			RETURNING id`,
			taskID, d, extra).Scan(&id)
		return id, err

	case AssetIP:
		ip := strings.TrimSpace(item.IP)
		if ip == "" {
			return 0, fmt.Errorf("ip requires ip")
		}
		cseg := calcCSegment(ip)
		err := tx.QueryRow(`
			INSERT INTO assets (type, ip, c_segment, task_ids, extra)
			VALUES ('ip', $2, $3::cidr, ARRAY[$1::bigint], $4)
			ON CONFLICT (ip) WHERE type='ip' DO UPDATE
			SET task_ids = `+mergeTasks+`, last_seen = now()
			RETURNING id`,
			taskID, ip, cseg, extra).Scan(&id)
		return id, err

	case AssetSubdomain:
		d := DomainKey(item.Domain)
		if d == "" {
			return 0, fmt.Errorf("subdomain requires domain")
		}
		rd, _ := RootDomain(d)
		err := tx.QueryRow(`
			INSERT INTO assets (type, domain, root_domain, ip, task_ids, extra)
			VALUES ('subdomain', $2, $3, NULLIF($4,''), ARRAY[$1::bigint], $5)
			ON CONFLICT (domain, COALESCE(record_type,'')) WHERE type='subdomain' DO UPDATE
			SET task_ids = `+mergeTasks+`,
			    ip = COALESCE(NULLIF(EXCLUDED.ip,''), assets.ip),
			    last_seen = now()
			RETURNING id`,
			taskID, d, rd, strings.TrimSpace(item.IP), extra).Scan(&id)
		return id, err

	case AssetService:
		svcType := item.ServiceType
		if svcType == "" {
			svcType = "other"
		}
		if svcType == "http" {
			if item.URL == "" {
				return 0, fmt.Errorf("http service requires url")
			}
			err := tx.QueryRow(`
				INSERT INTO assets (type, service_type, url, domain, ip, port, technologies, task_ids, extra)
				VALUES ('service', 'http', $2, NULLIF($3,''), NULLIF($4,''), $5, $6::text[], ARRAY[$1::bigint], $7)
				ON CONFLICT (url) WHERE type='service' AND service_type='http' DO UPDATE
				SET task_ids = `+mergeTasks+`,
				    technologies = COALESCE((SELECT array_agg(DISTINCT t) FROM unnest(assets.technologies || EXCLUDED.technologies) t), '{}'),
				    last_seen = now()
				RETURNING id`,
				taskID, item.URL, DomainKey(item.Domain), strings.TrimSpace(item.IP), item.Port, stringArray(item.Technologies), extra).Scan(&id)
			return id, err
		}
		if item.ServiceName == "" {
			return 0, fmt.Errorf("other service requires service_name")
		}
		err := tx.QueryRow(`
			INSERT INTO assets (type, service_type, service_name, domain, ip, port, task_ids, extra)
			VALUES ('service', 'other', $2, NULLIF($3,''), NULLIF($4,''), $5, ARRAY[$1::bigint], $6)
			ON CONFLICT (COALESCE(domain,''), COALESCE(ip,''), port, service_name) WHERE type='service' AND service_type='other' DO UPDATE
			SET task_ids = `+mergeTasks+`, last_seen = now()
			RETURNING id`,
			taskID, item.ServiceName, DomainKey(item.Domain), strings.TrimSpace(item.IP), item.Port, extra).Scan(&id)
		return id, err

	case AssetEndpoint:
		if item.URL == "" || item.Method == "" {
			return 0, fmt.Errorf("endpoint requires url and method")
		}
		err := tx.QueryRow(`
			INSERT INTO assets (type, url, method, domain, ip, task_ids, extra)
			VALUES ('endpoint', $2, $3, NULLIF($4,''), NULLIF($5,''), ARRAY[$1::bigint], $6)
			ON CONFLICT (url, method) WHERE type='endpoint' DO UPDATE
			SET task_ids = `+mergeTasks+`, last_seen = now()
			RETURNING id`,
			taskID, item.URL, strings.ToUpper(item.Method), DomainKey(item.Domain), strings.TrimSpace(item.IP), extra).Scan(&id)
		return id, err

	case AssetApp:
		if item.BundleID != "" {
			err := tx.QueryRow(`
				INSERT INTO assets (type, bundle_id, app_name, domain, task_ids, extra)
				VALUES ('app', $2, NULLIF($3,''), NULLIF($4,''), ARRAY[$1::bigint], $5)
				ON CONFLICT (bundle_id) WHERE type='app' AND bundle_id IS NOT NULL DO UPDATE
				SET task_ids = `+mergeTasks+`,
				    app_name = COALESCE(NULLIF(EXCLUDED.app_name,''), assets.app_name),
				    last_seen = now()
				RETURNING id`,
				taskID, item.BundleID, item.AppName, DomainKey(item.Domain), extra).Scan(&id)
			return id, err
		}
		if item.AppName == "" {
			return 0, fmt.Errorf("app requires bundle_id or app_name")
		}
		err := tx.QueryRow(`
			INSERT INTO assets (type, app_name, domain, task_ids, extra)
			VALUES ('app', $2, NULLIF($3,''), ARRAY[$1::bigint], $4)
			ON CONFLICT (app_name) WHERE type='app' AND bundle_id IS NULL DO UPDATE
			SET task_ids = `+mergeTasks+`, last_seen = now()
			RETURNING id`,
			taskID, item.AppName, DomainKey(item.Domain), extra).Scan(&id)
		return id, err

	default:
		return 0, fmt.Errorf("unknown asset type: %s", item.Type)
	}
}

func (s *Store) ListAssets(ctx context.Context, filter tools.AssetFilter) ([]tools.Asset, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	if filter.ID > 0 {
		row := s.QueryRowContext(ctx, "SELECT "+assetSelectCols+" FROM assets WHERE id = $1", filter.ID)
		assets, err := scanAssetRow(row)
		if err == sql.ErrNoRows {
			return nil, 0, nil
		}
		if err != nil {
			return nil, 0, err
		}
		return []tools.Asset{assetToTools(assets)}, 1, nil
	}

	if len(filter.IDs) > 0 {
		placeholders := make([]string, len(filter.IDs))
		args := make([]any, len(filter.IDs))
		for i, id := range filter.IDs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}
		q := "SELECT " + assetSelectCols + " FROM assets WHERE id IN (" + strings.Join(placeholders, ",") + ") ORDER BY id"
		rows, err := s.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()
		assets, err := scanAssets(rows)
		if err != nil {
			return nil, 0, err
		}
		out := make([]tools.Asset, len(assets))
		for i, a := range assets {
			out[i] = assetToTools(a)
		}
		return out, len(out), nil
	}

	var conditions []string
	var args []any
	nextParam := 1

	if filter.DSL != "" {
		where, dslArgs, err := buildDSLWhere(filter.DSL)
		if err != nil {
			return nil, 0, err
		}
		if where != "" && where != "TRUE" {
			conditions = append(conditions, where)
			args = append(args, dslArgs...)
			nextParam = len(args) + 1
		}
	}

	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", nextParam))
		args = append(args, filter.Type)
		nextParam++
	}

	if filter.TaskID > 0 {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(task_ids)", nextParam))
		args = append(args, filter.TaskID)
		nextParam++
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.QueryRowContext(ctx, "SELECT COUNT(*) FROM assets"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQ := "SELECT " + assetSelectCols + " FROM assets" + where + " ORDER BY last_seen DESC, id DESC"
	dataQ += fmt.Sprintf(" LIMIT $%d OFFSET $%d", nextParam, nextParam+1)
	args = append(args, limit, offset)

	rows, err := s.QueryContext(ctx, dataQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	assets, err := scanAssets(rows)
	if err != nil {
		return nil, 0, err
	}
	out := make([]tools.Asset, len(assets))
	for i, a := range assets {
		out[i] = assetToTools(a)
	}
	return out, total, nil
}

func assetToTools(a Asset) tools.Asset {
	data := map[string]any{}
	if a.Domain != "" {
		data["domain"] = a.Domain
	}
	if a.RootDomain != "" {
		data["root_domain"] = a.RootDomain
	}
	if a.IP != "" {
		data["ip"] = a.IP
	}
	if a.URL != "" {
		data["url"] = a.URL
	}
	if a.ServiceName != "" {
		data["service_name"] = a.ServiceName
	}
	if a.BundleID != "" {
		data["bundle_id"] = a.BundleID
	}
	if a.AppName != "" {
		data["app_name"] = a.AppName
	}
	if a.Method != "" {
		data["method"] = a.Method
	}
	if a.Port != nil {
		data["port"] = *a.Port
	}
	if len(a.Technologies) > 0 {
		data["technologies"] = a.Technologies
	}
	if a.PageTitle != "" {
		data["page_title"] = a.PageTitle
	}
	if a.StatusCode != nil {
		data["status_code"] = *a.StatusCode
	}
	if len(a.Extra) > 0 {
		var m map[string]any
		if json.Unmarshal(a.Extra, &m) == nil {
			for k, v := range m {
				data[k] = v
			}
		}
	}
	label := assetLabel(a)
	return tools.Asset{ID: a.ID, Type: a.Type, Label: label, Data: data}
}

func assetLabel(a Asset) string {
	switch a.Type {
	case AssetRootDomain, AssetSubdomain:
		return a.Domain
	case AssetIP:
		return a.IP
	case AssetService:
		if a.URL != "" {
			return a.URL
		}
		return a.ServiceName
	case AssetEndpoint:
		return a.Method + " " + a.URL
	case AssetApp:
		if a.BundleID != "" {
			return a.BundleID
		}
		return a.AppName
	}
	return fmt.Sprintf("asset:%d", a.ID)
}

func scanAssetRow(row *sql.Row) (Asset, error) {
	var a Asset
	var taskIDs int64Array
	var boundDomains, recordValue, technologies stringArray
	var extra []byte
	err := row.Scan(
		&a.ID, &a.Type, &a.CompanyID, &taskIDs,
		&a.Domain, &a.RootDomain, &a.IP, &a.CSegment, &a.Port,
		&a.ICP, &boundDomains,
		&a.RecordType, &recordValue,
		&a.BundleID, &a.AppName, &a.Category, &a.AppDescription, &a.AppICP,
		&a.URL, &a.ServiceType, &a.ServiceName,
		&a.FaviconMMH3, &a.StatusCode, &a.ContentLength, &a.PageTitle,
		&technologies, &a.Method, &extra,
		&a.CreatedAt, &a.LastSeen, &a.UpdatedAt,
	)
	if err != nil {
		return a, err
	}
	a.TaskIDs = []int64(taskIDs)
	a.BoundDomains = []string(boundDomains)
	a.RecordValue = []string(recordValue)
	a.Technologies = []string(technologies)
	if len(extra) > 0 {
		a.Extra = extra
	}
	return a, nil
}

func (s *Store) ListUntestedAssets(ctx context.Context, taskID int64, assetType string, limit, offset int) ([]tools.Asset, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	typeFilter := ""
	args := []any{taskID}
	nextArg := 2
	if assetType != "" {
		typeFilter = fmt.Sprintf(" AND a.type = $%d", nextArg)
		args = append(args, assetType)
		nextArg++
	}

	baseQ := `
		WITH scope_assets AS (
			SELECT DISTINCT a.id
			FROM task_scope ts
			JOIN assets a ON (
				(ts.kind = 'root_domain' AND a.root_domain = ts.domain) OR
				(ts.kind = 'subdomain' AND a.domain = ts.domain) OR
				(ts.kind = 'ip' AND a.ip = host(ts.net)) OR
				(ts.kind = 'cidr' AND try_inet(a.ip) <<= ts.net) OR
				(ts.kind = 'company' AND a.company_id = ts.company_id)
			)
			WHERE ts.task_id = $1` + typeFilter + `
		)
		SELECT %s
		FROM assets a
		WHERE a.id IN (SELECT id FROM scope_assets)
		  AND a.id NOT IN (
			SELECT ea.asset_id
			FROM exploration_anchors ea
			JOIN exploration_nodes en ON en.id = ea.node_id
			JOIN tasks t ON t.exploration_id = en.exploration_id
			WHERE t.id = $1 AND en.kind = 'fact'
		  )`

	countQ := fmt.Sprintf(baseQ, "COUNT(*)")
	var total int
	if err := s.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	selectCols := strings.Replace(assetSelectCols, "id,", "a.id,", 1)
	dataQ := fmt.Sprintf(baseQ, selectCols) + " ORDER BY a.last_seen DESC"
	dataQ += fmt.Sprintf(" LIMIT $%d OFFSET $%d", nextArg, nextArg+1)
	dataArgs := append(append([]any{}, args...), limit, offset)

	rows, err := s.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	assets, err := scanAssets(rows)
	if err != nil {
		return nil, 0, err
	}
	out := make([]tools.Asset, len(assets))
	for i, a := range assets {
		out[i] = assetToTools(a)
	}
	return out, total, nil
}

func (s *Store) DeleteAssetsByHost(host string) (int64, error) {
	h := DomainKey(host)
	if h == "" {
		return 0, fmt.Errorf("host is required")
	}
	res, err := s.Exec(`DELETE FROM assets WHERE domain = $1 OR ip = $1`, h)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

type stringArray []string

func (a stringArray) Value() (interface{}, error) {
	if a == nil {
		return "{}", nil
	}
	parts := make([]string, len(a))
	for i, s := range a {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		parts[i] = `"` + s + `"`
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (a *stringArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return a.parsePostgresArray(string(v))
	case string:
		return a.parsePostgresArray(v)
	}
	return fmt.Errorf("stringArray: unsupported type %T", src)
}

func (a *stringArray) parsePostgresArray(s string) error {
	s = strings.TrimSpace(s)
	if s == "{}" || s == "" {
		*a = nil
		return nil
	}
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return fmt.Errorf("stringArray: invalid format")
	}
	s = s[1 : len(s)-1]
	var result []string
	var current strings.Builder
	inQuote := false
	escaped := false
	for _, r := range s {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == ',' && !inQuote {
			result = append(result, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 || len(result) > 0 {
		result = append(result, current.String())
	}
	*a = result
	return nil
}

type int64Array []int64

func (a *int64Array) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return a.parsePostgresArray(string(v))
	case string:
		return a.parsePostgresArray(v)
	}
	return fmt.Errorf("int64Array: unsupported type %T", src)
}

func (a *int64Array) parsePostgresArray(s string) error {
	s = strings.TrimSpace(s)
	if s == "{}" || s == "" {
		*a = nil
		return nil
	}
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return fmt.Errorf("int64Array: invalid format")
	}
	parts := strings.Split(s[1:len(s)-1], ",")
	result := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "NULL" {
			continue
		}
		var n int64
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
			return fmt.Errorf("int64Array: parse %q: %w", p, err)
		}
		result = append(result, n)
	}
	*a = result
	return nil
}
