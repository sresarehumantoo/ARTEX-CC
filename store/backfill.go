package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/jackc/pgx/v5/pgconn"
)

const backfillsTable = `
CREATE TABLE IF NOT EXISTS store_backfills (
    name       TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

const assetURLBackfill = "asset_url_normalize"

// BackfillAssetURLs rewrites asset URLs stored before they were normalized on
// insert, recording any stripped userinfo as credentials of the asset's tasks.
// It runs once per database and needs the credential key already set. A URL
// whose normalized form belongs to another asset only has its userinfo
// stripped, and is left as it is when even that collides.
func (s *Store) BackfillAssetURLs(ctx context.Context) error {
	return withSchemaMigrationLock(ctx, s.DB, func(conn *sql.Conn) error {
		if _, err := conn.ExecContext(ctx, backfillsTable); err != nil {
			return err
		}
		var done bool
		if err := conn.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM store_backfills WHERE name = $1)`, assetURLBackfill).Scan(&done); err != nil {
			return err
		}
		if done {
			return nil
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback() //nolint:errcheck
		if err := s.backfillAssetURLs(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO store_backfills (name) VALUES ($1)`, assetURLBackfill); err != nil {
			return err
		}
		return tx.Commit()
	})
}

type assetURLRow struct {
	id      int64
	url     string
	taskIDs []int64
}

func (s *Store) backfillAssetURLs(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, url, ARRAY(SELECT t FROM unnest(task_ids) t WHERE EXISTS (SELECT 1 FROM tasks WHERE id = t))
		FROM assets WHERE url IS NOT NULL ORDER BY id FOR UPDATE`)
	if err != nil {
		return err
	}
	var pending []assetURLRow
	for rows.Next() {
		var r assetURLRow
		var tasks int64Array
		if err := rows.Scan(&r.id, &r.url, &tasks); err != nil {
			rows.Close()
			return err
		}
		if NormalizeAssetURL(r.url) != r.url {
			r.taskIDs = tasks
			pending = append(pending, r)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range pending {
		for _, taskID := range r.taskIDs {
			if err := s.saveAssetURLCredential(ctx, tx, taskID, r.url); err != nil {
				return fmt.Errorf("asset %d: save url credential: %w", r.id, err)
			}
		}
		candidates := []string{NormalizeAssetURL(r.url)}
		if stripped := tools.RedactURLUserinfo(strings.TrimSpace(r.url)); stripped != r.url && stripped != candidates[0] {
			candidates = append(candidates, stripped)
		}
		rewritten := false
		for _, u := range candidates {
			ok, err := updateAssetURL(ctx, tx, r.id, u)
			if err != nil {
				return fmt.Errorf("asset %d: %w", r.id, err)
			}
			if ok {
				rewritten = true
				break
			}
		}
		if !rewritten {
			slog.Warn("asset url left unnormalized: another asset already has that url", "asset_id", r.id)
		}
	}
	return nil
}

func updateAssetURL(ctx context.Context, tx *sql.Tx, id int64, u string) (bool, error) {
	if _, err := tx.ExecContext(ctx, `SAVEPOINT asset_url`); err != nil {
		return false, err
	}
	_, err := tx.ExecContext(ctx, `UPDATE assets SET url = $2 WHERE id = $1`, id, u)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT asset_url`)
		return false, err
	}
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `RELEASE SAVEPOINT asset_url`)
	return true, err
}
