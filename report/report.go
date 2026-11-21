package report

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Input struct {
	Title       string
	Goal        string
	GeneratedAt time.Time
	AssetCounts map[string]int
	Findings    []ReportFinding
	Coverage    *CoverageLedger
	Quality     map[int64]FindingQuality
}

const reportMaxGaps = 200

type findingView struct {
	ReportFinding
	Q Quality
}

var sevRank = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "": 4}

func Markdown(in Input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Penetration Test Report — %s\n\n", nz(in.Title, "Unnamed task"))
	fmt.Fprintf(&b, "- **Task goal**: %s\n", nz(in.Goal, "(unspecified)"))
	fmt.Fprintf(&b, "- **Generated at**: %s\n\n", in.GeneratedAt.Format("2006-01-02 15:04:05"))

	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "- Confirmed findings: **%d**\n", len(in.Findings))
	fmt.Fprintf(&b, "- Assets: ")
	var types []string
	for t := range in.AssetCounts {
		types = append(types, t)
	}
	sort.Strings(types)
	for i, t := range types {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s %d", t, in.AssetCounts[t])
	}
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "## Findings\n\n")
	if len(in.Findings) == 0 {
		b.WriteString("_No vulnerabilities confirmed this time._\n\n")
	} else {
		fs := make([]findingView, 0, len(in.Findings))
		for _, f := range in.Findings {
			v := findingView{ReportFinding: f}
			if in.Quality != nil {
				v.Q = qualityFromRows(f.ID, in.Quality)
			}
			fs = append(fs, v)
		}
		sort.SliceStable(fs, func(i, j int) bool { return sevRank[fs[i].Severity] < sevRank[fs[j].Severity] })
		for i, f := range fs {
			fmt.Fprintf(&b, "### %d. [%s] %s\n\n", i+1, strings.ToUpper(nz(f.Severity, "info")), nz(f.Name, nz(f.VulnClass, "Uncategorized")))
			if f.Summary != "" {
				fmt.Fprintf(&b, "%s\n\n", f.Summary)
			}
			if in.Quality != nil {
				b.WriteString(qualityBullets(f.Q, true))
			}
			if f.Evidence != "" {
				fmt.Fprintf(&b, "**PoC / Evidence:**\n\n```\n%s\n```\n\n", f.Evidence)
			}
			if in.Quality != nil {
				b.WriteString(qualityNarrative(f.Q))
				b.WriteString(staleReportNotice(f.Q.ReportStale()))
				b.WriteString(detailedReport(f.Q.Report, "**Detailed report:**"))
			}
		}
	}

	if c := in.Coverage; c != nil {
		fmt.Fprintf(&b, "## Coverage gaps\n\n")
		if c.Completeness != nil {
			fmt.Fprintf(&b, "- **Completeness**: %s. %s\n", c.Completeness.State, c.Completeness.Caveat)
		}
		fmt.Fprintf(&b, "- Ledger entries: %d, open gaps: %d\n", len(c.Entries), len(c.Gaps))
		for i, g := range c.Gaps {
			if i == reportMaxGaps {
				fmt.Fprintf(&b, "- and %d more (see the coverage API)\n", len(c.Gaps)-reportMaxGaps)
				break
			}
			fmt.Fprintf(&b, "- %s\n", g)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func nz(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
