package store

import (
	"context"
	"fmt"
	"io/fs"
	"reflect"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func runCredentialDomainMigration(t *testing.T, s *Store) {
	t.Helper()
	body, err := fs.ReadFile(migrationsFS(), "0007_backfill_credential_domains.sql")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		t.Fatalf("migration: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

type rawCredential struct {
	ID        int64
	Domain    string
	Username  string
	Password  string
	Source    string
	Note      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func insertRawCredential(t *testing.T, s *Store, taskID int64, c rawCredential) int64 {
	t.Helper()
	var id int64
	err := s.QueryRowContext(context.Background(), `
		INSERT INTO task_credentials (task_id, domain, username, password, source, note, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		taskID, c.Domain, c.Username, c.Password, c.Source, c.Note, c.CreatedAt, c.UpdatedAt).Scan(&id)
	if err != nil {
		t.Fatalf("insert credential %q: %v", c.Domain, err)
	}
	return id
}

func taskCredentials(t *testing.T, s *Store, taskID int64) []rawCredential {
	t.Helper()
	rows, err := s.QueryContext(context.Background(), `
		SELECT id, domain, username, password, source, note, created_at, updated_at
		FROM task_credentials WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []rawCredential
	for rows.Next() {
		var c rawCredential
		if err := rows.Scan(&c.ID, &c.Domain, &c.Username, &c.Password, &c.Source, &c.Note, &c.CreatedAt, &c.UpdatedAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCredentialDomainMigrationMatchesDomainKey(t *testing.T) {
	s := testDB(t)
	taskID, _ := testEngagement(t, s, "cred-domain-key")
	domains := []string{
		"Example.com",
		"EXAMPLE.COM.",
		"  app.Example.com.\t",
		" intra.example.com　",
		"double.dot..",
		"dot.then.space. ",
		"ÄBC.Example.com",
		"Straße.DE",
		"10.0.0.1",
		"already.normal",
	}
	now := time.Now()
	for i, d := range domains {
		insertRawCredential(t, s, taskID, rawCredential{
			Domain: d, Username: fmt.Sprintf("user%d", i), Password: "pw", Source: "user", CreatedAt: now, UpdatedAt: now,
		})
	}
	runCredentialDomainMigration(t, s)
	got := taskCredentials(t, s, taskID)
	if len(got) != len(domains) {
		t.Fatalf("got %d credentials, want %d", len(got), len(domains))
	}
	for i, c := range got {
		if want := DomainKey(domains[i]); c.Domain != want {
			t.Errorf("domain %q: migration gave %q, DomainKey gives %q", domains[i], c.Domain, want)
		}
	}
}

func TestCredentialDomainMigrationMergesCollisions(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "cred-domain-merge")
	otherTask, _ := testEngagement(t, s, "cred-domain-merge-other")
	now := time.Now().Truncate(time.Microsecond)

	oldest := insertRawCredential(t, s, taskID, rawCredential{
		Domain: "Example.com", Username: "admin", Password: "enc:AAA", Source: "agent", Note: "from config",
		CreatedAt: now.Add(-5 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour),
	})
	newest := insertRawCredential(t, s, taskID, rawCredential{
		Domain: "EXAMPLE.COM.", Username: "admin", Password: "enc:BBB", Source: "user",
		CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-1 * time.Hour),
	})
	insertRawCredential(t, s, taskID, rawCredential{
		Domain: "example.com", Username: "admin", Source: "user", Note: "seen in mail",
		CreatedAt: now.Add(-4 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour),
	})
	insertRawCredential(t, s, taskID, rawCredential{
		Domain: "Dup.Example.com", Username: "admin", Password: "enc:DUP", Source: "agent", Note: "same note",
		CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour),
	})
	insertRawCredential(t, s, taskID, rawCredential{
		Domain: "dup.example.com", Username: "admin", Password: "enc:DUP2", Source: "agent", Note: "same note",
		CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now.Add(-1 * time.Hour),
	})
	emptyWinner := insertRawCredential(t, s, taskID, rawCredential{
		Domain: "Admin.Example.com", Username: "root", Source: "user",
		CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now.Add(-1 * time.Hour),
	})
	insertRawCredential(t, s, taskID, rawCredential{
		Domain: "admin.example.com", Username: "root", Password: "enc:EEE", Source: "agent",
		CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour),
	})
	caseUser := insertRawCredential(t, s, taskID, rawCredential{
		Domain: "Example.com", Username: "Admin", Password: "plain", Source: "user",
		CreatedAt: now, UpdatedAt: now,
	})
	otherID := insertRawCredential(t, s, otherTask, rawCredential{
		Domain: "EXAMPLE.com", Username: "admin", Password: "other", Source: "user",
		CreatedAt: now, UpdatedAt: now,
	})

	runCredentialDomainMigration(t, s)

	byID := map[int64]rawCredential{}
	for _, c := range taskCredentials(t, s, taskID) {
		byID[c.ID] = c
	}
	if len(byID) != 4 {
		t.Fatalf("got %d credentials, want 4: %+v", len(byID), byID)
	}
	want := rawCredential{
		ID: newest, Domain: "example.com", Username: "admin", Password: "enc:BBB", Source: "user",
		Note: "seen in mail\nfrom config", CreatedAt: now.Add(-5 * time.Hour), UpdatedAt: now.Add(-1 * time.Hour),
	}
	if got := byID[newest]; !got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("merged timestamps = %v/%v, want %v/%v", got.CreatedAt, got.UpdatedAt, want.CreatedAt, want.UpdatedAt)
	}
	got := byID[newest]
	got.CreatedAt, got.UpdatedAt, want.CreatedAt, want.UpdatedAt = time.Time{}, time.Time{}, time.Time{}, time.Time{}
	if got != want {
		t.Errorf("merged credential = %+v, want %+v", got, want)
	}
	if _, ok := byID[oldest]; ok {
		t.Error("older duplicate was not deleted")
	}
	var dup rawCredential
	for _, c := range byID {
		if c.Domain == "dup.example.com" {
			dup = c
		}
	}
	if dup.Password != "enc:DUP2" || dup.Note != "same note" {
		t.Errorf("dup merge = %+v, want newest password and one copy of the note", dup)
	}
	if c := byID[emptyWinner]; c.Domain != "admin.example.com" || c.Password != "enc:EEE" || c.Source != "user" {
		t.Errorf("empty-password winner = %+v, want domain admin.example.com, password enc:EEE, source user", c)
	}
	if c := byID[caseUser]; c.Domain != "example.com" || c.Username != "Admin" || c.Password != "plain" {
		t.Errorf("distinct username = %+v, want it kept apart", c)
	}
	other := taskCredentials(t, s, otherTask)
	if len(other) != 1 || other[0].ID != otherID || other[0].Domain != "example.com" {
		t.Errorf("other task credentials = %+v, want one normalized row", other)
	}

	before := taskCredentials(t, s, taskID)
	runCredentialDomainMigration(t, s)
	if after := taskCredentials(t, s, taskID); !reflect.DeepEqual(before, after) {
		t.Errorf("second run changed credentials:\nbefore %+v\nafter  %+v", before, after)
	}

	if _, err := s.SaveCredential(ctx, taskID, tools.CredentialInput{
		Domain: "Example.COM", Username: "admin", Password: "fresh", Source: "user",
	}); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range taskCredentials(t, s, taskID) {
		if c.Username == "admin" && c.Domain == "example.com" {
			n++
			if c.ID != newest || c.Password != "fresh" {
				t.Errorf("save after migration = %+v, want update of row %d", c, newest)
			}
		}
	}
	if n != 1 {
		t.Errorf("got %d example.com/admin rows after save, want 1", n)
	}
}
