package report

import "time"

type ReportFinding struct {
	ID        int64
	VulnClass string
	Name      string
	Severity  string
	Summary   string
	Evidence  string
}

type CoverageLedger struct {
	Entries      []CoverageEntry
	Gaps         []CoverageGap
	Completeness *CoverageCompleteness
}

type CoverageEntry struct {
	Surface  string
	RiskArea string
	Outcome  string
}

type CoverageGap struct {
	Kind     string
	Surface  string
	RiskArea string
}

func (g CoverageGap) String() string {
	if g.Surface == "" {
		return g.Kind + " " + g.RiskArea
	}
	return g.Kind + " " + g.Surface + " " + g.RiskArea
}

type CoverageCompleteness struct {
	State  string
	Caveat string
}

type FindingQuality struct {
	ID                    int64
	PoeLevel              int
	VerificationStatus    string
	VerificationNote      string
	Counterevidence       string
	ConfidenceRationale   string
	TrafficCount          int
	Report                string
	EvidenceVersion       int64
	ReportEvidenceVersion int64
}

func (q FindingQuality) ReportStale() bool {
	return q.Report != "" && q.EvidenceVersion != q.ReportEvidenceVersion
}

type ExportFinding struct {
	ID              int64
	VulnClass       string
	Name            string
	Severity        string
	Summary         string
	Evidence        string
	Status          string
	ReportState     string
	TaskDescription string
	Report          string
	CreatedAt       time.Time
}

const (
	VerificationVerified      = "verified"
	VerificationUnverified    = "unverified"
	VerificationBlocked       = "blocked"
	VerificationNotAssessed   = "not_assessed"
	VerificationFalsePositive = "false_positive"
)
