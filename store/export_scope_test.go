package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestExportImportScopeAndCompanies(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "export-scope")
	suffix := time.Now().UnixNano()
	companyName := fmt.Sprintf("Export Scope Co %d", suffix)
	root := fmt.Sprintf("export-scope-%d.com", suffix)
	companyDomain := fmt.Sprintf("export-scope-co-%d.example", suffix)

	companyID, err := s.GetOrCreateCompany(ctx, companyName, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, invalid, errs, err := s.AddCompanyScope(ctx, companyID, []string{companyDomain, "10.77.0.0/16"}, "client list"); err != nil || invalid != 0 {
		t.Fatalf("AddCompanyScope: invalid=%d errs=%v err=%v", invalid, errs, err)
	}
	for _, sc := range [][2]string{{"root_domain", root}, {"cidr", "192.168.77.0/24"}, {"company", companyName}} {
		if err := s.AddTaskScope(ctx, taskID, sc[0], sc[1], "", "manual"); err != nil {
			t.Fatalf("AddTaskScope(%s): %v", sc[0], err)
		}
	}
	if _, err := s.ExecContext(ctx, `UPDATE tasks SET company_id = $1 WHERE id = $2`, companyID, taskID); err != nil {
		t.Fatal(err)
	}
	assetID, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(ctx, `UPDATE assets SET company_id = $1 WHERE id = $2`, companyID, assetID); err != nil {
		t.Fatal(err)
	}

	targets := []string{"www." + root, "192.168.77.5", "api." + companyDomain, "10.77.1.1", "unrelated.org", "8.8.8.8"}
	before := scopeAnswers(t, s, taskID, targets)

	export, err := s.ExportEngagementData(ctx, taskID)
	if err != nil {
		t.Fatalf("ExportEngagementData: %v", err)
	}
	raw, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	var decoded tools.EngagementExport
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	// Deleting the company makes the import recreate it, as it would in a fresh database.
	if _, err := s.ExecContext(ctx, `DELETE FROM companies WHERE id = $1`, companyID); err != nil {
		t.Fatal(err)
	}

	newTaskID, _, err := s.ImportEngagementData(ctx, &decoded)
	if err != nil {
		t.Fatalf("ImportEngagementData: %v", err)
	}
	newCompanyID := checkImportedCompanyScope(t, s, newTaskID, companyName, root, companyDomain)
	if newCompanyID == companyID {
		t.Fatalf("company was not recreated: id %d", newCompanyID)
	}
	var assetCompany sql.NullInt64
	if err := s.QueryRowContext(ctx, `SELECT company_id FROM assets WHERE id = $1`, assetID).Scan(&assetCompany); err != nil {
		t.Fatal(err)
	}
	if assetCompany.Int64 != newCompanyID {
		t.Errorf("asset company_id = %v, want %d", assetCompany, newCompanyID)
	}
	if after := scopeAnswers(t, s, newTaskID, targets); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("IsInScope after import = %v, before = %v", after, before)
	}

	againTaskID, _, err := s.ImportEngagementData(ctx, &decoded)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if id := checkImportedCompanyScope(t, s, againTaskID, companyName, root, companyDomain); id != newCompanyID {
		t.Errorf("second import used company %d, want existing %d", id, newCompanyID)
	}
}

func TestImportExportWithoutCompanies(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	root := fmt.Sprintf("legacy-export-%d.com", time.Now().UnixNano())
	legacy := `{"version":1,"exported_at":"2026-10-01T00:00:00Z","goal":"legacy","description":"v0.6.1 export",
		"nodes":[],"edges":[],"constraints":[],"assets":[{"id":7,"type":"root_domain","domain":"` + root + `"}],
		"findings":[],"credentials":[],"coverage":[],
		"scope":[{"kind":"root_domain","value":"` + root + `","source":"manual"},{"kind":"ip","value":"10.9.9.9/32","source":"agent"}]}`
	var data tools.EngagementExport
	if err := json.Unmarshal([]byte(legacy), &data); err != nil {
		t.Fatal(err)
	}
	taskID, _, err := s.ImportEngagementData(ctx, &data)
	if err != nil {
		t.Fatalf("ImportEngagementData: %v", err)
	}
	rules, err := s.ListScopeRules(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("scope rules = %+v, want 2", rules)
	}
	export, err := s.ExportEngagementData(ctx, taskID)
	if err != nil {
		t.Fatalf("ExportEngagementData: %v", err)
	}
	if export.CompanyID != 0 || len(export.Companies) != 0 || len(export.Scope) != 2 {
		t.Errorf("export company_id=%d companies=%v scope=%v", export.CompanyID, export.Companies, export.Scope)
	}
}

func scopeAnswers(t *testing.T, s *Store, taskID int64, targets []string) []bool {
	t.Helper()
	out := make([]bool, len(targets))
	for i, target := range targets {
		ok, _, _, err := s.IsInScope(context.Background(), taskID, target)
		if err != nil {
			t.Fatalf("IsInScope(%s): %v", target, err)
		}
		out[i] = ok
	}
	return out
}

func checkImportedCompanyScope(t *testing.T, s *Store, taskID int64, companyName, root, companyDomain string) int64 {
	t.Helper()
	ctx := context.Background()
	var companyID int64
	if err := s.QueryRowContext(ctx, `SELECT id FROM companies WHERE nkey = $1`, companyNKey(companyName)).Scan(&companyID); err != nil {
		t.Fatalf("company %q: %v", companyName, err)
	}
	task, err := s.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.CompanyID == nil || *task.CompanyID != companyID {
		t.Errorf("task company_id = %v, want %d", task.CompanyID, companyID)
	}

	rows, err := s.QueryContext(ctx,
		`SELECT kind, COALESCE(domain, net::text, ''), COALESCE(company_id, 0) FROM task_scope WHERE task_id = $1`, taskID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var scope []string
	for rows.Next() {
		var kind, value string
		var cid int64
		if err := rows.Scan(&kind, &value, &cid); err != nil {
			t.Fatal(err)
		}
		if kind == "company" {
			value = fmt.Sprint(cid == companyID)
		}
		scope = append(scope, kind+"="+value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(scope)
	if want := fmt.Sprint([]string{"cidr=192.168.77.0/24", "company=true", "root_domain=" + root}); fmt.Sprint(scope) != want {
		t.Errorf("task scope = %v, want %v", scope, want)
	}

	lines, err := s.ListCompanyScope(companyID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range lines {
		got = append(got, l.Kind+"="+l.Domain+l.Net+" raw="+l.Raw+" reason="+l.Reason)
	}
	sort.Strings(got)
	want := []string{"cidr=10.77.0.0/16 raw=10.77.0.0/16 reason=client list", "domain=" + companyDomain + " raw=" + companyDomain + " reason=client list"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("company scope = %v, want %v", got, want)
	}
	return companyID
}

func TestImportServiceWithoutTechnologies(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "export-no-tech")
	url := fmt.Sprintf("https://no-tech-%d.example/", time.Now().UnixNano())
	assetID, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "service", URL: url})
	if err != nil {
		t.Fatal(err)
	}
	export, err := s.ExportEngagementData(ctx, taskID)
	if err != nil {
		t.Fatalf("ExportEngagementData: %v", err)
	}
	raw, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	var decoded tools.EngagementExport
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(ctx, `DELETE FROM assets WHERE id = $1`, assetID); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.ImportEngagementData(ctx, &decoded); err != nil {
		t.Fatalf("ImportEngagementData: %v", err)
	}
	var empty bool
	if err := s.QueryRowContext(ctx, `SELECT technologies = '{}' FROM assets WHERE url = $1`, url).Scan(&empty); err != nil {
		t.Fatal(err)
	}
	if !empty {
		t.Error("imported technologies are not empty")
	}
}
