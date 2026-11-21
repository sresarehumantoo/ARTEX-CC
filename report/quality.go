package report

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Quality struct {
	PoE                   int
	Verification          string
	VerificationNote      string
	ConfidenceRationale   string
	Counterevidence       string
	TrafficCount          int
	RecordID              int64
	Report                string
	EvidenceVersion       int64
	ReportEvidenceVersion int64
	Missing               bool
	Unavailable           bool
}

func (q Quality) ReportStale() bool {
	return reportStale(q.Report, q.EvidenceVersion, q.ReportEvidenceVersion)
}

func reportStale(report string, evidenceVersion, reportEvidenceVersion int64) bool {
	return report != "" && evidenceVersion != reportEvidenceVersion
}

func qualityFromRows(nodeID int64, rows map[int64]FindingQuality) Quality {
	if rows == nil {
		return Quality{Unavailable: true}
	}
	q, ok := rows[nodeID]
	if !ok {
		return Quality{Missing: true}
	}
	return Quality{
		PoE:                   q.PoeLevel,
		Verification:          q.VerificationStatus,
		VerificationNote:      q.VerificationNote,
		ConfidenceRationale:   q.ConfidenceRationale,
		Counterevidence:       q.Counterevidence,
		TrafficCount:          q.TrafficCount,
		RecordID:              q.ID,
		Report:                q.Report,
		EvidenceVersion:       q.EvidenceVersion,
		ReportEvidenceVersion: q.ReportEvidenceVersion,
	}
}

func qualityBullets(q Quality, alwaysProof bool) string {
	var b strings.Builder
	if q.Unavailable {
		b.WriteString("- **Verification**: unknown (quality lookup failed)\n\n")
		return b.String()
	}
	if q.Missing {
		b.WriteString("- **Verification**: not assessed\n\n")
		return b.String()
	}
	if q.RecordID > 0 {
		fmt.Fprintf(&b, "- **Finding record**: #%d\n", q.RecordID)
	}
	fmt.Fprintf(&b, "- **PoE level**: %d\n", q.PoE)
	fmt.Fprintf(&b, "- **Verification**: %s\n", verificationLabel(q.Verification))
	if alwaysProof || q.TrafficCount == 0 {
		if q.TrafficCount > 0 {
			fmt.Fprintf(&b, "- **Proof**: %d bound traffic exchange(s)\n", q.TrafficCount)
		} else {
			b.WriteString("- **Proof**: no bound traffic evidence\n")
		}
	}
	if note := oneLine(q.VerificationNote); note != "" {
		fmt.Fprintf(&b, "- **Verification note**: %s\n", note)
	}
	b.WriteString("\n")
	return b.String()
}

func qualityNarrative(q Quality) string {
	if q.Missing || q.Unavailable {
		return ""
	}
	var b strings.Builder
	if s := strings.TrimSpace(q.ConfidenceRationale); s != "" {
		fmt.Fprintf(&b, "**Confidence rationale**:\n\n%s\n\n", s)
	}
	if s := strings.TrimSpace(q.Counterevidence); s != "" {
		fmt.Fprintf(&b, "**Counterevidence**:\n\n%s\n\n", s)
	}
	return b.String()
}

func staleReportNotice(stale bool) string {
	if !stale {
		return ""
	}
	return "The evidence has changed; the detailed report needs updating.\n\n"
}

const reportMaxReportBytes = 8 << 10

func detailedReport(body, heading string) string {
	return detailedReportLimit(body, heading, reportMaxReportBytes)
}

func detailedReportFull(body, heading string) string {
	return detailedReportLimit(body, heading, 0)
}

func detailedReportLimit(body, heading string, limit int) string {
	s := strings.TrimSpace(body)
	if s == "" {
		return ""
	}
	if limit > 0 && len(s) > limit {
		cut := s[:limit]
		for len(cut) > 0 && !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		cut = strings.TrimSpace(cut)
		if unbalancedFence(cut) {
			cut += "\n```"
		}
		return heading + "\n\n" + cut +
			fmt.Sprintf("\n\n_Detailed report truncated at %d KiB; read the full text from the finding record._\n\n", limit>>10)
	}
	return heading + "\n\n" + s + "\n\n"
}

func unbalancedFence(s string) bool {
	fences := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
			fences++
		}
	}
	return fences%2 == 1
}

func verificationLabel(s string) string {
	switch s {
	case "", VerificationUnverified:
		return "Unverified"
	case VerificationVerified:
		return "Verified"
	case VerificationBlocked:
		return "Blocked"
	case VerificationNotAssessed:
		return "Not assessed"
	case VerificationFalsePositive:
		return "False positive"
	}
	return s
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
