package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type reportMockStore struct {
	mockStore
	goal        string
	rootErr     error
	findings    []ReportFinding
	findingsErr error
	assets      []Asset
	assetsErr   error
}

func (m *reportMockStore) Root(_ context.Context, _ int64) (string, string, error) {
	if m.rootErr != nil {
		return "", "", m.rootErr
	}
	return "desc", m.goal, nil
}

func (m *reportMockStore) ListFindingsForReport(_ context.Context, _ int64) ([]ReportFinding, error) {
	return m.findings, m.findingsErr
}

func (m *reportMockStore) ListAssets(_ context.Context, _ AssetFilter) ([]Asset, int, error) {
	if m.assetsErr != nil {
		return nil, 0, m.assetsErr
	}
	return m.assets, len(m.assets), nil
}

func TestGenerateReportHappyPath(t *testing.T) {
	t.Parallel()
	ms := &reportMockStore{
		goal: "pentest the web app",
		findings: []ReportFinding{
			{ID: 1, VulnClass: "xss", Name: "Reflected XSS", Severity: "high", Summary: "XSS in search", Evidence: "alert(1)"},
			{ID: 2, VulnClass: "sqli", Name: "SQL Injection", Severity: "critical", Summary: "SQLi in login"},
		},
		assets: []Asset{
			{ID: 1, Type: "subdomain", Label: "app.example.com"},
			{ID: 2, Type: "ip", Label: "10.0.0.1"},
			{ID: 3, Type: "subdomain", Label: "api.example.com"},
		},
	}
	reg := NewRegistry()
	RegisterReportTools(reg, ms, 1, 1)

	tool, ok := reg.Lookup("generate_report")
	if !ok {
		t.Fatal("generate_report not found")
	}
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "Penetration Test Report") {
		t.Errorf("expected report header, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "pentest the web app") {
		t.Errorf("expected goal in report, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "Reflected XSS") {
		t.Errorf("expected finding name, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "SQL Injection") {
		t.Errorf("expected SQL Injection, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "alert(1)") {
		t.Errorf("expected evidence, got: %s", r.Content)
	}
}

func TestGenerateReportNoFindings(t *testing.T) {
	t.Parallel()
	ms := &reportMockStore{goal: "clean app"}
	reg := NewRegistry()
	RegisterReportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("generate_report")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "No vulnerabilities confirmed") {
		t.Errorf("expected no-findings message, got: %s", r.Content)
	}
}

func TestGenerateReportRootError(t *testing.T) {
	t.Parallel()
	ms := &reportMockStore{rootErr: fmt.Errorf("exploration not found")}
	reg := NewRegistry()
	RegisterReportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("generate_report")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error from Root failure")
	}
	if !strings.Contains(r.Content, "exploration not found") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestGenerateReportFindingsError(t *testing.T) {
	t.Parallel()
	ms := &reportMockStore{goal: "test", findingsErr: fmt.Errorf("findings broken")}
	reg := NewRegistry()
	RegisterReportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("generate_report")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error from ListFindingsForReport failure")
	}
	if !strings.Contains(r.Content, "findings broken") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestGenerateReportAssetsError(t *testing.T) {
	t.Parallel()
	ms := &reportMockStore{goal: "test", assetsErr: fmt.Errorf("assets broken")}
	reg := NewRegistry()
	RegisterReportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("generate_report")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error from ListAssets failure")
	}
	if !strings.Contains(r.Content, "assets broken") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestGenerateReportSeverityOrdering(t *testing.T) {
	t.Parallel()
	ms := &reportMockStore{
		goal: "test",
		findings: []ReportFinding{
			{ID: 1, VulnClass: "low-thing", Severity: "low", Summary: "minor"},
			{ID: 2, VulnClass: "crit-thing", Severity: "critical", Summary: "major"},
		},
	}
	reg := NewRegistry()
	RegisterReportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("generate_report")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	critIdx := strings.Index(r.Content, "CRITICAL")
	lowIdx := strings.Index(r.Content, "LOW")
	if critIdx < 0 || lowIdx < 0 {
		t.Fatalf("expected both CRITICAL and LOW in report, got: %s", r.Content)
	}
	if critIdx > lowIdx {
		t.Error("expected CRITICAL before LOW in report (severity ordering)")
	}
}

func TestGenerateReportAssetCounts(t *testing.T) {
	t.Parallel()
	ms := &reportMockStore{
		goal: "test",
		assets: []Asset{
			{ID: 1, Type: "subdomain"},
			{ID: 2, Type: "subdomain"},
			{ID: 3, Type: "ip"},
		},
	}
	reg := NewRegistry()
	RegisterReportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("generate_report")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "ip 1") {
		t.Errorf("expected 'ip 1', got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "subdomain 2") {
		t.Errorf("expected 'subdomain 2', got: %s", r.Content)
	}
}

func TestGenerateReportEmptyGoal(t *testing.T) {
	t.Parallel()
	ms := &reportMockStore{goal: ""}
	reg := NewRegistry()
	RegisterReportTools(reg, ms, 1, 1)

	tool, _ := reg.Lookup("generate_report")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success even with empty goal, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "unspecified") || !strings.Contains(r.Content, "Unnamed") {
		t.Errorf("expected fallback text for empty goal, got: %s", r.Content)
	}
}

type reportChangesMockStore struct {
	reportMockStore
	changes []TargetChange
}

func (m *reportChangesMockStore) ListTargetChanges(_ context.Context, _ int64, unrestoredOnly bool) ([]TargetChange, error) {
	return m.changes, nil
}

func TestGenerateReportListsTargetChanges(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	m := &reportChangesMockStore{
		reportMockStore: reportMockStore{goal: "g"},
		changes: []TargetChange{
			{ID: 1, Target: "WIN01", Resource: "a.ps1", Kind: "modify", Baseline: "sha256:a", RecordedBy: "h-1", RestoredAt: &at, RestoredBy: "h-1"},
			{ID: 2, Target: "WIN01", Resource: "b.ps1", Kind: "modify", Baseline: "sha256:b", RecordedBy: "h-2", LastObserved: "sha256:evil", RestoreAttempts: 1},
		},
	}
	r, err := makeGenerateReport(m, 1, 1).Run(context.Background(), json.RawMessage(`{}`))
	if err != nil || r.IsError {
		t.Fatalf("err=%v content=%s", err, r.Content)
	}
	for _, want := range []string{
		"Target changes: 2, unrestored: **1**",
		"## Target changes",
		"#1 restored by h-1",
		"#2 **NOT RESTORED**: `b.ps1` on `WIN01`",
		"baseline `sha256:b`, last observed `sha256:evil`",
	} {
		if !strings.Contains(r.Content, want) {
			t.Errorf("report missing %q:\n%s", want, r.Content)
		}
	}
}
