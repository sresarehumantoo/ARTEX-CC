package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type credentialMockStore struct {
	mockStore
	savedID     int64
	saveErr     error
	credentials []Credential
	listErr     error
}

func (m *credentialMockStore) SaveCredential(_ context.Context, _ int64, _ CredentialInput) (int64, error) {
	return m.savedID, m.saveErr
}

func (m *credentialMockStore) ListCredentials(_ context.Context, _ int64) ([]Credential, error) {
	return m.credentials, m.listErr
}

func TestSaveTaskCredentialHappyPath(t *testing.T) {
	t.Parallel()
	ms := &credentialMockStore{savedID: 42}
	reg := NewRegistry()
	RegisterCredentialTools(reg, ms, 1)

	tool, ok := reg.Lookup("save_task_credential")
	if !ok {
		t.Fatal("save_task_credential not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"domain": "app.example.com",
		"username": "testuser",
		"password": "P@ssw0rd!",
		"note": "registered via signup"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"id":42`) {
		t.Errorf("expected id 42, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"domain":"app.example.com"`) {
		t.Errorf("expected domain, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"source":"agent"`) {
		t.Errorf("expected source agent, got: %s", r.Content)
	}
}

func TestSaveTaskCredentialNoTask(t *testing.T) {
	t.Parallel()
	ms := &credentialMockStore{}
	reg := NewRegistry()
	RegisterCredentialTools(reg, ms, 0)

	tool, _ := reg.Lookup("save_task_credential")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"domain": "app.example.com",
		"username": "testuser",
		"password": "P@ssw0rd!"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when taskID is 0")
	}
	if !strings.Contains(r.Content, "only available inside a task") {
		t.Errorf("expected task context error, got: %s", r.Content)
	}
}

func TestSaveTaskCredentialStoreError(t *testing.T) {
	t.Parallel()
	ms := &credentialMockStore{saveErr: fmt.Errorf("encryption failed")}
	reg := NewRegistry()
	RegisterCredentialTools(reg, ms, 1)

	tool, _ := reg.Lookup("save_task_credential")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"domain": "app.example.com",
		"username": "user",
		"password": "pass"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "encryption failed") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestSaveTaskCredentialInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &credentialMockStore{}
	reg := NewRegistry()
	RegisterCredentialTools(reg, ms, 1)

	tool, _ := reg.Lookup("save_task_credential")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(r.Content, "invalid input:") {
		t.Errorf("expected 'invalid input:', got: %s", r.Content)
	}
}

func TestListTaskCredentialsHappyPath(t *testing.T) {
	t.Parallel()
	ms := &credentialMockStore{
		credentials: []Credential{
			{ID: 1, Domain: "app.example.com", Username: "user1", Source: "agent"},
			{ID: 2, Domain: "api.example.com", Username: "admin", Source: "operator"},
		},
	}
	reg := NewRegistry()
	RegisterCredentialTools(reg, ms, 1)

	tool, ok := reg.Lookup("list_task_credentials")
	if !ok {
		t.Fatal("list_task_credentials not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"credentials"`) {
		t.Errorf("expected credentials key, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"user1"`) {
		t.Errorf("expected user1 in result, got: %s", r.Content)
	}
}

func TestListTaskCredentialsEmpty(t *testing.T) {
	t.Parallel()
	ms := &credentialMockStore{credentials: []Credential{}}
	reg := NewRegistry()
	RegisterCredentialTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_task_credentials")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestListTaskCredentialsNoTask(t *testing.T) {
	t.Parallel()
	ms := &credentialMockStore{}
	reg := NewRegistry()
	RegisterCredentialTools(reg, ms, 0)

	tool, _ := reg.Lookup("list_task_credentials")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when taskID is 0")
	}
	if !strings.Contains(r.Content, "only available inside a task") {
		t.Errorf("expected task context error, got: %s", r.Content)
	}
}

func TestListTaskCredentialsStoreError(t *testing.T) {
	t.Parallel()
	ms := &credentialMockStore{listErr: fmt.Errorf("access denied")}
	reg := NewRegistry()
	RegisterCredentialTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_task_credentials")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "access denied") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}
