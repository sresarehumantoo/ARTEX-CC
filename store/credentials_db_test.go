package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestSaveCredentialDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO task_credentials").
		WithArgs(int64(1), "app.example.com", "admin", "secret123", "agent", "found in config").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(5)))

	id, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
		Domain:   "app.example.com",
		Username: "admin",
		Password: "secret123",
		Source:   "agent",
		Note:     "found in config",
	})
	if err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	if id != 5 {
		t.Errorf("id = %d, want 5", id)
	}
}

func TestSaveCredentialDefaultSource(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO task_credentials").
		WithArgs(int64(1), "app.example.com", "admin", "pass", "agent", "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
		Domain:   "app.example.com",
		Username: "admin",
		Password: "pass",
	})
	if err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
}

func TestSaveCredentialEmptyDomain(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
		Username: "admin",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestSaveCredentialEmptyUsername(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
		Domain:   "app.example.com",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestSaveCredentialAgentNoPassword(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)
	ctx := context.Background()

	_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
		Domain:   "app.example.com",
		Username: "admin",
		Source:   "agent",
	})
	if err == nil {
		t.Fatal("expected error for agent without password")
	}
}

func TestSaveCredentialManualNoPassword(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO task_credentials").
		WithArgs(int64(1), "app.example.com", "admin", "", "manual", "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
		Domain:   "app.example.com",
		Username: "admin",
		Source:   "manual",
	})
	if err != nil {
		t.Fatalf("SaveCredential: %v (manual source should allow empty password)", err)
	}
}

func TestSaveCredentialWithEncryptionKey(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	s.credentialKey = make([]byte, 32)
	for i := range s.credentialKey {
		s.credentialKey[i] = byte(i)
	}
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO task_credentials").
		WithArgs(int64(1), "app.example.com", "admin", sqlmock.AnyArg(), "agent", "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
		Domain:   "app.example.com",
		Username: "admin",
		Password: "secret123",
		Source:   "agent",
	})
	if err != nil {
		t.Fatalf("SaveCredential with encryption: %v", err)
	}
}

func TestListCredentialsDB(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "domain", "username", "password", "source", "note"}).
		AddRow(int64(1), "app.example.com", "admin", "hunter2", "agent", "login form").
		AddRow(int64(2), "api.example.com", "svc", "token123", "manual", "")

	mock.ExpectQuery("SELECT .+ FROM task_credentials").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	creds, err := s.ListCredentials(ctx, 1)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("got %d creds, want 2", len(creds))
	}
	if creds[0].Domain != "app.example.com" {
		t.Errorf("Domain = %q", creds[0].Domain)
	}
	if creds[0].Password != "hunter2" {
		t.Errorf("Password = %q, want %q", creds[0].Password, "hunter2")
	}
}

func TestListCredentialsWithDecryption(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	s.credentialKey = key
	ctx := context.Background()

	encrypted, err := aesGCMEncrypt(key, "decrypted_pass")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	rows := sqlmock.NewRows([]string{"id", "domain", "username", "password", "source", "note"}).
		AddRow(int64(1), "app.example.com", "admin", "enc:"+encrypted, "agent", "")

	mock.ExpectQuery("SELECT .+ FROM task_credentials").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	creds, err := s.ListCredentials(ctx, 1)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("got %d creds, want 1", len(creds))
	}
	if creds[0].Password != "decrypted_pass" {
		t.Errorf("Password = %q, want %q", creds[0].Password, "decrypted_pass")
	}
}

func TestListCredentialsEmpty(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .+ FROM task_credentials").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "domain", "username", "password", "source", "note"}))

	creds, err := s.ListCredentials(ctx, 1)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	if creds != nil {
		t.Errorf("expected nil, got %d", len(creds))
	}
}
