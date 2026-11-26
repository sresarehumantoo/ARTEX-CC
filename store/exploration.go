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
	Owner         string          `json:"-"`
	BlockedReason string          `json:"blocked_reason,omitempty"`
	DeleteReason  string          `json:"delete_reason,omitempty"`
	Anchors       []int64         `json:"anchors,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ClaimExpires  sql.NullTime    `json:"-"`
}

func (n Node) claimLive(now time.Time) bool {
	return n.State == "running" && n.ClaimExpires.Valid && n.ClaimExpires.Time.After(now)
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
		`UPDATE exploration_nodes SET state = $1, blocked_reason = NULL, owner = NULL, claim_expires_at = NULL WHERE id = $2`,
		state, nodeID,
	)
	return err
}

// ClaimIntent atomically marks an intent running and owned by owner. It
// succeeds when the intent is open, when it is running with no live claim, or
// when owner already holds the claim (a renewal).
func (s *Store) ClaimIntent(ctx context.Context, nodeID int64, owner string, lease time.Duration) (*tools.IntentClaim, error) {
	claim := &tools.IntentClaim{NodeID: nodeID, Owner: owner}
	err := s.QueryRowContext(ctx,
		`WITH prev AS (
			SELECT id, state, owner, claim_expires_at
			FROM exploration_nodes WHERE id = $1 AND kind = 'intent'
			FOR UPDATE
		)
		UPDATE exploration_nodes n
		SET state = 'running', owner = $2, claim_expires_at = now() + make_interval(secs => $3)
		FROM prev
		WHERE n.id = prev.id AND (
			prev.state = 'open' OR (prev.state = 'running' AND (
				prev.claim_expires_at IS NULL OR prev.claim_expires_at <= now() OR prev.owner = $2)))
		RETURNING n.claim_expires_at,
			prev.state = 'running' AND prev.owner IS NOT DISTINCT FROM $2
				AND prev.claim_expires_at IS NOT NULL AND prev.claim_expires_at > now()`,
		nodeID, owner, lease.Seconds(),
	).Scan(&claim.ExpiresAt, &claim.Renewed)
	if err == nil {
		return claim, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	var kind, state, holder string
	var expires sql.NullTime
	err = s.QueryRowContext(ctx,
		`SELECT kind, state, COALESCE(owner, ''), claim_expires_at FROM exploration_nodes WHERE id = $1`,
		nodeID,
	).Scan(&kind, &state, &holder, &expires)
	switch {
	case err == sql.ErrNoRows:
		return nil, fmt.Errorf("node %d not found", nodeID)
	case err != nil:
		return nil, err
	case kind != KindIntent:
		return nil, fmt.Errorf("node %d is a %s, not an intent", nodeID, kind)
	case state == "running" && expires.Valid:
		return nil, &tools.ClaimConflictError{NodeID: nodeID, Owner: holder, ExpiresAt: expires.Time}
	default:
		return nil, fmt.Errorf("intent %d is %q, must be open to start", nodeID, state)
	}
}

// RenewIntentClaims extends the lease on the given intents that are still
// running under holder or an owner subdividing it, including lapsed claims
// nobody has taken over. Claims with more than half a lease left are not
// touched, which keeps each write from bumping the intent in list_changes.
func (s *Store) RenewIntentClaims(ctx context.Context, nodeIDs []int64, holder string, lease time.Duration) error {
	_, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET claim_expires_at = now() + make_interval(secs => $3)
		 WHERE id = ANY($1) AND kind = 'intent' AND state = 'running'
		   AND (owner = $2 OR starts_with(owner, $2 || '/owner:'))
		   AND claim_expires_at < now() + make_interval(secs => $3 / 2)`,
		nodeIDs, holder, lease.Seconds())
	return err
}

func (s *Store) UpdateNodeFields(ctx context.Context, nodeID int64, state string, payloadMerge map[string]any) error {
	if state != "" {
		if _, err := s.ExecContext(ctx,
			`UPDATE exploration_nodes SET state = $1, owner = NULL, claim_expires_at = NULL WHERE id = $2`,
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

// ProveGoal links evidenceID to goalID with a proves edge and marks the goal
// met, refusing evidence that is dismissed or a merged finding. The evidence
// row lock makes a concurrent dismissal either wait for the edge (and revoke
// it) or commit first (and be seen here).
func (s *Store) ProveGoal(ctx context.Context, goalID, evidenceID int64) error {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var explorationID int64
	var state string
	if err := tx.QueryRowContext(ctx,
		`SELECT exploration_id, state FROM exploration_nodes WHERE id = $1 FOR SHARE`, evidenceID,
	).Scan(&explorationID, &state); err != nil {
		return fmt.Errorf("evidence node %d: %w", evidenceID, err)
	}
	var deadFinding bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM findings WHERE node_id = $1 AND (dismissed OR merged_into IS NOT NULL))`, evidenceID,
	).Scan(&deadFinding); err != nil {
		return fmt.Errorf("evidence finding %d: %w", evidenceID, err)
	}
	if state == "dismissed" || deadFinding {
		return tools.ErrDeadEvidence
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO exploration_edges(exploration_id, src_id, rel, dst_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (exploration_id, src_id, rel, dst_id) DO NOTHING`,
		explorationID, evidenceID, RelProves, goalID); err != nil {
		return fmt.Errorf("link proof: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE exploration_nodes SET state = 'met', blocked_reason = NULL, owner = NULL, claim_expires_at = NULL WHERE id = $1`,
		goalID); err != nil {
		return fmt.Errorf("mark goal %d met: %w", goalID, err)
	}
	return tx.Commit()
}

const nodeSelectCols = `n.id, n.kind, n.payload, n.priority, n.state,
	COALESCE(n.origin, ''), COALESCE(n.owner, ''),
	COALESCE(n.blocked_reason, ''), COALESCE(n.delete_reason, ''),
	COALESCE((SELECT json_agg(asset_id) FROM exploration_anchors WHERE node_id = n.id), '[]'::json),
	n.created_at, n.updated_at, n.claim_expires_at`

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
			&anchorsJSON, &n.CreatedAt, &n.UpdatedAt, &n.ClaimExpires,
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

	var asOf time.Time
	if err := s.QueryRowContext(ctx, `SELECT now()`).Scan(&asOf); err != nil {
		return nil, err
	}
	ov.ChangesCursor = tools.ChangeCursor{At: asOf}.String()

	var desc, goal, status sql.NullString
	if err := s.QueryRowContext(ctx,
		`SELECT COALESCE(description,''), COALESCE(goal,''), status FROM explorations WHERE id = $1`,
		explorationID).Scan(&desc, &goal, &status); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	ov.Task["description"] = desc.String
	ov.Task["goal"] = goal.String
	ov.Task["status"] = status.String

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

	now := time.Now()
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
				if n.claimLive(now) {
					m["claimed_by"] = tools.OwnerLabel(n.Owner)
					m["claim_expires_at"] = n.ClaimExpires.Time.UTC().Format(time.RFC3339)
					ov.RunningIntents = append(ov.RunningIntents, m)
					break
				}
				m["claim_expired"] = true
				if n.Owner != "" {
					m["previous_owner"] = tools.OwnerLabel(n.Owner)
				}
				ov.OpenIntents = append(ov.OpenIntents, m)
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

	unrestored, err := s.ListTargetChanges(ctx, explorationID, true)
	if err != nil {
		return nil, err
	}
	if len(unrestored) > 0 {
		ov.UnrestoredTargetChanges = tools.UnrestoredSummaries(unrestored)
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
		&anchorsJSON, &n.CreatedAt, &n.UpdatedAt, &n.ClaimExpires,
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
		WHERE n.exploration_id = $1 AND n.kind = 'intent'
		  AND (n.state = 'open' OR (n.state = 'running' AND (n.claim_expires_at IS NULL OR n.claim_expires_at <= now())))
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

// AddGoals adds goals in one transaction that holds the exploration row lock,
// so concurrent callers (several agents setting goals at startup) see each
// other's goals instead of both inserting.
func (s *Store) AddGoals(ctx context.Context, explorationID int64, goals []tools.GoalInput, appendGoals bool) (*tools.GoalsResult, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var locked int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM explorations WHERE id = $1 FOR UPDATE`, explorationID,
	).Scan(&locked); err != nil {
		return nil, fmt.Errorf("lock exploration %d: %w", explorationID, err)
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, COALESCE(payload->>'text', ''), state FROM exploration_nodes
		 WHERE exploration_id = $1 AND kind = $2 ORDER BY id`,
		explorationID, KindGoal)
	if err != nil {
		return nil, err
	}
	var existing []tools.GoalRef
	for rows.Next() {
		var g tools.GoalRef
		if err := rows.Scan(&g.ID, &g.Text, &g.State); err != nil {
			rows.Close()
			return nil, err
		}
		existing = append(existing, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	res := &tools.GoalsResult{Created: []tools.GoalRef{}}
	if len(existing) > 0 && !appendGoals {
		res.Existing = existing
		for _, g := range goals {
			res.Skipped = append(res.Skipped, tools.SkippedGoal{Text: g.Text, Reason: tools.GoalSkipAlreadySet})
		}
		return res, tx.Commit()
	}

	seen := make(map[string]int64, len(existing)+len(goals))
	for _, g := range existing {
		seen[tools.NormalizeGoalText(g.Text)] = g.ID
	}

	var origin int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM exploration_nodes WHERE exploration_id = $1 AND kind = 'fact' AND (state = 'origin' OR origin = 'system') ORDER BY id LIMIT 1`,
		explorationID).Scan(&origin); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	for _, g := range goals {
		key := tools.NormalizeGoalText(g.Text)
		if id, dup := seen[key]; dup {
			res.Skipped = append(res.Skipped, tools.SkippedGoal{Text: g.Text, Reason: tools.GoalSkipDuplicate, ExistingID: id})
			continue
		}
		payload := map[string]any{"text": g.Text}
		if g.VulnClass != "" {
			payload["vulnclass"] = g.VulnClass
		}
		raw, _ := json.Marshal(payload)
		var id int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO exploration_nodes(exploration_id, kind, payload, priority, state, origin)
			 VALUES ($1, $2, $3, 0, 'open', 'system') RETURNING id`,
			explorationID, KindGoal, string(raw)).Scan(&id); err != nil {
			return nil, err
		}
		if origin > 0 {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO exploration_edges(exploration_id, src_id, rel, dst_id)
				 VALUES ($1, $2, $3, $4)
				 ON CONFLICT (exploration_id, src_id, rel, dst_id) DO NOTHING`,
				explorationID, origin, RelSpawns, id); err != nil {
				return nil, err
			}
		}
		seen[key] = id
		res.Created = append(res.Created, tools.GoalRef{ID: id, Text: g.Text, State: "open"})
	}
	return res, tx.Commit()
}

const openIntentsListed = 10

// AddIntents adds intents in one transaction that holds the exploration row
// lock, so agents planning at the same time see each other's intents. An
// intent whose IntentKey matches an open or running intent, or an earlier one
// in the same call, is skipped.
func (s *Store) AddIntents(ctx context.Context, explorationID int64, intents []tools.IntentInput) (*tools.IntentsResult, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var locked int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM explorations WHERE id = $1 FOR UPDATE`, explorationID,
	).Scan(&locked); err != nil {
		return nil, fmt.Errorf("lock exploration %d: %w", explorationID, err)
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT n.id, COALESCE(n.payload->>'summary', ''), n.priority, n.state,
			n.state = 'open' OR n.claim_expires_at IS NULL OR n.claim_expires_at <= now(),
			COALESCE((SELECT json_agg(asset_id) FROM exploration_anchors WHERE node_id = n.id), '[]'::json)
		 FROM exploration_nodes n
		 WHERE n.exploration_id = $1 AND n.kind = $2 AND n.state IN ('open', 'running')
		 ORDER BY n.priority DESC, n.id`,
		explorationID, KindIntent)
	if err != nil {
		return nil, err
	}
	type existingIntent struct {
		ref       tools.IntentRef
		state     string
		claimable bool
	}
	var existing []existingIntent
	seen := map[string]existingIntent{}
	for rows.Next() {
		var e existingIntent
		var anchorsJSON string
		if err := rows.Scan(&e.ref.ID, &e.ref.Summary, &e.ref.Priority, &e.state, &e.claimable, &anchorsJSON); err != nil {
			rows.Close()
			return nil, err
		}
		var anchors []int64
		_ = json.Unmarshal([]byte(anchorsJSON), &anchors)
		existing = append(existing, e)
		key := tools.IntentKey(e.ref.Summary, anchors)
		if _, dup := seen[key]; !dup {
			seen[key] = e
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var origin int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM exploration_nodes WHERE exploration_id = $1 AND kind = 'fact' AND (state = 'origin' OR origin = 'system') ORDER BY id LIMIT 1`,
		explorationID).Scan(&origin); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var assetIDs []int64
	for _, it := range intents {
		assetIDs = append(assetIDs, it.AssetIDs...)
	}
	assetExists := map[int64]bool{}
	if len(assetIDs) > 0 {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM assets WHERE id = ANY($1)`, assetIDs)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			assetExists[id] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	res := &tools.IntentsResult{IDs: make([]int64, len(intents)), Errors: map[int]string{}}
	matched := map[int64]bool{}
items:
	for i, it := range intents {
		for _, a := range it.AssetIDs {
			if !assetExists[a] {
				res.Errors[i] = fmt.Sprintf("asset_id %d does not exist", a)
				continue items
			}
		}
		key := tools.IntentKey(it.Summary, it.AssetIDs)
		if e, dup := seen[key]; dup {
			res.Skipped = append(res.Skipped, tools.SkippedIntent{Index: i, Summary: it.Summary, ExistingID: e.ref.ID, State: e.state})
			matched[e.ref.ID] = true
			continue
		}
		payload := map[string]any{"summary": it.Summary}
		if len(it.AssetIDs) > 0 {
			payload["asset_ids"] = it.AssetIDs
		}
		raw, _ := json.Marshal(payload)
		var id int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO exploration_nodes(exploration_id, kind, payload, priority, state, origin)
			 VALUES ($1, $2, $3, $4, 'open', 'planner') RETURNING id`,
			explorationID, KindIntent, string(raw), it.Priority).Scan(&id); err != nil {
			return nil, err
		}
		for _, a := range it.AssetIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO exploration_anchors(node_id, asset_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				id, a); err != nil {
				return nil, err
			}
		}
		parents := it.ParentIDs
		if len(parents) == 0 && origin > 0 {
			parents = []int64{origin}
		}
		for _, p := range parents {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO exploration_edges(exploration_id, src_id, rel, dst_id)
				 VALUES ($1, $2, $3, $4)
				 ON CONFLICT (exploration_id, src_id, rel, dst_id) DO NOTHING`,
				explorationID, p, RelDerivedFrom, id); err != nil {
				return nil, err
			}
		}
		seen[key] = existingIntent{ref: tools.IntentRef{ID: id, Summary: it.Summary, Priority: it.Priority}, state: "open"}
		res.IDs[i] = id
	}

	for _, e := range existing {
		if len(res.Open) == openIntentsListed {
			break
		}
		if e.claimable && !matched[e.ref.ID] {
			res.Open = append(res.Open, e.ref)
		}
	}
	return res, tx.Commit()
}
