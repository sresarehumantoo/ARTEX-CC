package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type ingestMockStore struct {
	mockStore
	insertedID  int64
	insertErr   error
	addFindID   int64
	addFindErr  error
	insertCount int
}

func (m *ingestMockStore) InsertAsset(_ context.Context, _ int64, _ AssetInput) (int64, error) {
	if m.insertErr != nil {
		return 0, m.insertErr
	}
	m.insertCount++
	return m.insertedID, nil
}

func (m *ingestMockStore) AddFinding(_ context.Context, _ FindingInput) (int64, int64, error) {
	return m.addFindID, 0, m.addFindErr
}

func TestIngestScannerOutputMissingScanner(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("ingest_scanner_output")
	if !ok {
		t.Fatal("ingest_scanner_output not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"scanner": "",
		"output": "some data"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing scanner")
	}
	if !strings.Contains(r.Content, "scanner is required") {
		t.Errorf("expected scanner required, got: %s", r.Content)
	}
}

func TestIngestScannerOutputMissingOutput(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_scanner_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"scanner": "nmap",
		"output": ""
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing output")
	}
	if !strings.Contains(r.Content, "output is required") {
		t.Errorf("expected output required, got: %s", r.Content)
	}
}

func TestIngestScannerOutputUnsupportedScanner(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_scanner_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"scanner": "zmap",
		"output": "some data"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for unsupported scanner")
	}
	if !strings.Contains(r.Content, "unsupported scanner") {
		t.Errorf("expected unsupported scanner, got: %s", r.Content)
	}
}

func TestIngestScannerOutputInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_scanner_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIngestScannerOutputHttpxHappy(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{insertedID: 1}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_scanner_output")
	httpxLine := `{"url":"https://example.com","status_code":200,"title":"Example","host":"93.184.216.34","port":"443","scheme":"https","content_length":1256}`
	r, err := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{
		"scanner": "httpx",
		"output": %q
	}`, httpxLine)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"assets_created"`) {
		t.Errorf("expected assets_created in result, got: %s", r.Content)
	}
}

func TestIngestScannerOutputNucleiHappy(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{insertedID: 1, addFindID: 1}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_scanner_output")
	nucleiLine := `{"template-id":"cve-2021-44228","info":{"name":"Log4Shell","severity":"critical","description":"Remote code execution via Log4j"},"type":"http","host":"https://example.com","matched-at":"https://example.com/api","ip":"93.184.216.34"}`
	r, err := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{
		"scanner": "nuclei",
		"output": %q
	}`, nucleiLine)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"findings_created"`) {
		t.Errorf("expected findings_created in result, got: %s", r.Content)
	}
}

func TestIngestScannerOutputStoreError(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{insertErr: fmt.Errorf("insert failed")}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_scanner_output")
	httpxLine := `{"url":"https://example.com","status_code":200,"host":"93.184.216.34","port":"443","scheme":"https"}`
	r, err := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{
		"scanner": "httpx",
		"output": %q
	}`, httpxLine)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected partial success, got full error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "insert failed") {
		t.Errorf("expected store error in errors, got: %s", r.Content)
	}
}

func TestIngestScannerOutputCaseInsensitive(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_scanner_output")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"scanner": "NMAP",
		"output": "not valid nmap output but tests normalization"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatalf("expected parse error for invalid nmap data, got: %s", r.Content)
	}
	if strings.Contains(r.Content, "unsupported scanner") {
		t.Errorf("NMAP should be recognized as nmap, got: %s", r.Content)
	}
}

func TestIngestMasscanMissingFormat(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("ingest_masscan")
	if !ok {
		t.Fatal("ingest_masscan not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"format": "",
		"data": "some data"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing format")
	}
	if !strings.Contains(r.Content, "format is required") {
		t.Errorf("expected format required, got: %s", r.Content)
	}
}

func TestIngestMasscanMissingData(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_masscan")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"format": "xml",
		"data": ""
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing data")
	}
	if !strings.Contains(r.Content, "data is required") {
		t.Errorf("expected data required, got: %s", r.Content)
	}
}

func TestIngestMasscanUnsupportedFormat(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_masscan")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"format": "csv",
		"data": "some data"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(r.Content, "unsupported format") {
		t.Errorf("expected unsupported format, got: %s", r.Content)
	}
}

func TestIngestMasscanInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_masscan")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIngestMasscanJSONHappy(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{insertedID: 1}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_masscan")
	masscanJSON := "[\n{\"ip\":\"10.0.0.1\",\"ports\":[{\"port\":80,\"proto\":\"tcp\",\"status\":\"open\"}]}\n]"
	r, err := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{
		"format": "json",
		"data": %q
	}`, masscanJSON)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"assets_created"`) {
		t.Errorf("expected assets_created, got: %s", r.Content)
	}
}

func TestIngestMasscanXMLHappy(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{insertedID: 1}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_masscan")
	masscanXML := `<?xml version="1.0"?>
<nmaprun scanner="masscan">
<host><address addr="10.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="80"><state state="open"/></port></ports></host>
</nmaprun>`
	r, err := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{
		"format": "xml",
		"data": %q
	}`, masscanXML)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"assets_created"`) {
		t.Errorf("expected assets_created, got: %s", r.Content)
	}
}

func TestIngestMasscanStoreError(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{insertErr: fmt.Errorf("disk full")}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_masscan")
	masscanJSON := "[\n{\"ip\":\"10.0.0.1\",\"ports\":[{\"port\":80,\"proto\":\"tcp\",\"status\":\"open\"}]}\n]"
	r, err := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{
		"format": "json",
		"data": %q
	}`, masscanJSON)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected partial success, got full error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "disk full") {
		t.Errorf("expected store error in errors, got: %s", r.Content)
	}
}

func TestIngestScannerOutputStoreErrorRedactsUserinfo(t *testing.T) {
	t.Parallel()
	ms := &ingestMockStore{insertErr: fmt.Errorf("insert failed")}
	reg := NewRegistry()
	RegisterIngestTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("ingest_scanner_output")
	httpxLine := `{"url":"https://admin:s3cret@example.com/","status_code":200,"host":"93.184.216.34","port":"443","scheme":"https"}`
	r, err := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{
		"scanner": "httpx",
		"output": %q
	}`, httpxLine)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(r.Content, "asset https://example.com/") || strings.Contains(r.Content, "s3cret") {
		t.Errorf("expected redacted asset ref in errors, got: %s", r.Content)
	}
}
