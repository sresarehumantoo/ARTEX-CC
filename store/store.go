package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var _ tools.Store = (*Store)(nil)

//go:embed schema.sql
var schemaSQL string

//go:embed all:migrations
var embeddedMigrationFiles embed.FS

const schemaMigrationLockKey int64 = 7337741001

type Store struct {
	*sql.DB
	credentialKey []byte // 32-byte AES-256-GCM key; nil means plaintext
}

func extractHost(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return "unknown"
	}
	return u.Hostname()
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	_ = ensureDatabase(dsn)

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(32)
	sqlDB.SetMaxIdleConns(32)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	s := &Store{DB: sqlDB}

	err = withSchemaMigrationLock(context.Background(), sqlDB, func(conn *sql.Conn) error {
		if err := applySchema(context.Background(), conn); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		if err := applyMigrations(context.Background(), conn, migrationsFS()); err != nil {
			return fmt.Errorf("apply migrations: %w", err)
		}
		return nil
	})
	if err != nil {
		sqlDB.Close()
		return nil, err
	}
	slog.Info("store opened", "dsn_host", extractHost(dsn))
	slog.Info("schema applied")
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) SetCredentialKey(key []byte) { s.credentialKey = key }

func (s *Store) CreateEngagement(ctx context.Context, goal, description string) (taskID int64, explorationID int64, err error) {
	explorationID, err = s.CreateExploration(description, goal)
	if err != nil {
		return 0, 0, err
	}
	err = s.QueryRowContext(ctx,
		`INSERT INTO tasks (name, description, goal, exploration_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		goal, description, goal, explorationID,
	).Scan(&taskID)
	if err != nil {
		return 0, 0, err
	}
	_, err = s.AddNode(ctx, explorationID, KindFact, map[string]any{"text": "task root"}, 0, StateOrigin, "system", nil)
	if err != nil {
		return 0, 0, err
	}
	slog.Info("engagement created", "task_id", taskID, "exploration_id", explorationID)
	return taskID, explorationID, nil
}

func ensureDatabase(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" || dbName == "postgres" {
		return nil
	}
	adminDSN := *u
	adminDSN.Path = "/postgres"
	admin, err := sql.Open("pgx", adminDSN.String())
	if err != nil {
		return nil
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		return nil
	}
	var exists bool
	_ = admin.QueryRow(`SELECT true FROM pg_database WHERE datname=$1`, dbName).Scan(&exists)
	if !exists {
		if _, err := admin.Exec("CREATE DATABASE " + pgx.Identifier{dbName}.Sanitize()); err != nil {
			return fmt.Errorf("create database %q: %w", dbName, err)
		}
	}
	return nil
}

func withSchemaMigrationLock(ctx context.Context, db *sql.DB, fn func(*sql.Conn) error) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, schemaMigrationLockKey); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() {
		if _, unlockErr := conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, schemaMigrationLockKey); unlockErr != nil && err == nil {
			err = fmt.Errorf("advisory unlock: %w", unlockErr)
		}
	}()
	return fn(conn)
}

const schemaStateTable = `
CREATE TABLE IF NOT EXISTS schema_state (
    id       BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    checksum TEXT NOT NULL
);
`

// applySchema runs schema.sql only when it differs from the last applied
// copy. Its DDL locks many tables in one implicit transaction, which can
// deadlock with live traffic from another server or CLI on the same database.
func applySchema(ctx context.Context, conn *sql.Conn) error {
	sum := sha256.Sum256([]byte(schemaSQL))
	checksum := hex.EncodeToString(sum[:])
	if _, err := conn.ExecContext(ctx, schemaStateTable); err != nil {
		return err
	}
	var applied string
	err := conn.QueryRowContext(ctx, `SELECT checksum FROM schema_state`).Scan(&applied)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if applied == checksum {
		return nil
	}
	if _, err := conn.ExecContext(ctx, schemaSQL); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, `INSERT INTO schema_state (id, checksum) VALUES (true, $1)
		ON CONFLICT (id) DO UPDATE SET checksum = EXCLUDED.checksum`, checksum)
	return err
}

// Migration system

const schemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT,
    checksum   TEXT,
    applied_at TIMESTAMPTZ DEFAULT now()
);
`

var migrationNameRE = regexp.MustCompile(`^([0-9]{4})_[a-z0-9_]+\.sql$`)

type migration struct {
	version  int
	name     string
	sql      string
	checksum string
}

func migrationsFS() fs.FS {
	sub, err := fs.Sub(embeddedMigrationFiles, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var out []migration
	seen := map[int]string{}
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".sql" {
			continue
		}
		m := migrationNameRE.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration file %s: name must be NNNN_slug.sql", e.Name())
		}
		version, _ := strconv.Atoi(m[1])
		if version == 0 {
			return nil, fmt.Errorf("migration file %s: version must be > 0", e.Name())
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("migration version %04d is used by both %s and %s", version, prev, e.Name())
		}
		seen[version] = e.Name()
		content, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		sum := sha256.Sum256(content)
		out = append(out, migration{
			version:  version,
			name:     e.Name(),
			sql:      string(content),
			checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func applyMigrations(ctx context.Context, conn *sql.Conn, fsys fs.FS) error {
	migrations, err := loadMigrations(fsys)
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, schemaMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	type appliedRow struct{ name, checksum string }
	applied := map[int]appliedRow{}
	rows, err := conn.QueryContext(ctx, `SELECT version, COALESCE(name,''), COALESCE(checksum,'') FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	var appliedVersions []int
	for rows.Next() {
		var v int
		var r appliedRow
		if err := rows.Scan(&v, &r.name, &r.checksum); err != nil {
			rows.Close()
			return fmt.Errorf("read schema_migrations: %w", err)
		}
		applied[v] = r
		appliedVersions = append(appliedVersions, v)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}

	byVersion := map[int]migration{}
	for _, m := range migrations {
		byVersion[m.version] = m
	}
	for _, v := range appliedVersions {
		r := applied[v]
		m, ok := byVersion[v]
		if !ok {
			return fmt.Errorf("migration %04d (%s) is applied but missing from this binary", v, r.name)
		}
		if r.name != m.name {
			return fmt.Errorf("migration %04d: database records %s but binary has %s", v, r.name, m.name)
		}
		if r.checksum != m.checksum {
			return fmt.Errorf("migration %04d (%s): checksum changed (database %s, file %s)", v, m.name, r.checksum, m.checksum)
		}
	}

	if len(appliedVersions) > 0 {
		maxVersion := appliedVersions[len(appliedVersions)-1]
		for _, m := range migrations {
			if _, done := applied[m.version]; !done && m.version < maxVersion {
				return fmt.Errorf("migration %s (%04d) is older than applied %s (%04d); migrations must be appended",
					m.name, m.version, applied[maxVersion].name, maxVersion)
			}
		}
	}

	for _, m := range migrations {
		if _, done := applied[m.version]; done {
			continue
		}
		if err := applyMigration(ctx, conn, m); err != nil {
			return fmt.Errorf("migration %04d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, conn *sql.Conn, m migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, checksum) VALUES ($1, $2, $3)`, m.version, m.name, m.checksum); err != nil {
		return fmt.Errorf("record: %w", err)
	}
	return tx.Commit()
}
