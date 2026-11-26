package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
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

	if task.CompanyID != nil {
		export.CompanyID = *task.CompanyID
	}
	companies, err := s.exportCompanies(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("export companies: %w", err)
	}
	export.Companies = companies

	changes, err := s.ListTargetChanges(ctx, explorationID, false)
	if err != nil {
		return nil, fmt.Errorf("export target changes: %w", err)
	}
	for _, c := range changes {
		m := map[string]any{
			"id":                   c.ID,
			"target":               c.Target,
			"resource":             c.Resource,
			"kind":                 c.Kind,
			"original_fingerprint": c.OriginalFingerprint,
			"baseline":             c.Baseline,
			"baseline_change_id":   c.BaselineChangeID,
			"recorded_by":          c.RecordedBy,
			"recorded_at":          c.RecordedAt.UTC().Format(time.RFC3339Nano),
		}
		if c.IntentID != 0 {
			m["intent_id"] = c.IntentID
		}
		if c.Backup != "" {
			m["backup"] = c.Backup
		}
		if c.Note != "" {
			m["note"] = c.Note
		}
		if c.RestoredAt != nil {
			m["restored_at"] = c.RestoredAt.UTC().Format(time.RFC3339Nano)
			m["restored_by"] = c.RestoredBy
		}
		if c.RestoreAttempts > 0 {
			m["last_observed"] = c.LastObserved
			m["restore_attempts"] = c.RestoreAttempts
		}
		export.TargetChanges = append(export.TargetChanges, m)
	}

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
			m["owner"] = tools.OwnerLabel(owner)
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
			bundle_id, app_name, method, technologies, extra, company_id, company_source
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
		var companyID sql.NullInt64
		var companySource string
		if err := rows.Scan(&id, &assetType, &domain, &rootDomain, &ip, &port,
			&urlVal, &svcType, &svcName, &bundleID, &appName, &method, &technologies, &extra,
			&companyID, &companySource); err != nil {
			return nil, err
		}
		m := map[string]any{"id": id, "type": assetType}
		if companyID.Valid {
			m["company_id"] = companyID.Int64
			m["company_source"] = companySource
		}
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
		`SELECT ts.kind, COALESCE(ts.domain, ''), COALESCE(ts.net::text, ''), COALESCE(ts.value, ''),
			ts.company_id, COALESCE(c.name, ''), ts.source, COALESCE(ts.reason, '')
		 FROM task_scope ts LEFT JOIN companies c ON c.id = ts.company_id
		 WHERE ts.task_id = $1 ORDER BY ts.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var kind, domain, netStr, value, companyName, source, reason string
		var companyID sql.NullInt64
		if err := rows.Scan(&kind, &domain, &netStr, &value, &companyID, &companyName, &source, &reason); err != nil {
			return nil, err
		}
		m := map[string]any{"kind": kind, "source": source}
		if domain != "" {
			m["value"] = domain
		} else if netStr != "" {
			m["value"] = netStr
		} else if value != "" {
			m["value"] = value
		} else if companyName != "" {
			m["value"] = companyName
		}
		if companyID.Valid {
			m["company_id"] = companyID.Int64
		}
		if reason != "" {
			m["reason"] = reason
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) exportCompanies(ctx context.Context, taskID int64) ([]map[string]any, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT id, name, COALESCE(logo, '') FROM companies WHERE id IN (
			SELECT company_id FROM task_scope WHERE task_id = $1
			UNION SELECT company_id FROM assets WHERE $1 = ANY(task_ids)
			UNION SELECT company_id FROM tasks WHERE id = $1
		 ) ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id int64
		var name, logo string
		if err := rows.Scan(&id, &name, &logo); err != nil {
			return nil, err
		}
		m := map[string]any{"id": id, "name": name}
		if logo != "" {
			m["logo"] = logo
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, m := range out {
		rules, err := s.ListCompanyScope(m["id"].(int64))
		if err != nil {
			return nil, err
		}
		var scope []map[string]any
		for _, r := range rules {
			line := map[string]any{"kind": r.Kind, "raw": r.Raw}
			switch {
			case r.Domain != "":
				line["value"] = r.Domain
			case r.Net != "":
				line["value"] = r.Net
			default:
				line["value"] = r.Value
			}
			if r.Reason != "" {
				line["reason"] = r.Reason
			}
			scope = append(scope, line)
		}
		if len(scope) > 0 {
			m["scope"] = scope
		}
	}
	return out, nil
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

	companyIDMap := map[int64]int64{}
	for _, c := range data.Companies {
		oldID := exportMapInt64(c, "id")
		name := strings.TrimSpace(exportMapString(c, "name"))
		if name == "" {
			return 0, 0, fmt.Errorf("import company (old id %d): name is required", oldID)
		}
		var newID int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO companies(name, nkey, logo) VALUES ($1, $2, NULLIF($3,''))
			 ON CONFLICT (nkey) DO UPDATE SET logo = COALESCE(NULLIF(EXCLUDED.logo,''), companies.logo)
			 RETURNING id`,
			name, companyNKey(name), exportMapString(c, "logo"),
		).Scan(&newID); err != nil {
			return 0, 0, fmt.Errorf("import company %q: %w", name, err)
		}
		if oldID > 0 {
			companyIDMap[oldID] = newID
		}
		lines, _ := c["scope"].([]any)
		for _, l := range lines {
			line, _ := l.(map[string]any)
			kind := exportMapString(line, "kind")
			value := exportMapString(line, "value")
			raw := exportMapString(line, "raw")
			if raw == "" {
				raw = value
			}
			var domainVal, netVal, valueVal any
			switch kind {
			case "domain":
				domainVal = value
			case "ip", "cidr":
				netVal = value
			default:
				valueVal = value
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO company_scope(company_id, kind, domain, net, value, raw, reason)
				 VALUES ($1, $2, $3, $4::cidr, $5, $6, NULLIF($7, ''))
				 ON CONFLICT DO NOTHING`,
				newID, kind, domainVal, netVal, valueVal, raw, exportMapString(line, "reason"),
			); err != nil {
				return 0, 0, fmt.Errorf("import company %q scope %q: %w", name, raw, err)
			}
		}
	}

	var taskID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO tasks(name, description, goal, exploration_id, company_id) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		data.Goal, data.Description, data.Goal, explorationID, nullID(companyIDMap[data.CompanyID]),
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
	var assetURLs []string
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
		if companyID := companyIDMap[exportMapInt64(a, "company_id")]; companyID > 0 {
			if _, err := tx.ExecContext(ctx,
				`UPDATE assets SET company_id = $1, company_source = COALESCE(NULLIF($2, ''), 'explicit')
				 WHERE id = $3 AND company_id IS NULL`,
				companyID, exportMapString(a, "company_source"), id,
			); err != nil {
				return 0, 0, fmt.Errorf("import asset company (old id %d): %w", oldID, err)
			}
		}
		assetURLs = append(assetURLs, input.URL)
	}

	for _, n := range data.Nodes {
		oldNodeID := exportMapInt64(n, "id")
		newNodeID := nodeIDMap[oldNodeID]
		if newNodeID == 0 {
			continue
		}
		anchors := n["anchors"]
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

	findingIDMap := map[int64]int64{}
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

		var newID int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO findings(task_id, node_id, vulnclass, name, severity, summary, evidence, worker, asset_ids, status, report, dismissed, dismiss_reason)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, '', $11, $12) RETURNING id`,
			taskID, nullID(newNodeID),
			exportMapString(f, "vulnclass"), exportMapString(f, "name"),
			exportMapString(f, "severity"), exportMapString(f, "summary"),
			exportMapString(f, "evidence"), exportMapString(f, "worker"),
			assetIDsJSON, exportMapString(f, "status"),
			dismissed, exportMapString(f, "dismiss_reason"),
		).Scan(&newID); err != nil {
			return 0, 0, fmt.Errorf("import finding: %w", err)
		}
		if oldID := exportMapInt64(f, "id"); oldID > 0 {
			findingIDMap[oldID] = newID
		}
	}

	for _, f := range data.Findings {
		newID := findingIDMap[exportMapInt64(f, "id")]
		newTarget := findingIDMap[exportMapInt64(f, "merged_into")]
		if newID == 0 || newTarget == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE findings SET merged_into = $1 WHERE id = $2`, newTarget, newID); err != nil {
			return 0, 0, fmt.Errorf("import finding merge: %w", err)
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
			taskID, DomainKey(exportMapString(c, "domain")), exportMapString(c, "username"),
			password, exportMapString(c, "source"), exportMapString(c, "note"),
		); err != nil {
			return 0, 0, fmt.Errorf("import credential: %w", err)
		}
	}

	for _, u := range assetURLs {
		if err := s.saveAssetURLCredential(ctx, tx, taskID, u); err != nil {
			return 0, 0, fmt.Errorf("import url credential: %w", err)
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
		var domainVal, netVal, valueVal, companyVal any
		switch kind {
		case "company":
			companyID := companyIDMap[exportMapInt64(sc, "company_id")]
			if companyID == 0 {
				return 0, 0, fmt.Errorf("import scope: company %q is not in the export's companies", value)
			}
			companyVal = companyID
		case "root_domain", "subdomain":
			domainVal = value
		case "ip", "cidr":
			netVal = value
		default:
			valueVal = value
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO task_scope(task_id, kind, company_id, domain, net, value, source, reason)
			 VALUES ($1, $2, $3, $4, $5::cidr, $6, $7, NULLIF($8, ''))
			 ON CONFLICT DO NOTHING`,
			taskID, kind, companyVal, domainVal, netVal, valueVal, source, reason,
		); err != nil {
			return 0, 0, fmt.Errorf("import scope: %w", err)
		}
	}

	changeIDMap := map[int64]int64{}
	for _, c := range data.TargetChanges {
		oldID := exportMapInt64(c, "id")
		var restoredAt, recordedAt any
		if v := exportMapString(c, "restored_at"); v != "" {
			restoredAt = v
		}
		if v := exportMapString(c, "recorded_at"); v != "" {
			recordedAt = v
		}
		var newID int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO target_changes (exploration_id, target, resource, kind, intent_id, original_fingerprint,
				baseline, baseline_change_id, backup, note, recorded_by, created_at, restored_at, restored_by,
				last_observed, restore_attempts)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, COALESCE($12::timestamptz, now()), $13::timestamptz, $14, $15, $16)
			 RETURNING id`,
			explorationID, exportMapString(c, "target"), exportMapString(c, "resource"), exportMapString(c, "kind"),
			nullID(nodeIDMap[exportMapInt64(c, "intent_id")]), exportMapString(c, "original_fingerprint"),
			exportMapString(c, "baseline"), nullID(changeIDMap[exportMapInt64(c, "baseline_change_id")]),
			exportMapString(c, "backup"), exportMapString(c, "note"), exportMapString(c, "recorded_by"),
			recordedAt, restoredAt, exportMapString(c, "restored_by"),
			exportMapString(c, "last_observed"), int(exportMapFloat64(c, "restore_attempts")),
		).Scan(&newID); err != nil {
			return 0, 0, fmt.Errorf("import target change (old id %d): %w", oldID, err)
		}
		if oldID > 0 {
			changeIDMap[oldID] = newID
		}
		if exportMapInt64(c, "baseline_change_id") == oldID {
			if _, err := tx.ExecContext(ctx,
				`UPDATE target_changes SET baseline_change_id = id WHERE id = $1`, newID); err != nil {
				return 0, 0, fmt.Errorf("import target change (old id %d): %w", oldID, err)
			}
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

	input.Technologies = []string{}
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
