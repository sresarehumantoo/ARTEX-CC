package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type findingMockStore struct {
	mockStore
	addFindingID     int64
	addFindingErr    error
	addFindingNodeID int64
	findings         []Finding
	findingsErr      error
	trafficErr       error
	trafficList      []TrafficBinding
	bindErr          error
	mergedFinding    *MergedFinding
	mergeErr         error
	dismissErr       error
	dupes            []Finding
	trafficCount     int
	trafficCountFn   func(int64) (int, error)
	boundSnapshot    string
}

func (m *findingMockStore) AddFinding(_ context.Context, _ FindingInput) (int64, int64, error) {
	return m.addFindingID, m.addFindingNodeID, m.addFindingErr
}

func (m *findingMockStore) ListFindings(_ context.Context, _ int64) ([]Finding, error) {
	return m.findings, m.findingsErr
}

func (m *findingMockStore) ListFindingTraffic(_ context.Context, _ int64) ([]TrafficBinding, error) {
	return m.trafficList, m.trafficErr
}

func (m *findingMockStore) BindFindingTraffic(_ context.Context, _ int64, snapshotID, _, _ string, _ int) error {
	m.boundSnapshot = snapshotID
	return m.bindErr
}

func (m *findingMockStore) FindingTrafficCount(_ context.Context, findingID int64) (int, error) {
	if m.trafficCountFn != nil {
		return m.trafficCountFn(findingID)
	}
	return m.trafficCount, nil
}

func (m *findingMockStore) MergeFindings(_ context.Context, _ int64, _ []int64) (*MergedFinding, error) {
	return m.mergedFinding, m.mergeErr
}

func (m *findingMockStore) DismissFinding(_ context.Context, _ int64, _ string) error {
	return m.dismissErr
}

func (m *findingMockStore) FindPotentialDuplicates(_ context.Context, _ int64, _, _ string, _ []int64) ([]Finding, error) {
	return m.dupes, nil
}

func TestReportFindingHappyPath(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{addFindingID: 7, addFindingNodeID: 40}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("report_finding")
	if !ok {
		t.Fatal("report_finding not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"vulnclass": "xss",
		"severity": "high",
		"summary": "Reflected XSS in search param",
		"evidence": "curl output shows injection"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"finding_id":7`) {
		t.Errorf("expected finding_id 7, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"node_id":40`) {
		t.Errorf("expected node_id 40, got: %s", r.Content)
	}
}

func TestReportFindingWithDuplicates(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{addFindingID: 8, dupes: []Finding{{ID: 3, VulnClass: "SQLi", Severity: "high", Summary: "SQLi in login"}}}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("report_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"vulnclass": "sqli",
		"severity": "critical",
		"summary": "SQL injection in login",
		"asset_ids": [1, 2]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"possible_duplicates":[{"id":3,"vulnclass":"SQLi","severity":"high","summary":"SQLi in login"}]`) {
		t.Errorf("expected possible_duplicates, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "merge_findings") {
		t.Errorf("expected a merge hint, got: %s", r.Content)
	}
}

func TestNormalizeVulnClass(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"IDOR":                               "idor",
		" idor ":                             "idor",
		"Insecure Direct Object Reference":   "idor",
		"insecure-direct-object-references":  "idor",
		"SQL Injection":                      "sqli",
		"sql_injection":                      "sqli",
		"Cross-Site Scripting (XSS)":         "xss",
		"Server-Side Request Forgery (SSRF)": "ssrf",
		"Stored XSS":                         "storedxss",
		"information-disclosure":             "informationdisclosure",
	}
	for in, want := range cases {
		if got := NormalizeVulnClass(in); got != want {
			t.Errorf("NormalizeVulnClass(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReportFindingMissingVulnclass(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("report_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"severity": "high",
		"summary": "test"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing vulnclass")
	}
	if !strings.Contains(r.Content, "vulnclass is required") {
		t.Errorf("expected vulnclass required, got: %s", r.Content)
	}
}

func TestReportFindingMissingSummary(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("report_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"vulnclass": "xss",
		"severity": "low"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing summary")
	}
	if !strings.Contains(r.Content, "summary is required") {
		t.Errorf("expected summary required, got: %s", r.Content)
	}
}

func TestReportFindingInvalidSeverity(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("report_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"vulnclass": "xss",
		"severity": "mega",
		"summary": "test"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid severity")
	}
	if !strings.Contains(r.Content, "not valid") {
		t.Errorf("expected severity validation error, got: %s", r.Content)
	}
}

func TestReportFindingSeverityNormalization(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{addFindingID: 1}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("report_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"vulnclass": "xss",
		"severity": "HIGH",
		"summary": "test finding"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success for uppercase severity, got error: %s", r.Content)
	}
}

func TestReportFindingStoreError(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{addFindingErr: fmt.Errorf("unique constraint")}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("report_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"vulnclass": "xss",
		"severity": "high",
		"summary": "test"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "unique constraint") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestReportFindingInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("report_finding")
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

func TestListFindingsHappyPath(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{
		findings: []Finding{
			{ID: 1, VulnClass: "xss", Severity: "high", Summary: "XSS in search"},
			{ID: 2, VulnClass: "sqli", Severity: "critical", Summary: "SQLi in login", Dismissed: true, DismissReason: "false positive"},
		},
	}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("list_findings")
	if !ok {
		t.Fatal("list_findings not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"xss"`) {
		t.Errorf("expected xss vulnclass, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"dismissed":true`) {
		t.Errorf("expected dismissed flag, got: %s", r.Content)
	}
}

func TestListFindingsEmpty(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{findings: []Finding{}}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("list_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `[]`) {
		t.Errorf("expected empty array, got: %s", r.Content)
	}
}

func TestListFindingsStoreError(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{findingsErr: fmt.Errorf("connection reset")}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("list_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "connection reset") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestListFindingsWithMergedInto(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{
		findings: []Finding{
			{ID: 5, Summary: "merged one", MergedInto: 3},
		},
	}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("list_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"merged_into":3`) {
		t.Errorf("expected merged_into, got: %s", r.Content)
	}
}

func TestListFindingsWithTrafficCount(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{
		findings:     []Finding{{ID: 10, Summary: "has traffic"}},
		trafficCount: 3,
	}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("list_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"traffic_count":3`) {
		t.Errorf("expected traffic_count, got: %s", r.Content)
	}
}

func snapshotFound(string) (bool, error) { return true, nil }

func TestBindFindingTrafficUnknownSnapshot(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	tool := makeBindFindingTraffic(ms, func(string) (bool, error) { return false, nil })

	r, err := tool.Run(context.Background(), json.RawMessage(`{"finding_id": 7, "snapshot_id": "does-not-exist"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError || !strings.Contains(r.Content, "not found") {
		t.Fatalf("expected not-found error, got: %s", r.Content)
	}
	if ms.boundSnapshot != "" {
		t.Errorf("store.BindFindingTraffic called with %q for unknown snapshot", ms.boundSnapshot)
	}
}

func TestBindFindingTrafficHappyPath(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{trafficList: []TrafficBinding{{ID: 1}}}
	tool := makeBindFindingTraffic(ms, snapshotFound)

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 7,
		"snapshot_id": "snap-abc123",
		"role": "proof",
		"note": "shows the XSS payload"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "snap-abc123") {
		t.Errorf("expected snapshot_id in result, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "proof") {
		t.Errorf("expected role in result, got: %s", r.Content)
	}
}

func TestBindFindingTrafficDefaultRole(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	tool := makeBindFindingTraffic(ms, snapshotFound)
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 7,
		"snapshot_id": "snap-def456"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "supporting") {
		t.Errorf("expected default role 'supporting', got: %s", r.Content)
	}
}

func TestBindFindingTrafficMissingFindingID(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	tool := makeBindFindingTraffic(ms, snapshotFound)
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"snapshot_id": "snap-123"
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

func TestBindFindingTrafficMissingSnapshotID(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	tool := makeBindFindingTraffic(ms, snapshotFound)
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 7,
		"snapshot_id": ""
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty snapshot_id")
	}
	if !strings.Contains(r.Content, "snapshot_id is required") {
		t.Errorf("expected snapshot_id required, got: %s", r.Content)
	}
}

func TestBindFindingTrafficInvalidRole(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	tool := makeBindFindingTraffic(ms, snapshotFound)
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 7,
		"snapshot_id": "snap-123",
		"role": "invalid_role"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(r.Content, "not valid") {
		t.Errorf("expected role validation error, got: %s", r.Content)
	}
}

func TestBindFindingTrafficStoreError(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{bindErr: fmt.Errorf("foreign key")}
	tool := makeBindFindingTraffic(ms, snapshotFound)
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 7,
		"snapshot_id": "snap-123",
		"role": "proof"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "foreign key") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestBindFindingTrafficInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	tool := makeBindFindingTraffic(ms, snapshotFound)
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMergeFindingsHappyPath(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{
		mergedFinding: &MergedFinding{
			ID:        1,
			VulnClass: "xss",
			Severity:  "high",
			Summary:   "merged XSS",
			Status:    "confirmed",
		},
	}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("merge_findings")
	if !ok {
		t.Fatal("merge_findings not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_ids": [1, 2, 3],
		"primary_id": 1
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"vulnclass":"xss"`) {
		t.Errorf("expected merged vulnclass, got: %s", r.Content)
	}
}

func TestMergeFindingsReportsResolvedPrimaryAndSkipped(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{
		mergedFinding: &MergedFinding{
			ID:                 7,
			Status:             "confirmed",
			RequestedPrimaryID: 1,
			Skipped:            []SkippedFinding{{ID: 2, Reason: "already merged into finding 7"}},
		},
	}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)
	tool, _ := reg.Lookup("merge_findings")

	r, err := tool.Run(context.Background(), json.RawMessage(`{"finding_ids": [1, 2], "primary_id": 1}`))
	if err != nil || r.IsError {
		t.Fatalf("merge_findings: %v %s", err, r.Content)
	}
	for _, want := range []string{`"id":7`, `"requested_primary_id":1`, `"skipped":[{"id":2,"reason":"already merged into finding 7"}]`} {
		if !strings.Contains(r.Content, want) {
			t.Errorf("result missing %s: %s", want, r.Content)
		}
	}
}

func TestMergeFindingsMissingPrimaryID(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("merge_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_ids": [1, 2]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing primary_id")
	}
	if !strings.Contains(r.Content, "primary_id is required") {
		t.Errorf("expected primary_id required, got: %s", r.Content)
	}
}

func TestMergeFindingsTooFewIDs(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("merge_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_ids": [1],
		"primary_id": 1
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for too few finding IDs")
	}
	if !strings.Contains(r.Content, "at least two") {
		t.Errorf("expected at least two error, got: %s", r.Content)
	}
}

func TestMergeFindingsPrimaryNotInList(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("merge_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_ids": [2, 3],
		"primary_id": 1
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when primary_id not in finding_ids")
	}
	if !strings.Contains(r.Content, "must include primary_id") {
		t.Errorf("expected must include primary_id, got: %s", r.Content)
	}
}

func TestMergeFindingsStoreError(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{mergeErr: fmt.Errorf("merge conflict")}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("merge_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_ids": [1, 2],
		"primary_id": 1
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "merge conflict") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestMergeFindingsInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("merge_findings")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDismissFindingHappyPath(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("dismiss_finding")
	if !ok {
		t.Fatal("dismiss_finding not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 5,
		"reason": "false positive: test data"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "finding 5 dismissed") {
		t.Errorf("expected dismissal confirmation, got: %s", r.Content)
	}
}

func TestDismissFindingMissingFindingID(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("dismiss_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"reason": "not real"
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

func TestDismissFindingMissingReason(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("dismiss_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 5,
		"reason": ""
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty reason")
	}
	if !strings.Contains(r.Content, "reason is required") {
		t.Errorf("expected reason required, got: %s", r.Content)
	}
}

func TestDismissFindingStoreError(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{dismissErr: fmt.Errorf("not found")}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("dismiss_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"finding_id": 999,
		"reason": "gone"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "not found") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestDismissFindingInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &findingMockStore{}
	reg := NewRegistry()
	RegisterFindingTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("dismiss_finding")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}
