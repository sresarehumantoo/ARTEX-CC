package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

var reopenOutcomeKeys = []string{"result", "completed_by", "cancelled_by", "cancel_reason"}

const historyFieldLimit = 2000

// liveProofSource holds for a proves edge source src that still supports a
// goal: not dismissed, not yielded by an intent since reopened, and, for an
// intent used as its own evidence, still done.
const liveProofSource = `src.state <> 'dismissed' AND NOT (src.payload ? 'superseded_by_reopen')
	AND (src.kind <> 'intent' OR src.state = 'done')`

// ReopenNode moves a done or stopped intent, or a met goal, back to open so
// agents redo it. The old outcome moves into a payload history entry. An
// intent's yielded facts and findings are kept and marked superseded, and a
// met goal whose live proofs all came from the intent or its yields reopens
// too. When a goal reopens, an achieved engagement goes back to running.
// The exploration row is locked first, as target change adoption and restore
// lock it, then nodes as merges and dismissals take them: non-goal nodes,
// then goals in id order.
func (s *Store) ReopenNode(ctx context.Context, explorationID, nodeID int64, by, reason string) (*tools.ReopenResult, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var kind string
	err = tx.QueryRowContext(ctx,
		`SELECT kind FROM exploration_nodes WHERE id = $1 AND exploration_id = $2`,
		nodeID, explorationID).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("node %d: %w", nodeID, tools.ErrNodeNotFound)
	}
	if err != nil {
		return nil, err
	}

	if err := lockExploration(ctx, tx, explorationID); err != nil {
		return nil, err
	}
	var now time.Time
	if err := tx.QueryRowContext(ctx, `SELECT now()`).Scan(&now); err != nil {
		return nil, err
	}
	res := &tools.ReopenResult{ID: nodeID, Kind: kind}
	at := now.UTC().Format(time.RFC3339)
	switch kind {
	case KindIntent:
		err = reopenIntentTx(ctx, tx, res, by, reason, at)
	case KindGoal:
		if _, err = tx.ExecContext(ctx,
			`SELECT id FROM exploration_nodes WHERE id = $1 FOR NO KEY UPDATE`, nodeID); err != nil {
			return nil, fmt.Errorf("lock goal %d: %w", nodeID, err)
		}
		err = reopenNodeTx(ctx, tx, nodeID, []string{"met"}, by, reason, at)
	default:
		return nil, fmt.Errorf("node %d is a %s: %w", nodeID, kind, tools.ErrNotReopenable)
	}
	if err != nil {
		return nil, err
	}

	if kind == KindGoal || len(res.ReopenedGoals) > 0 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE explorations SET status = 'open' WHERE id = $1 AND status = 'achieved'`, explorationID); err != nil {
			return nil, fmt.Errorf("reopen exploration %d: %w", explorationID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE tasks SET status = 'running' WHERE exploration_id = $1 AND status = 'done'`, explorationID); err != nil {
			return nil, fmt.Errorf("reopen task of exploration %d: %w", explorationID, err)
		}
	}
	return res, tx.Commit()
}

func reopenIntentTx(ctx context.Context, tx *sql.Tx, res *tools.ReopenResult, by, reason, at string) error {
	findings, err := queryIDList(ctx, tx, `
		SELECT id FROM findings
		WHERE node_id IN (SELECT dst_id FROM exploration_edges WHERE src_id = $1 AND rel = $2)
		ORDER BY id FOR NO KEY UPDATE`,
		res.ID, RelYields)
	if err != nil {
		return fmt.Errorf("lock findings of intent %d: %w", res.ID, err)
	}

	sources, err := queryIDList(ctx, tx, `
		SELECT id FROM exploration_nodes
		WHERE kind <> $3 AND (id = $1 OR id IN (
			SELECT dst_id FROM exploration_edges WHERE src_id = $1 AND rel = $2))
		ORDER BY id FOR NO KEY UPDATE`,
		res.ID, RelYields, KindGoal)
	if err != nil {
		return fmt.Errorf("lock intent %d and its yields: %w", res.ID, err)
	}

	// Locking the goals before reading their proofs serializes this with a
	// concurrent dismissal of another proof, as in revokeGoalProofsTx.
	if _, err := tx.ExecContext(ctx, `
		SELECT id FROM exploration_nodes
		WHERE kind = $3 AND id IN (
			SELECT dst_id FROM exploration_edges WHERE src_id = ANY($1) AND rel = $2)
		ORDER BY id FOR NO KEY UPDATE`,
		sources, RelProves, KindGoal); err != nil {
		return fmt.Errorf("lock goals proved by intent %d: %w", res.ID, err)
	}

	// The intent's unrestored target changes have been unowned since it
	// closed. Record that window now; reopened_at starts the next one.
	if _, err := tx.ExecContext(ctx, closeUnownedWindows(`tc.intent_id = $1 AND n.state IN ('done', 'stopped')`), res.ID); err != nil {
		return fmt.Errorf("close unowned windows of intent %d: %w", res.ID, err)
	}
	if err := reopenNodeTx(ctx, tx, res.ID, []string{"done", "stopped"}, by, reason, at); err != nil {
		return err
	}

	superseded, err := queryIDList(ctx, tx, `
		UPDATE exploration_nodes
		SET payload = payload || jsonb_build_object('superseded_by_reopen', $2::bigint, 'superseded_at', $3::text)
		WHERE id = ANY($1) AND id <> $2 AND kind IN ('fact', 'finding')
		RETURNING id`,
		sources, res.ID, at)
	if err != nil {
		return fmt.Errorf("mark yields of intent %d superseded: %w", res.ID, err)
	}
	slices.Sort(superseded)
	res.Superseded = superseded
	if _, err := tx.ExecContext(ctx, `UPDATE findings SET updated_at = now() WHERE id = ANY($1)`, findings); err != nil {
		return fmt.Errorf("touch findings of intent %d: %w", res.ID, err)
	}

	goals, err := queryIDList(ctx, tx, `
		SELECT g.id FROM exploration_nodes g
		WHERE g.kind = $3 AND g.state = 'met' AND g.id IN (
			SELECT dst_id FROM exploration_edges WHERE src_id = ANY($1) AND rel = $2)
		  AND NOT EXISTS (
			SELECT 1 FROM exploration_edges pe
			JOIN exploration_nodes src ON src.id = pe.src_id
			WHERE pe.dst_id = g.id AND pe.rel = $2
			  AND pe.src_id <> ALL($1) AND `+liveProofSource+`)
		ORDER BY g.id`,
		sources, RelProves, KindGoal)
	if err != nil {
		return fmt.Errorf("goals proved only by intent %d: %w", res.ID, err)
	}
	goalReason := fmt.Sprintf("intent %d reopened: %s", res.ID, reason)
	for _, g := range goals {
		if err := reopenNodeTx(ctx, tx, g, []string{"met"}, by, goalReason, at); err != nil {
			return err
		}
	}
	res.ReopenedGoals = goals
	return nil
}

// reopenNodeTx sets a locked node open after checking its state is one of
// allowed, moving its outcome fields into a new payload history entry.
func reopenNodeTx(ctx context.Context, tx *sql.Tx, id int64, allowed []string, by, reason, at string) error {
	var kind, state string
	var raw []byte
	if err := tx.QueryRowContext(ctx,
		`SELECT kind, state, payload FROM exploration_nodes WHERE id = $1`, id,
	).Scan(&kind, &state, &raw); err != nil {
		return fmt.Errorf("node %d: %w", id, err)
	}
	if !slices.Contains(allowed, state) {
		return fmt.Errorf("%s %d is %q: %w", kind, id, state, tools.ErrNotReopenable)
	}

	payload := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("payload of node %d: %w", id, err)
	}
	str := func(s string) json.RawMessage {
		b, _ := json.Marshal(s)
		return b
	}
	entry := map[string]json.RawMessage{
		"state":       str(state),
		"reopened_by": str(by),
		"reason":      str(reason),
		"at":          str(at),
	}
	for _, k := range reopenOutcomeKeys {
		if v, ok := payload[k]; ok {
			if s := string(v); s != `""` && s != "null" {
				entry[k] = truncateJSONString(v, historyFieldLimit)
			}
			delete(payload, k)
		}
	}
	var history []json.RawMessage
	if h, ok := payload["history"]; ok {
		if err := json.Unmarshal(h, &history); err != nil {
			return fmt.Errorf("history of node %d: %w", id, err)
		}
	}
	e, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if payload["history"], err = json.Marshal(append(history, e)); err != nil {
		return err
	}
	payload["reopened_by"] = str(by)
	payload["reopen_reason"] = str(reason)
	payload["reopened_at"] = str(at)
	out, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE exploration_nodes
		 SET state = 'open', owner = NULL, claim_expires_at = NULL, blocked_reason = NULL, payload = $2
		 WHERE id = $1`,
		id, string(out)); err != nil {
		return fmt.Errorf("reopen node %d: %w", id, err)
	}
	return nil
}

func queryIDList(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// truncateJSONString shortens a JSON string value to at most limit runes and
// returns any other JSON value unchanged.
func truncateJSONString(v json.RawMessage, limit int) json.RawMessage {
	var str string
	if json.Unmarshal(v, &str) != nil {
		return v
	}
	r := []rune(str)
	if len(r) <= limit {
		return v
	}
	out, _ := json.Marshal(string(r[:limit]) + "...")
	return out
}
