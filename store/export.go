package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/tools"
)

func (s *Store) ExportEngagementData(ctx context.Context, taskID int64) (*tools.EngagementExport, error) {
	task, err := s.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("engagement %d not found", taskID)
	}

	export := &tools.EngagementExport{
		Version:     1,
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
		Goal:        task.Goal,
		Description: task.Description,
	}

	explorationID := task.ExplorationID

	nodes, err := s.exportNodes(ctx, explorationID)
	if err != nil {
		return nil, fmt.Errorf("export nodes: %w", err)
	}
	export.Nodes = nodes

	edges, err := s.exportEdges(ctx, explorationID)
	if err != nil {
		return nil, fmt.Errorf("export edges: %w", err)
	}
	export.Edges = edges

	constraints, err := s.exportConstraints(ctx, explorationID)
	if err != nil {
		return nil, fmt.Errorf("export constraints: %w", err)
	}
	export.Constraints = constraints

	assets, err := s.exportAssets(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("export assets: %w", err)
	}
	export.Assets = assets

	findings, err := s.exportFindings(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("export findings: %w", err)
	}
	export.Findings = findings

	creds, err := s.exportCredentials(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("export credentials: %w", err)
	}
	export.Credentials = creds

	coverage, err := s.exportCoverage(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("export coverage: %w", err)
	}
	export.Coverage = coverage

	scope, err := s.exportScope(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("export scope: %w", err)
	}
	export.Scope = scope

	return export, nil
}

func (s *Store) exportNodes(ctx context.Context, explorationID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT n.id, n.kind, n.payload, n.priority, n.state,
			COALESCE(n.origin, ''), COALESCE(n.owner, ''),
			COALESCE((SELECT json_agg(asset_id) FROM exploration_anchors WHERE node_id = n.id), '[]'::json)
		 FROM exploration_nodes n
		 WHERE n.exploration_id = $1
		 ORDER BY n.id`, explorationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id int64
		var kind, state, origin, owner string
		var payload json.RawMessage
		var priority int
		var anchorsJSON string
		if err := rows.Scan(&id, &kind, &payload, &priority, &state, &origin, &owner, &anchorsJSON); err != nil {
			return nil, err
		}
		m := map[string]any{
			"id":       id,
			"kind":     kind,
			"priority": priority,
			"state":    state,
		}
		if len(payload) > 0 {
			var p map[string]any
			if json.Unmarshal(payload, &p) == nil {
				m["payload"] = p
			}
		}
		if origin != "" {
			m["origin"] = origin
		}
		if owner != "" {
			m["owner"] = owner
		}
		var anchors []int64
		if anchorsJSON != "" && anchorsJSON != "[]" {
			_ = json.Unmarshal([]byte(anchorsJSON), &anchors)
		}
		if len(anchors) > 0 {
			m["anchors"] = anchors
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) exportEdges(ctx context.Context, explorationID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT src_id, rel, dst_id FROM exploration_edges WHERE exploration_id = $1 ORDER BY src_id, dst_id`,
		explorationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var from, to int64
		var rel string
		if err := rows.Scan(&from, &rel, &to); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"from": from, "rel": rel, "to": to})
	}
	return out, rows.Err()
}

func (s *Store) exportConstraints(ctx context.Context, explorationID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT kind, text, COALESCE(origin, '') FROM task_constraints WHERE exploration_id = $1 ORDER BY id`,
		explorationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var kind, text, origin string
		if err := rows.Scan(&kind, &text, &origin); err != nil {
			return nil, err
		}
		m := map[string]any{"kind": kind, "text": text}
		if origin != "" {
			m["origin"] = origin
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) exportAssets(ctx context.Context, taskID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT id, type, domain, root_domain, ip, port, url, service_type, service_name,
			bundle_id, app_name, method, technologies, extra
		 FROM assets WHERE $1 = ANY(task_ids)
		 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id int64
		var assetType string
		var domain, rootDomain, ip, svcType, svcName, bundleID, appName, method sql.NullString
		var urlVal sql.NullString
		var port sql.NullInt32
		var technologies stringArray
		var extra []byte
		if err := rows.Scan(&id, &assetType, &domain, &rootDomain, &ip, &port,
			&urlVal, &svcType, &svcName, &bundleID, &appName, &method, &technologies, &extra); err != nil {
			return nil, err
		}
		m := map[string]any{"id": id, "type": assetType}
		if domain.Valid && domain.String != "" {
			m["domain"] = domain.String
		}
		if rootDomain.Valid && rootDomain.String != "" {
			m["root_domain"] = rootDomain.String
		}
		if ip.Valid && ip.String != "" {
			m["ip"] = ip.String
		}
		if port.Valid {
			m["port"] = int(port.Int32)
		}
		if urlVal.Valid && urlVal.String != "" {
			m["url"] = urlVal.String
		}
		if svcType.Valid && svcType.String != "" {
			m["service_type"] = svcType.String
		}
		if svcName.Valid && svcName.String != "" {
			m["service_name"] = svcName.String
		}
		if bundleID.Valid && bundleID.String != "" {
			m["bundle_id"] = bundleID.String
		}
		if appName.Valid && appName.String != "" {
			m["app_name"] = appName.String
		}
		if method.Valid && method.String != "" {
			m["method"] = method.String
		}
		if len(technologies) > 0 {
			m["technologies"] = []string(technologies)
		}
		if len(extra) > 0 && string(extra) != "{}" {
			var e map[string]any
			if json.Unmarshal(extra, &e) == nil && len(e) > 0 {
				m["extra"] = e
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) exportFindings(ctx context.Context, taskID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT id, node_id, vulnclass, name, severity, summary, evidence, worker, asset_ids, status,
		        merged_into, dismissed, dismiss_reason
		 FROM findings WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id int64
		var nodeID, mergedInto sql.NullInt64
		var vulnclass, name, severity, summary, evidence, worker, status, dismissReason string
		var assetIDs json.RawMessage
		var dismissed bool
		if err := rows.Scan(&id, &nodeID, &vulnclass, &name, &severity, &summary, &evidence, &worker,
			&assetIDs, &status, &mergedInto, &dismissed, &dismissReason); err != nil {
			return nil, err
		}
		m := map[string]any{
			"id":        id,
			"vulnclass": vulnclass,
			"severity":  severity,
			"summary":   summary,
			"status":    status,
		}
		if name != "" {
			m["name"] = name
		}
		if evidence != "" {
			m["evidence"] = evidence
		}
		if worker != "" {
			m["worker"] = worker
		}
		if nodeID.Valid {
			m["node_id"] = nodeID.Int64
		}
		if len(assetIDs) > 0 && string(assetIDs) != "[]" {
			var ids []int64
			if json.Unmarshal(assetIDs, &ids) == nil && len(ids) > 0 {
				m["asset_ids"] = ids
			}
		}
		if mergedInto.Valid {
			m["merged_into"] = mergedInto.Int64
		}
		if dismissed {
			m["dismissed"] = true
			m["dismiss_reason"] = dismissReason
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) exportCredentials(ctx context.Context, taskID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT domain, username, password, source, note
		 FROM task_credentials WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var domain, username, password, source, note string
		if err := rows.Scan(&domain, &username, &password, &source, &note); err != nil {
			return nil, err
		}
		if s.credentialKey != nil && strings.HasPrefix(password, "enc:") {
			plain, err := aesGCMDecrypt(s.credentialKey, password[4:])
			if err == nil {
				password = plain
			}
		}
		m := map[string]any{
			"domain":   domain,
			"username": username,
			"password": password,
			"source":   source,
		}
		if note != "" {
			m["note"] = note
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) exportCoverage(ctx context.Context, taskID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT surface, risk_area, outcome, evidence, reason, worker
		 FROM coverage_ledger WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var surface, riskArea, outcome, evidence, reason, worker string
		if err := rows.Scan(&surface, &riskArea, &outcome, &evidence, &reason, &worker); err != nil {
			return nil, err
		}
		m := map[string]any{
			"surface":   surface,
			"risk_area": riskArea,
			"outcome":   outcome,
		}
		if evidence != "" {
			m["evidence"] = evidence
		}
		if reason != "" {
			m["reason"] = reason
		}
		if worker != "" {
			m["worker"] = worker
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) exportScope(ctx context.Context, taskID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT kind, domain, COALESCE(net::text, ''), value, source, COALESCE(reason, '')
		 FROM task_scope WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var kind, domain, netStr, value, source, reason string
		if err := rows.Scan(&kind, &domain, &netStr, &value, &source, &reason); err != nil {
			return nil, err
		}
		m := map[string]any{"kind": kind, "source": source}
		if domain != "" {
			m["value"] = domain
		} else if netStr != "" {
			m["value"] = netStr
		} else if value != "" {
			m["value"] = value
		}
		if reason != "" {
			m["reason"] = reason
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ImportEngagementData(ctx context.Context, data *tools.EngagementExport) (int64, int64, error) {
	if data.Version < 1 {
		return 0, 0, fmt.Errorf("unsupported export version %d", data.Version)
	}

	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	var explorationID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO explorations(description, goal) VALUES ($1, $2) RETURNING id`,
		data.Description, data.Goal,
	).Scan(&explorationID); err != nil {
		return 0, 0, fmt.Errorf("create exploration: %w", err)
	}

	var taskID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO tasks(name, description, goal, exploration_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		data.Goal, data.Description, data.Goal, explorationID,
	).Scan(&taskID); err != nil {
		return 0, 0, fmt.Errorf("create task: %w", err)
	}

	nodeIDMap := map[int64]int64{}
	for _, n := range data.Nodes {
		oldID := exportMapInt64(n, "id")
		kind := exportMapString(n, "kind")
		state := exportMapString(n, "state")
		origin := exportMapString(n, "origin")
		priority := int(exportMapFloat64(n, "priority"))

		payloadRaw := []byte("{}")
		if p, ok := n["payload"]; ok {
			if raw, err := json.Marshal(p); err == nil {
				payloadRaw = raw
			}
		}

		var newID int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO exploration_nodes(exploration_id, kind, payload, priority, state, origin)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			explorationID, kind, string(payloadRaw), priority, state, origin,
		).Scan(&newID); err != nil {
			return 0, 0, fmt.Errorf("import node (old id %d): %w", oldID, err)
		}
		if oldID > 0 {
			nodeIDMap[oldID] = newID
		}
	}

	assetIDMap := map[int64]int64{}
	for _, a := range data.Assets {
		oldID := exportMapInt64(a, "id")
		input := exportMapToAssetInput(a)
		id, err := insertAssetReturningID(tx, taskID, input)
		if err != nil {
			return 0, 0, fmt.Errorf("import asset (old id %d): %w", oldID, err)
		}
		if oldID > 0 {
			assetIDMap[oldID] = id
		}
	}

	for _, n := range data.Nodes {
		oldNodeID := exportMapInt64(n, "id")
		newNodeID := nodeIDMap[oldNodeID]
		if newNodeID == 0 {
			continue
		}
		anchors, _ := n["anchors"]
		if anchors == nil {
			continue
		}
		anchorList := exportToInt64Slice(anchors)
		for _, oldAssetID := range anchorList {
			newAssetID := assetIDMap[oldAssetID]
			if newAssetID == 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO exploration_anchors(node_id, asset_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				newNodeID, newAssetID,
			); err != nil {
				return 0, 0, fmt.Errorf("import anchor: %w", err)
			}
		}
	}

	for _, e := range data.Edges {
		oldFrom := exportMapInt64(e, "from")
		oldTo := exportMapInt64(e, "to")
		rel := exportMapString(e, "rel")
		newFrom := nodeIDMap[oldFrom]
		newTo := nodeIDMap[oldTo]
		if newFrom == 0 || newTo == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO exploration_edges(exploration_id, src_id, rel, dst_id)
			 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			explorationID, newFrom, rel, newTo,
		); err != nil {
			return 0, 0, fmt.Errorf("import edge: %w", err)
		}
	}

	for _, c := range data.Constraints {
		kind := exportMapString(c, "kind")
		text := exportMapString(c, "text")
		origin := exportMapString(c, "origin")
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO task_constraints(exploration_id, kind, text, origin) VALUES ($1, $2, $3, $4)`,
			explorationID, kind, text, origin,
		); err != nil {
			return 0, 0, fmt.Errorf("import constraint: %w", err)
		}
	}

	for _, f := range data.Findings {
		var oldNodeID int64
		if v, ok := f["node_id"]; ok {
			oldNodeID = exportToInt64(v)
		}
		newNodeID := nodeIDMap[oldNodeID]

		var remappedAssetIDs []int64
		if aids, ok := f["asset_ids"]; ok {
			for _, oldAID := range exportToInt64Slice(aids) {
				if newAID := assetIDMap[oldAID]; newAID > 0 {
					remappedAssetIDs = append(remappedAssetIDs, newAID)
				}
			}
		}
		assetIDsJSON := "[]"
		if len(remappedAssetIDs) > 0 {
			raw, _ := json.Marshal(remappedAssetIDs)
			assetIDsJSON = string(raw)
		}

		dismissed := false
		if v, ok := f["dismissed"]; ok {
			dismissed, _ = v.(bool)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO findings(task_id, node_id, vulnclass, name, severity, summary, evidence, worker, asset_ids, status, report, dismissed, dismiss_reason)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, '', $11, $12)`,
			taskID, nullID(newNodeID),
			exportMapString(f, "vulnclass"), exportMapString(f, "name"),
			exportMapString(f, "severity"), exportMapString(f, "summary"),
			exportMapString(f, "evidence"), exportMapString(f, "worker"),
			assetIDsJSON, exportMapString(f, "status"),
			dismissed, exportMapString(f, "dismiss_reason"),
		); err != nil {
			return 0, 0, fmt.Errorf("import finding: %w", err)
		}
	}

	for _, c := range data.Credentials {
		password := exportMapString(c, "password")
		if s.credentialKey != nil && password != "" {
			enc, err := aesGCMEncrypt(s.credentialKey, password)
			if err != nil {
				return 0, 0, fmt.Errorf("encrypt credential: %w", err)
			}
			password = "enc:" + enc
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO task_credentials(task_id, domain, username, password, source, note)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (task_id, domain, username) DO NOTHING`,
			taskID, exportMapString(c, "domain"), exportMapString(c, "username"),
			password, exportMapString(c, "source"), exportMapString(c, "note"),
		); err != nil {
			return 0, 0, fmt.Errorf("import credential: %w", err)
		}
	}

	for _, cv := range data.Coverage {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO coverage_ledger(task_id, surface, risk_area, outcome, evidence, reason, worker)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (task_id, surface, risk_area) DO NOTHING`,
			taskID, exportMapString(cv, "surface"), exportMapString(cv, "risk_area"),
			exportMapString(cv, "outcome"), exportMapString(cv, "evidence"),
			exportMapString(cv, "reason"), exportMapString(cv, "worker"),
		); err != nil {
			return 0, 0, fmt.Errorf("import coverage: %w", err)
		}
	}

	for _, sc := range data.Scope {
		kind := exportMapString(sc, "kind")
		value := exportMapString(sc, "value")
		source := exportMapString(sc, "source")
		reason := exportMapString(sc, "reason")
		if kind == "" || value == "" {
			continue
		}
		var domainVal, netVal, valueVal any
		switch kind {
		case "root_domain", "subdomain":
			domainVal = value
		case "ip", "cidr":
			netVal = value
		default:
			valueVal = value
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO task_scope(task_id, kind, domain, net, value, source, reason)
			 VALUES ($1, $2, $3, $4::cidr, $5, $6, NULLIF($7, ''))
			 ON CONFLICT DO NOTHING`,
			taskID, kind, domainVal, netVal, valueVal, source, reason,
		); err != nil {
			return 0, 0, fmt.Errorf("import scope: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return taskID, explorationID, nil
}

func exportMapToAssetInput(m map[string]any) AssetInput {
	input := AssetInput{
		Type:     exportMapString(m, "type"),
		Domain:   exportMapString(m, "domain"),
		IP:       exportMapString(m, "ip"),
		URL:      exportMapString(m, "url"),
		Method:   exportMapString(m, "method"),
		BundleID: exportMapString(m, "bundle_id"),
		AppName:  exportMapString(m, "app_name"),
	}
	input.ServiceName = exportMapString(m, "service_name")
	input.ServiceType = exportMapString(m, "service_type")

	if p := int(exportMapFloat64(m, "port")); p > 0 {
		input.Port = &p
	}

	if techs, ok := m["technologies"]; ok {
		input.Technologies = exportToStringSlice(techs)
	}

	if extra, ok := m["extra"]; ok {
		if raw, err := json.Marshal(extra); err == nil {
			input.Extra = raw
		}
	}

	return input
}

func exportMapString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func exportMapFloat64(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

func exportMapInt64(m map[string]any, key string) int64 {
	return int64(exportMapFloat64(m, key))
}

func exportToInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

func exportToInt64Slice(v any) []int64 {
	switch s := v.(type) {
	case []any:
		out := make([]int64, 0, len(s))
		for _, item := range s {
			if id := exportToInt64(item); id > 0 {
				out = append(out, id)
			}
		}
		return out
	case []int64:
		return s
	case []float64:
		out := make([]int64, 0, len(s))
		for _, f := range s {
			out = append(out, int64(f))
		}
		return out
	}
	return nil
}

func exportToStringSlice(v any) []string {
	switch s := v.(type) {
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return s
	}
	return nil
}
