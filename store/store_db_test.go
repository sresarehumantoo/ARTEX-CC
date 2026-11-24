package store

import (
	"testing"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestClose(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{DB: db}
	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSetCredentialKey(t *testing.T) {
	t.Parallel()
	s := &Store{}
	key := make([]byte, 32)
	s.SetCredentialKey(key)
	if s.credentialKey == nil {
		t.Fatal("credentialKey should be set")
	}
	if len(s.credentialKey) != 32 {
		t.Fatalf("credentialKey length = %d, want 32", len(s.credentialKey))
	}
}

func TestApplyMigrations_AllNew(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	fsys := fstest.MapFS{
		"0001_first.sql":  {Data: []byte("CREATE TABLE t1 (id int);")},
		"0002_second.sql": {Data: []byte("ALTER TABLE t1 ADD col text;")},
	}

	conn, err := db.Conn(ctx())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT version, COALESCE\\(name,''\\), COALESCE\\(checksum,''\\) FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}))

	// migration 1
	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE t1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(1, "0001_first.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// migration 2
	mock.ExpectBegin()
	mock.ExpectExec("ALTER TABLE t1 ADD col text").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(2, "0002_second.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := applyMigrations(ctx(), conn, fsys); err != nil {
		t.Fatalf("applyMigrations: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyMigrations_AlreadyApplied(t *testing.T) {
	t.Parallel()
	content := []byte("CREATE TABLE t1 (id int);")
	fsys := fstest.MapFS{
		"0001_first.sql": {Data: content},
	}
	migs, _ := loadMigrations(fsys)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	conn, err := db.Conn(ctx())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT version, COALESCE\\(name,''\\), COALESCE\\(checksum,''\\) FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}).
			AddRow(1, "0001_first.sql", migs[0].checksum))

	if err := applyMigrations(ctx(), conn, fsys); err != nil {
		t.Fatalf("applyMigrations: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyMigrations_ChecksumMismatch(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"0001_first.sql": {Data: []byte("CREATE TABLE t1 (id int);")},
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	conn, err := db.Conn(ctx())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT version, COALESCE\\(name,''\\), COALESCE\\(checksum,''\\) FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}).
			AddRow(1, "0001_first.sql", "wrong_checksum"))

	err = applyMigrations(ctx(), conn, fsys)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestApplyMigrations_MissingFromBinary(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	conn, err := db.Conn(ctx())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT version, COALESCE\\(name,''\\), COALESCE\\(checksum,''\\) FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}).
			AddRow(1, "0001_ghost.sql", "abc123"))

	err = applyMigrations(ctx(), conn, fsys)
	if err == nil {
		t.Fatal("expected missing-from-binary error")
	}
}

func TestApplyMigrations_OutOfOrder(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"0001_first.sql": {Data: []byte("SELECT 1;")},
		"0003_third.sql": {Data: []byte("SELECT 3;")},
	}
	migs, _ := loadMigrations(fsys)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	conn, err := db.Conn(ctx())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT version, COALESCE\\(name,''\\), COALESCE\\(checksum,''\\) FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}).
			AddRow(3, "0003_third.sql", migs[1].checksum))

	err = applyMigrations(ctx(), conn, fsys)
	if err == nil {
		t.Fatal("expected out-of-order error")
	}
}

func TestApplyMigrations_NameMismatch(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"0001_first.sql": {Data: []byte("SELECT 1;")},
	}
	migs, _ := loadMigrations(fsys)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	conn, err := db.Conn(ctx())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT version, COALESCE\\(name,''\\), COALESCE\\(checksum,''\\) FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}).
			AddRow(1, "0001_other.sql", migs[0].checksum))

	err = applyMigrations(ctx(), conn, fsys)
	if err == nil {
		t.Fatal("expected name mismatch error")
	}
}
