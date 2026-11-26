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

func TestMarkdownWithCoverage(t *testing.T) {
	t.Parallel()
	in := Input{
		Title:       "Test",
		GeneratedAt: time.Now(),
		Coverage: &CoverageLedger{
			Entries: []CoverageEntry{{Surface: "/a", RiskArea: "xss", Outcome: "reported"}},
			Gaps:    []CoverageGap{{Kind: "needs_follow_up", Surface: "/a", RiskArea: "xss"}},
			Completeness: &CoverageCompleteness{
				State:  "partial",
				Caveat: "not all tested",
			},
		},
	}
	out := Markdown(in)
	for _, want := range []string{"Coverage gaps", "Completeness", "partial", "not all tested", "Ledger entries: 1", "open gaps: 1", "needs_follow_up /a xss"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in coverage output", want)
		}
	}
}

func TestMarkdownCoverageWithoutCompleteness(t *testing.T) {
	t.Parallel()
	in := Input{
		GeneratedAt: time.Now(),
		Coverage: &CoverageLedger{
			Entries: []CoverageEntry{},
			Gaps:    []CoverageGap{{Kind: "unrecorded", RiskArea: "sqli"}},
		},
	}
	out := Markdown(in)
	if strings.Contains(out, "Completeness") {
		t.Error("should not contain completeness when nil")
	}
	if !strings.Contains(out, "unrecorded sqli") {
		t.Error("should render gap without surface")
	}
}

func TestMarkdownWithQuality(t *testing.T) {
	t.Parallel()
	in := Input{
		Title:       "QA Test",
		GeneratedAt: time.Now(),
		Findings:    []ReportFinding{{ID: 1, Name: "SQLi", Severity: "critical", Summary: "injection"}},
		Quality: map[int64]FindingQuality{
			1: {ID: 10, PoeLevel: 3, VerificationStatus: "verified", TrafficCount: 2, ConfidenceRationale: "confirmed via replay", Report: "full analysis"},
		},
	}
	out := Markdown(in)
	for _, want := range []string{"PoE level", "Verified", "2 bound traffic", "Confidence rationale", "confirmed via replay", "full analysis"} {
		if !strings.Contains(out, want) {
			t.Errorf("quality output missing %q", want)
		}
	}
}

func TestMarkdownQualityMissing(t *testing.T) {
	t.Parallel()
	in := Input{
		GeneratedAt: time.Now(),
		Findings:    []ReportFinding{{ID: 99, Name: "Missing", Severity: "low"}},
		Quality:     map[int64]FindingQuality{},
	}
	out := Markdown(in)
	if !strings.Contains(out, "not assessed") {
		t.Error("missing quality row should show 'not assessed'")
	}
}

func TestMarkdownQualityUnavailable(t *testing.T) {
	t.Parallel()
	q := qualityFromRows(1, nil)
	out := qualityBullets(q, false)
	if !strings.Contains(out, "quality lookup failed") {
		t.Errorf("unavailable quality should mention lookup failed, got: %s", out)
	}
}

func TestQualityStaleReport(t *testing.T) {
	t.Parallel()
	in := Input{
		GeneratedAt: time.Now(),
		Findings:    []ReportFinding{{ID: 1, Name: "Stale", Severity: "high"}},
		Quality: map[int64]FindingQuality{
			1: {ID: 5, Report: "old report", EvidenceVersion: 3, ReportEvidenceVersion: 1},
		},
	}
	out := Markdown(in)
	if !strings.Contains(out, "evidence has changed") {
		t.Error("stale report notice missing")
	}
}

func TestVerificationLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", "Unverified"},
		{VerificationUnverified, "Unverified"},
		{VerificationVerified, "Verified"},
		{VerificationBlocked, "Blocked"},
		{VerificationNotAssessed, "Not assessed"},
		{VerificationFalsePositive, "False positive"},
		{"custom_state", "custom_state"},
	}
	for _, c := range cases {
		if got := verificationLabel(c.in); got != c.want {
			t.Errorf("verificationLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSortExportFindings(t *testing.T) {
	t.Parallel()
	now := time.Now()
	fs := []*ExportFinding{
		{ID: 1, Severity: "low", CreatedAt: now.Add(-time.Hour)},
		{ID: 2, Severity: "critical", CreatedAt: now},
		{ID: 3, Severity: "low", CreatedAt: now},
		{ID: 4, Severity: "high", CreatedAt: now},
	}
	sortExportFindings(fs)
	if fs[0].ID != 2 || fs[1].ID != 4 || fs[2].ID != 3 || fs[3].ID != 1 {
		t.Errorf("sort order wrong: %d,%d,%d,%d", fs[0].ID, fs[1].ID, fs[2].ID, fs[3].ID)
	}
}

func TestFindingsMarkdownEmpty(t *testing.T) {
	t.Parallel()
	out := FindingsMarkdown(nil, time.Now())
	if !strings.Contains(out, "No matching vulnerabilities") {
		t.Error("empty findings should show no matching")
	}
}

func TestFindingsMarkdownWithDetails(t *testing.T) {
	t.Parallel()
	now := time.Now()
	fs := []*ExportFinding{
		{ID: 1, Name: "SQLi", Severity: "critical", Summary: "injection", Evidence: "proof", Status: "confirmed", VulnClass: "injection", TaskDescription: "pentest acme", CreatedAt: now, Report: "detailed analysis"},
	}
	out := FindingsMarkdown(fs, now)
	for _, want := range []string{"[CRITICAL]", "SQLi", "injection", "proof", "confirmed", "Class", "Task", "pentest acme", "detailed analysis"} {
		if !strings.Contains(out, want) {
			t.Errorf("findings markdown missing %q", want)
		}
	}
}

func TestFindingFilename(t *testing.T) {
	t.Parallel()
	cases := []struct {
		f    *ExportFinding
		want string
	}{
		{&ExportFinding{ID: 1, Severity: "high", Name: "SQL Injection"}, "high_SQL_Injection__1.md"},
		{&ExportFinding{ID: 2, Severity: "", Name: ""}, "info_Uncategorized__2.md"},
		{&ExportFinding{ID: 3, Severity: "low", Name: "", VulnClass: "xss"}, "low_xss__3.md"},
	}
	for _, c := range cases {
		got := FindingFilename(c.f)
		if got != c.want {
			t.Errorf("FindingFilename(%+v) = %q, want %q", c.f, got, c.want)
		}
	}
}

func TestDetailedReportTruncation(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 10000)
	out := detailedReport(long, "**Report:**")
	if !strings.Contains(out, "truncated") {
		t.Error("long report should be truncated")
	}
	short := "short report"
	out = detailedReport(short, "**Report:**")
	if strings.Contains(out, "truncated") {
		t.Error("short report should not be truncated")
	}
	if !strings.Contains(out, short) {
		t.Error("short report content missing")
	}
}

func TestDetailedReportEmpty(t *testing.T) {
	t.Parallel()
	if out := detailedReport("", "heading"); out != "" {
		t.Errorf("empty body should return empty, got %q", out)
	}
	if out := detailedReport("   ", "heading"); out != "" {
		t.Errorf("whitespace body should return empty, got %q", out)
	}
}

func TestUnbalancedFence(t *testing.T) {
	t.Parallel()
	if !unbalancedFence("```\ncode\n") {
		t.Error("single fence should be unbalanced")
	}
	if unbalancedFence("```\ncode\n```") {
		t.Error("paired fences should be balanced")
	}
	if unbalancedFence("no fences") {
		t.Error("no fences should be balanced")
	}
}

func TestDetailedReportTruncationClosesFence(t *testing.T) {
	t.Parallel()
	body := "```\n" + strings.Repeat("x", 10000)
	out := detailedReport(body, "**R:**")
	if !strings.Contains(out, "truncated") {
		t.Fatal("should be truncated")
	}
	fences := strings.Count(out, "```")
	if fences%2 != 0 {
		t.Error("truncated report with open fence should close it")
	}
}

func TestQualityBulletsNoTraffic(t *testing.T) {
	t.Parallel()
	q := Quality{PoE: 2, Verification: "verified", TrafficCount: 0, RecordID: 5, VerificationNote: "checked manually"}
	out := qualityBullets(q, false)
	if !strings.Contains(out, "no bound traffic") {
		t.Error("zero traffic should show 'no bound traffic'")
	}
	if !strings.Contains(out, "#5") {
		t.Error("should show record ID")
	}
	if !strings.Contains(out, "checked manually") {
		t.Error("should show verification note")
	}
}

func TestCoverageGapString(t *testing.T) {
	t.Parallel()
	g1 := CoverageGap{Kind: "missing", Surface: "/login", RiskArea: "sqli"}
	if s := g1.String(); s != "missing /login sqli" {
		t.Errorf("got %q", s)
	}
	g2 := CoverageGap{Kind: "unrecorded", RiskArea: "xss"}
	if s := g2.String(); s != "unrecorded xss" {
		t.Errorf("got %q", s)
	}
}

func TestFindingQualityReportStale(t *testing.T) {
	t.Parallel()
	q := FindingQuality{Report: "r", EvidenceVersion: 2, ReportEvidenceVersion: 1}
	if !q.ReportStale() {
		t.Error("should be stale")
	}
	q2 := FindingQuality{Report: "r", EvidenceVersion: 1, ReportEvidenceVersion: 1}
	if q2.ReportStale() {
		t.Error("should not be stale")
	}
	q3 := FindingQuality{Report: "", EvidenceVersion: 2, ReportEvidenceVersion: 1}
	if q3.ReportStale() {
		t.Error("empty report should not be stale")
	}
}

func TestNz(t *testing.T) {
	t.Parallel()
	if nz("", "default") != "default" {
		t.Error("empty should return default")
	}
	if nz("  ", "default") != "default" {
		t.Error("whitespace should return default")
	}
	if nz("value", "default") != "value" {
		t.Error("non-empty should return value")
	}
}

func TestOneLine(t *testing.T) {
	t.Parallel()
	if got := oneLine("  hello\n  world  "); got != "hello world" {
		t.Errorf("oneLine = %q", got)
	}
}

func TestMarkdownTargetChanges(t *testing.T) {
	t.Parallel()
	in := Input{Title: "T", GeneratedAt: time.Now(), TargetChanges: []TargetChange{
		{ID: 4, Target: "WIN01", Resource: `C:\ProgramData\x.ps1`, Kind: "modify", Baseline: "sha256:a", RecordedBy: "h-1"},
	}}
	out := Markdown(in)
	for _, want := range []string{
		"- Target changes: 1, unrestored: **1**",
		"**1 change(s) were not restored",
		"- #4 **NOT RESTORED**: `C:\\ProgramData\\x.ps1` on `WIN01` (modify, recorded by h-1), baseline `sha256:a`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q:\n%s", want, out)
		}
	}

	in.TargetChanges[0].Restored = true
	in.TargetChanges[0].RestoredBy = "h-2"
	out = Markdown(in)
	if strings.Contains(out, "NOT RESTORED") || !strings.Contains(out, "Every recorded change was restored") || !strings.Contains(out, "restored by h-2") {
		t.Errorf("restored report:\n%s", out)
	}

	if out := Markdown(Input{GeneratedAt: time.Now()}); strings.Contains(out, "Target changes") {
		t.Errorf("report without changes has a Target changes section:\n%s", out)
	}
}
