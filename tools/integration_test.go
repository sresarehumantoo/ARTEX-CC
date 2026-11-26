package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// statefulMock is an in-memory Store that tracks state across multiple tool
// calls, enabling integration tests that exercise realistic multi-tool workflows.
type statefulMock struct {
	mockStore
	assets        []Asset
	nextAssetID   int64
	findings      []Finding
	nextFindingID int64
	nodes         []Node
	nextNodeID    int64
	edges         []Edge
	coverage      []CoverageEntry
}

func newStatefulMock() *statefulMock {
	return &statefulMock{
		nextAssetID:   1,
		nextFindingID: 1,
		nextNodeID:    10,
	}
}

var _ Store = (*statefulMock)(nil)

func (m *statefulMock) InsertAsset(_ context.Context, _ int64, item AssetInput) (int64, error) {
	id := m.nextAssetID
	m.nextAssetID++
	label := item.Domain
	if label == "" {
		label = item.IP
	}
	if label == "" {
		label = item.URL
	}
	m.assets = append(m.assets, Asset{ID: id, Type: item.Type, Label: label})
	return id, nil
}

func (m *statefulMock) ListAssets(_ context.Context, f AssetFilter) ([]Asset, int, error) {
	if f.ID > 0 {
		for _, a := range m.assets {
			if a.ID == f.ID {
				return []Asset{a}, 1, nil
			}
		}
		return nil, 0, nil
	}
	if len(f.IDs) > 0 {
		set := make(map[int64]bool, len(f.IDs))
		for _, id := range f.IDs {
			set[id] = true
		}
		var out []Asset
		for _, a := range m.assets {
			if set[a.ID] {
				out = append(out, a)
			}
		}
		return out, len(out), nil
	}
	return m.assets, len(m.assets), nil
}

func (m *statefulMock) AddNode(_ context.Context, _ int64, kind string, payload map[string]any, _ int, state, origin string, _ []int64) (int64, error) {
	id := m.nextNodeID
	m.nextNodeID++
	m.nodes = append(m.nodes, Node{ID: id, Kind: kind, State: state, Payload: payload, Origin: origin})
	return id, nil
}

func (m *statefulMock) AddIntents(ctx context.Context, explorationID int64, intents []IntentInput) (*IntentsResult, error) {
	res := &IntentsResult{}
	seen := map[string]Node{}
	for _, n := range m.nodes {
		if n.Kind == KindIntent && (n.State == "open" || n.State == "running") {
			summary, _ := n.Payload["summary"].(string)
			seen[IntentKey(summary, nil)] = n
		}
	}
	for i, it := range intents {
		key := IntentKey(it.Summary, it.AssetIDs)
		if n, dup := seen[key]; dup {
			res.IDs = append(res.IDs, 0)
			res.Skipped = append(res.Skipped, SkippedIntent{Index: i, Summary: it.Summary, ExistingID: n.ID, State: n.State})
			continue
		}
		id, _ := m.AddNode(ctx, explorationID, KindIntent, map[string]any{"summary": it.Summary}, it.Priority, "open", "planner", it.AssetIDs)
		seen[key] = Node{ID: id, State: "open"}
		res.IDs = append(res.IDs, id)
	}
	return res, nil
}

func (m *statefulMock) NodeDetail(_ context.Context, id int64) (*Node, error) {
	for i := range m.nodes {
		if m.nodes[i].ID == id {
			return &m.nodes[i], nil
		}
	}
	return nil, nil
}

func (m *statefulMock) UpdateNodeState(_ context.Context, id int64, state string) error {
	for i := range m.nodes {
		if m.nodes[i].ID == id {
			m.nodes[i].State = state
		}
	}
	return nil
}

func (m *statefulMock) UpdateNodeFields(_ context.Context, id int64, state string, merge map[string]any) error {
	for i := range m.nodes {
		if m.nodes[i].ID == id {
			m.nodes[i].State = state
			if m.nodes[i].Payload == nil {
				m.nodes[i].Payload = make(map[string]any)
			}
			for k, v := range merge {
				m.nodes[i].Payload[k] = v
			}
		}
	}
	return nil
}

func (m *statefulMock) AddFinding(_ context.Context, f FindingInput) (int64, int64, error) {
	id := m.nextFindingID
	m.nextFindingID++
	m.findings = append(m.findings, Finding{
		ID: id, VulnClass: f.VulnClass, Severity: f.Severity,
		Summary: f.Summary, State: "confirmed", Status: "open",
	})
	return id, 0, nil
}

func (m *statefulMock) ListFindings(_ context.Context, _ int64) ([]Finding, error) {
	return m.findings, nil
}

func (m *statefulMock) MergeFindings(_ context.Context, primaryID int64, secondaryIDs []int64) (*MergedFinding, error) {
	sec := make(map[int64]bool, len(secondaryIDs))
	for _, id := range secondaryIDs {
		sec[id] = true
	}
	for i := range m.findings {
		if sec[m.findings[i].ID] {
			m.findings[i].MergedInto = primaryID
			m.findings[i].Status = "merged"
		}
	}
	for _, f := range m.findings {
		if f.ID == primaryID {
			return &MergedFinding{
				ID: f.ID, VulnClass: f.VulnClass, Severity: f.Severity,
				Summary: f.Summary, Status: "open",
			}, nil
		}
	}
	return &MergedFinding{ID: primaryID, Status: "open"}, nil
}

func (m *statefulMock) DismissFinding(_ context.Context, id int64, reason string) error {
	for i := range m.findings {
		if m.findings[i].ID == id {
			m.findings[i].Dismissed = true
			m.findings[i].DismissReason = reason
		}
	}
	return nil
}

func (m *statefulMock) AddEdge(_ context.Context, from int64, rel string, to int64) error {
	m.edges = append(m.edges, Edge{From: from, Rel: rel, To: to})
	return nil
}

func (m *statefulMock) OriginFactID(_ context.Context, _ int64) (int64, error) {
	for _, n := range m.nodes {
		if n.Kind == KindFact {
			return n.ID, nil
		}
	}
	return 0, nil
}

func (m *statefulMock) RecordCoverage(_ context.Context, _ int64, entry CoverageEntry) error {
	m.coverage = append(m.coverage, entry)
	return nil
}

func (m *statefulMock) ListCoverageGaps(_ context.Context, _ int64, _ int64) (*CoverageGaps, error) {
	covered := make(map[string]bool, len(m.coverage))
	for _, c := range m.coverage {
		covered[c.Surface] = true
	}
	var untargeted []Asset
	for _, a := range m.assets {
		if !covered[a.Label] {
			untargeted = append(untargeted, a)
		}
	}
	return &CoverageGaps{UntargetedAssets: untargeted}, nil
}

func (m *statefulMock) GraphOverview(_ context.Context, _ int64) (*Overview, error) {
	return &Overview{
		Task:          map[string]any{"goal": "test"},
		FindingsTotal: len(m.findings),
		Coverage:      map[string]any{"entries": len(m.coverage)},
	}, nil
}

func (m *statefulMock) ExportEngagementData(_ context.Context, _ int64) (*EngagementExport, error) {
	assets := make([]map[string]any, len(m.assets))
	for i, a := range m.assets {
		assets[i] = map[string]any{"id": a.ID, "type": a.Type, "label": a.Label}
	}
	findings := make([]map[string]any, len(m.findings))
	for i, f := range m.findings {
		findings[i] = map[string]any{"id": f.ID, "severity": f.Severity, "summary": f.Summary, "status": f.Status}
	}
	return &EngagementExport{
		Version: 1, ExportedAt: "2024-01-01T00:00:00Z",
		Goal: "test goal", Description: "test description",
		Assets: assets, Findings: findings,
	}, nil
}

func (m *statefulMock) ImportEngagementData(_ context.Context, _ *EngagementExport) (int64, int64, error) {
	return 2, 2, nil
}

func iRunToolOK(t *testing.T, reg *Registry, name, input string) Result {
	t.Helper()
	tool, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("tool %q not registered", name)
	}
	r, err := tool.Run(WithSessionID(context.Background(), "sess-test"), json.RawMessage(input))
	if err != nil {
		t.Fatalf("%s: unexpected Go error: %v", name, err)
	}
	if r.IsError {
		t.Fatalf("%s: tool error: %s", name, r.Content)
	}
	return r
}

func iRunToolAny(t *testing.T, reg *Registry, name, input string) Result {
	t.Helper()
	tool, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("tool %q not registered", name)
	}
	r, err := tool.Run(WithSessionID(context.Background(), "sess-test"), json.RawMessage(input))
	if err != nil {
		t.Fatalf("%s: unexpected Go error: %v", name, err)
	}
	return r
}

func iParseObj(t *testing.T, r Result) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Content), &m); err != nil {
		t.Fatalf("parse JSON object: %v\ncontent: %s", err, r.Content)
	}
	return m
}

func iParseArr(t *testing.T, r Result) []any {
	t.Helper()
	var arr []any
	if err := json.Unmarshal([]byte(r.Content), &arr); err != nil {
		t.Fatalf("parse JSON array: %v\ncontent: %s", err, r.Content)
	}
	return arr
}

func iInt64(v any) int64 {
	if f, ok := v.(float64); ok {
		return int64(f)
	}
	return 0
}

func iTrailingID(t *testing.T, s string) int64 {
	t.Helper()
	parts := strings.Fields(s)
	if len(parts) == 0 {
		t.Fatalf("empty string in iTrailingID")
	}
	id, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		t.Fatalf("iTrailingID(%q): %v", s, err)
	}
	return id
}

func TestIntegrationReconWorkflow(t *testing.T) {
	t.Parallel()
	ms := newStatefulMock()
	reg := NewRegistry()
	RegisterAll(reg, ms, 1, 1)

	r := iRunToolOK(t, reg, "create_engagement",
		`{"goal":"Recon example.com","description":"Full external recon"}`)
	eng := iParseObj(t, r)
	if iInt64(eng["task_id"]) <= 0 || iInt64(eng["exploration_id"]) <= 0 {
		t.Fatalf("expected positive IDs, got task=%v expl=%v", eng["task_id"], eng["exploration_id"])
	}

	r = iRunToolOK(t, reg, "insert_assets", `{"assets":[
		{"type":"root_domain","domain":"example.com"},
		{"type":"ip","ip":"93.184.216.34"}
	]}`)
	ins := iParseObj(t, r)
	results := ins["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	domainID := iInt64(results[0].(map[string]any)["id"])
	ipID := iInt64(results[1].(map[string]any)["id"])

	enrichR := iRunToolAny(t, reg, "enrich_domain", `{"domain":"example.com","timeout":5}`)
	t.Logf("enrich_domain: IsError=%v content=%s", enrichR.IsError, truncRunes(enrichR.Content, 200))

	r = iRunToolOK(t, reg, "list_assets", fmt.Sprintf(`{"ids":[%d,%d]}`, domainID, ipID))
	list := iParseObj(t, r)
	if iInt64(list["count"]) != 2 {
		t.Fatalf("expected count=2, got %v", list["count"])
	}
	assets := list["assets"].([]any)
	types := map[string]bool{}
	for _, a := range assets {
		types[a.(map[string]any)["type"].(string)] = true
	}
	if !types["root_domain"] || !types["ip"] {
		t.Fatalf("expected root_domain and ip, got %v", types)
	}
}

func TestIntegrationFindingLifecycle(t *testing.T) {
	t.Parallel()
	ms := newStatefulMock()
	reg := NewRegistry()
	RegisterAll(reg, ms, 1, 1)

	r := iRunToolOK(t, reg, "insert_assets",
		`{"assets":[{"type":"root_domain","domain":"target.com"}]}`)
	assetID := iInt64(iParseObj(t, r)["results"].([]any)[0].(map[string]any)["id"])

	r = iRunToolOK(t, reg, "add_intent",
		`{"summary":"Test SQL injection on login form"}`)
	intentID := iTrailingID(t, r.Content)

	r = iRunToolOK(t, reg, "start_intent",
		fmt.Sprintf(`{"node_id":%d}`, intentID))
	if !strings.Contains(r.Content, "is now running") {
		t.Fatalf("expected 'is now running', got %q", r.Content)
	}

	r = iRunToolOK(t, reg, "report_finding", fmt.Sprintf(
		`{"vulnclass":"sqli","severity":"high","summary":"SQL injection in login","intent_id":%d,"asset_ids":[%d]}`,
		intentID, assetID))
	findingID := iInt64(iParseObj(t, r)["finding_id"])
	if findingID <= 0 {
		t.Fatal("expected positive finding_id")
	}

	r = iRunToolOK(t, reg, "complete_intent", fmt.Sprintf(
		`{"node_id":%d,"result":"Confirmed blind SQLi"}`, intentID))
	if !strings.Contains(r.Content, "marked done") {
		t.Fatalf("expected 'marked done', got %q", r.Content)
	}

	var intentNode *Node
	for i := range ms.nodes {
		if ms.nodes[i].ID == intentID {
			intentNode = &ms.nodes[i]
			break
		}
	}
	if intentNode == nil {
		t.Fatal("intent node not found in mock")
	}
	if intentNode.State != "done" {
		t.Fatalf("intent state: expected done, got %q", intentNode.State)
	}
	if intentNode.Payload["result"] != "Confirmed blind SQLi" {
		t.Fatalf("intent result not stored: %v", intentNode.Payload)
	}

	r = iRunToolOK(t, reg, "list_findings", `{}`)
	findings := iParseArr(t, r)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0].(map[string]any)
	if f["vulnclass"] != "sqli" || f["severity"] != "high" {
		t.Fatalf("unexpected finding data: vulnclass=%v severity=%v", f["vulnclass"], f["severity"])
	}
}

func TestIntegrationFindingTriage(t *testing.T) {
	t.Parallel()
	ms := newStatefulMock()
	reg := NewRegistry()
	RegisterAll(reg, ms, 1, 1)

	var ids [3]int64
	inputs := []string{
		`{"vulnclass":"xss","severity":"high","summary":"Reflected XSS in search"}`,
		`{"vulnclass":"xss","severity":"high","summary":"Reflected XSS in search (dup)"}`,
		`{"vulnclass":"idor","severity":"medium","summary":"IDOR in user profile"}`,
	}
	for i, input := range inputs {
		r := iRunToolOK(t, reg, "report_finding", input)
		ids[i] = iInt64(iParseObj(t, r)["finding_id"])
	}

	r := iRunToolOK(t, reg, "merge_findings", fmt.Sprintf(
		`{"finding_ids":[%d,%d],"primary_id":%d}`, ids[0], ids[1], ids[0]))
	merged := iParseObj(t, r)
	if iInt64(merged["id"]) != ids[0] {
		t.Fatalf("merge primary: expected %d, got %v", ids[0], merged["id"])
	}

	r = iRunToolOK(t, reg, "dismiss_finding", fmt.Sprintf(
		`{"finding_id":%d,"reason":"false positive"}`, ids[2]))
	if !strings.Contains(r.Content, "dismissed") {
		t.Fatalf("expected dismissed, got %q", r.Content)
	}

	r = iRunToolOK(t, reg, "list_findings", `{}`)
	arr := iParseArr(t, r)
	if len(arr) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(arr))
	}
	byID := map[int64]map[string]any{}
	for _, f := range arr {
		fm := f.(map[string]any)
		byID[iInt64(fm["id"])] = fm
	}

	if byID[ids[0]]["status"] != "open" {
		t.Errorf("finding 1: expected status=open, got %v", byID[ids[0]]["status"])
	}
	if iInt64(byID[ids[1]]["merged_into"]) != ids[0] {
		t.Errorf("finding 2: expected merged_into=%d, got %v", ids[0], byID[ids[1]]["merged_into"])
	}
	if byID[ids[2]]["dismissed"] != true {
		t.Errorf("finding 3: expected dismissed=true, got %v", byID[ids[2]]["dismissed"])
	}
}

func TestIntegrationCoverageWorkflow(t *testing.T) {
	t.Parallel()
	ms := newStatefulMock()
	reg := NewRegistry()
	RegisterAll(reg, ms, 1, 1)

	r := iRunToolOK(t, reg, "insert_assets", `{"assets":[
		{"type":"root_domain","domain":"target.com"},
		{"type":"service","url":"https://target.com/api","service_name":"http"}
	]}`)
	if len(iParseObj(t, r)["results"].([]any)) != 2 {
		t.Fatal("expected 2 assets")
	}

	r = iRunToolOK(t, reg, "record_coverage",
		`{"surface":"target.com","risk_area":"sqli","outcome":"no_issue_found","evidence":"fact:1"}`)
	if r.Content != "recorded" {
		t.Fatalf("expected 'recorded', got %q", r.Content)
	}

	r = iRunToolOK(t, reg, "engagement_status", `{}`)
	status := iParseObj(t, r)
	if status["coverage"] == nil {
		t.Fatal("expected coverage in engagement status")
	}

	r = iRunToolOK(t, reg, "list_coverage_gaps", `{}`)
	gaps := iParseObj(t, r)
	if gaps["untargeted_assets"] == nil {
		t.Fatal("expected untargeted_assets in gaps")
	}
	untargeted := gaps["untargeted_assets"].([]any)
	if len(untargeted) != 1 {
		t.Fatalf("expected 1 untargeted asset, got %d", len(untargeted))
	}
	if untargeted[0].(map[string]any)["type"] != "service" {
		t.Fatalf("expected untargeted type=service, got %v", untargeted[0].(map[string]any)["type"])
	}
}

func TestIntegrationExportImportRoundTrip(t *testing.T) {
	t.Parallel()
	ms := newStatefulMock()
	reg := NewRegistry()
	RegisterAll(reg, ms, 1, 1)

	iRunToolOK(t, reg, "create_engagement",
		`{"goal":"Export test","description":"Integration test"}`)

	r := iRunToolOK(t, reg, "insert_assets",
		`{"assets":[{"type":"root_domain","domain":"export-test.com"}]}`)
	if len(iParseObj(t, r)["results"].([]any)) != 1 {
		t.Fatal("expected 1 asset")
	}

	r = iRunToolOK(t, reg, "report_finding",
		`{"vulnclass":"xss","severity":"medium","summary":"Stored XSS in comments"}`)
	if iInt64(iParseObj(t, r)["finding_id"]) <= 0 {
		t.Fatal("expected positive finding_id")
	}

	exportR := iRunToolOK(t, reg, "export_engagement", `{}`)
	exportData := iParseObj(t, exportR)

	exportAssets := exportData["assets"].([]any)
	if len(exportAssets) != 1 {
		t.Fatalf("expected 1 asset in export, got %d", len(exportAssets))
	}
	if exportAssets[0].(map[string]any)["type"] != "root_domain" {
		t.Fatalf("exported asset type: expected root_domain, got %v", exportAssets[0].(map[string]any)["type"])
	}

	exportFindings := exportData["findings"].([]any)
	if len(exportFindings) != 1 {
		t.Fatalf("expected 1 finding in export, got %d", len(exportFindings))
	}
	if exportFindings[0].(map[string]any)["severity"] != "medium" {
		t.Fatalf("exported finding severity: expected medium, got %v", exportFindings[0].(map[string]any)["severity"])
	}

	importInput, err := json.Marshal(map[string]string{"data": exportR.Content})
	if err != nil {
		t.Fatalf("marshal import input: %v", err)
	}
	r = iRunToolOK(t, reg, "import_engagement", string(importInput))
	imp := iParseObj(t, r)
	if iInt64(imp["task_id"]) <= 0 || iInt64(imp["exploration_id"]) <= 0 {
		t.Fatalf("expected positive import IDs, got task=%v expl=%v", imp["task_id"], imp["exploration_id"])
	}
	if imp["message"] != "engagement imported successfully" {
		t.Fatalf("unexpected import message: %v", imp["message"])
	}
}
