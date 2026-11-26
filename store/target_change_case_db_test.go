package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/jackc/pgx/v5"
)

func recordResource(t *testing.T, s *Store, expID int64, target, resource, fp string) *tools.TargetChangeRecord {
	t.Helper()
	rec, err := s.RecordTargetChange(context.Background(), expID, tools.TargetChangeInput{
		Target: target, Resource: resource, Kind: "modify", OriginalFingerprint: fp, RecordedBy: "agent",
	})
	if err != nil {
		t.Fatalf("RecordTargetChange(%s, %s): %v", target, resource, err)
	}
	return rec
}

func existingIDs(rec *tools.TargetChangeRecord) []int64 {
	var ids []int64
	for _, e := range rec.Existing {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestTargetChangeResourceCase(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "target-change-case")

	upper := recordResource(t, s, expID, "lnx01", "/srv/App.conf", "sha256:upper")
	lower := recordResource(t, s, expID, " LNX01 ", "/srv/app.conf", "sha256:lower")
	if len(lower.Existing) != 0 || lower.Change.Baseline != "sha256:lower" {
		t.Fatalf("POSIX path differing in case shares a baseline: %+v", lower)
	}
	if again := recordResource(t, s, expID, "Lnx01", "/srv/App.conf", "sha256:x"); !slices.Equal(existingIDs(again), []int64{upper.Change.ID}) {
		t.Errorf("same POSIX path on host differing in case: existing = %v", existingIDs(again))
	}

	for _, pair := range [][2]string{
		{`C:\Windows\Win.ini`, `c:\WINDOWS\win.INI`},
		{`C:/Temp/Drop.ps1`, `c:/temp/drop.ps1`},
		{`\\FS01\Share\Run.bat`, `\\fs01\share\run.bat`},
		{`HKLM\Software\Vendor\Run`, `hklm\SOFTWARE\vendor\run`},
		{`HKEY_LOCAL_MACHINE\System\X`, `hkey_local_machine\system\x`},
		{`HKCU:\Environment`, `hkcu:\environment`},
	} {
		first := recordResource(t, s, expID, "WIN01", pair[0], "sha256:orig")
		second := recordResource(t, s, expID, "win01", pair[1], "sha256:payload")
		if !slices.Equal(existingIDs(second), []int64{first.Change.ID}) || second.Change.Baseline != "sha256:orig" {
			t.Errorf("%s and %s should share a baseline: %+v", pair[0], pair[1], second)
		}
	}

	if r := recordResource(t, s, expID, "lnx01", "SSHD", "x"); len(r.Existing) != 0 {
		t.Fatal(r.Existing)
	}
	if r := recordResource(t, s, expID, "lnx01", "sshd", "y"); len(r.Existing) != 0 {
		t.Errorf("service names differing in case share a baseline: %v", existingIDs(r))
	}

	out, err := s.RecordTargetRestore(ctx, expID, []int64{lower.Change.ID}, "sha256:lower", "agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Status != tools.RestoreStatusRestored || len(out[0].AlsoRestored) != 0 {
		t.Fatalf("restore outcome = %+v", out)
	}
	live, err := s.ListTargetChanges(ctx, expID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(live, func(c tools.TargetChange) bool { return c.ID == upper.Change.ID }) {
		t.Error("restoring /srv/app.conf restored /srv/App.conf")
	}
}

func TestTargetChangeCaseMigrationOnExistingRows(t *testing.T) {
	dsn := os.Getenv("ARTEX_TEST_PG")
	if dsn == "" {
		t.Skip("ARTEX_TEST_PG not set")
	}
	ctx := context.Background()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	dbName := fmt.Sprintf("artex_case_migration_%d", time.Now().UnixNano())
	u.Path = "/" + dbName
	if err := ensureDatabase(u.String()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin := *u
		admin.Path = "/postgres"
		db, err := sql.Open("pgx", admin.String())
		if err != nil {
			return
		}
		defer db.Close()
		_, _ = db.Exec("DROP DATABASE IF EXISTS " + pgx.Identifier{dbName}.Sanitize() + " WITH (FORCE)")
	})

	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Store{DB: db}

	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() >= "0008" {
			continue
		}
		body, err := fs.ReadFile(migrationsFS(), e.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[e.Name()] = &fstest.MapFile{Data: body}
	}
	migrate := func(fsys fs.FS) {
		t.Helper()
		if err := withSchemaMigrationLock(ctx, db, func(conn *sql.Conn) error {
			if err := applySchema(ctx, conn); err != nil {
				return err
			}
			return applyMigrations(ctx, conn, fsys)
		}); err != nil {
			t.Fatal(err)
		}
	}
	migrate(before)

	_, expID := testEngagement(t, s, "case-migration")
	insert := func(target, resource, fp string) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRowContext(ctx,
			`INSERT INTO target_changes (exploration_id, target, resource, kind, original_fingerprint, baseline)
			 VALUES ($1, $2, $3, 'modify', $4, $4) RETURNING id`, expID, target, resource, fp).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE target_changes SET baseline_change_id = id WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	posixUpper := insert("lnx01", "/srv/App.conf", "sha256:upper")
	insert("LNX01", "/srv/app.conf", "sha256:lower")
	winUpper := insert("WIN01", `C:\Temp\A.txt`, "sha256:a")
	winLower := insert("win01", `c:\temp\a.txt`, "sha256:b")

	migrate(migrationsFS())

	var oldIndex, newIndex bool
	if err := db.QueryRowContext(ctx,
		`SELECT to_regclass('idx_target_changes_live') IS NOT NULL, to_regclass('idx_target_changes_live_key') IS NOT NULL`,
	).Scan(&oldIndex, &newIndex); err != nil {
		t.Fatal(err)
	}
	if oldIndex || !newIndex {
		t.Errorf("old index present %v, new index present %v", oldIndex, newIndex)
	}

	if r := recordResource(t, s, expID, "Lnx01", "/srv/App.conf", "x"); !slices.Equal(existingIDs(r), []int64{posixUpper}) {
		t.Errorf("POSIX existing = %v, want [%d]", existingIDs(r), posixUpper)
	}
	if r := recordResource(t, s, expID, "WIN01", `C:\TEMP\a.TXT`, "x"); !slices.Equal(existingIDs(r), []int64{winUpper, winLower}) {
		t.Errorf("Windows existing = %v, want [%d %d]", existingIDs(r), winUpper, winLower)
	}
}
