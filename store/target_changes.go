package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

const targetChangeCols = `id, target, resource, kind, COALESCE(intent_id, 0), original_fingerprint, baseline,
	COALESCE(baseline_change_id, id), backup, note, recorded_by, created_at, restored_at, restored_by,
	last_observed, restore_attempts`

const sameResource = `lower(btrim(target)) = lower(btrim($2)) AND target_change_resource_key(resource) = target_change_resource_key($3)`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTargetChange(r rowScanner) (tools.TargetChange, error) {
	var c tools.TargetChange
	var restoredAt sql.NullTime
	err := r.Scan(&c.ID, &c.Target, &c.Resource, &c.Kind, &c.IntentID, &c.OriginalFingerprint, &c.Baseline,
		&c.BaselineChangeID, &c.Backup, &c.Note, &c.RecordedBy, &c.RecordedAt, &restoredAt, &c.RestoredBy,
		&c.LastObserved, &c.RestoreAttempts)
	if restoredAt.Valid {
		t := restoredAt.Time
		c.RestoredAt = &t
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
		`SELECT id FROM explorations WHERE id = $1 FOR UPDATE`, explorationID,
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
		`SELECT `+targetChangeCols+` FROM target_changes
		 WHERE exploration_id = $1 AND restored_at IS NULL AND `+sameResource+`
		 ORDER BY id`, explorationID, in.Target, in.Resource)
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
		`SELECT `+targetChangeCols+` FROM target_changes WHERE id = $1`, id))
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
		`SELECT `+targetChangeCols+` FROM target_changes
		 WHERE exploration_id = $1 AND id = ANY($2) ORDER BY id`, explorationID, changeIDs)
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
		}
		out = append(out, o)
	}
	return out, tx.Commit()
}

func (s *Store) ListTargetChanges(ctx context.Context, explorationID int64, unrestoredOnly bool) ([]tools.TargetChange, error) {
	return queryTargetChanges(ctx, s,
		`SELECT `+targetChangeCols+` FROM target_changes
		 WHERE exploration_id = $1 AND (NOT $2 OR restored_at IS NULL)
		 ORDER BY id`, explorationID, unrestoredOnly)
}
