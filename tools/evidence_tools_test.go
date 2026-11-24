package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/evidence"
)

type evidenceMockStore struct {
	mockStore
	findingExists    bool
	findingExistsErr error
	inScope          bool
	scopeReason      string
}

func (m *evidenceMockStore) FindingExists(_ context.Context, _ int64) (bool, error) {
	return m.findingExists, m.findingExistsErr
}

func (m *evidenceMockStore) IsInScope(_ context.Context, _ int64, _ string) (bool, string, string, error) {
	return m.inScope, m.scopeReason, "", nil
}

func newEvidenceStore(t *testing.T) *evidence.Store {
	t.Helper()
	return evidence.NewStore(t.TempDir())
}

func TestSaveEvidenceHappy(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: true}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, ok := reg.Lookup("save_evidence")
	if !ok {
		t.Fatal("save_evidence not found")
	}

	content := base64.StdEncoding.EncodeToString([]byte("test evidence content"))
	input := fmt.Sprintf(`{
		"finding_id": 1,
		"kind": "poc_output",
		"filename": "test.txt",
		"content_base64": %q
	}`, content)

	r, err := tool.Run(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"sha256"`) {
		t.Errorf("expected sha256, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"id"`) {
		t.Errorf("expected id, got: %s", r.Content)
	}
}

func TestSaveEvidenceMissingFindingID(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("save_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"kind": "screenshot",
		"filename": "test.png",
		"content_base64": "dGVzdA=="
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing finding_id")
	}
	if !strings.Contains(r.Content, "finding_id is required") {
		t.Errorf("expected finding_id required, got: %s", r.Content)
	}
}

func TestSaveEvidenceMissingKind(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("save_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"kind": "",
		"filename": "test.png",
		"content_base64": "dGVzdA=="
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing kind")
	}
	if !strings.Contains(r.Content, "kind is required") {
		t.Errorf("expected kind required, got: %s", r.Content)
	}
}

func TestSaveEvidenceMissingFilename(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("save_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"kind": "screenshot",
		"filename": "",
		"content_base64": "dGVzdA=="
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing filename")
	}
	if !strings.Contains(r.Content, "filename is required") {
		t.Errorf("expected filename required, got: %s", r.Content)
	}
}

func TestSaveEvidenceMissingContent(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("save_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"kind": "screenshot",
		"filename": "test.png",
		"content_base64": ""
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing content")
	}
	if !strings.Contains(r.Content, "content_base64 is required") {
		t.Errorf("expected content_base64 required, got: %s", r.Content)
	}
}

func TestSaveEvidenceFindingNotFound(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: false}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("save_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 999,
		"kind": "poc_output",
		"filename": "test.txt",
		"content_base64": "dGVzdA=="
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing finding")
	}
	if !strings.Contains(r.Content, "not found") {
		t.Errorf("expected finding not found, got: %s", r.Content)
	}
}

func TestSaveEvidenceInvalidBase64(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: true}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("save_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"kind": "poc_output",
		"filename": "test.txt",
		"content_base64": "not!valid!base64!!!"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid base64")
	}
	if !strings.Contains(r.Content, "invalid base64") {
		t.Errorf("expected invalid base64 error, got: %s", r.Content)
	}
}

func TestSaveEvidenceInvalidJSON(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("save_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveEvidenceFindingExistsError(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExistsErr: fmt.Errorf("db down")}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("save_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"kind": "poc_output",
		"filename": "test.txt",
		"content_base64": "dGVzdA=="
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on finding check failure")
	}
	if !strings.Contains(r.Content, "db down") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestListEvidenceHappy(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: true}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	saveTool, _ := reg.Lookup("save_evidence")
	content := base64.StdEncoding.EncodeToString([]byte("list test"))
	input := fmt.Sprintf(`{
		"finding_id": 1,
		"kind": "poc_output",
		"filename": "list.txt",
		"content_base64": %q
	}`, content)
	saveTool.Run(context.Background(), json.RawMessage(input))

	tool, ok := reg.Lookup("list_evidence")
	if !ok {
		t.Fatal("list_evidence not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"count":1`) {
		t.Errorf("expected count 1, got: %s", r.Content)
	}
}

func TestListEvidenceWithFindingID(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: true}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	saveTool, _ := reg.Lookup("save_evidence")
	content := base64.StdEncoding.EncodeToString([]byte("evidence data"))
	input := fmt.Sprintf(`{
		"finding_id": 5,
		"kind": "poc_output",
		"filename": "test.txt",
		"content_base64": %q
	}`, content)
	saveTool.Run(context.Background(), json.RawMessage(input))

	tool, _ := reg.Lookup("list_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"finding_id": 5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"count":1`) {
		t.Errorf("expected count 1, got: %s", r.Content)
	}
}

func TestListEvidenceEmpty(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("list_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"count":0`) {
		t.Errorf("expected count 0, got: %s", r.Content)
	}
}

func TestListEvidenceInvalidJSON(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("list_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetEvidenceHappy(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: true}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	saveTool, _ := reg.Lookup("save_evidence")
	content := base64.StdEncoding.EncodeToString([]byte("get test data"))
	input := fmt.Sprintf(`{
		"finding_id": 1,
		"kind": "poc_output",
		"filename": "get.txt",
		"content_base64": %q
	}`, content)
	saveRes, _ := saveTool.Run(context.Background(), json.RawMessage(input))

	var saved struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(saveRes.Content), &saved)

	tool, ok := reg.Lookup("get_evidence")
	if !ok {
		t.Fatal("get_evidence not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{"id": %q}`, saved.ID)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"content_base64"`) {
		t.Errorf("expected content_base64, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"kind":"poc_output"`) {
		t.Errorf("expected kind, got: %s", r.Content)
	}
}

func TestGetEvidenceMissingID(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("get_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"id": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty id")
	}
	if !strings.Contains(r.Content, "id is required") {
		t.Errorf("expected id required, got: %s", r.Content)
	}
}

func TestGetEvidenceNotFound(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("get_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"id": "nonexistent"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for nonexistent evidence")
	}
	if !strings.Contains(r.Content, "not found") {
		t.Errorf("expected not found, got: %s", r.Content)
	}
}

func TestGetEvidenceInvalidJSON(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("get_evidence")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCaptureCommandOutputMissingFindingID(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, ok := reg.Lookup("capture_command_output")
	if !ok {
		t.Fatal("capture_command_output not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"command": "echo hello"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing finding_id")
	}
	if !strings.Contains(r.Content, "finding_id is required") {
		t.Errorf("expected finding_id required, got: %s", r.Content)
	}
}

func TestCaptureCommandOutputMissingCommand(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("capture_command_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"command": ""
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty command")
	}
	if !strings.Contains(r.Content, "command is required") {
		t.Errorf("expected command required, got: %s", r.Content)
	}
}

func TestCaptureCommandOutputFindingNotFound(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: false}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("capture_command_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 999,
		"command": "echo hello"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for nonexistent finding")
	}
	if !strings.Contains(r.Content, "not found") {
		t.Errorf("expected not found error, got: %s", r.Content)
	}
}

func TestCaptureCommandOutputOutOfScope(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: true, inScope: false, scopeReason: "not authorized"}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("capture_command_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"command": "curl http://evil.example.com/admin"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for out-of-scope target")
	}
	if !strings.Contains(r.Content, "out-of-scope") {
		t.Errorf("expected out-of-scope error, got: %s", r.Content)
	}
}

func TestCaptureCommandOutputHappy(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: true, inScope: true}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("capture_command_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"command": "echo 'hello world'",
		"description": "test echo"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"evidence_id"`) {
		t.Errorf("expected evidence_id, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"exit_code":0`) {
		t.Errorf("expected exit_code 0, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "hello world") {
		t.Errorf("expected output preview, got: %s", r.Content)
	}
}

func TestCaptureCommandOutputNonZeroExit(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{findingExists: true, inScope: true}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("capture_command_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 1,
		"command": "exit 42"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success even for non-zero exit, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"exit_code":42`) {
		t.Errorf("expected exit_code 42, got: %s", r.Content)
	}
}

func TestCaptureCommandOutputInvalidJSON(t *testing.T) {
	t.Parallel()
	es := newEvidenceStore(t)
	ms := &evidenceMockStore{}
	reg := NewRegistry()
	RegisterEvidenceTools(reg, es, ms, 1)

	tool, _ := reg.Lookup("capture_command_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractTargets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{"ipv4", "curl http://10.0.0.1/admin", []string{"10.0.0.1"}},
		{"domain", "curl http://target.example.com/path", []string{"target.example.com"}},
		{"loopback_skipped", "curl http://127.0.0.1/test", nil},
		{"mixed", "nmap 10.0.0.1 target.example.com", []string{"10.0.0.1", "target.example.com"}},
		{"no_targets", "echo hello", nil},
		{"dedup", "curl 10.0.0.1 10.0.0.1", []string{"10.0.0.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTargets(tt.command)
			if len(got) != len(tt.want) {
				t.Errorf("extractTargets(%q) = %v, want %v", tt.command, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractTargets(%q)[%d] = %q, want %q", tt.command, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSanitizeForFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal", "test-file_name", "test-file_name"},
		{"spaces", "hello world", "hello_world"},
		{"special_chars", "foo/bar:baz", "foo_bar_baz"},
		{"long", strings.Repeat("a", 100), strings.Repeat("a", 50)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeForFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeForFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
