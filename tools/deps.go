package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Store is the interface tools use to access the data layer.
// Implemented by store.Store.
type Store interface {
	// Assets
	InsertAsset(ctx context.Context, taskID int64, item AssetInput) (int64, error)
	ListAssets(ctx context.Context, filter AssetFilter) ([]Asset, int, error)
	ListUntestedAssets(ctx context.Context, taskID int64, assetType string, limit, offset int) ([]Asset, int, error)

	// Companies
	ListCompanies(ctx context.Context) ([]Company, error)
	GetOrCreateCompany(ctx context.Context, name, logo string) (int64, error)
	AddCompanyScope(ctx context.Context, companyID int64, lines []string, reason string) (added, skipped, invalid int, errors []string, err error)

	// Findings
	AddFinding(ctx context.Context, f FindingInput) (findingID, nodeID int64, err error)
	ListFindings(ctx context.Context, explorationID int64) ([]Finding, error)
	FindingExists(ctx context.Context, findingID int64) (bool, error)
	FindPotentialDuplicates(ctx context.Context, explorationID int64, vulnclass, severity string, assetIDs []int64) ([]Finding, error)
	MergeFindings(ctx context.Context, primaryID int64, secondaryIDs []int64) (*MergedFinding, error)
	DismissFinding(ctx context.Context, findingID int64, reason string) error

	// Exploration graph
	GraphOverview(ctx context.Context, explorationID int64) (*Overview, error)
	AddNode(ctx context.Context, explorationID int64, kind string, payload map[string]any, priority int, state, origin string, anchors []int64) (int64, error)
	UpdateNodeState(ctx context.Context, nodeID int64, state string) error
	ClaimIntent(ctx context.Context, nodeID int64, owner string, lease time.Duration) (*IntentClaim, error)
	RenewIntentClaims(ctx context.Context, nodeIDs []int64, holder string, lease time.Duration) error
	ListNodes(ctx context.Context, explorationID int64, kind string, limit int) ([]Node, error)
	NodeDetail(ctx context.Context, nodeID int64) (*Node, error)
	AddEdge(ctx context.Context, from int64, rel string, to int64) error
	ProveGoal(ctx context.Context, goalID, evidenceID int64) error
	Frontier(ctx context.Context, explorationID int64, limit int) ([]Node, error)
	CountFinishedIntents(ctx context.Context, explorationID int64) (int, error)
	CountOpenIntents(ctx context.Context, explorationID int64) (int, error)
	OriginFactID(ctx context.Context, explorationID int64) (int64, error)
	AddGoals(ctx context.Context, explorationID int64, goals []GoalInput, appendGoals bool) (*GoalsResult, error)
	AddIntents(ctx context.Context, explorationID int64, intents []IntentInput) (*IntentsResult, error)
	Edges(ctx context.Context, explorationID int64, limit int) ([]Edge, error)
	ListChanges(ctx context.Context, explorationID, taskID int64, after ChangeCursor, limit int) ([]Change, error)
	ListClaims(ctx context.Context, explorationID int64, owner string) ([]HeldClaim, error)
	Root(ctx context.Context, explorationID int64) (description, goal string, err error)
	AddConstraint(ctx context.Context, explorationID int64, kind, text, origin string) (int64, error)
	ListConstraints(ctx context.Context, explorationID int64) ([]Constraint, error)

	// Task scope
	AddTaskScope(ctx context.Context, taskID int64, kind, value, reason, source string) error
	ListTaskScope(ctx context.Context, taskID int64) ([]int64, error)
	ListScopeRules(ctx context.Context, taskID int64) ([]ScopeRule, error)
	IsInScope(ctx context.Context, taskID int64, target string) (bool, string, string, error)

	// Credentials
	SaveCredential(ctx context.Context, taskID int64, cred CredentialInput) (int64, error)
	ListCredentials(ctx context.Context, taskID int64) ([]Credential, error)

	// Coverage
	RecordCoverage(ctx context.Context, taskID int64, entry CoverageEntry) error
	ListCoverageGaps(ctx context.Context, taskID int64, explorationID int64) (*CoverageGaps, error)

	// Engagements
	CreateEngagement(ctx context.Context, goal, description string) (taskID int64, explorationID int64, err error)
	ListEngagements(ctx context.Context) ([]EngagementSummary, error)
	GetEngagement(ctx context.Context, id int64) (*EngagementDetail, error)

	// Finding-traffic bindings
	BindFindingTraffic(ctx context.Context, findingID int64, snapshotID, role, note string, position int) error
	ListFindingTraffic(ctx context.Context, findingID int64) ([]TrafficBinding, error)
	FindingTrafficCount(ctx context.Context, findingID int64) (int, error)

	// Engagement management
	CompleteEngagement(ctx context.Context, taskID int64, summary string) error
	UpdateEngagement(ctx context.Context, taskID int64, goal, description *string) error
	DeleteEngagement(ctx context.Context, taskID int64) error
	EngagementActivity(ctx context.Context, explorationID int64, limit int) ([]TimelineEntry, error)

	// Activity log
	LogActivity(ctx context.Context, explorationID int64, entry ActivityEntry) (int64, error)
	ListActivity(ctx context.Context, explorationID int64, filter ActivityFilter) ([]Activity, error)

	// Report
	ListFindingsForReport(ctx context.Context, explorationID int64) ([]ReportFinding, error)

	// Intent lifecycle
	UpdateNodeFields(ctx context.Context, nodeID int64, state string, payloadMerge map[string]any) error

	// Target change journal
	RecordTargetChange(ctx context.Context, explorationID int64, in TargetChangeInput) (*TargetChangeRecord, error)
	RecordTargetRestore(ctx context.Context, explorationID int64, changeIDs []int64, observed, restoredBy string) ([]TargetRestoreOutcome, error)
	ListTargetChanges(ctx context.Context, explorationID int64, unrestoredOnly bool) ([]TargetChange, error)

	// Export/Import
	ExportEngagementData(ctx context.Context, taskID int64) (*EngagementExport, error)
	ImportEngagementData(ctx context.Context, data *EngagementExport) (taskID int64, explorationID int64, err error)
}

// AssetInput is one item for asset insertion.
type AssetInput struct {
	Type string `json:"type"`

	// root_domain / subdomain
	Domain      string   `json:"domain,omitempty"`
	ICP         string   `json:"icp,omitempty"`
	RecordType  string   `json:"record_type,omitempty"`
	RecordValue []string `json:"record_value,omitempty"`

	// ip
	IP           string        `json:"ip,omitempty"`
	BoundDomains []string      `json:"bound_domains,omitempty"`
	OpenPorts    []PortService `json:"open_ports,omitempty"`

	// app
	AppName     string `json:"app_name,omitempty"`
	BundleID    string `json:"bundle_id,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	AppICP      string `json:"app_icp,omitempty"`
	CompanyID   *int64 `json:"company_id,omitempty"`

	// service (http)
	URL           string           `json:"url,omitempty"`
	Technologies  []string         `json:"technologies,omitempty"`
	StatusCode    *int             `json:"status_code,omitempty"`
	ContentLength *int64           `json:"content_length,omitempty"`
	PageTitle     string           `json:"page_title,omitempty"`
	FaviconMMH3   string           `json:"favicon_mmh3,omitempty"`
	Auth          []map[string]any `json:"auth,omitempty"`
	ServiceName   string           `json:"service_name,omitempty"`
	ServiceIP     string           `json:"service_ip,omitempty"`

	// service (other)
	Port  int    `json:"port,omitempty"`
	Proto string `json:"proto,omitempty"`

	// endpoint
	Method string           `json:"method,omitempty"`
	Params []map[string]any `json:"params,omitempty"`
}

// PortService is a port number and optional service name for an IP asset.
type PortService struct {
	Port    int    `json:"port"`
	Service string `json:"service,omitempty"`
}

// AssetFilter holds query parameters for listing assets.
type AssetFilter struct {
	TaskID int64   `json:"task_id,omitempty"`
	DSL    string  `json:"dsl,omitempty"`
	Type   string  `json:"type,omitempty"`
	ID     int64   `json:"id,omitempty"`
	IDs    []int64 `json:"ids,omitempty"`
	Limit  int     `json:"limit,omitempty"`
	Offset int     `json:"offset,omitempty"`
}

// Asset is a stored asset with its denormalized label and type-specific data.
type Asset struct {
	ID    int64          `json:"id"`
	Type  string         `json:"type"`
	Label string         `json:"label"`
	Data  map[string]any `json:"data,omitempty"`
}

// Company is a company with its scope rules and asset count.
type Company struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	AssetCount int      `json:"asset_count"`
	Scope      []string `json:"scope"`
}

// ScopeRule is one scope entry: a domain, IP, CIDR, company, ICP or keyword.
type ScopeRule struct {
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Reason string `json:"reason,omitempty"`
}

// FindingInput holds the parameters for creating a finding.
type FindingInput struct {
	TaskID        int64   `json:"task_id"`
	ExplorationID int64   `json:"exploration_id"`
	IntentID      int64   `json:"intent_id,omitempty"`
	VulnClass     string  `json:"vulnclass"`
	Name          string  `json:"name,omitempty"`
	Severity      string  `json:"severity"`
	Summary       string  `json:"summary"`
	Evidence      string  `json:"evidence,omitempty"`
	AssetIDs      []int64 `json:"asset_ids,omitempty"`
	Worker        string  `json:"worker,omitempty"`
}

// Finding is a stored vulnerability finding.
type Finding struct {
	ID            int64  `json:"id"`
	NodeID        int64  `json:"node_id,omitempty"`
	VulnClass     string `json:"vulnclass,omitempty"`
	Severity      string `json:"severity,omitempty"`
	Summary       string `json:"summary"`
	State         string `json:"state,omitempty"`
	Status        string `json:"status,omitempty"`
	MergedInto    int64  `json:"merged_into,omitempty"`
	Dismissed     bool   `json:"dismissed,omitempty"`
	DismissReason string `json:"dismiss_reason,omitempty"`
}

// MergedFinding is the result of merging multiple findings into one.
type MergedFinding struct {
	ID        int64   `json:"id"`
	VulnClass string  `json:"vulnclass"`
	Name      string  `json:"name"`
	Severity  string  `json:"severity"`
	Summary   string  `json:"summary"`
	Evidence  string  `json:"evidence"`
	AssetIDs  []int64 `json:"asset_ids"`
	Status    string  `json:"status"`
	// RequestedPrimaryID is set when the requested primary was already merged
	// and ID is the live finding it resolved to.
	RequestedPrimaryID int64            `json:"requested_primary_id,omitempty"`
	Skipped            []SkippedFinding `json:"skipped,omitempty"`
}

// SkippedFinding is a secondary that a merge left untouched.
type SkippedFinding struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// Overview is aggregated exploration state returned by GraphOverview.
type Overview struct {
	Task              map[string]any   `json:"task,omitempty"`
	Goals             []map[string]any `json:"goals,omitempty"`
	Hints             []map[string]any `json:"hints,omitempty"`
	OpenIntents       []map[string]any `json:"open_intents,omitempty"`
	RunningIntents    []map[string]any `json:"running_intents,omitempty"`
	RecentDone        []map[string]any `json:"recent_done_intents,omitempty"`
	DoneIntentsTotal  int              `json:"done_intents_total"`
	FrontierOpen      int              `json:"frontier_open"`
	FindingsTotal     int              `json:"findings_total"`
	FindingsDismissed int              `json:"findings_dismissed"`
	FindingsMerged    int              `json:"findings_merged"`
	FactsTotal        int              `json:"facts"`
	FindingList       []map[string]any `json:"finding_list,omitempty"`
	RecentFacts       []map[string]any `json:"recent_facts,omitempty"`
	Coverage          map[string]any   `json:"coverage,omitempty"`
	Credentials       []map[string]any `json:"task_credentials,omitempty"`
	ChangesCursor     string           `json:"changes_cursor,omitempty"`

	UnrestoredTargetChanges []map[string]any `json:"unrestored_target_changes,omitempty"`
}

// Node is a vertex in the exploration graph (goal, hint, intent, fact, or finding).
type Node struct {
	ID      int64          `json:"id"`
	Kind    string         `json:"kind"`
	State   string         `json:"state"`
	Payload map[string]any `json:"payload,omitempty"`
	Origin  string         `json:"origin,omitempty"`
}

// NodeFilter holds query parameters for listing exploration nodes.
type NodeFilter struct {
	Kind  string `json:"kind,omitempty"`
	State string `json:"state,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// Edge is a directed relationship between two exploration nodes.
type Edge struct {
	From int64  `json:"from"`
	Rel  string `json:"rel"`
	To   int64  `json:"to"`
}

// Constraint is an operator-imposed restriction on exploration behavior.
type GoalInput struct {
	Text      string
	VulnClass string
}

type GoalRef struct {
	ID    int64  `json:"id"`
	Text  string `json:"text"`
	State string `json:"state,omitempty"`
}

type SkippedGoal struct {
	Text       string `json:"text"`
	Reason     string `json:"reason"`
	ExistingID int64  `json:"existing_id,omitempty"`
}

// GoalsResult reports what AddGoals did. Existing is set only when goals
// already existed and appendGoals was false, so nothing was added.
type GoalsResult struct {
	Created  []GoalRef     `json:"created"`
	Skipped  []SkippedGoal `json:"skipped,omitempty"`
	Existing []GoalRef     `json:"existing_goals,omitempty"`
}

const (
	GoalSkipDuplicate  = "duplicate"
	GoalSkipAlreadySet = "goals_already_set"
)

// NormalizeGoalText is the key goals are deduplicated on: case, surrounding
// whitespace, internal whitespace runs and trailing periods are ignored.
func NormalizeGoalText(s string) string {
	return strings.TrimRight(strings.Join(strings.Fields(strings.ToLower(s)), " "), ".")
}

type IntentInput struct {
	Summary   string
	Priority  int
	AssetIDs  []int64
	ParentIDs []int64
}

type IntentRef struct {
	ID       int64  `json:"id"`
	Summary  string `json:"summary"`
	Priority int    `json:"priority"`
}

type SkippedIntent struct {
	Index      int    `json:"index"`
	Summary    string `json:"summary"`
	ExistingID int64  `json:"existing_id"`
	State      string `json:"existing_state"`
}

// IntentsResult reports what AddIntents did. IDs has one entry per input,
// 0 where the intent was skipped as a duplicate or failed (Errors, keyed by
// input index). Open lists the claimable intents that existed before the call
// and were not duplicates of it.
type IntentsResult struct {
	IDs     []int64
	Skipped []SkippedIntent
	Errors  map[int]string
	Open    []IntentRef
}

// IntentKey is what add_intent deduplicates on: the summary's words ignoring
// case, punctuation and spacing, plus the set of asset ids.
func IntentKey(summary string, assetIDs []int64) string {
	words := strings.FieldsFunc(strings.ToLower(summary), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	ids := slices.Compact(slices.Sorted(slices.Values(assetIDs)))
	return strings.Join(words, " ") + "|" + fmt.Sprint(ids)
}

type Constraint struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// CredentialInput holds the parameters for saving a discovered credential.
type CredentialInput struct {
	Domain   string `json:"domain"`
	Username string `json:"username"`
	Password string `json:"password"`
	Source   string `json:"source,omitempty"`
	Note     string `json:"note,omitempty"`
}

// Credential is a stored credential discovered during testing.
type Credential struct {
	ID       int64  `json:"id"`
	Domain   string `json:"domain"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Source   string `json:"source,omitempty"`
	Note     string `json:"note,omitempty"`
}

// CoverageEntry records one test-coverage data point against an attack surface and risk area.
type CoverageEntry struct {
	Surface  string `json:"surface"`
	RiskArea string `json:"risk_area"`
	Outcome  string `json:"outcome"`
	Evidence string `json:"evidence,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Worker   string `json:"worker,omitempty"`
	IntentID int64  `json:"intent_id,omitempty"`
}

// CoverageGaps summarizes what hasn't been tested or recorded in the engagement.
type CoverageGaps struct {
	UntargetedAssets   []Asset `json:"untargeted_assets"`
	UnresolvedIntents  []Node  `json:"unresolved_intents"`
	UnprovenGoals      []Node  `json:"unproven_goals"`
	UnenrichedServices []Asset `json:"unenriched_services"`
}

// EngagementSummary is a lightweight view of an engagement for list endpoints.
type EngagementSummary struct {
	ID            int64  `json:"id"`
	Goal          string `json:"goal"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	ExplorationID int64  `json:"exploration_id"`
	CreatedAt     string `json:"created_at"`
	NodeCount     int    `json:"node_count"`
	FindingCount  int    `json:"finding_count"`
	AssetCount    int    `json:"asset_count"`
	GoalsMet      bool   `json:"goals_met"`
}

// EngagementDetail is a full engagement including its description.
type EngagementDetail struct {
	EngagementSummary
	Description string `json:"description"`
}

// ReportFinding is a finding formatted for report generation.
type ReportFinding struct {
	ID        int64  `json:"id"`
	VulnClass string `json:"vulnclass"`
	Name      string `json:"name"`
	Severity  string `json:"severity"`
	Summary   string `json:"summary"`
	Evidence  string `json:"evidence"`
	Status    string `json:"status"`
}

// TrafficBinding links a finding to a captured traffic exchange used as evidence.
type TrafficBinding struct {
	ID         int64  `json:"id"`
	FindingID  int64  `json:"finding_id"`
	SnapshotID string `json:"snapshot_id"`
	Role       string `json:"role"`
	Note       string `json:"note"`
	Position   int    `json:"position"`
}

// ActivityEntry holds the parameters for logging an exploration activity event.
type ActivityEntry struct {
	NodeID  int64  `json:"node_id,omitempty"`
	Worker  string `json:"worker,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Tool    string `json:"tool,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
	Summary string `json:"summary,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Activity is a stored exploration activity log entry.
type Activity struct {
	ID            int64  `json:"id"`
	ExplorationID int64  `json:"exploration_id"`
	NodeID        int64  `json:"node_id,omitempty"`
	Worker        string `json:"worker,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Tool          string `json:"tool,omitempty"`
	IsError       bool   `json:"is_error,omitempty"`
	Summary       string `json:"summary,omitempty"`
	Detail        string `json:"detail,omitempty"`
	CreatedAt     string `json:"created_at"`
}

// ActivityFilter holds query parameters for listing activity entries.
type ActivityFilter struct {
	NodeID int64  `json:"node_id,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Since  string `json:"since,omitempty"`
}

// EngagementExport is a serializable snapshot of an engagement for export and import.
type EngagementExport struct {
	Version     int              `json:"version"`
	ExportedAt  string           `json:"exported_at"`
	Goal        string           `json:"goal"`
	Description string           `json:"description"`
	Nodes       []map[string]any `json:"nodes"`
	Edges       []map[string]any `json:"edges"`
	Constraints []map[string]any `json:"constraints"`
	Assets      []map[string]any `json:"assets"`
	Findings    []map[string]any `json:"findings"`
	Credentials []map[string]any `json:"credentials"`
	Coverage    []map[string]any `json:"coverage"`
	Scope       []map[string]any `json:"scope"`

	CompanyID     int64            `json:"company_id,omitempty"`
	Companies     []map[string]any `json:"companies,omitempty"`
	TargetChanges []map[string]any `json:"target_changes,omitempty"`
}

// TimelineEntry is one row in the engagement activity timeline.
type TimelineEntry struct {
	Timestamp string `json:"timestamp"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
}

const (
	KindGoal    = "goal"
	KindHint    = "hint"
	KindIntent  = "intent"
	KindFact    = "fact"
	KindFinding = "finding"

	StateHintActive = "active"

	RelDerivedFrom = "derived_from"
	RelYields      = "yields"
	RelSpawns      = "spawns"
	RelProves      = "proves"
)
