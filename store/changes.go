package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// ListChanges returns rows created or updated after the cursor, oldest first,
// keyed on updated_at. now() stamps a row with its transaction's start time,
// so a write still uncommitted when a later-stamped row is read can be missed;
// tool writes are short single transactions, which keeps that window small.
func (s *Store) ListChanges(ctx context.Context, explorationID, taskID int64, after tools.ChangeCursor, limit int) ([]tools.Change, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT kind, id, changed_at, created_at FROM (
			SELECT 'finding' AS kind, f.id, f.updated_at AS changed_at, f.created_at
			FROM findings f JOIN exploration_nodes n ON n.id = f.node_id
			WHERE n.exploration_id = $1
			UNION ALL
			SELECT n.kind, n.id, n.updated_at, n.created_at
			FROM exploration_nodes n
			WHERE n.exploration_id = $1 AND n.kind IN ('fact', 'intent', 'goal', 'hint')
			UNION ALL
			SELECT 'asset', a.id, a.updated_at, a.created_at
			FROM assets a
			WHERE $2::bigint = ANY(a.task_ids)
		) c
		WHERE (changed_at, kind, id) > ($3::timestamptz, $4::text, $5::bigint)
		ORDER BY changed_at, kind, id
		LIMIT $6`,
		explorationID, taskID, after.At, after.Kind, after.ID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type key struct {
		kind    string
		id      int64
		at      time.Time
		created time.Time
	}
	var keys []key
	var nodeIDs, findingIDs, assetIDs []int64
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.kind, &k.id, &k.at, &k.created); err != nil {
			return nil, err
		}
		keys = append(keys, k)
		switch k.kind {
		case "finding":
			findingIDs = append(findingIDs, k.id)
		case "asset":
			assetIDs = append(assetIDs, k.id)
		default:
			nodeIDs = append(nodeIDs, k.id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, nil
	}

	nodes, err := s.changedNodes(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	findings, err := s.changedFindings(ctx, findingIDs)
	if err != nil {
		return nil, err
	}
	assets, err := s.changedAssets(ctx, assetIDs)
	if err != nil {
		return nil, err
	}

	out := make([]tools.Change, 0, len(keys))
	for _, k := range keys {
		var fields map[string]any
		switch k.kind {
		case "finding":
			fields = findings[k.id]
		case "asset":
			fields = assets[k.id]
		default:
			fields = nodes[k.id]
		}
		if fields == nil {
			continue
		}
		change := "updated"
		if k.created.After(after.At) {
			change = "new"
		}
		fields["change"] = change
		fields["changed_at"] = k.at.UTC().Format(time.RFC3339)
		out = append(out, tools.Change{
			Kind:   k.kind,
			Cursor: tools.ChangeCursor{At: k.at, Kind: k.kind, ID: k.id},
			Fields: fields,
		})
	}
	return out, nil
}

func (s *Store) changedNodes(ctx context.Context, ids []int64) (map[int64]map[string]any, error) {
	out := map[int64]map[string]any{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.QueryContext(ctx,
		`SELECT `+nodeSelectCols+` FROM exploration_nodes n WHERE n.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes, err := scanNodes(rows)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, n := range nodes {
		m := map[string]any{}
		if len(n.Payload) > 0 {
			_ = json.Unmarshal(n.Payload, &m)
		}
		m["id"] = n.ID
		m["state"] = n.State
		if n.Kind == KindIntent && n.State == "running" {
			if n.claimLive(now) {
				m["claimed_by"] = tools.OwnerLabel(n.Owner)
				m["claim_expires_at"] = n.ClaimExpires.Time.UTC().Format(time.RFC3339)
			} else {
				m["claim_expired"] = true
			}
		}
		out[n.ID] = m
	}
	return out, nil
}

func (s *Store) changedFindings(ctx context.Context, ids []int64) (map[int64]map[string]any, error) {
	out := map[int64]map[string]any{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.QueryContext(ctx,
		`SELECT `+findingSelectCols+` FROM findings WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		m := map[string]any{
			"id":        f.ID,
			"vulnclass": f.VulnClass,
			"name":      f.Name,
			"severity":  f.Severity,
			"summary":   f.Summary,
			"status":    f.Status,
		}
		if f.NodeID != nil {
			m["node_id"] = *f.NodeID
		}
		if f.MergedInto != nil {
			m["merged_into"] = *f.MergedInto
		}
		if f.Dismissed {
			m["dismissed"] = true
			m["dismiss_reason"] = f.DismissReason
		}
		out[f.ID] = m
	}
	return out, rows.Err()
}

func (s *Store) changedAssets(ctx context.Context, ids []int64) (map[int64]map[string]any, error) {
	out := map[int64]map[string]any{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.QueryContext(ctx,
		`SELECT `+assetSelectCols+` FROM assets WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assets, err := scanAssets(rows)
	if err != nil {
		return nil, err
	}
	for _, a := range assets {
		t := assetToTools(a)
		out[a.ID] = map[string]any{"id": t.ID, "type": t.Type, "label": t.Label, "data": t.Data}
	}
	return out, nil
}

// ListClaims returns the running intents owner holds, including ones whose
// lease has lapsed but that no other session has taken yet.
func (s *Store) ListClaims(ctx context.Context, explorationID int64, owner string) ([]tools.HeldClaim, error) {
	rows, err := s.QueryContext(ctx, `SELECT `+nodeSelectCols+`
		FROM exploration_nodes n
		WHERE n.exploration_id = $1 AND n.kind = 'intent' AND n.state = 'running' AND n.owner = $2
		ORDER BY n.claim_expires_at NULLS FIRST, n.id`, explorationID, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes, err := scanNodes(rows)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := []tools.HeldClaim{}
	for _, n := range nodes {
		c := tools.HeldClaim{NodeID: n.ID, Expired: !n.claimLive(now)}
		if n.ClaimExpires.Valid {
			c.ExpiresAt = n.ClaimExpires.Time.UTC()
		}
		if len(n.Payload) > 0 {
			_ = json.Unmarshal(n.Payload, &c.Payload)
		}
		out = append(out, c)
	}
	return out, nil
}
