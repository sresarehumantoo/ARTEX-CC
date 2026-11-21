package tools

import "context"

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
	AddFinding(ctx context.Context, f FindingInput) (int64, error)
	ListFindings(ctx context.Context, explorationID int64) ([]Finding, error)
	FindingExists(ctx context.Context, findingID int64) (bool, error)
	FindPotentialDuplicates(ctx context.Context, severity string, assetIDs []int64) ([]int64, error)
	MergeFindings(ctx context.Context, primaryID int64, secondaryIDs []int64) (*MergedFinding, error)
	DismissFinding(ctx context.Context, findingID int64, reason string) error

	// Exploration graph
	GraphOverview(ctx context.Context, explorationID int64) (*Overview, error)
	AddNode(ctx context.Context, explorationID int64, kind string, payload map[string]any, priority int, state, origin string, anchors []int64) (int64, error)
	UpdateNodeState(ctx context.Context, nodeID int64, state string) error
	ListNodes(ctx context.Context, explorationID int64, kind string, limit int) ([]Node, error)
	NodeDetail(ctx context.Context, nodeID int64) (*Node, error)
	AddEdge(ctx context.Context, from int64, rel string, to int64) error
	Frontier(ctx context.Context, explorationID int64, limit int) ([]Node, error)
	CountFinishedIntents(ctx context.Context, explorationID int64) (int, error)
	CountOpenIntents(ctx context.Context, explorationID int64) (int, error)
	OriginFactID(ctx context.Context, explorationID int64) (int64, error)
	Edges(ctx context.Context, explorationID int64, limit int) ([]Edge, error)
	Root(ctx context.Context, explorationID int64) (description, goal string, err error)
	AddConstraint(ctx context.Context, explorationID int64, kind, text, origin string) (int64, error)
	ListConstraints(ctx context.Context, explorationID int64) ([]Constraint, error)

	// Task scope
	AddTaskScope(ctx context.Context, taskID int64, kind, value, reason, source string) error
	ListTaskScope(ctx context.Context, taskID int64) ([]int64, error)
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

	// Engagement completion
	CompleteEngagement(ctx context.Context, taskID int64, summary string) error

	// Activity log
	LogActivity(ctx context.Context, explorationID int64, entry ActivityEntry) (int64, error)
	ListActivity(ctx context.Context, explorationID int64, filter ActivityFilter) ([]Activity, error)

	// Report
	ListFindingsForReport(ctx context.Context, explorationID int64) ([]ReportFinding, error)

	// Intent lifecycle
	UpdateNodeFields(ctx context.Context, nodeID int64, state string, payloadMerge map[string]any) error

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
	TaskID int64  `json:"task_id,omitempty"`
	DSL    string `json:"dsl,omitempty"`
	Type   string `json:"type,omitempty"`
	ID     int64  `json:"id,omitempty"`
	IDs    []int64 `json:"ids,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// Asset is a stored asset with its denormalized label and type-specific data.
type Asset struct {
	ID     int64          `json:"id"`
	Type   string         `json:"type"`
	Label  string         `json:"label"`
	Data   map[string]any `json:"data,omitempty"`
}

// Company is a company with its scope rules and asset count.
type Company struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	AssetCount int      `json:"asset_count"`
	Scope      []string `json:"scope"`
}

// ScopeRule is one wildcard or CIDR rule governing what targets are in scope.
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
	ID        int64  `json:"id"`
	NodeID    int64  `json:"node_id,omitempty"`
	VulnClass string `json:"vulnclass,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Summary   string `json:"summary"`
	State     string `json:"state,omitempty"`
	Status    string `json:"status,omitempty"`
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
}

// Overview is aggregated exploration state returned by GraphOverview.
type Overview struct {
	Task             map[string]any   `json:"task,omitempty"`
	Goals            []map[string]any `json:"goals,omitempty"`
	Hints            []map[string]any `json:"hints,omitempty"`
	OpenIntents      []map[string]any `json:"open_intents,omitempty"`
	RunningIntents   []map[string]any `json:"running_intents,omitempty"`
	RecentDone       []map[string]any `json:"recent_done_intents,omitempty"`
	DoneIntentsTotal int              `json:"done_intents_total"`
	FrontierOpen     int              `json:"frontier_open"`
	FindingsTotal    int              `json:"findings_total"`
	FactsTotal       int              `json:"facts"`
	FindingList      []map[string]any `json:"finding_list,omitempty"`
	RecentFacts      []map[string]any `json:"recent_facts,omitempty"`
	Coverage         map[string]any   `json:"coverage,omitempty"`
	Credentials      []map[string]any `json:"task_credentials,omitempty"`
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
	ID             int64  `json:"id"`
	ExplorationID  int64  `json:"exploration_id"`
	NodeID         int64  `json:"node_id,omitempty"`
	Worker         string `json:"worker,omitempty"`
	Kind           string `json:"kind,omitempty"`
	Tool           string `json:"tool,omitempty"`
	IsError        bool   `json:"is_error,omitempty"`
	Summary        string `json:"summary,omitempty"`
	Detail         string `json:"detail,omitempty"`
	CreatedAt      string `json:"created_at"`
}

// ActivityFilter holds query parameters for listing activity entries.
type ActivityFilter struct {
	NodeID int64 `json:"node_id,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Limit  int   `json:"limit,omitempty"`
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
}

const (
	KindGoal       = "goal"
	KindHint       = "hint"
	KindIntent     = "intent"
	KindFact       = "fact"
	KindFinding    = "finding"

	RelDerivedFrom = "derived_from"
	RelYields      = "yields"
	RelSpawns      = "spawns"
	RelProves      = "proves"
)
