package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// targetChangeFrom joins each change to its intent, so the columns below can
// say whether a live claim owns its restoration.
const targetChangeFrom = `target_changes tc LEFT JOIN exploration_nodes n ON n.id = tc.intent_id`

const unownedReason = `CASE
		WHEN tc.restored_at IS NOT NULL THEN ''
		WHEN n.id IS NULL THEN 'no_intent'
		WHEN n.state = 'running' AND n.claim_expires_at > now() THEN ''
		WHEN n.state = 'running' THEN 'claim_lapsed'
		ELSE 'intent_' || n.state
	END`

// unownedSince is when the change lost its live owner: when the intent's
// claim lapsed or it stopped running, or when the change was recorded if it
// never had one. NULL while owned or restored. An imported intent has no
// claim and was created at import, so the node's own times keep time before
// the import from counting; an imported change with no intent still counts
// from its recorded time. A reopened intent not yet claimed again counts from
// when it closed (payload closed_at), since reopening gives its changes no
// owner.
const unownedSince = `CASE
		WHEN tc.restored_at IS NOT NULL THEN NULL
		WHEN n.id IS NULL THEN tc.created_at
		WHEN n.state = 'open' THEN GREATEST(tc.created_at, n.created_at, (n.payload->>'closed_at')::timestamptz)
		WHEN n.state = 'running' AND n.claim_expires_at > now() THEN NULL
		WHEN n.state = 'running' THEN GREATEST(tc.created_at, COALESCE(n.claim_expires_at, n.updated_at))
		ELSE GREATEST(tc.created_at, n.updated_at)
	END`

const targetChangeCols = `tc.id, tc.target, tc.resource, tc.kind, COALESCE(tc.intent_id, 0), tc.original_fingerprint, tc.baseline,
	COALESCE(tc.baseline_change_id, tc.id), tc.backup, tc.note, tc.recorded_by, tc.created_at, tc.restored_at, tc.restored_by,
	tc.last_observed, tc.restore_attempts, ` + unownedReason + `, ` + unownedSince + `, tc.max_unowned_seconds, now()`

const sameResource = `lower(btrim(target)) = lower(btrim($2)) AND target_change_resource_key(resource) = target_change_resource_key($3)`

// closeUnownedWindows records, on the unrestored changes matching filter
// (over tc and n as in targetChangeFrom), how long they have been unowned.
// Run it just before giving them an owner or restoring them.
func closeUnownedWindows(filter string) string {
	return `UPDATE target_changes t
		SET max_unowned_seconds = GREATEST(t.max_unowned_seconds, CEIL(EXTRACT(EPOCH FROM now() - w.since))::int)
		FROM (SELECT tc.id, ` + unownedSince + ` AS since FROM ` + targetChangeFrom + `
		      WHERE tc.restored_at IS NULL AND ` + filter + `) w
		WHERE t.id = w.id AND w.since IS NOT NULL`
}

// closeUnownedWindowsCTE is closeUnownedWindows as a WITH entry, for a claim
// statement on exploration_nodes: it sees the claim as it was before.
func closeUnownedWindowsCTE(filter string) string {
	return `unowned_windows AS (` + closeUnownedWindows(filter) + `)`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTargetChange(r rowScanner) (tools.TargetChange, error) {
	var c tools.TargetChange
	var restoredAt, unownedAt sql.NullTime
	err := r.Scan(&c.ID, &c.Target, &c.Resource, &c.Kind, &c.IntentID, &c.OriginalFingerprint, &c.Baseline,
		&c.BaselineChangeID, &c.Backup, &c.Note, &c.RecordedBy, &c.RecordedAt, &restoredAt, &c.RestoredBy,
		&c.LastObserved, &c.RestoreAttempts, &c.Unowned, &unownedAt, &c.MaxUnownedSeconds, &c.AsOf)
	if restoredAt.Valid {
		t := restoredAt.Time
		c.RestoredAt = &t
	}
	if unownedAt.Valid {
		t := unownedAt.Time
		c.UnownedSince = &t
	}
	return c, err
}

func queryTargetChanges(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, query string, args ...any) ([]tools.TargetChange, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.TargetChange
	for rows.Next() {
		c, err := scanTargetChange(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func lockExploration(ctx context.Context, tx *sql.Tx, explorationID int64) error {
	var locked int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM explorations WHERE id = $1 FOR NO KEY UPDATE`, explorationID,
	).Scan(&locked); err != nil {
		return fmt.Errorf("lock exploration %d: %w", explorationID, err)
	}
	return nil
}

// RecordTargetChange journals a change under the exploration row lock, so
// when several agents record the first change on one resource at the same
// moment, exactly one becomes the baseline and the others see it.
func (s *Store) RecordTargetChange(ctx context.Context, explorationID int64, in tools.TargetChangeInput) (*tools.TargetChangeRecord, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := lockExploration(ctx, tx, explorationID); err != nil {
		return nil, err
	}

	var intentID any
	if in.IntentID != 0 {
		var kind string
		err := tx.QueryRowContext(ctx,
			`SELECT kind FROM exploration_nodes WHERE id = $1 AND exploration_id = $2`,
			in.IntentID, explorationID).Scan(&kind)
		if err == sql.ErrNoRows || (err == nil && kind != KindIntent) {
			return nil, fmt.Errorf("intent_id %d is not an intent in this engagement", in.IntentID)
		}
		if err != nil {
			return nil, err
		}
		intentID = in.IntentID
	}

	existing, err := queryTargetChanges(ctx, tx,
		`SELECT `+targetChangeCols+` FROM `+targetChangeFrom+`
		 WHERE tc.exploration_id = $1 AND tc.restored_at IS NULL AND `+sameResource+`
		 ORDER BY tc.id`, explorationID, in.Target, in.Resource)
	if err != nil {
		return nil, err
	}

	baseline := in.OriginalFingerprint
	var baselineID any
	if len(existing) > 0 {
		baseline = existing[0].Baseline
		baselineID = existing[0].BaselineChangeID
	}

	var id int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO target_changes (exploration_id, target, resource, kind, intent_id, original_fingerprint,
			baseline, baseline_change_id, backup, note, recorded_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		explorationID, in.Target, in.Resource, in.Kind, intentID, in.OriginalFingerprint,
		baseline, baselineID, in.Backup, in.Note, in.RecordedBy,
	).Scan(&id); err != nil {
		return nil, err
	}
	if baselineID == nil {
		if _, err := tx.ExecContext(ctx,
			`UPDATE target_changes SET baseline_change_id = id WHERE id = $1`, id); err != nil {
			return nil, err
		}
	}

	c, err := scanTargetChange(tx.QueryRowContext(ctx,
		`SELECT `+targetChangeCols+` FROM `+targetChangeFrom+` WHERE tc.id = $1`, id))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &tools.TargetChangeRecord{Change: c, Existing: existing}, nil
}

// RecordTargetRestore marks changes restored only when observed matches
// their baseline. A match restores every unrestored change on the same
// resource, since they share that baseline; a mismatch only counts the attempt.
func (s *Store) RecordTargetRestore(ctx context.Context, explorationID int64, changeIDs []int64, observed, restoredBy string) ([]tools.TargetRestoreOutcome, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := lockExploration(ctx, tx, explorationID); err != nil {
		return nil, err
	}

	requested, err := queryTargetChanges(ctx, tx,
		`SELECT `+targetChangeCols+` FROM `+targetChangeFrom+`
		 WHERE tc.exploration_id = $1 AND tc.id = ANY($2) ORDER BY tc.id`, explorationID, changeIDs)
	if err != nil {
		return nil, err
	}
	found := map[int64]tools.TargetChange{}
	for _, c := range requested {
		found[c.ID] = c
	}
	for _, id := range changeIDs {
		if _, ok := found[id]; !ok {
			return nil, fmt.Errorf("target change %d not found in this engagement", id)
		}
	}

	restoredNow := map[int64]bool{}
	seen := map[int64]bool{}
	var out []tools.TargetRestoreOutcome
	for _, id := range changeIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		c := found[id]
		o := tools.TargetRestoreOutcome{
			ChangeID: c.ID, Target: c.Target, Resource: c.Resource,
			Observed: observed, Baseline: c.Baseline, BaselineChangeID: c.BaselineChangeID,
		}
		match := tools.FingerprintsMatch(observed, c.Baseline)
		switch {
		case restoredNow[c.ID]:
			o.Status = tools.RestoreStatusRestored
			o.Message = fmt.Sprintf("restored: observed fingerprint matches baseline %s", c.Baseline)
		case c.RestoredAt != nil:
			o.Status = tools.RestoreStatusAlreadyRestored
			o.Message = fmt.Sprintf("already restored by %s at %s", c.RestoredBy, c.RestoredAt.UTC().Format(time.RFC3339))
			if !match {
				o.Message += fmt.Sprintf("; observed %s now differs from baseline %s, so the resource changed again: record that change", observed, c.Baseline)
			}
		case match:
			if _, err := tx.ExecContext(ctx,
				closeUnownedWindows(`tc.exploration_id = $1 AND `+sameResource),
				explorationID, c.Target, c.Resource); err != nil {
				return nil, err
			}
			rows, err := tx.QueryContext(ctx,
				`UPDATE target_changes
				 SET restored_at = now(), restored_by = $4, last_observed = $5, restore_attempts = restore_attempts + 1
				 WHERE exploration_id = $1 AND restored_at IS NULL AND `+sameResource+`
				 RETURNING id`,
				explorationID, c.Target, c.Resource, restoredBy, observed)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var rid int64
				if err := rows.Scan(&rid); err != nil {
					rows.Close()
					return nil, err
				}
				restoredNow[rid] = true
				if rid != c.ID {
					o.AlsoRestored = append(o.AlsoRestored, rid)
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
			o.Status = tools.RestoreStatusRestored
			o.Message = fmt.Sprintf("restored: observed fingerprint matches baseline %s", c.Baseline)
		default:
			if _, err := tx.ExecContext(ctx,
				`UPDATE target_changes SET last_observed = $2, restore_attempts = restore_attempts + 1 WHERE id = $1`,
				c.ID, observed); err != nil {
				return nil, err
			}
			o.Status = tools.RestoreStatusNotRestored
			o.Message = fmt.Sprintf("not restored: observed %s, baseline %s (first recorded in change #%d). "+
				"Restore the resource to the baseline and call record_target_restore again",
				observed, c.Baseline, c.BaselineChangeID)
			o.RecordedBy = c.RecordedBy
			o.PreviousObserved = c.LastObserved
			o.RestoreAttempts = c.RestoreAttempts + 1
			o.BaselineBackup = c.Backup
			if c.BaselineChangeID != c.ID {
				if err := tx.QueryRowContext(ctx,
					`SELECT backup FROM target_changes WHERE id = $1`, c.BaselineChangeID,
				).Scan(&o.BaselineBackup); err != nil && err != sql.ErrNoRows {
					return nil, err
				}
			}
		}
		out = append(out, o)
	}
	return out, tx.Commit()
}

// AdoptTargetChanges moves unrestored changes that have no live owner under
// intentID, which holder must hold a live claim on, so that claim owns their
// restoration. Changes another live claim owns are refused, not moved.
//
// The target and source intents are locked in id order before the changes,
// the order a claim takes them in, so a concurrent takeover of a source
// intent either commits first and the change is refused, or waits and never
// lists a change this moved.
func (s *Store) AdoptTargetChanges(ctx context.Context, explorationID, intentID int64, holder string, changeIDs []int64) (*tools.AdoptResult, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := lockExploration(ctx, tx, explorationID); err != nil {
		return nil, err
	}
	locked, err := queryIDs(ctx, tx,
		`SELECT id FROM exploration_nodes
		 WHERE kind <> 'goal' AND (id = $1 OR id IN (
			SELECT intent_id FROM target_changes WHERE exploration_id = $2 AND id = ANY($3)))
		 ORDER BY id FOR NO KEY UPDATE`, intentID, explorationID, changeIDs)
	if err != nil {
		return nil, err
	}

	var held bool
	err = tx.QueryRowContext(ctx,
		`SELECT state = 'running' AND owner = $3 AND claim_expires_at > now()
		 FROM exploration_nodes WHERE id = $1 AND exploration_id = $2 AND kind = 'intent'`,
		intentID, explorationID, holder).Scan(&held)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if !held {
		return nil, fmt.Errorf("adopting target changes needs a live claim of yours on intent %d", intentID)
	}

	changes, err := queryTargetChanges(ctx, tx,
		`SELECT `+targetChangeCols+` FROM `+targetChangeFrom+`
		 WHERE tc.exploration_id = $1 AND tc.id = ANY($2) ORDER BY tc.id FOR UPDATE OF tc`, explorationID, changeIDs)
	if err != nil {
		return nil, err
	}
	found := map[int64]tools.TargetChange{}
	for _, c := range changes {
		found[c.ID] = c
	}
	res := &tools.AdoptResult{Refused: map[int64]string{}}
	var move, keep []int64
	for _, id := range changeIDs {
		c, ok := found[id]
		switch {
		case !ok:
			res.Refused[id] = "not found in this engagement"
		case c.RestoredAt != nil:
			res.Refused[id] = "already restored"
		case c.IntentID == intentID:
			keep = append(keep, id)
		case c.IntentID != 0 && !locked[c.IntentID]:
			res.Refused[id] = "moved to another intent while adopting; try again"
		case c.Unowned == "":
			res.Refused[id] = fmt.Sprintf("owned by intent %d, whose claim is live", c.IntentID)
		default:
			move = append(move, id)
		}
	}
	if len(move) > 0 {
		if _, err := tx.ExecContext(ctx, closeUnownedWindows(`tc.id = ANY($1)`), move); err != nil {
			return nil, err
		}
		moved, err := queryIDs(ctx, tx,
			`UPDATE target_changes tc SET intent_id = $1
			 WHERE tc.id = ANY($2) AND tc.restored_at IS NULL AND NOT EXISTS (
				SELECT 1 FROM exploration_nodes n
				WHERE n.id = tc.intent_id AND n.state = 'running' AND n.claim_expires_at > now())
			 RETURNING tc.id`, intentID, move)
		if err != nil {
			return nil, err
		}
		for _, id := range move {
			if moved[id] {
				keep = append(keep, id)
			} else {
				res.Refused[id] = "gained a live owner while adopting"
			}
		}
	}
	if len(keep) > 0 {
		if res.Adopted, err = queryTargetChanges(ctx, tx,
			`SELECT `+targetChangeCols+` FROM `+targetChangeFrom+` WHERE tc.id = ANY($1) ORDER BY tc.id`, keep); err != nil {
			return nil, err
		}
	}
	return res, tx.Commit()
}

func queryIDs(ctx context.Context, tx *sql.Tx, query string, args ...any) (map[int64]bool, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

func (s *Store) ListTargetChanges(ctx context.Context, explorationID int64, unrestoredOnly bool) ([]tools.TargetChange, error) {
	return queryTargetChanges(ctx, s,
		`SELECT `+targetChangeCols+` FROM `+targetChangeFrom+`
		 WHERE tc.exploration_id = $1 AND (NOT $2 OR tc.restored_at IS NULL)
		 ORDER BY tc.id`, explorationID, unrestoredOnly)
}
