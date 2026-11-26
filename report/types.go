package report

import "time"

// ReportFinding is a finding record used for report generation.
type ReportFinding struct {
	ID        int64
	VulnClass string
	Name      string
	Severity  string
	Summary   string
	Evidence  string
}

// TargetChange is a change made to a target during the engagement and
// whether it was verifiably restored to its baseline.
type TargetChange struct {
	ID              int64
	Target          string
	Resource        string
	Kind            string
	Baseline        string
	RecordedBy      string
	Restored        bool
	RestoredBy      string
	RestoredAt      time.Time
	LastObserved    string
	RestoreAttempts int
}

// CoverageLedger is aggregated test coverage with identified gaps.
type CoverageLedger struct {
	Entries      []CoverageEntry
	Gaps         []CoverageGap
	Completeness *CoverageCompleteness
}

// CoverageEntry is one record of test coverage against an attack surface and risk area.
type CoverageEntry struct {
	Surface  string
	RiskArea string
	Outcome  string
}

// CoverageGap is an identified gap in test coverage.
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

// CoverageCompleteness is the overall completeness assessment of test coverage.
type CoverageCompleteness struct {
	State  string
	Caveat string
}

// FindingQuality holds quality and verification metadata for a single finding.
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

// ExportFinding is a finding formatted for external export.
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
