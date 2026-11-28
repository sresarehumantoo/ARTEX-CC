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

// migrationDB opens a fresh database and returns a migrate func that applies
// the schema plus the embedded migrations whose names sort below before.
func migrationDB(t *testing.T) (*Store, func(before string)) {
	t.Helper()
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
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		admin := *u
		admin.Path = "/postgres"
		adb, err := sql.Open("pgx", admin.String())
		if err != nil {
			return
		}
		defer adb.Close()
		_, _ = adb.Exec("DROP DATABASE IF EXISTS " + pgx.Identifier{dbName}.Sanitize() + " WITH (FORCE)")
	})

	migrate := func(before string) {
		t.Helper()
		fsys := fstest.MapFS{}
		entries, err := fs.ReadDir(migrationsFS(), ".")
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() >= before {
				continue
			}
			body, err := fs.ReadFile(migrationsFS(), e.Name())
			if err != nil {
				t.Fatal(err)
			}
			fsys[e.Name()] = &fstest.MapFile{Data: body}
		}
		if err := withSchemaMigrationLock(ctx, db, func(conn *sql.Conn) error {
			if err := applySchema(ctx, conn); err != nil {
				return err
			}
			return applyMigrations(ctx, conn, fsys)
		}); err != nil {
			t.Fatal(err)
		}
	}
	return &Store{DB: db}, migrate
}

func TestTargetChangeCaseMigrationOnExistingRows(t *testing.T) {
	s, migrate := migrationDB(t)
	db := s.DB
	ctx := context.Background()
	migrate("0008")

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

	migrate("9999")

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

func TestTargetChangeBaselineRecomputeMigration(t *testing.T) {
	s, migrate := migrationDB(t)
	ctx := context.Background()
	migrate("0009")
	// The journal code below reads 0010's columns; adding them first leaves
	// 0009 to run against the rows it would have met.
	additive, err := fs.ReadFile(migrationsFS(), "0010_unowned_target_changes.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(ctx, string(additive)); err != nil {
		t.Fatal(err)
	}
	_, expID := testEngagement(t, s, "baseline-recompute")

	u1 := recordResource(t, s, expID, "lnx01", "/srv/App.conf", "sha256:upper").Change.ID
	recordResource(t, s, expID, "lnx01", "/srv/App.conf", "sha256:upper2")
	if _, err := s.RecordTargetRestore(ctx, expID, []int64{u1}, "sha256:upper", "agent"); err != nil {
		t.Fatal(err)
	}
	u3 := recordResource(t, s, expID, "lnx01", "/srv/App.conf", "sha256:upper3").Change.ID
	l1 := recordResource(t, s, expID, "LNX01", "/srv/app.conf", "sha256:lower").Change.ID
	l2 := recordResource(t, s, expID, "lnx01", "/srv/app.conf", "sha256:lower2").Change.ID
	recordResource(t, s, expID, "WIN01", `C:\Temp\A.txt`, "sha256:a")
	recordResource(t, s, expID, "win01", `c:\temp\a.txt`, "sha256:b")

	type row struct {
		baseline   string
		baselineID int64
		xmin       string
	}
	snapshot := func() map[int64]row {
		t.Helper()
		rows, err := s.QueryContext(ctx,
			`SELECT id, baseline, baseline_change_id, xmin::text FROM target_changes WHERE exploration_id = $1`, expID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[int64]row{}
		for rows.Next() {
			var id int64
			var r row
			if err := rows.Scan(&id, &r.baseline, &r.baselineID, &r.xmin); err != nil {
				t.Fatal(err)
			}
			out[id] = r
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	want := snapshot()

	// 0005 folded case, so both /srv/app.conf changes joined /srv/App.conf's live chain.
	if _, err := s.ExecContext(ctx,
		`UPDATE target_changes SET baseline = 'sha256:upper3', baseline_change_id = $1 WHERE id IN ($2, $3)`,
		u3, l1, l2); err != nil {
		t.Fatal(err)
	}
	corrupted := snapshot()

	migrate("9999")

	got := snapshot()
	for id, w := range want {
		g := got[id]
		if g.baseline != w.baseline || g.baselineID != w.baselineID {
			t.Errorf("change %d baseline = %s (#%d), want %s (#%d)", id, g.baseline, g.baselineID, w.baseline, w.baselineID)
		}
		if id != l1 && id != l2 && g.xmin != corrupted[id].xmin {
			t.Errorf("change %d was rewritten although its baseline was correct", id)
		}
	}
	if got[l2].baselineID != l1 || got[l2].baseline != "sha256:lower" {
		t.Errorf("/srv/app.conf follow-up baseline = %+v, want sha256:lower (#%d)", got[l2], l1)
	}

	if r := recordResource(t, s, expID, "lnx01", "/srv/app.conf", "x"); r.Change.BaselineChangeID != l1 || r.Change.Baseline != "sha256:lower" {
		t.Errorf("new /srv/app.conf change baseline = %s (#%d), want sha256:lower (#%d)", r.Change.Baseline, r.Change.BaselineChangeID, l1)
	}
}
