package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

var (
	sharedStore     *Store
	sharedStoreOnce sync.Once
	sharedStoreErr  error
)

func testDB(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("ARTEX_TEST_PG")
	if dsn == "" {
		t.Skip("ARTEX_TEST_PG not set")
	}
	sharedStoreOnce.Do(func() {
		sharedStore, sharedStoreErr = Open(context.Background(), dsn)
	})
	if sharedStoreErr != nil {
		t.Fatal(sharedStoreErr)
	}
	return sharedStore
}

func testEngagement(t *testing.T, s *Store, label string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	taskID, expID, err := s.CreateEngagement(ctx, label+" goal", label+" description")
	if err != nil {
		t.Fatalf("CreateEngagement(%s): %v", label, err)
	}
	return taskID, expID
}

func TestCreateEngagement(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()

	taskID, expID, err := s.CreateEngagement(ctx, "test-create goal", "test-create desc")
	if err != nil {
		t.Fatalf("CreateEngagement: %v", err)
	}
	if taskID <= 0 {
		t.Errorf("taskID = %d, want > 0", taskID)
	}
	if expID <= 0 {
		t.Errorf("explorationID = %d, want > 0", expID)
	}
}

func TestInsertAndListAssets(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "insert-list-assets")

	assets := []tools.AssetInput{
		{Type: "root_domain", Domain: fmt.Sprintf("assets-test-%d.com", taskID)},
		{Type: "ip", IP: "198.51.100.1"},
		{Type: "service", URL: fmt.Sprintf("https://assets-test-%d.com/app", taskID)},
	}
	for i, a := range assets {
		id, err := s.InsertAsset(ctx, taskID, a)
		if err != nil {
			t.Fatalf("InsertAsset[%d]: %v", i, err)
		}
		if id <= 0 {
			t.Fatalf("InsertAsset[%d] id = %d, want > 0", i, id)
		}
	}

	list, total, err := s.ListAssets(ctx, tools.AssetFilter{TaskID: taskID, Limit: 50})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if total < 3 {
		t.Errorf("total = %d, want >= 3", total)
	}
	if len(list) < 3 {
		t.Errorf("len(list) = %d, want >= 3", len(list))
	}
}

func TestListAssetsDSL(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "dsl-assets")

	_, err := s.InsertAsset(ctx, taskID, tools.AssetInput{
		Type:   "service",
		URL:    "https://test.example.com/api",
		Domain: "test.example.com",
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}

	list, _, err := s.ListAssets(ctx, tools.AssetFilter{
		DSL:   "type=service domain=example",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListAssets DSL: %v", err)
	}
	found := false
	for _, a := range list {
		if a.Type == "service" {
			if u, ok := a.Data["url"].(string); ok && u == "https://test.example.com/api" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("DSL query did not return the inserted service; got %d results", len(list))
	}
}

func TestCompanyScope(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()

	name := fmt.Sprintf("TestCo-%d", os.Getpid())
	companyID, err := s.GetOrCreateCompany(ctx, name, "")
	if err != nil {
		t.Fatalf("GetOrCreateCompany: %v", err)
	}
	if companyID <= 0 {
		t.Fatalf("companyID = %d, want > 0", companyID)
	}

	added, _, _, _, err := s.AddCompanyScope(ctx, companyID, []string{"scope-test.example.com"}, "test")
	if err != nil {
		t.Fatalf("AddCompanyScope: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}

	companies, err := s.ListCompanies(ctx)
	if err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	var found *tools.Company
	for i := range companies {
		if companies[i].ID == companyID {
			found = &companies[i]
			break
		}
	}
	if found == nil {
		t.Fatal("company not found in ListCompanies")
	}
	hasScopeLine := false
	for _, s := range found.Scope {
		if s == "scope-test.example.com" {
			hasScopeLine = true
		}
	}
	if !hasScopeLine {
		t.Errorf("scope = %v, want to contain scope-test.example.com", found.Scope)
	}
}

func TestExplorationGraph(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "exploration-graph")

	goalID, err := s.AddNode(ctx, expID, KindGoal, map[string]any{"text": "find xss"}, 0, "open", "system", nil)
	if err != nil {
		t.Fatalf("AddNode goal: %v", err)
	}

	intentID, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "test login form"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode intent: %v", err)
	}

	factID, err := s.AddNode(ctx, expID, KindFact, map[string]any{"summary": "login form accepts <script>"}, 5, "confirmed", "worker", nil)
	if err != nil {
		t.Fatalf("AddNode fact: %v", err)
	}

	if err := s.AddEdge(ctx, goalID, RelSpawns, intentID); err != nil {
		t.Fatalf("AddEdge goal->intent: %v", err)
	}
	if err := s.AddEdge(ctx, intentID, RelYields, factID); err != nil {
		t.Fatalf("AddEdge intent->fact: %v", err)
	}

	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	if len(ov.Goals) < 1 {
		t.Errorf("Goals = %d, want >= 1", len(ov.Goals))
	}
	if len(ov.OpenIntents) < 1 {
		t.Errorf("OpenIntents = %d, want >= 1", len(ov.OpenIntents))
	}
	// FactsTotal includes the origin fact seeded by CreateEngagement plus the one we added.
	if ov.FactsTotal < 2 {
		t.Errorf("FactsTotal = %d, want >= 2", ov.FactsTotal)
	}
}

func TestFindingLifecycle(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "finding-lifecycle")

	findingID, err := s.AddFinding(ctx, tools.FindingInput{
		TaskID:        taskID,
		ExplorationID: expID,
		VulnClass:     "xss",
		Name:          "Reflected XSS in search",
		Severity:      SeverityHigh,
		Summary:       "The search parameter reflects unescaped input.",
		Evidence:      "<script>alert(1)</script>",
	})
	if err != nil {
		t.Fatalf("AddFinding: %v", err)
	}
	if findingID <= 0 {
		t.Fatalf("findingID = %d, want > 0", findingID)
	}

	findings, err := s.ListFindings(ctx, expID)
	if err != nil {
		t.Fatalf("ListFindings: %v", err)
	}
	if len(findings) < 1 {
		t.Fatal("ListFindings returned 0 findings")
	}
	f := findings[0]
	if f.VulnClass != "xss" {
		t.Errorf("VulnClass = %q, want %q", f.VulnClass, "xss")
	}
	if f.Severity != SeverityHigh {
		t.Errorf("Severity = %q, want %q", f.Severity, SeverityHigh)
	}
	if f.Summary != "The search parameter reflects unescaped input." {
		t.Errorf("Summary = %q", f.Summary)
	}
	if f.Status != FindingStatusPending {
		t.Errorf("Status = %q, want %q", f.Status, FindingStatusPending)
	}
}

func TestCredentials(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "credentials")

	id, err := s.SaveCredential(ctx, taskID, tools.CredentialInput{
		Domain:   "app.example.com",
		Username: "testuser",
		Password: "hunter2",
		Source:   "agent",
		Note:     "registered during recon",
	})
	if err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	if id <= 0 {
		t.Fatalf("credential id = %d, want > 0", id)
	}

	creds, err := s.ListCredentials(ctx, taskID)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	if len(creds) < 1 {
		t.Fatal("ListCredentials returned 0 credentials")
	}
	c := creds[0]
	if c.Domain != "app.example.com" {
		t.Errorf("Domain = %q, want %q", c.Domain, "app.example.com")
	}
	if c.Username != "testuser" {
		t.Errorf("Username = %q, want %q", c.Username, "testuser")
	}
	if c.Password != "hunter2" {
		t.Errorf("Password = %q, want %q", c.Password, "hunter2")
	}
}

func TestCoverage(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "coverage")

	err := s.RecordCoverage(ctx, taskID, tools.CoverageEntry{
		Surface:  "login form",
		RiskArea: "authentication",
		Outcome:  OutcomeReported,
		Evidence: "brute force possible",
		Reason:   "no lockout after 100 attempts",
		Worker:   "test",
	})
	if err != nil {
		t.Fatalf("RecordCoverage: %v", err)
	}

	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	entries, ok := ov.Coverage["entries"].(int)
	if !ok || entries < 1 {
		t.Errorf("Coverage entries = %v, want >= 1", ov.Coverage["entries"])
	}
}

func TestStaleIntentDetection(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "stale-intent")

	_, err := s.ExecContext(ctx,
		`INSERT INTO exploration_nodes(exploration_id, kind, payload, priority, state, origin, created_at, updated_at)
		 VALUES ($1, 'intent', '{"summary":"long running intent"}'::jsonb, 5, 'running', 'worker',
		         NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour')`,
		expID)
	if err != nil {
		t.Fatalf("insert backdated intent: %v", err)
	}

	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	if len(ov.RunningIntents) < 1 {
		t.Fatal("RunningIntents is empty")
	}
	var staleFound bool
	for _, ri := range ov.RunningIntents {
		if stale, ok := ri["possibly_stale"].(bool); ok && stale {
			staleFound = true
			break
		}
	}
	if !staleFound {
		t.Error("expected at least one running intent with possibly_stale=true")
	}
}
