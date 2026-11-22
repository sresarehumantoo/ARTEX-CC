package store

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestLoadMigrations(t *testing.T) {
	t.Parallel()

	t.Run("valid single migration", func(t *testing.T) {
		fsys := fstest.MapFS{
			"0001_initial.sql": {Data: []byte("CREATE TABLE t (id int);")},
		}
		migs, err := loadMigrations(fsys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(migs) != 1 {
			t.Fatalf("got %d migrations, want 1", len(migs))
		}
		if migs[0].version != 1 {
			t.Errorf("version = %d, want 1", migs[0].version)
		}
		if migs[0].name != "0001_initial.sql" {
			t.Errorf("name = %q, want %q", migs[0].name, "0001_initial.sql")
		}
		if migs[0].sql != "CREATE TABLE t (id int);" {
			t.Errorf("sql = %q", migs[0].sql)
		}
		if migs[0].checksum == "" {
			t.Error("checksum should not be empty")
		}
	})

	t.Run("multiple migrations sorted", func(t *testing.T) {
		fsys := fstest.MapFS{
			"0003_third.sql":  {Data: []byte("ALTER TABLE t ADD col3;")},
			"0001_first.sql":  {Data: []byte("CREATE TABLE t;")},
			"0002_second.sql": {Data: []byte("ALTER TABLE t ADD col2;")},
		}
		migs, err := loadMigrations(fsys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(migs) != 3 {
			t.Fatalf("got %d migrations, want 3", len(migs))
		}
		if migs[0].version != 1 || migs[1].version != 2 || migs[2].version != 3 {
			t.Errorf("versions = [%d, %d, %d], want [1, 2, 3]",
				migs[0].version, migs[1].version, migs[2].version)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		fsys := fstest.MapFS{}
		migs, err := loadMigrations(fsys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(migs) != 0 {
			t.Errorf("got %d migrations, want 0", len(migs))
		}
	})

	t.Run("skips non-sql files", func(t *testing.T) {
		fsys := fstest.MapFS{
			"0001_first.sql": {Data: []byte("CREATE TABLE t;")},
			"README.md":      {Data: []byte("not a migration")},
			"notes.txt":      {Data: []byte("also not a migration")},
		}
		migs, err := loadMigrations(fsys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(migs) != 1 {
			t.Fatalf("got %d migrations, want 1", len(migs))
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		fsys := fstest.MapFS{
			"0001_first.sql": {Data: []byte("CREATE TABLE t;")},
			"subdir":         {Mode: fs.ModeDir},
		}
		migs, err := loadMigrations(fsys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(migs) != 1 {
			t.Fatalf("got %d migrations, want 1", len(migs))
		}
	})

	t.Run("invalid name format", func(t *testing.T) {
		fsys := fstest.MapFS{
			"bad_name.sql": {Data: []byte("CREATE TABLE t;")},
		}
		_, err := loadMigrations(fsys)
		if err == nil {
			t.Error("expected error for invalid migration name")
		}
	})

	t.Run("version zero", func(t *testing.T) {
		fsys := fstest.MapFS{
			"0000_zero.sql": {Data: []byte("CREATE TABLE t;")},
		}
		_, err := loadMigrations(fsys)
		if err == nil {
			t.Error("expected error for version 0")
		}
	})

	t.Run("duplicate versions", func(t *testing.T) {
		fsys := fstest.MapFS{
			"0001_first.sql":     {Data: []byte("CREATE TABLE t;")},
			"0001_duplicate.sql": {Data: []byte("ALTER TABLE t;")},
		}
		_, err := loadMigrations(fsys)
		if err == nil {
			t.Error("expected error for duplicate version")
		}
	})

	t.Run("checksum is deterministic", func(t *testing.T) {
		content := []byte("SELECT 1;")
		fsys := fstest.MapFS{
			"0001_test.sql": {Data: content},
		}
		migs1, _ := loadMigrations(fsys)
		migs2, _ := loadMigrations(fsys)
		if migs1[0].checksum != migs2[0].checksum {
			t.Errorf("checksums differ: %q vs %q", migs1[0].checksum, migs2[0].checksum)
		}
	})
}

func TestMigrationNameRegexp(t *testing.T) {
	t.Parallel()
	valid := []string{
		"0001_initial.sql",
		"0042_add_column.sql",
		"9999_final.sql",
		"0001_a.sql",
		"0001_a_b_c.sql",
		"0001_add_123.sql",
	}
	for _, name := range valid {
		if !migrationNameRE.MatchString(name) {
			t.Errorf("expected %q to match migration name pattern", name)
		}
	}

	invalid := []string{
		"bad.sql",
		"001_short.sql",
		"0001_CamelCase.sql",
		"0001_.sql",
		"initial.sql",
		"0001_initial.txt",
		"0001-dashes.sql",
	}
	for _, name := range invalid {
		if migrationNameRE.MatchString(name) {
			t.Errorf("expected %q to NOT match migration name pattern", name)
		}
	}
}

func TestExtractHost(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dsn  string
		want string
	}{
		{"postgres://user:pass@localhost:5432/db", "localhost"},
		{"postgres://user:pass@10.0.0.1:5432/db", "10.0.0.1"},
		{"postgres://user:pass@db.example.com/mydb", "db.example.com"},
		{"not-a-url", "unknown"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		got := extractHost(tt.dsn)
		if got != tt.want {
			t.Errorf("extractHost(%q) = %q, want %q", tt.dsn, got, tt.want)
		}
	}
}

func TestNilIfEmpty(t *testing.T) {
	t.Parallel()
	if nilIfEmpty("") != nil {
		t.Error("nilIfEmpty(\"\") should return nil")
	}
	if nilIfEmpty("hello") != "hello" {
		t.Error("nilIfEmpty(\"hello\") should return \"hello\"")
	}
}

func TestMigrationsFS(t *testing.T) {
	t.Parallel()
	fsys := migrationsFS()
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatalf("failed to read migrations FS: %v", err)
	}
	if len(entries) == 0 {
		t.Error("migrations directory is empty")
	}
	for _, e := range entries {
		if !migrationNameRE.MatchString(e.Name()) && e.Name()[len(e.Name())-4:] == ".sql" {
			t.Errorf("migration file %q does not match naming pattern", e.Name())
		}
	}
}

func TestLoadEmbeddedMigrations(t *testing.T) {
	t.Parallel()
	migs, err := loadMigrations(migrationsFS())
	if err != nil {
		t.Fatalf("loadMigrations on embedded FS: %v", err)
	}
	if len(migs) == 0 {
		t.Error("no embedded migrations found")
	}
	for i := 1; i < len(migs); i++ {
		if migs[i].version <= migs[i-1].version {
			t.Errorf("migrations not sorted: version %d after %d", migs[i].version, migs[i-1].version)
		}
	}
}
