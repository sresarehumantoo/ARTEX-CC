package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type Node struct {
	ID            int64           `json:"id"`
	Kind          string          `json:"kind"`
	Payload       json.RawMessage `json:"payload"`
	Priority      int             `json:"priority"`
	State         string          `json:"state"`
	Origin        string          `json:"origin,omitempty"`
	Owner         string          `json:"owner,omitempty"`
	BlockedReason string          `json:"blocked_reason,omitempty"`
	DeleteReason  string          `json:"delete_reason,omitempty"`
	Anchors       []int64         `json:"anchors,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type Edge struct {
	From int64  `json:"from"`
	Rel  string `json:"rel"`
	To   int64  `json:"to"`
}

type Activity struct {
	ID        int64           `json:"id"`
	NodeID    *int64          `json:"node_id,omitempty"`
	Worker    string          `json:"worker,omitempty"`
	Kind      string          `json:"kind,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	IsError   bool            `json:"is_error"`
	Summary   string          `json:"summary,omitempty"`
	Detail    string          `json:"-"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

func (s *Store) CreateExploration(description, goal string) (int64, error) {
	var id int64
	err := s.QueryRow(
		`INSERT INTO explorations(description, goal) VALUES ($1, $2) RETURNING id`,
		description, goal,
	).Scan(&id)
	return id, err
}

func nodeToTools(n Node) tools.Node {
	payload := map[string]any{}
	if len(n.Payload) > 0 {
		_ = json.Unmarshal(n.Payload, &payload)
	}
	return tools.Node{
		ID:      n.ID,
		Kind:    n.Kind,
		State:   n.State,
		Payload: payload,
		Origin:  n.Origin,
	}
}

func (s *Store) AddNode(ctx context.Context, explorationID int64, kind string, payload map[string]any, priority int, state, origin string, anchors []int64) (int64, error) {
	raw, _ := json.Marshal(payload)
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	var id int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO exploration_nodes(exploration_id, kind, payload, priority, state, origin)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		explorationID, kind, string(raw), priority, state, origin,
	).Scan(&id); err != nil {
		return 0, err
	}
	for _, a := range anchors {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO exploration_anchors(node_id, asset_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			id, a,
		); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

func (s *Store) Link(explorationID, from int64, rel string, to int64) error {
	_, err := s.Exec(
		`INSERT INTO exploration_edges(exploration_id, src_id, rel, dst_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (exploration_id, src_id, rel, dst_id) DO NOTHING`,
		explorationID, from, rel, to,
	)
	return err
}

func (s *Store) UpdateNodeState(ctx context.Context, nodeID int64, state string) error {
	_, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET state = $1, blocked_reason = NULL WHERE id = $2`,
		state, nodeID,
	)
	return err
}

func (s *Store) UpdateNodeFields(ctx context.Context, nodeID int64, state string, payloadMerge map[string]any) error {
	if state != "" {
		if _, err := s.ExecContext(ctx,
			`UPDATE exploration_nodes SET state = $1 WHERE id = $2`,
			state, nodeID); err != nil {
			return err
		}
	}
	if len(payloadMerge) > 0 {
		merge, err := json.Marshal(payloadMerge)
		if err != nil {
			return err
		}
		if _, err := s.ExecContext(ctx,
			`UPDATE exploration_nodes SET payload = payload || $1::jsonb WHERE id = $2`,
			string(merge), nodeID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) AddEdge(ctx context.Context, from int64, rel string, to int64) error {
	var explorationID int64
	err := s.QueryRowContext(ctx,
		`SELECT exploration_id FROM exploration_nodes WHERE id = $1`, from,
	).Scan(&explorationID)
	if err != nil {
		return fmt.Errorf("lookup exploration for node %d: %w", from, err)
	}
	return s.Link(explorationID, from, rel, to)
}

const nodeSelectCols = `n.id, n.kind, n.payload, n.priority, n.state,
	COALESCE(n.origin, ''), COALESCE(n.owner, ''),
	COALESCE(n.blocked_reason, ''), COALESCE(n.delete_reason, ''),
	COALESCE((SELECT json_agg(asset_id) FROM exploration_anchors WHERE node_id = n.id), '[]'::json),
	n.created_at, n.updated_at`

func (s *Store) ListNodes(ctx context.Context, explorationID int64, kind string, limit int) ([]tools.Node, error) {
	query := `SELECT ` + nodeSelectCols + `
		FROM exploration_nodes n WHERE n.exploration_id = $1`
	args := []any{explorationID}
	nextArg := 2
	if kind != "" {
		query += fmt.Sprintf(` AND n.kind = $%d`, nextArg)
		args = append(args, kind)
		nextArg++
	}
	query += ` ORDER BY n.id`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, nextArg)
		args = append(args, limit)
	}

	rows, err := s.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes, err := scanNodes(rows)
	if err != nil {
		return nil, err
	}
	out := make([]tools.Node, len(nodes))
	for i, n := range nodes {
		out[i] = nodeToTools(n)
	}
	return out, nil
}

func scanNodes(rows *sql.Rows) ([]Node, error) {
	var out []Node
	for rows.Next() {
		var n Node
		var anchorsJSON string
		if err := rows.Scan(
			&n.ID, &n.Kind, &n.Payload, &n.Priority, &n.State,
			&n.Origin, &n.Owner, &n.BlockedReason, &n.DeleteReason,
			&anchorsJSON, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if anchorsJSON != "" && anchorsJSON != "[]" {
			_ = json.Unmarshal([]byte(anchorsJSON), &n.Anchors)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) AddEdges(ctx context.Context, explorationID int64, edges []Edge) error {
	if len(edges) == 0 {
		return nil
	}
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, e := range edges {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO exploration_edges(exploration_id, src_id, rel, dst_id)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (exploration_id, src_id, rel, dst_id) DO NOTHING`,
			explorationID, e.From, e.Rel, e.To,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GraphOverview(ctx context.Context, explorationID int64) (*tools.Overview, error) {
	ov := &tools.Overview{
		Task:     map[string]any{},
		Coverage: map[string]any{},
	}

	var desc, goal sql.NullString
	if err := s.QueryRowContext(ctx,
		`SELECT COALESCE(description,''), COALESCE(goal,'') FROM explorations WHERE id = $1`,
		explorationID).Scan(&desc, &goal); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	ov.Task["description"] = desc.String
	ov.Task["goal"] = goal.String

	nodeRows, err := s.QueryContext(ctx,
		`SELECT `+nodeSelectCols+`
		 FROM exploration_nodes n
		 WHERE n.exploration_id = $1 AND n.state != 'deleted'
		 ORDER BY n.id`, explorationID)
	if err != nil {
		return nil, err
	}
	defer nodeRows.Close()
	nodes, err := scanNodes(nodeRows)
	if err != nil {
		return nil, err
	}

	for _, n := range nodes {
		payload := map[string]any{}
		if len(n.Payload) > 0 {
			_ = json.Unmarshal(n.Payload, &payload)
		}
		m := map[string]any{"id": n.ID, "state": n.State}
		for k, v := range payload {
			m[k] = v
		}

		switch n.Kind {
		case KindGoal:
			ov.Goals = append(ov.Goals, m)
		case KindHint:
			ov.Hints = append(ov.Hints, m)
		case KindIntent:
			switch n.State {
			case "open":
				ov.OpenIntents = append(ov.OpenIntents, m)
			case "running":
				if time.Since(n.UpdatedAt) > 30*time.Minute {
					m["possibly_stale"] = true
				}
				ov.RunningIntents = append(ov.RunningIntents, m)
			case "done", "exhausted", "stopped":
				ov.RecentDone = append(ov.RecentDone, m)
			}
		case KindFact:
			ov.RecentFacts = append(ov.RecentFacts, m)
		case KindFinding:
			if n.State != "dismissed" {
				ov.FindingList = append(ov.FindingList, m)
			}
		}
	}

	ov.FrontierOpen = len(ov.OpenIntents)

	var doneTotal int
	if err := s.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exploration_nodes WHERE exploration_id = $1 AND kind = 'intent' AND state IN ('done','exhausted','stopped')`,
		explorationID).Scan(&doneTotal); err != nil {
		return nil, err
	}
	ov.DoneIntentsTotal = doneTotal

	var findingsTotal int
	if err := s.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exploration_nodes WHERE exploration_id = $1 AND kind = 'finding' AND state != 'dismissed'`,
		explorationID).Scan(&findingsTotal); err != nil {
		return nil, err
	}
	ov.FindingsTotal = findingsTotal

	var taskIDForFindings int64
	if err := s.QueryRowContext(ctx,
		`SELECT id FROM tasks WHERE exploration_id = $1 LIMIT 1`, explorationID).Scan(&taskIDForFindings); err == nil {
		var dismissedCount, mergedCount int
		_ = s.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM findings WHERE task_id = $1 AND dismissed = true`, taskIDForFindings).Scan(&dismissedCount)
		_ = s.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM findings WHERE task_id = $1 AND merged_into IS NOT NULL`, taskIDForFindings).Scan(&mergedCount)
		ov.FindingsDismissed = dismissedCount
		ov.FindingsMerged = mergedCount
	}

	var factsTotal int
	if err := s.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exploration_nodes WHERE exploration_id = $1 AND kind = 'fact'`,
		explorationID).Scan(&factsTotal); err != nil {
		return nil, err
	}
	ov.FactsTotal = factsTotal

	// Trim recent done intents to last 10
	if len(ov.RecentDone) > 10 {
		ov.RecentDone = ov.RecentDone[len(ov.RecentDone)-10:]
	}
	// Trim recent facts to last 10
	if len(ov.RecentFacts) > 10 {
		ov.RecentFacts = ov.RecentFacts[len(ov.RecentFacts)-10:]
	}

	var taskID int64
	if err := s.QueryRowContext(ctx,
		`SELECT id FROM tasks WHERE exploration_id = $1 LIMIT 1`, explorationID).Scan(&taskID); err == nil {
		credRows, err := s.QueryContext(ctx,
			`SELECT id, domain, username, password, source, note FROM task_credentials WHERE task_id = $1 ORDER BY id`, taskID)
		if err == nil {
			defer credRows.Close()
			for credRows.Next() {
				var id int64
				var domain, username, password, source, note string
				if err := credRows.Scan(&id, &domain, &username, &password, &source, &note); err == nil {
					ov.Credentials = append(ov.Credentials, map[string]any{
						"id": id, "domain": domain, "username": username,
						"source": source, "note": note,
					})
				}
			}
		}

		var covEntries, covGaps int
		_ = s.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM coverage_ledger WHERE task_id = $1`, taskID).Scan(&covEntries)
		_ = s.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM coverage_ledger WHERE task_id = $1 AND outcome = 'needs_follow_up'`, taskID).Scan(&covGaps)
		ov.Coverage["entries"] = covEntries
		ov.Coverage["gaps"] = covGaps
	}

	return ov, nil
}

func (s *Store) NodeDetail(ctx context.Context, nodeID int64) (*tools.Node, error) {
	row := s.QueryRowContext(ctx,
		`SELECT `+nodeSelectCols+` FROM exploration_nodes n WHERE n.id = $1`, nodeID)
	var n Node
	var anchorsJSON string
	err := row.Scan(
		&n.ID, &n.Kind, &n.Payload, &n.Priority, &n.State,
		&n.Origin, &n.Owner, &n.BlockedReason, &n.DeleteReason,
		&anchorsJSON, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node %d not found", nodeID)
	}
	if err != nil {
		return nil, err
	}
	tn := nodeToTools(n)
	return &tn, nil
}

func (s *Store) Frontier(ctx context.Context, explorationID int64, limit int) ([]tools.Node, error) {
	query := `SELECT ` + nodeSelectCols + `
		FROM exploration_nodes n
		WHERE n.exploration_id = $1 AND n.kind = 'intent' AND n.state = 'open'
		ORDER BY n.priority DESC, n.id`
	args := []any{explorationID}
	if limit > 0 {
		query += ` LIMIT $2`
		args = append(args, limit)
	}
	rows, err := s.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes, err := scanNodes(rows)
	if err != nil {
		return nil, err
	}
	out := make([]tools.Node, len(nodes))
	for i, n := range nodes {
		out[i] = nodeToTools(n)
	}
	return out, nil
}

func (s *Store) CountFinishedIntents(ctx context.Context, explorationID int64) (int, error) {
	var count int
	err := s.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exploration_nodes WHERE exploration_id = $1 AND kind = 'intent' AND state IN ('done','exhausted','stopped')`,
		explorationID).Scan(&count)
	return count, err
}

func (s *Store) CountOpenIntents(ctx context.Context, explorationID int64) (int, error) {
	var count int
	err := s.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exploration_nodes WHERE exploration_id = $1 AND kind = 'intent' AND state = 'open'`,
		explorationID).Scan(&count)
	return count, err
}

func (s *Store) OriginFactID(ctx context.Context, explorationID int64) (int64, error) {
	var id int64
	err := s.QueryRowContext(ctx,
		`SELECT id FROM exploration_nodes WHERE exploration_id = $1 AND kind = 'fact' AND (state = 'origin' OR origin = 'system') ORDER BY id LIMIT 1`,
		explorationID).Scan(&id)
	return id, err
}

func (s *Store) Edges(ctx context.Context, explorationID int64, limit int) ([]tools.Edge, error) {
	query := `SELECT src_id, rel, dst_id FROM exploration_edges WHERE exploration_id = $1 ORDER BY src_id, dst_id`
	args := []any{explorationID}
	if limit > 0 {
		query += ` LIMIT $2`
		args = append(args, limit)
	}
	rows, err := s.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.Edge
	for rows.Next() {
		var e tools.Edge
		if err := rows.Scan(&e.From, &e.Rel, &e.To); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) Root(ctx context.Context, explorationID int64) (description, goal string, err error) {
	err = s.QueryRowContext(ctx,
		`SELECT COALESCE(description,''), COALESCE(goal,'') FROM explorations WHERE id = $1`,
		explorationID).Scan(&description, &goal)
	return
}

func (s *Store) AddConstraint(ctx context.Context, explorationID int64, kind, text, origin string) (int64, error) {
	var id int64
	err := s.QueryRowContext(ctx,
		`INSERT INTO task_constraints(exploration_id, kind, text, origin) VALUES ($1, $2, $3, $4) RETURNING id`,
		explorationID, kind, text, origin).Scan(&id)
	return id, err
}

func (s *Store) ListConstraints(ctx context.Context, explorationID int64) ([]tools.Constraint, error) {
	rows, err := s.QueryContext(ctx,
		`SELECT id, kind, text FROM task_constraints WHERE exploration_id = $1 ORDER BY id`, explorationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.Constraint
	for rows.Next() {
		var c tools.Constraint
		if err := rows.Scan(&c.ID, &c.Kind, &c.Text); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func utf8Clean(s string) string {
	if strings.IndexByte(s, 0) >= 0 {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	return strings.ToValidUTF8(s, "�")
}

func jsonbClean(b []byte) []byte {
	if !bytes.Contains(b, []byte(`\u0000`)) {
		return b
	}
	out := make([]byte, 0, len(b))
	bs := 0
	for i := 0; i < len(b); i++ {
		if b[i] == '\\' && bs%2 == 0 && i+5 < len(b) &&
			b[i+1] == 'u' && b[i+2] == '0' && b[i+3] == '0' && b[i+4] == '0' && b[i+5] == '0' {
			i += 5
			bs = 0
			continue
		}
		if b[i] == '\\' {
			bs++
		} else {
			bs = 0
		}
		out = append(out, b[i])
	}
	return out
}

func nullID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}
