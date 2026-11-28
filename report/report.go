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

	TargetChanges []TargetChange
	// UnownedThreshold flags changes that went longer than this without a
	// live owner; zero flags none.
	UnownedThreshold time.Duration
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
	b.WriteString("\n")
	unrestored, overdue := 0, 0
	for _, c := range in.TargetChanges {
		if !c.Restored {
			unrestored++
		}
		if pastThreshold(c, in.UnownedThreshold) {
			overdue++
		}
	}
	if len(in.TargetChanges) > 0 {
		fmt.Fprintf(&b, "- Target changes: %d, unrestored: **%d**", len(in.TargetChanges), unrestored)
		if overdue > 0 {
			fmt.Fprintf(&b, ", unowned past %s: **%d**", shortDuration(in.UnownedThreshold), overdue)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

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

	if len(in.TargetChanges) > 0 {
		writeTargetChanges(&b, in.TargetChanges, unrestored, overdue, in.UnownedThreshold)
	}

	return b.String()
}

func pastThreshold(c TargetChange, threshold time.Duration) bool {
	return threshold > 0 && c.LongestUnowned >= threshold
}

func writeTargetChanges(b *strings.Builder, changes []TargetChange, unrestored, overdue int, threshold time.Duration) {
	b.WriteString("## Target changes\n\n")
	if unrestored > 0 {
		fmt.Fprintf(b, "**%d change(s) were not restored to their recorded baseline. The affected hosts are not back in their original state.**\n\n", unrestored)
	} else {
		b.WriteString("Every recorded change was restored to its baseline fingerprint.\n\n")
	}
	if overdue > 0 {
		fmt.Fprintf(b, "**%d change(s) went %s or longer with no agent holding a live claim on their restoration.**\n\n", overdue, shortDuration(threshold))
	}
	for _, c := range changes {
		status := "**NOT RESTORED**"
		if c.Restored {
			status = fmt.Sprintf("restored by %s at %s", c.RestoredBy, c.RestoredAt.UTC().Format("2006-01-02 15:04:05"))
		}
		fmt.Fprintf(b, "- #%d %s: `%s` on `%s` (%s, recorded by %s), baseline `%s`", c.ID, status, c.Resource, c.Target, c.Kind, c.RecordedBy, c.Baseline)
		if !c.Restored && c.RestoreAttempts > 0 {
			fmt.Fprintf(b, ", last observed `%s` after %d restore attempt(s)", c.LastObserved, c.RestoreAttempts)
		}
		if !c.Restored && c.Unowned != "" {
			fmt.Fprintf(b, ", **no live owner** (%s)", strings.ReplaceAll(c.Unowned, "_", " "))
		}
		if c.LongestUnowned >= time.Second {
			fmt.Fprintf(b, ", unowned for up to %s", shortDuration(c.LongestUnowned))
			if pastThreshold(c, threshold) {
				b.WriteString(" (**past threshold**)")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// shortDuration renders d to the second without trailing zero units, so a
// 10 minute threshold reads 10m rather than 10m0s.
func shortDuration(d time.Duration) string {
	s := d.Round(time.Second).String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

func nz(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
