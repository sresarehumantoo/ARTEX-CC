package report

import (
	"strings"
	"testing"
	"time"
)

func TestMarkdownReport(t *testing.T) {
	t.Parallel()
	in := Input{
		Title:       "Acme Corp",
		Goal:        "Find vulnerabilities",
		GeneratedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		AssetCounts: map[string]int{"subdomain": 5, "ip": 3},
		Findings: []ReportFinding{
			{ID: 1, Name: "SQL Injection", Severity: "critical", Summary: "Found SQLi in login", Evidence: "sqlmap output"},
			{ID: 2, Name: "XSS", Severity: "high", Summary: "Reflected XSS", Evidence: "alert(1)"},
		},
	}
	out := Markdown(in)
	for _, want := range []string{
		"# Penetration Test Report",
		"Acme Corp",
		"Find vulnerabilities",
		"2026-01-15",
		"Confirmed findings: **2**",
		"SQL Injection",
		"[CRITICAL]",
		"XSS",
		"[HIGH]",
		"sqlmap output",
		"subdomain 5",
		"ip 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestEmptyReport(t *testing.T) {
	t.Parallel()
	in := Input{
		GeneratedAt: time.Now(),
	}
	out := Markdown(in)
	if !strings.Contains(out, "No vulnerabilities confirmed") {
		t.Error("empty report should say no vulnerabilities confirmed")
	}
	if !strings.Contains(out, "Confirmed findings: **0**") {
		t.Error("empty report should show 0 findings")
	}
}

func TestFindingsCSV(t *testing.T) {
	t.Parallel()
	fs := []*ExportFinding{
		{ID: 1, Name: "SQLi", Severity: "critical", Status: "confirmed", Summary: "injection found"},
		{ID: 2, Name: "XSS", Severity: "high", Status: "pending", Summary: "reflected"},
	}
	data := FindingsCSV(fs)
	out := string(data)
	if !strings.HasPrefix(out, "\xEF\xBB\xBF") {
		t.Error("CSV should start with UTF-8 BOM")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (header + 2 data); got %d", len(lines))
	}
	header := lines[0]
	for _, col := range []string{"ID", "Name", "Severity", "Status", "Summary"} {
		if !strings.Contains(header, col) {
			t.Errorf("header missing column %q", col)
		}
	}
	if !strings.Contains(out, "SQLi") {
		t.Error("CSV missing SQLi finding")
	}
	if !strings.Contains(out, "XSS") {
		t.Error("CSV missing XSS finding")
	}
}

func TestSingleFindingMarkdown(t *testing.T) {
	t.Parallel()
	f := &ExportFinding{
		ID:       42,
		Name:     "SSRF",
		Severity: "high",
		Summary:  "Server-side request forgery",
		Evidence: "curl output",
		Status:   "confirmed",
	}
	out := SingleFindingMarkdown(f, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	for _, want := range []string{
		"[HIGH] SSRF",
		"Server-side request forgery",
		"curl output",
		"confirmed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("single finding markdown missing %q", want)
		}
	}
}

func TestCSVInjectionSanitization(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"=cmd()", "'=cmd()"},
		{"+cmd()", "'+cmd()"},
		{"-cmd()", "'-cmd()"},
		{"@cmd()", "'@cmd()"},
		{"\tcmd()", "'\tcmd()"},
		{"\rcmd()", "'\rcmd()"},
		{"normal", "normal"},
		{"", ""},
	}
	for _, tc := range tests {
		got := csvCell(tc.input)
		if got != tc.want {
			t.Errorf("csvCell(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
