package store

// Exploration node kinds.
const (
	KindBegin   = "begin"
	KindGoal    = "goal"
	KindIntent  = "intent"
	KindFact    = "fact"
	KindFinding = "finding"
	KindHint    = "hint"
)

// StateOrigin marks the task root fact seeded at task creation.
const StateOrigin = "origin"

// StateIntentDeleted marks an intent soft-deleted by the user.
const StateIntentDeleted = "deleted"

// Exploration edge relations.
const (
	RelSpawns      = "spawns"
	RelDerivedFrom = "derived_from"
	RelYields      = "yields"
	RelProves      = "proves"
)

// Coverage ledger outcomes.
const (
	OutcomeReported      = "reported"
	OutcomeNoIssueFound  = "no_issue_found"
	OutcomeRuledOut      = "ruled_out"
	OutcomeNotApplicable = "not_applicable"
	OutcomeNeedsFollowUp = "needs_follow_up"
)

// Finding status values.
const (
	FindingStatusPending       = "pending"
	FindingStatusConfirmed     = "confirmed"
	FindingStatusFalsePositive = "false_positive"
	FindingStatusResolved      = "resolved"
	FindingStatusFixed         = "fixed"
	FindingStatusIgnored       = "ignored"
	FindingStatusDuplicate     = "duplicate"
	FindingStatusRiskAccepted  = "risk_accepted"
)

// Finding severity values.
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)
