package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestOpenSkipsSchemaWhileTablesInUse(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `LOCK TABLE tasks IN ROW EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}

	opened := make(chan error, 1)
	go func() {
		s2, err := Open(ctx, os.Getenv("ARTEX_TEST_PG"))
		if err == nil {
			s2.Close()
		}
		opened <- err
	}()
	select {
	case err := <-opened:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Open blocked on a table held by an open transaction")
	}
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

	findingID, _, err := s.AddFinding(ctx, tools.FindingInput{
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

	node, err := s.NodeDetail(ctx, f.NodeID)
	if err != nil || node == nil {
		t.Fatalf("NodeDetail(%d): %v", f.NodeID, err)
	}
	if node.Kind != KindFinding {
		t.Errorf("node kind = %q, want %q", node.Kind, KindFinding)
	}
	if got, _ := node.Payload["finding_id"].(float64); int64(got) != findingID {
		t.Errorf("node payload finding_id = %v, want %d", node.Payload["finding_id"], findingID)
	}
	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	if ov.FindingsTotal < 1 {
		t.Errorf("FindingsTotal = %d, want >= 1", ov.FindingsTotal)
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
	if len(ov.RunningIntents) != 0 {
		t.Errorf("RunningIntents = %v, want none for an unclaimed running intent", ov.RunningIntents)
	}
	var reopened bool
	for _, oi := range ov.OpenIntents {
		if expired, ok := oi["claim_expired"].(bool); ok && expired {
			reopened = true
			break
		}
	}
	if !reopened {
		t.Error("expected the unclaimed running intent in OpenIntents with claim_expired=true")
	}
}

func claimIntentNode(t *testing.T, s *Store, label string) (int64, int64) {
	t.Helper()
	_, expID := testEngagement(t, s, label)
	id, err := s.AddNode(context.Background(), expID, KindIntent, map[string]any{"summary": label}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	return expID, id
}

func TestClaimIntentRace(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, id := claimIntentNode(t, s, "claim-race")

	const sessions = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	winners := make(chan string, sessions)
	conflicts := make(chan error, sessions)
	for i := range sessions {
		owner := fmt.Sprintf("sess-%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claim, err := s.ClaimIntent(ctx, id, owner, tools.IntentLease)
			if err != nil {
				conflicts <- err
				return
			}
			winners <- claim.Owner
		}()
	}
	close(start)
	wg.Wait()
	close(winners)
	close(conflicts)

	var won []string
	for w := range winners {
		won = append(won, w)
	}
	if len(won) != 1 {
		t.Fatalf("winners = %v, want exactly one", won)
	}
	for err := range conflicts {
		var ce *tools.ClaimConflictError
		if !errors.As(err, &ce) {
			t.Errorf("loser error = %v, want ClaimConflictError", err)
			continue
		}
		if ce.Owner != won[0] {
			t.Errorf("conflict names holder %q, want %q", ce.Owner, won[0])
		}
	}

	n, err := s.NodeDetail(ctx, id)
	if err != nil {
		t.Fatalf("NodeDetail: %v", err)
	}
	if n.State != "running" {
		t.Errorf("state = %q, want running", n.State)
	}
}

func TestClaimIntentRenewal(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, id := claimIntentNode(t, s, "claim-renew")

	first, err := s.ClaimIntent(ctx, id, "sess-a", time.Minute)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if first.Renewed {
		t.Error("first claim reported as renewal")
	}
	second, err := s.ClaimIntent(ctx, id, "sess-a", time.Hour)
	if err != nil {
		t.Fatalf("renewal: %v", err)
	}
	if !second.Renewed {
		t.Error("second claim by same session not reported as renewal")
	}
	if !second.ExpiresAt.After(first.ExpiresAt) {
		t.Errorf("renewal expiry %v not after first %v", second.ExpiresAt, first.ExpiresAt)
	}

	_, err = s.ClaimIntent(ctx, id, "sess-b", time.Minute)
	var ce *tools.ClaimConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("other session err = %v, want ClaimConflictError", err)
	}
	if ce.Owner != "sess-a" || !ce.ExpiresAt.Equal(second.ExpiresAt) {
		t.Errorf("conflict = %+v, want owner sess-a until %v", ce, second.ExpiresAt)
	}
}

func TestClaimIntentLeaseExpiry(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	expID, id := claimIntentNode(t, s, "claim-expiry")

	if _, err := s.ClaimIntent(ctx, id, "sess-a", time.Hour); err != nil {
		t.Fatalf("claim: %v", err)
	}

	frontier, err := s.Frontier(ctx, expID, 0)
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(frontier) != 0 {
		t.Errorf("Frontier = %v, want claimed intent hidden", frontier)
	}
	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	if ov.FrontierOpen != 0 || len(ov.RunningIntents) != 1 {
		t.Fatalf("FrontierOpen = %d, RunningIntents = %v; want 0 and the claimed intent", ov.FrontierOpen, ov.RunningIntents)
	}
	if got := ov.RunningIntents[0]["claimed_by"]; got != tools.OwnerLabel("sess-a") {
		t.Errorf("claimed_by = %v, want label of sess-a", got)
	}
	if _, ok := ov.RunningIntents[0]["claim_expires_at"]; !ok {
		t.Error("claim_expires_at missing from running intent")
	}

	if _, err := s.ExecContext(ctx,
		`UPDATE exploration_nodes SET claim_expires_at = now() - interval '1 second' WHERE id = $1`, id); err != nil {
		t.Fatalf("expire claim: %v", err)
	}

	frontier, err = s.Frontier(ctx, expID, 0)
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(frontier) != 1 || frontier[0].ID != id {
		t.Errorf("Frontier = %v, want expired intent %d", frontier, id)
	}
	ov, err = s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	if ov.FrontierOpen != 1 || ov.OpenIntents[0]["previous_owner"] != tools.OwnerLabel("sess-a") {
		t.Errorf("OpenIntents = %v, want expired intent with previous_owner sess-a", ov.OpenIntents)
	}

	claim, err := s.ClaimIntent(ctx, id, "sess-b", time.Minute)
	if err != nil {
		t.Fatalf("takeover of expired claim: %v", err)
	}
	if claim.Owner != "sess-b" || claim.Renewed {
		t.Errorf("takeover = %+v, want fresh claim by sess-b", claim)
	}
}

func TestClaimIntentReleasedOnCompletion(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, id := claimIntentNode(t, s, "claim-release")

	if _, err := s.ClaimIntent(ctx, id, "sess-a", time.Hour); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := s.UpdateNodeFields(ctx, id, "done", map[string]any{"result": "ok"}); err != nil {
		t.Fatalf("UpdateNodeFields: %v", err)
	}
	var owner sql.NullString
	var expires sql.NullTime
	if err := s.QueryRowContext(ctx,
		`SELECT owner, claim_expires_at FROM exploration_nodes WHERE id = $1`, id).Scan(&owner, &expires); err != nil {
		t.Fatalf("select claim: %v", err)
	}
	if owner.Valid || expires.Valid {
		t.Errorf("claim not cleared: owner=%v expires=%v", owner, expires)
	}

	_, err := s.ClaimIntent(ctx, id, "sess-b", time.Minute)
	if err == nil || !strings.Contains(err.Error(), "must be open") {
		t.Errorf("claim of done intent err = %v, want 'must be open'", err)
	}
}

func TestClaimIntentNotAnIntent(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "claim-kind")
	factID, err := s.AddNode(ctx, expID, KindFact, map[string]any{"summary": "x"}, 0, "confirmed", "worker", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if _, err := s.ClaimIntent(ctx, factID, "sess-a", time.Minute); err == nil || !strings.Contains(err.Error(), "not an intent") {
		t.Errorf("err = %v, want 'not an intent'", err)
	}
	if _, err := s.ClaimIntent(ctx, 1<<40, "sess-a", time.Minute); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want 'not found'", err)
	}
}

func TestInsertAssetNormalizesURL(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "url-normalize")
	host := fmt.Sprintf("norm-%d.example.com", taskID)

	first, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "service", URL: "https://admin:s3cret@" + host + "/Login?Next=A"})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	second, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "service", URL: "HTTPS://" + strings.ToUpper(host) + ":443/Login?Next=A"})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if first != second {
		t.Errorf("ids = %d, %d, want the same asset", first, second)
	}

	want := "https://" + host + "/Login?Next=A"
	list, total, err := s.ListAssets(ctx, tools.AssetFilter{TaskID: taskID, DSL: `url=="https://user@` + strings.ToUpper(host) + `/Login?Next=A"`})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("total = %d, len = %d, want 1", total, len(list))
	}
	if got := list[0].Data["url"]; got != want {
		t.Errorf("url = %v, want %q", got, want)
	}
}

func TestInsertAssetRecordsURLCredential(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "url-credential")
	host := fmt.Sprintf("creds-%d.example.com", taskID)

	if _, err := s.SaveCredential(ctx, taskID, tools.CredentialInput{Domain: host, Username: "known", Password: "original", Source: "agent"}); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	for _, in := range []tools.AssetInput{
		{Type: "service", URL: "https://ad%40min:p%3Ass@" + strings.ToUpper(host) + "/"},
		{Type: "service", URL: "https://known:changed@" + host + "/login"},
		{Type: "endpoint", URL: "https://guest@" + host + "/api", Method: "GET"},
		{Type: "service", URL: "https://ad%40min:other@" + host + "/again"},
	} {
		if _, err := s.InsertAsset(ctx, taskID, in); err != nil {
			t.Fatalf("InsertAsset(%s): %v", in.URL, err)
		}
	}

	creds, err := s.ListCredentials(ctx, taskID)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	got := map[string]tools.Credential{}
	for _, c := range creds {
		if c.Domain != host {
			t.Errorf("domain = %q, want %q", c.Domain, host)
		}
		got[c.Username] = c
	}
	want := map[string][2]string{
		"known":  {"original", "agent"},
		"ad@min": {"p:ss", "asset_url"},
		"guest":  {"", "asset_url"},
	}
	if len(got) != len(want) {
		t.Fatalf("credentials = %+v, want %d entries", creds, len(want))
	}
	for user, w := range want {
		c, ok := got[user]
		if !ok {
			t.Errorf("missing credential for %q", user)
			continue
		}
		if c.Password != w[0] || c.Source != w[1] {
			t.Errorf("%s: password, source = %q, %q, want %q, %q", user, c.Password, c.Source, w[0], w[1])
		}
	}
}

func TestImportRecordsURLCredential(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	host := fmt.Sprintf("import-creds-%d.example.com", os.Getpid())

	taskID, _, err := s.ImportEngagementData(ctx, &tools.EngagementExport{
		Version: 1,
		Goal:    "import url credentials",
		Assets: []map[string]any{
			{"id": float64(1), "type": "endpoint", "url": "https://captured:fresh@" + host + "/", "method": "GET"},
			{"id": float64(2), "type": "endpoint", "url": "https://known:fromurl@" + host + "/login", "method": "GET"},
		},
		Credentials: []map[string]any{
			{"domain": strings.ToUpper(host), "username": "known", "password": "exported", "source": "agent", "note": "kept"},
		},
	})
	if err != nil {
		t.Fatalf("ImportEngagementData: %v", err)
	}

	creds, err := s.ListCredentials(ctx, taskID)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	got := map[string]tools.Credential{}
	for _, c := range creds {
		got[c.Username] = c
	}
	if len(got) != 2 {
		t.Fatalf("credentials = %+v, want 2 entries", creds)
	}
	if c := got["captured"]; c.Domain != host || c.Password != "fresh" || c.Source != "asset_url" {
		t.Errorf("captured = %+v, want password fresh, source asset_url", c)
	}
	if c := got["known"]; c.Domain != host || c.Password != "exported" || c.Source != "agent" || c.Note != "kept" {
		t.Errorf("known = %+v, want exported row unchanged", c)
	}
}

func TestSaveCredentialDomainKey(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "credential-domain-key")
	host := fmt.Sprintf("case-%d.example.com", taskID)

	first, err := s.SaveCredential(ctx, taskID, tools.CredentialInput{Domain: " " + strings.ToUpper(host) + ". ", Username: "admin", Password: "one"})
	if err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	second, err := s.SaveCredential(ctx, taskID, tools.CredentialInput{Domain: host, Username: "admin", Password: "two"})
	if err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	if first != second {
		t.Errorf("ids = %d, %d, want the same credential", first, second)
	}
	if _, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "service", URL: "https://admin:three@" + strings.ToUpper(host) + "/"}); err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}

	creds, err := s.ListCredentials(ctx, taskID)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	if len(creds) != 1 || creds[0].Domain != host || creds[0].Password != "two" {
		t.Errorf("credentials = %+v, want one %s/admin with password two", creds, host)
	}
}

func TestIsInScopeWildcardDomainDB(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "wildcard-scope")

	companyID, err := s.GetOrCreateCompany(ctx, fmt.Sprintf("wildcard-scope-%d", taskID), "")
	if err != nil {
		t.Fatalf("GetOrCreateCompany: %v", err)
	}
	if _, _, _, _, err := s.AddCompanyScope(ctx, companyID, []string{"*.wild-scope.example"}, "test"); err != nil {
		t.Fatalf("AddCompanyScope: %v", err)
	}
	if _, err := s.ExecContext(ctx, `UPDATE tasks SET company_id = $1 WHERE id = $2`, companyID, taskID); err != nil {
		t.Fatalf("set task company: %v", err)
	}

	tests := []struct {
		target string
		want   bool
	}{
		{"wild-scope.example", false},
		{"api.wild-scope.example", true},
		{"https://a.b.wild-scope.example/login", true},
		{"badwild-scope.example", false},
	}
	for _, tt := range tests {
		got, reason, _, err := s.IsInScope(ctx, taskID, tt.target)
		if err != nil {
			t.Fatalf("IsInScope(%q): %v", tt.target, err)
		}
		if got != tt.want {
			t.Errorf("IsInScope(%q) = %v (%s), want %v", tt.target, got, reason, tt.want)
		}
	}
}
