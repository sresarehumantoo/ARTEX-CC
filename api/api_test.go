package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type mockStore struct {
	graphOverview     *tools.Overview
	graphOverviewErr  error
	rootDesc          string
	rootGoal          string
	rootErr           error
	listNodes         []tools.Node
	listNodesErr      error
	edges             []tools.Edge
	edgesErr          error
	listFindings      []tools.Finding
	listFindingsErr   error
	listFindingTraff  []tools.TrafficBinding
	listAssets        []tools.Asset
	listAssetsTotal   int
	listAssetsErr     error
	listActivity      []tools.Activity
	listActivityErr   error
	listCoverageGaps  *tools.CoverageGaps
	listCoverageErr   error
	listCredentials   []tools.Credential
	listCredErr       error
	listConstraints   []tools.Constraint
	listConstraintErr error
	dismissErr        error
	addNodeID         int64
	addNodeErr        error
	addConstraintID   int64
	addConstraintErr  error
}

func (m *mockStore) InsertAsset(context.Context, int64, tools.AssetInput) (int64, error) {
	return 0, nil
}
func (m *mockStore) ListAssets(_ context.Context, _ tools.AssetFilter) ([]tools.Asset, int, error) {
	return m.listAssets, m.listAssetsTotal, m.listAssetsErr
}
func (m *mockStore) ListUntestedAssets(context.Context, int64, string, int, int) ([]tools.Asset, int, error) {
	return nil, 0, nil
}
func (m *mockStore) ListCompanies(context.Context) ([]tools.Company, error) { return nil, nil }
func (m *mockStore) GetOrCreateCompany(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (m *mockStore) AddCompanyScope(context.Context, int64, []string, string) (int, int, int, []string, error) {
	return 0, 0, 0, nil, nil
}
func (m *mockStore) AddFinding(context.Context, tools.FindingInput) (int64, int64, error) {
	return 0, 0, nil
}
func (m *mockStore) ListFindings(_ context.Context, _ int64) ([]tools.Finding, error) {
	return m.listFindings, m.listFindingsErr
}
func (m *mockStore) FindingExists(context.Context, int64) (bool, error) { return false, nil }
func (m *mockStore) FindPotentialDuplicates(context.Context, int64, string, string, []int64) ([]tools.Finding, error) {
	return nil, nil
}
func (m *mockStore) MergeFindings(context.Context, int64, []int64) (*tools.MergedFinding, error) {
	return nil, nil
}
func (m *mockStore) DismissFinding(_ context.Context, _ int64, _ string) error {
	return m.dismissErr
}
func (m *mockStore) GraphOverview(_ context.Context, _ int64) (*tools.Overview, error) {
	return m.graphOverview, m.graphOverviewErr
}
func (m *mockStore) AddNode(_ context.Context, _ int64, _ string, _ map[string]any, _ int, _ string, _ string, _ []int64) (int64, error) {
	return m.addNodeID, m.addNodeErr
}
func (m *mockStore) UpdateNodeState(context.Context, int64, string) error { return nil }
func (m *mockStore) ClaimIntent(context.Context, int64, string, time.Duration) (*tools.IntentClaim, error) {
	return nil, nil
}
func (m *mockStore) RenewIntentClaims(context.Context, []int64, string, time.Duration) error {
	return nil
}
func (m *mockStore) ListChanges(context.Context, int64, int64, tools.ChangeCursor, int) ([]tools.Change, error) {
	return nil, nil
}
func (m *mockStore) ListClaims(context.Context, int64, string) ([]tools.HeldClaim, error) {
	return nil, nil
}
func (m *mockStore) RecordTargetChange(context.Context, int64, tools.TargetChangeInput) (*tools.TargetChangeRecord, error) {
	return nil, nil
}
func (m *mockStore) RecordTargetRestore(context.Context, int64, []int64, string, string) ([]tools.TargetRestoreOutcome, error) {
	return nil, nil
}
func (m *mockStore) ListTargetChanges(context.Context, int64, bool) ([]tools.TargetChange, error) {
	return nil, nil
}
func (m *mockStore) ListNodes(_ context.Context, _ int64, _ string, _ int) ([]tools.Node, error) {
	return m.listNodes, m.listNodesErr
}
func (m *mockStore) NodeDetail(context.Context, int64) (*tools.Node, error) { return nil, nil }
func (m *mockStore) AddEdge(context.Context, int64, string, int64) error    { return nil }
func (m *mockStore) ProveGoal(context.Context, int64, int64) error          { return nil }
func (m *mockStore) Frontier(context.Context, int64, int) ([]tools.Node, error) {
	return nil, nil
}
func (m *mockStore) CountFinishedIntents(context.Context, int64) (int, error) { return 0, nil }
func (m *mockStore) CountOpenIntents(context.Context, int64) (int, error)     { return 0, nil }
func (m *mockStore) OriginFactID(context.Context, int64) (int64, error)       { return 0, nil }
func (m *mockStore) AddGoals(context.Context, int64, []tools.GoalInput, bool) (*tools.GoalsResult, error) {
	return &tools.GoalsResult{}, nil
}
func (m *mockStore) AddIntents(context.Context, int64, []tools.IntentInput) (*tools.IntentsResult, error) {
	return &tools.IntentsResult{}, nil
}
func (m *mockStore) Edges(_ context.Context, _ int64, _ int) ([]tools.Edge, error) {
	return m.edges, m.edgesErr
}
func (m *mockStore) Root(_ context.Context, _ int64) (string, string, error) {
	return m.rootDesc, m.rootGoal, m.rootErr
}
func (m *mockStore) AddConstraint(_ context.Context, _ int64, _ string, _ string, _ string) (int64, error) {
	return m.addConstraintID, m.addConstraintErr
}
func (m *mockStore) ListConstraints(_ context.Context, _ int64) ([]tools.Constraint, error) {
	return m.listConstraints, m.listConstraintErr
}
func (m *mockStore) AddTaskScope(context.Context, int64, string, string, string, string) error {
	return nil
}
func (m *mockStore) ListTaskScope(context.Context, int64) ([]int64, error) { return nil, nil }
func (m *mockStore) ListScopeRules(context.Context, int64) ([]tools.ScopeRule, error) {
	return nil, nil
}
func (m *mockStore) IsInScope(context.Context, int64, string) (bool, string, string, error) {
	return false, "", "", nil
}
func (m *mockStore) SaveCredential(context.Context, int64, tools.CredentialInput) (int64, error) {
	return 0, nil
}
func (m *mockStore) ListCredentials(_ context.Context, _ int64) ([]tools.Credential, error) {
	return m.listCredentials, m.listCredErr
}
func (m *mockStore) RecordCoverage(context.Context, int64, tools.CoverageEntry) error { return nil }
func (m *mockStore) ListCoverageGaps(_ context.Context, _ int64, _ int64) (*tools.CoverageGaps, error) {
	return m.listCoverageGaps, m.listCoverageErr
}
func (m *mockStore) CreateEngagement(context.Context, string, string) (int64, int64, error) {
	return 0, 0, nil
}
func (m *mockStore) ListEngagements(context.Context) ([]tools.EngagementSummary, error) {
	return nil, nil
}
func (m *mockStore) GetEngagement(context.Context, int64) (*tools.EngagementDetail, error) {
	return nil, nil
}
func (m *mockStore) BindFindingTraffic(context.Context, int64, string, string, string, int) error {
	return nil
}
func (m *mockStore) ListFindingTraffic(_ context.Context, _ int64) ([]tools.TrafficBinding, error) {
	return m.listFindingTraff, nil
}
func (m *mockStore) FindingTrafficCount(context.Context, int64) (int, error) { return 0, nil }
func (m *mockStore) CompleteEngagement(context.Context, int64, string) error { return nil }
func (m *mockStore) UpdateEngagement(context.Context, int64, *string, *string) error {
	return nil
}
func (m *mockStore) DeleteEngagement(context.Context, int64) error { return nil }
func (m *mockStore) EngagementActivity(context.Context, int64, int) ([]tools.TimelineEntry, error) {
	return nil, nil
}
func (m *mockStore) LogActivity(context.Context, int64, tools.ActivityEntry) (int64, error) {
	return 0, nil
}
func (m *mockStore) ListActivity(_ context.Context, _ int64, _ tools.ActivityFilter) ([]tools.Activity, error) {
	return m.listActivity, m.listActivityErr
}
func (m *mockStore) ListFindingsForReport(context.Context, int64) ([]tools.ReportFinding, error) {
	return nil, nil
}
func (m *mockStore) UpdateNodeFields(context.Context, int64, string, map[string]any) error {
	return nil
}
func (m *mockStore) ExportEngagementData(context.Context, int64) (*tools.EngagementExport, error) {
	return nil, nil
}
func (m *mockStore) ImportEngagementData(context.Context, *tools.EngagementExport) (int64, int64, error) {
	return 0, 0, nil
}

type mockCoverageStore struct {
	mockStore
	stats    *store.CoverageStats
	statsErr error
}

func (m *mockCoverageStore) CoverageStats(_ int64) (*store.CoverageStats, error) {
	return m.stats, m.statsErr
}

func newTestAPI(s tools.Store) *API {
	return New(s, nil, 1, 10)
}

func decodeRecorder(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return m
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"ok": "true"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: want 201, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: want application/json, got %q", ct)
	}
	var m map[string]string
	json.NewDecoder(rec.Body).Decode(&m)
	if m["ok"] != "true" {
		t.Fatalf("body: want ok=true, got %v", m)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "bad input")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
	var m map[string]string
	json.NewDecoder(rec.Body).Decode(&m)
	if m["error"] != "bad input" {
		t.Fatalf("body: want error=bad input, got %v", m)
	}
}

func TestParseIntParam(t *testing.T) {
	tests := []struct {
		query string
		name  string
		def   int
		want  int
	}{
		{"", "limit", 50, 50},
		{"limit=10", "limit", 50, 10},
		{"limit=abc", "limit", 50, 50},
		{"offset=0", "offset", 10, 0},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/test?"+tc.query, nil)
		got := parseIntParam(req, tc.name, tc.def)
		if got != tc.want {
			t.Errorf("parseIntParam(%q, %q, %d) = %d, want %d", tc.query, tc.name, tc.def, got, tc.want)
		}
	}
}

func TestParseID(t *testing.T) {
	mux := http.NewServeMux()
	var gotID int64
	var gotOK bool
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotID, gotOK = parseID(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !gotOK || gotID != 42 {
		t.Fatalf("parseID(/items/42): got (%d, %v), want (42, true)", gotID, gotOK)
	}

	req = httptest.NewRequest(http.MethodGet, "/items/notanumber", nil)
	mux.ServeHTTP(rec, req)
	if gotOK {
		t.Fatalf("parseID(/items/notanumber): want ok=false")
	}
}

func TestHandleEngagement_OK(t *testing.T) {
	ms := &mockStore{
		graphOverview: &tools.Overview{FindingsTotal: 3, FactsTotal: 7},
		rootDesc:      "pentest acme",
		rootGoal:      "find vulns",
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/engagement", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	if body["description"] != "pentest acme" {
		t.Errorf("description: want 'pentest acme', got %v", body["description"])
	}
	if body["goal"] != "find vulns" {
		t.Errorf("goal: want 'find vulns', got %v", body["goal"])
	}
	if body["task_id"] != float64(1) {
		t.Errorf("task_id: want 1, got %v", body["task_id"])
	}
	if body["exploration_id"] != float64(10) {
		t.Errorf("exploration_id: want 10, got %v", body["exploration_id"])
	}
}

func TestHandleEngagement_Error(t *testing.T) {
	a := newTestAPI(&mockStore{graphOverviewErr: errors.New("db down")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/engagement", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleGraph_OK(t *testing.T) {
	ms := &mockStore{
		listNodes: []tools.Node{
			{ID: 1, Kind: "goal", State: "open", Payload: map[string]any{"label": "g1"}},
			{ID: 2, Kind: "fact", Payload: map[string]any{"label": "f1"}},
		},
		edges: []tools.Edge{{From: 1, Rel: "yields", To: 2}},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/graph", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	nodes, ok := body["nodes"].([]any)
	if !ok || len(nodes) != 2 {
		t.Fatalf("nodes: want 2 items, got %v", body["nodes"])
	}
	edges, ok := body["edges"].([]any)
	if !ok || len(edges) != 1 {
		t.Fatalf("edges: want 1 item, got %v", body["edges"])
	}
}

func TestHandleGraph_EmptyListsAreArrays(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/graph", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"edges":[],"nodes":[]}` {
		t.Fatalf("body: want empty arrays, got %s", got)
	}
}

func TestHandleGraph_NodesError(t *testing.T) {
	a := newTestAPI(&mockStore{listNodesErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/graph", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleGraph_EdgesError(t *testing.T) {
	a := newTestAPI(&mockStore{edgesErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/graph", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleFindings_OK(t *testing.T) {
	ms := &mockStore{
		listFindings: []tools.Finding{
			{ID: 1, VulnClass: "xss", Severity: "high", Summary: "reflected xss"},
			{ID: 2, VulnClass: "sqli", Severity: "critical", Summary: "sql injection"},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/findings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var findings []tools.Finding
	json.NewDecoder(rec.Body).Decode(&findings)
	if len(findings) != 2 {
		t.Fatalf("findings: want 2, got %d", len(findings))
	}
}

func TestHandleFindings_Error(t *testing.T) {
	a := newTestAPI(&mockStore{listFindingsErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/findings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleFindingDetail_OK(t *testing.T) {
	ms := &mockStore{
		listFindings: []tools.Finding{
			{ID: 5, VulnClass: "xss", Severity: "high", Summary: "stored xss"},
		},
		listFindingTraff: []tools.TrafficBinding{
			{ID: 1, FindingID: 5, SnapshotID: "snap-1", Role: "poc"},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/findings/5", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	if body["vulnclass"] != "xss" {
		t.Errorf("vulnclass: want xss, got %v", body["vulnclass"])
	}
	bindings, ok := body["traffic_bindings"].([]any)
	if !ok || len(bindings) != 1 {
		t.Errorf("traffic_bindings: want 1, got %v", body["traffic_bindings"])
	}
}

func TestHandleFindingDetail_NotFound(t *testing.T) {
	a := newTestAPI(&mockStore{listFindings: []tools.Finding{}})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/findings/999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", rec.Code)
	}
}

func TestHandleFindingDetail_BadID(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/findings/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleAssets_OK(t *testing.T) {
	ms := &mockStore{
		listAssets:      []tools.Asset{{ID: 1, Type: "domain", Label: "example.com"}},
		listAssetsTotal: 1,
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/assets?type=domain&limit=10", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	assets, ok := body["assets"].([]any)
	if !ok || len(assets) != 1 {
		t.Fatalf("assets: want 1, got %v", body["assets"])
	}
	if body["total"] != float64(1) {
		t.Errorf("total: want 1, got %v", body["total"])
	}
}

func TestHandleAssets_Error(t *testing.T) {
	a := newTestAPI(&mockStore{listAssetsErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/assets", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleAssetDetail_OK(t *testing.T) {
	ms := &mockStore{
		listAssets:      []tools.Asset{{ID: 7, Type: "ip", Label: "10.0.0.1"}},
		listAssetsTotal: 1,
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/7", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestHandleAssetDetail_NotFound(t *testing.T) {
	a := newTestAPI(&mockStore{listAssets: []tools.Asset{}})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", rec.Code)
	}
}

func TestHandleAssetDetail_BadID(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleAssetFindings_NoDBQuerier(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/1/findings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var arr []any
	json.NewDecoder(rec.Body).Decode(&arr)
	if len(arr) != 0 {
		t.Fatalf("want empty array, got %v", arr)
	}
}

func TestHandleAssetFindings_BadID(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/xyz/findings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleTraffic_NilTraffic(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/traffic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	if body["total"] != float64(0) {
		t.Errorf("total: want 0, got %v", body["total"])
	}
}

func TestHandleTrafficDetail_NilTraffic(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/traffic/abc-123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", rec.Code)
	}
}

func TestHandleActivity_OK(t *testing.T) {
	ms := &mockStore{
		listActivity: []tools.Activity{
			{ID: 1, ExplorationID: 10, Kind: "scan", Summary: "nmap done"},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/activity?limit=10&kind=scan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestHandleActivity_Error(t *testing.T) {
	a := newTestAPI(&mockStore{listActivityErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/activity", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleActivity_NodeIDFilter(t *testing.T) {
	a := newTestAPI(&mockStore{listActivity: []tools.Activity{}})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/activity?node_id=5", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestHandleSearch_EmptyQuery(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	results, ok := body["results"].([]any)
	if !ok || len(results) != 0 {
		t.Fatalf("want empty results, got %v", body["results"])
	}
}

func TestHandleSearch_MatchesFindings(t *testing.T) {
	ms := &mockStore{
		listFindings: []tools.Finding{
			{ID: 1, VulnClass: "XSS", Severity: "high", Summary: "reflected cross-site scripting"},
			{ID: 2, VulnClass: "SQLI", Severity: "critical", Summary: "sql injection"},
		},
		listAssets: []tools.Asset{},
		listNodes:  []tools.Node{},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=xss", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	results := body["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("want 1 result for 'xss', got %d", len(results))
	}
	r0 := results[0].(map[string]any)
	if r0["type"] != "finding" {
		t.Errorf("type: want finding, got %v", r0["type"])
	}
}

func TestHandleSearch_MatchesNodes(t *testing.T) {
	ms := &mockStore{
		listFindings: []tools.Finding{},
		listAssets:   []tools.Asset{},
		listNodes: []tools.Node{
			{ID: 10, Kind: "goal", State: "open", Payload: map[string]any{"label": "find admin panel", "summary": "locate admin"}},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=admin", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := decodeRecorder(t, rec)
	results := body["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("want 1 result for 'admin', got %d", len(results))
	}
	if results[0].(map[string]any)["type"] != "node" {
		t.Errorf("type: want node, got %v", results[0].(map[string]any)["type"])
	}
}

func TestHandleCoverage_NoCoverageStatter(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/coverage", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	entries := body["entries"].([]any)
	if len(entries) != 0 {
		t.Fatalf("want empty entries, got %v", entries)
	}
}

func TestHandleCoverage_WithStats(t *testing.T) {
	ms := &mockCoverageStore{
		stats: &store.CoverageStats{
			Entries: []store.CoverageEntry{
				{Surface: "web", RiskArea: "xss", Outcome: "reported"},
			},
			Gaps:       []store.CoverageGap{},
			Unexamined: []string{},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/coverage", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	entries := body["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
}

func TestHandleCoverage_Error(t *testing.T) {
	a := newTestAPI(&mockCoverageStore{statsErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/coverage", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleCoverageGaps_OK(t *testing.T) {
	ms := &mockStore{
		listCoverageGaps: &tools.CoverageGaps{
			UntargetedAssets:  []tools.Asset{},
			UnresolvedIntents: []tools.Node{},
			UnprovenGoals:     []tools.Node{},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/coverage/gaps", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestHandleCoverageGaps_Error(t *testing.T) {
	a := newTestAPI(&mockStore{listCoverageErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/coverage/gaps", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleCredentials_OK(t *testing.T) {
	ms := &mockStore{
		listCredentials: []tools.Credential{{ID: 1, Domain: "example.com", Username: "admin"}},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/credentials", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestHandleCredentials_Error(t *testing.T) {
	a := newTestAPI(&mockStore{listCredErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/credentials", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleConstraints_OK(t *testing.T) {
	ms := &mockStore{
		listConstraints: []tools.Constraint{{ID: 1, Kind: "scope", Text: "do not attack prod"}},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/constraints", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestHandleConstraints_Error(t *testing.T) {
	a := newTestAPI(&mockStore{listConstraintErr: errors.New("fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/constraints", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestBroadcast(t *testing.T) {
	a := newTestAPI(&mockStore{})

	ch, _ := a.hub.Subscribe(globalTopic)

	a.Broadcast("test_event", map[string]string{"key": "val"})

	select {
	case msg := <-ch:
		var payload map[string]any
		json.Unmarshal(msg, &payload)
		if payload["type"] != "test_event" {
			t.Errorf("type: want test_event, got %v", payload["type"])
		}
	case <-time.After(time.Second):
		t.Fatal("no message received")
	}
}

func TestBroadcast_NoSubscribers(t *testing.T) {
	a := newTestAPI(&mockStore{})
	a.Broadcast("test_event", "data")
}

func TestBroadcast_FullChannel(t *testing.T) {
	a := newTestAPI(&mockStore{})

	a.hub.Subscribe(globalTopic)
	for i := 0; i < subscriberBuffer; i++ {
		a.Broadcast("fill", nil)
	}

	a.Broadcast("overflow", nil)
}

func TestHandleSSE_ReceivesEvents(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.ServeHTTP(rec, req)
	}()

	deadline := time.After(2 * time.Second)
	for {
		a.hub.mu.RLock()
		n := len(a.hub.topics[globalTopic])
		a.hub.mu.RUnlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("subscriber not registered in time")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	a.Broadcast("hello", map[string]string{"msg": "world"})
	time.Sleep(50 * time.Millisecond)
	cancel()
	wg.Wait()

	body := rec.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatalf("want SSE data frame, got %q", body)
	}
	if !strings.Contains(body, "hello") {
		t.Fatalf("want 'hello' in body, got %q", body)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string: want hello, got %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("long string: want hello..., got %q", got)
	}
}

func TestNew(t *testing.T) {
	a := New(&mockStore{}, nil, 42, 99)
	if a.taskID != 42 {
		t.Errorf("taskID: want 42, got %d", a.taskID)
	}
	if a.explorationID != 99 {
		t.Errorf("explorationID: want 99, got %d", a.explorationID)
	}
	if a.hub == nil {
		t.Fatal("hub not initialized")
	}
	if a.claudeBin != "claude" {
		t.Errorf("claudeBin: want claude, got %q", a.claudeBin)
	}
	b := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()), WithClaudeBin("/x/claude"))
	if b.files == nil || b.claudeBin != "/x/claude" {
		t.Error("options not applied")
	}
}

func TestHandler_AllRoutes(t *testing.T) {
	ms := &mockStore{
		graphOverview:    &tools.Overview{},
		listNodes:        []tools.Node{},
		edges:            []tools.Edge{},
		listFindings:     []tools.Finding{},
		listAssets:       []tools.Asset{},
		listActivity:     []tools.Activity{},
		listCoverageGaps: &tools.CoverageGaps{},
		listCredentials:  []tools.Credential{},
		listConstraints:  []tools.Constraint{},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	routes := []struct {
		path string
		code int
	}{
		{"/api/engagement", 200},
		{"/api/graph", 200},
		{"/api/findings", 200},
		{"/api/assets", 200},
		{"/api/activity", 200},
		{"/api/coverage", 200},
		{"/api/coverage/gaps", 200},
		{"/api/credentials", 200},
		{"/api/constraints", 200},
		{"/api/search", 200},
		{"/api/traffic", 200},
	}
	for _, rt := range routes {
		req := httptest.NewRequest(http.MethodGet, rt.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != rt.code {
			t.Errorf("%s: want %d, got %d", rt.path, rt.code, rec.Code)
		}
	}
}

func TestHandleDismissFinding_OK(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	body := strings.NewReader(`{"reason":"duplicate"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/findings/5/dismiss", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	resp := decodeRecorder(t, rec)
	if resp["status"] != "ok" {
		t.Errorf("status: want ok, got %v", resp["status"])
	}
}

func TestHandleDismissFinding_BadID(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	body := strings.NewReader(`{"reason":"dup"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/findings/abc/dismiss", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleDismissFinding_BadBody(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/findings/5/dismiss", strings.NewReader(`{invalid`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleDismissFinding_StoreError(t *testing.T) {
	a := newTestAPI(&mockStore{dismissErr: errors.New("db down")})
	h := a.Handler()

	body := strings.NewReader(`{"reason":"dup"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/findings/5/dismiss", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleAddHint_OK(t *testing.T) {
	a := newTestAPI(&mockStore{addNodeID: 42})
	h := a.Handler()

	body := strings.NewReader(`{"text":"check /admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/hints", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	resp := decodeRecorder(t, rec)
	if resp["id"] != float64(42) {
		t.Errorf("id: want 42, got %v", resp["id"])
	}
}

func TestHandleAddHint_EmptyText(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	body := strings.NewReader(`{"text":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/hints", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleAddHint_BadBody(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/hints", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleAddHint_StoreError(t *testing.T) {
	a := newTestAPI(&mockStore{addNodeErr: errors.New("db down")})
	h := a.Handler()

	body := strings.NewReader(`{"text":"some hint"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/hints", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleAddConstraint_OK(t *testing.T) {
	a := newTestAPI(&mockStore{addConstraintID: 7})
	h := a.Handler()

	body := strings.NewReader(`{"kind":"scope","text":"no prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/constraints", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	resp := decodeRecorder(t, rec)
	if resp["id"] != float64(7) {
		t.Errorf("id: want 7, got %v", resp["id"])
	}
}

func TestHandleAddConstraint_MissingKind(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	body := strings.NewReader(`{"text":"no prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/constraints", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleAddConstraint_MissingText(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	body := strings.NewReader(`{"kind":"scope"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/constraints", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleAddConstraint_BadBody(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/constraints", strings.NewReader(`{nope`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
}

func TestHandleAddConstraint_StoreError(t *testing.T) {
	a := newTestAPI(&mockStore{addConstraintErr: errors.New("db down")})
	h := a.Handler()

	body := strings.NewReader(`{"kind":"scope","text":"no prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/constraints", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleSearch_MatchesAssets(t *testing.T) {
	ms := &mockStore{
		listFindings: []tools.Finding{},
		listAssets:   []tools.Asset{{ID: 1, Type: "domain", Label: "example.com"}},
		listNodes:    []tools.Node{},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=example", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := decodeRecorder(t, rec)
	results := body["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	r0 := results[0].(map[string]any)
	if r0["type"] != "asset" {
		t.Errorf("type: want asset, got %v", r0["type"])
	}
}

func TestHandleSearch_FiveFindingLimit(t *testing.T) {
	findings := make([]tools.Finding, 10)
	for i := range findings {
		findings[i] = tools.Finding{ID: int64(i + 1), VulnClass: "xss", Severity: "high", Summary: "xss variant"}
	}
	ms := &mockStore{
		listFindings: findings,
		listAssets:   []tools.Asset{},
		listNodes:    []tools.Node{},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=xss", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := decodeRecorder(t, rec)
	results := body["results"].([]any)
	findingCount := 0
	for _, r := range results {
		if r.(map[string]any)["type"] == "finding" {
			findingCount++
		}
	}
	if findingCount != 5 {
		t.Errorf("finding results: want 5, got %d", findingCount)
	}
}

func TestHandleSearch_FiveNodeLimit(t *testing.T) {
	nodes := make([]tools.Node, 10)
	for i := range nodes {
		nodes[i] = tools.Node{
			ID:      int64(i + 1),
			Kind:    "goal",
			State:   "open",
			Payload: map[string]any{"label": "admin related goal", "summary": "admin access"},
		}
	}
	ms := &mockStore{
		listFindings: []tools.Finding{},
		listAssets:   []tools.Asset{},
		listNodes:    nodes,
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=admin", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := decodeRecorder(t, rec)
	results := body["results"].([]any)
	nodeCount := 0
	for _, r := range results {
		if r.(map[string]any)["type"] == "node" {
			nodeCount++
		}
	}
	if nodeCount != 5 {
		t.Errorf("node results: want 5, got %d", nodeCount)
	}
}

func TestHandleSearch_NodeStateInSubtitle(t *testing.T) {
	ms := &mockStore{
		listFindings: []tools.Finding{},
		listAssets:   []tools.Asset{},
		listNodes: []tools.Node{
			{ID: 1, Kind: "intent", State: "done", Payload: map[string]any{"label": "scan ports"}},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=scan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := decodeRecorder(t, rec)
	results := body["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	subtitle := results[0].(map[string]any)["subtitle"].(string)
	if subtitle != "intent — done" {
		t.Errorf("subtitle: want %q, got %q", "intent — done", subtitle)
	}
}

func TestHandleSearch_FindingsError(t *testing.T) {
	ms := &mockStore{
		listFindingsErr: errors.New("db fail"),
		listAssets:      []tools.Asset{},
		listNodes:       []tools.Node{},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (search should be best-effort)", rec.Code)
	}
}

func TestHandleFindingDetail_ListError(t *testing.T) {
	a := newTestAPI(&mockStore{listFindingsErr: errors.New("db fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/findings/1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}

func TestHandleFindings_WithTimestamps(t *testing.T) {
	ms := &mockStore{
		listFindings: []tools.Finding{
			{ID: 1, VulnClass: "xss", Severity: "high", Summary: "stored xss"},
		},
	}
	a := newTestAPI(ms)
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/findings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestHandleAssetDetail_StoreError(t *testing.T) {
	a := newTestAPI(&mockStore{listAssetsErr: errors.New("db fail")})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
}
