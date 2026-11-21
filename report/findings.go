package report

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

func sortExportFindings(fs []*ExportFinding) {
	sort.SliceStable(fs, func(i, j int) bool {
		ri, rj := sevRank[fs[i].Severity], sevRank[fs[j].Severity]
		if ri != rj {
			return ri < rj
		}
		return fs[i].CreatedAt.After(fs[j].CreatedAt)
	})
}

func exportFindingTitle(f *ExportFinding) string {
	return nz(f.Name, nz(f.VulnClass, "Uncategorized"))
}

func FindingsMarkdown(fs []*ExportFinding, generatedAt time.Time) string {
	items := append([]*ExportFinding(nil), fs...)
	sortExportFindings(items)

	var b strings.Builder
	b.WriteString("# Vulnerability Findings Summary Report\n\n")
	fmt.Fprintf(&b, "- **Generated at**: %s\n", generatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "- **Total findings**: %d\n\n", len(items))

	counts := map[string]int{}
	for _, f := range items {
		counts[f.Severity]++
	}
	b.WriteString("## Summary\n\n")
	b.WriteString("| Severity | Count |\n| --- | --- |\n")
	for _, s := range []struct{ key, label string }{
		{"critical", "Critical"}, {"high", "High"}, {"medium", "Medium"}, {"low", "Low"},
	} {
		fmt.Fprintf(&b, "| %s | %d |\n", s.label, counts[s.key])
	}
	b.WriteString("\n")

	if len(items) == 0 {
		b.WriteString("_No matching vulnerabilities._\n")
		return b.String()
	}

	b.WriteString("## Vulnerability Details\n\n")
	for i, f := range items {
		fmt.Fprintf(&b, "### %d. [%s] %s\n\n", i+1, strings.ToUpper(nz(f.Severity, "info")), exportFindingTitle(f))
		if f.VulnClass != "" {
			fmt.Fprintf(&b, "- **Class**: %s\n", f.VulnClass)
		}
		fmt.Fprintf(&b, "- **Status**: %s\n", nz(f.Status, "pending"))
		fmt.Fprintf(&b, "- **Report stage**: %s\n", nz(f.ReportState, "none"))
		if desc := strings.TrimSpace(f.TaskDescription); desc != "" {
			fmt.Fprintf(&b, "- **Task**: %s\n", desc)
		}
		fmt.Fprintf(&b, "- **Found at**: %s\n\n", f.CreatedAt.Format("2006-01-02 15:04:05"))
		if s := strings.TrimSpace(f.Summary); s != "" {
			fmt.Fprintf(&b, "%s\n\n", s)
		}
		if e := strings.TrimSpace(f.Evidence); e != "" {
			fmt.Fprintf(&b, "**Evidence:**\n\n```\n%s\n```\n\n", e)
		}
		b.WriteString(detailedReportFull(f.Report, "**Detailed report:**"))
		b.WriteString("---\n\n")
	}
	return b.String()
}

func SingleFindingMarkdown(f *ExportFinding, generatedAt time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# [%s] %s\n\n", strings.ToUpper(nz(f.Severity, "info")), exportFindingTitle(f))
	if f.VulnClass != "" {
		fmt.Fprintf(&b, "- **Class**: %s\n", f.VulnClass)
	}
	fmt.Fprintf(&b, "- **Severity**: %s\n", nz(f.Severity, "info"))
	fmt.Fprintf(&b, "- **Status**: %s\n", nz(f.Status, "pending"))
	fmt.Fprintf(&b, "- **Report stage**: %s\n", nz(f.ReportState, "none"))
	if desc := strings.TrimSpace(f.TaskDescription); desc != "" {
		fmt.Fprintf(&b, "- **Task**: %s\n", desc)
	}
	fmt.Fprintf(&b, "- **Found at**: %s\n", f.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "- **Generated at**: %s\n\n", generatedAt.Format("2006-01-02 15:04:05"))
	if s := strings.TrimSpace(f.Summary); s != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", s)
	}
	if e := strings.TrimSpace(f.Evidence); e != "" {
		fmt.Fprintf(&b, "## Evidence\n\n```\n%s\n```\n\n", e)
	}
	b.WriteString(detailedReportFull(f.Report, "## Detailed Report"))
	return b.String()
}

var unsafeFilenameChars = regexp.MustCompile(`[^\p{Han}\p{L}\p{N}._-]+`)

func FindingFilename(f *ExportFinding) string {
	sev := nz(f.Severity, "info")
	title := exportFindingTitle(f)
	name := fmt.Sprintf("%s_%s_#%d", sev, title, f.ID)
	name = unsafeFilenameChars.ReplaceAllString(name, "_")
	name = strings.Trim(name, "._")
	if name == "" {
		name = fmt.Sprintf("finding_%d", f.ID)
	}
	name = path.Base(name)
	if len(name) > 120 {
		name = strings.ToValidUTF8(name[:120], "")
	}
	return name + ".md"
}

func FindingsCSV(fs []*ExportFinding) []byte {
	items := append([]*ExportFinding(nil), fs...)
	sortExportFindings(items)

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"ID", "Name", "Class", "Severity", "Status", "Report Stage", "Task", "Found At", "Summary"})
	for _, f := range items {
		_ = w.Write([]string{
			fmt.Sprintf("%d", f.ID),
			csvCell(exportFindingTitle(f)),
			csvCell(f.VulnClass),
			csvCell(nz(f.Severity, "info")),
			csvCell(nz(f.Status, "pending")),
			csvCell(nz(f.ReportState, "none")),
			csvCell(f.TaskDescription),
			csvCell(f.CreatedAt.Format("2006-01-02 15:04:05")),
			csvCell(strings.TrimSpace(f.Summary)),
		})
	}
	w.Flush()
	return buf.Bytes()
}

func csvCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}
