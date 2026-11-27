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

const (
	assetURLBackfill         = "asset_url_normalize"
	credentialDomainBackfill = "credential_domain_fold"
)

// BackfillAssetURLs rewrites asset URLs stored before they were normalized on
// insert, recording any stripped userinfo as credentials of the asset's tasks.
// It runs once per database and needs the credential key already set. A URL
// whose normalized form belongs to another asset only has its userinfo
// stripped, and is left as it is when even that collides.
func (s *Store) BackfillAssetURLs(ctx context.Context) error {
	return runBackfill(ctx, s.DB, assetURLBackfill, s.backfillAssetURLs)
}

// BackfillCredentialDomains lowercases stored credential domains in Go, since
// migration 0007 folded only ASCII letters under a C ctype, and merges the rows
// that then collide the same way 0007 does. Stored domains are already
// trimmed and stripped of one trailing dot, and running DomainKey on them
// again would strip a second one. It runs once per database and never
// decrypts a password.
func (s *Store) BackfillCredentialDomains(ctx context.Context) error {
	return runBackfill(ctx, s.DB, credentialDomainBackfill, backfillCredentialDomains)
}

func runBackfill(ctx context.Context, db *sql.DB, name string, fn func(context.Context, *sql.Tx) error) error {
	return withSchemaMigrationLock(ctx, db, func(conn *sql.Conn) error {
		if _, err := conn.ExecContext(ctx, backfillsTable); err != nil {
			return err
		}
		var done bool
		if err := conn.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM store_backfills WHERE name = $1)`, name).Scan(&done); err != nil {
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
		if err := fn(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO store_backfills (name) VALUES ($1)`, name); err != nil {
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

// mergeCredentialDomains is migration 0007 with domain_key taken from
// credential_domain_fold, which holds the lowercased domain of every row it changes.
const mergeCredentialDomains = `
CREATE TEMP TABLE credential_domain_keys ON COMMIT DROP AS
SELECT k.*,
       row_number() OVER (PARTITION BY task_id, domain_key, username ORDER BY updated_at DESC, id DESC) AS rank,
       count(*) OVER (PARTITION BY task_id, domain_key, username) AS group_size
FROM (
    SELECT c.id, c.task_id, c.username, c.password, c.note, c.created_at, c.updated_at,
           COALESCE(f.domain_key, c.domain) AS domain_key
    FROM task_credentials c LEFT JOIN credential_domain_fold f ON f.id = c.id
) k;

UPDATE task_credentials c
SET password   = CASE WHEN c.password = '' THEN COALESCE(m.password, '') ELSE c.password END,
    note       = m.note,
    created_at = m.created_at
FROM (
    SELECT w.id,
           (SELECT k.password FROM credential_domain_keys k
             WHERE (k.task_id, k.domain_key, k.username) = (w.task_id, w.domain_key, w.username)
               AND k.password <> ''
             ORDER BY k.rank LIMIT 1) AS password,
           (SELECT COALESCE(string_agg(n.note, E'\n' ORDER BY n.first_rank), '')
              FROM (SELECT k.note, min(k.rank) AS first_rank FROM credential_domain_keys k
                     WHERE (k.task_id, k.domain_key, k.username) = (w.task_id, w.domain_key, w.username)
                       AND k.note <> ''
                     GROUP BY k.note) n) AS note,
           (SELECT min(k.created_at) FROM credential_domain_keys k
             WHERE (k.task_id, k.domain_key, k.username) = (w.task_id, w.domain_key, w.username)) AS created_at
    FROM credential_domain_keys w
    WHERE w.rank = 1 AND w.group_size > 1
) m
WHERE c.id = m.id;

DELETE FROM task_credentials c
USING credential_domain_keys k
WHERE c.id = k.id AND k.rank > 1;

UPDATE task_credentials c
SET domain = k.domain_key
FROM credential_domain_keys k
WHERE c.id = k.id AND c.domain <> k.domain_key;
`

func backfillCredentialDomains(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, domain FROM task_credentials ORDER BY id FOR UPDATE`)
	if err != nil {
		return err
	}
	fold := map[int64]string{}
	for rows.Next() {
		var id int64
		var domain string
		if err := rows.Scan(&id, &domain); err != nil {
			rows.Close()
			return err
		}
		if k := strings.ToLower(domain); k != domain {
			fold[id] = k
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(fold) == 0 {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE credential_domain_fold (id BIGINT PRIMARY KEY, domain_key TEXT NOT NULL) ON COMMIT DROP`); err != nil {
		return err
	}
	for id, k := range fold {
		if _, err := tx.ExecContext(ctx, `INSERT INTO credential_domain_fold VALUES ($1, $2)`, id, k); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, mergeCredentialDomains)
	return err
}
