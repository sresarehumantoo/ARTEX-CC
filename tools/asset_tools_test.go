package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type assetMockStore struct {
	mockStore
	insertErr    error
	insertedID   int64
	listAssets   []Asset
	listTotal    int
	listErr      error
	companies    []Company
	companiesErr error
	scopeErr     error
	scopeAdded   int
	scopeSkipped int
	scopeInvalid int
	scopeErrors  []string
	untestedErr  error
	inScope      bool
	scopeReason  string
}

func (m *assetMockStore) InsertAsset(_ context.Context, _ int64, _ AssetInput) (int64, error) {
	if m.insertErr != nil {
		return 0, m.insertErr
	}
	return m.insertedID, nil
}

func (m *assetMockStore) ListAssets(_ context.Context, _ AssetFilter) ([]Asset, int, error) {
	return m.listAssets, m.listTotal, m.listErr
}

func (m *assetMockStore) ListCompanies(_ context.Context) ([]Company, error) {
	return m.companies, m.companiesErr
}

func (m *assetMockStore) AddCompanyScope(_ context.Context, _ int64, _ []string, _ string) (int, int, int, []string, error) {
	return m.scopeAdded, m.scopeSkipped, m.scopeInvalid, m.scopeErrors, m.scopeErr
}

func (m *assetMockStore) ListUntestedAssets(_ context.Context, _ int64, _ string, _, _ int) ([]Asset, int, error) {
	return m.listAssets, m.listTotal, m.untestedErr
}

func (m *assetMockStore) IsInScope(_ context.Context, _ int64, _ string) (bool, string, string, error) {
	return m.inScope, m.scopeReason, "", nil
}

func TestInsertAssetsHappyPath(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{insertedID: 42, inScope: true}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, ok := reg.Lookup("insert_assets")
	if !ok {
		t.Fatal("insert_assets not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"assets": [{"type": "root_domain", "domain": "example.com"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"id":42`) {
		t.Errorf("expected id 42 in result, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"type":"root_domain"`) {
		t.Errorf("expected type root_domain in result, got: %s", r.Content)
	}
}

func TestInsertAssetsMultiple(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{insertedID: 10, inScope: true}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("insert_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"assets": [
			{"type": "root_domain", "domain": "a.com"},
			{"type": "ip", "ip": "10.0.0.1"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"results"`) {
		t.Errorf("expected results array, got: %s", r.Content)
	}
}

func TestInsertAssetsEmptyArray(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{inScope: true}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("insert_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"assets": []}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty assets array")
	}
	if !strings.Contains(r.Content, "must not be empty") {
		t.Errorf("expected 'must not be empty', got: %s", r.Content)
	}
}

func TestInsertAssetsInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("insert_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(r.Content, "invalid input:") {
		t.Errorf("expected 'invalid input:', got: %s", r.Content)
	}
}

func TestInsertAssetsMissingType(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{inScope: true}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("insert_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"assets": [{"domain": "example.com"}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected partial success with error entry, got full error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "type is required") {
		t.Errorf("expected 'type is required' in errors, got: %s", r.Content)
	}
}

func TestInsertAssetsStoreError(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{insertErr: fmt.Errorf("db constraint violation"), inScope: true}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("insert_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"assets": [{"type": "root_domain", "domain": "test.com"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(r.Content, "db constraint violation") {
		t.Errorf("expected store error in response, got: %s", r.Content)
	}
}

func TestInsertAssetsOutOfScope(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{inScope: false, scopeReason: "not in scope"}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("insert_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"assets": [{"type": "root_domain", "domain": "evil.com"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(r.Content, "out of scope") {
		t.Errorf("expected out of scope error, got: %s", r.Content)
	}
}

func TestListAssetsHappyPath(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{
		listAssets: []Asset{
			{ID: 1, Type: "root_domain", Label: "example.com"},
			{ID: 2, Type: "ip", Label: "10.0.0.1"},
		},
		listTotal: 2,
	}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, ok := reg.Lookup("list_assets")
	if !ok {
		t.Fatal("list_assets not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"dsl": "domain=example"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"count":2`) {
		t.Errorf("expected count 2, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"example.com"`) {
		t.Errorf("expected example.com in result, got: %s", r.Content)
	}
}

func TestListAssetsEmptyDSLNoID(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty dsl without id")
	}
	if !strings.Contains(r.Content, "dsl must not be empty") {
		t.Errorf("expected dsl validation error, got: %s", r.Content)
	}
}

func TestListAssetsByID(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{
		listAssets: []Asset{{ID: 5, Type: "ip", Label: "192.168.1.1"}},
		listTotal:  1,
	}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"id": 5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"count":1`) {
		t.Errorf("expected count 1, got: %s", r.Content)
	}
}

func TestListAssetsByIDs(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{listAssets: []Asset{}, listTotal: 0}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"ids": [1, 2, 3]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestListAssetsStoreError(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{listErr: fmt.Errorf("connection lost")}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"dsl": "ip=10"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "connection lost") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestListAssetsInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(r.Content, "invalid input:") {
		t.Errorf("expected 'invalid input:', got: %s", r.Content)
	}
}

func TestListCompaniesHappyPath(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{
		companies: []Company{
			{ID: 1, Name: "Acme Corp", AssetCount: 5},
			{ID: 2, Name: "Beta Inc", AssetCount: 3},
		},
	}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, ok := reg.Lookup("list_companies")
	if !ok {
		t.Fatal("list_companies not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"count":2`) {
		t.Errorf("expected count 2, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"Acme Corp"`) {
		t.Errorf("expected Acme Corp, got: %s", r.Content)
	}
}

func TestListCompaniesWithSearch(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{
		companies: []Company{
			{ID: 1, Name: "Acme Corp", AssetCount: 5},
			{ID: 2, Name: "Beta Inc", AssetCount: 3},
		},
	}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_companies")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"search": "acme"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"count":1`) {
		t.Errorf("expected count 1, got: %s", r.Content)
	}
	if strings.Contains(r.Content, `"Beta Inc"`) {
		t.Error("expected Beta Inc to be filtered out")
	}
}

func TestListCompaniesStoreError(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{companiesErr: fmt.Errorf("db error")}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_companies")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "db error") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestListCompaniesInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_companies")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAddCompanyScopeHappyPath(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{scopeAdded: 2, scopeSkipped: 1}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, ok := reg.Lookup("add_company_scope")
	if !ok {
		t.Fatal("add_company_scope not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"company": "Acme",
		"scope": "*.acme.com\n10.0.0.0/24",
		"reason": "contract"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"added":2`) {
		t.Errorf("expected added 2, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"skipped":1`) {
		t.Errorf("expected skipped 1, got: %s", r.Content)
	}
}

func TestAddCompanyScopeEmptyCompany(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_company_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"company": "",
		"scope": "*.acme.com",
		"reason": "contract"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty company")
	}
	if !strings.Contains(r.Content, "company must not be empty") {
		t.Errorf("expected company validation error, got: %s", r.Content)
	}
}

func TestAddCompanyScopeInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_company_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAddCompanyScopeStoreError(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{scopeErr: fmt.Errorf("insert failed")}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_company_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"company": "Test",
		"scope": "*.test.com",
		"reason": "testing"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "insert failed") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestAddTaskScopeHappyPath(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, ok := reg.Lookup("add_task_scope")
	if !ok {
		t.Fatal("add_task_scope not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"kind": "root_domain",
		"value": "example.com",
		"reason": "primary target"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"added"`) {
		t.Errorf("expected added in result, got: %s", r.Content)
	}
}

func TestAddTaskScopeBatch(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_task_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"entries": [
			{"kind": "root_domain", "value": "a.com"},
			{"kind": "ip", "value": "10.0.0.1"}
		],
		"reason": "scope expansion"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestAddTaskScopeNoTask(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 0)

	tool, _ := reg.Lookup("add_task_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"kind": "root_domain",
		"value": "example.com",
		"reason": "test"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when taskID is 0")
	}
	if !strings.Contains(r.Content, "requires a task context") {
		t.Errorf("expected task context error, got: %s", r.Content)
	}
}

func TestAddTaskScopeInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_task_scope")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestListUntestedAssetsHappyPath(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{
		listAssets: []Asset{{ID: 3, Type: "service", Label: "https://api.example.com"}},
		listTotal:  1,
	}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, ok := reg.Lookup("list_untested_assets")
	if !ok {
		t.Fatal("list_untested_assets not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"total":1`) {
		t.Errorf("expected total 1, got: %s", r.Content)
	}
}

func TestListUntestedAssetsWithType(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{listAssets: []Asset{}, listTotal: 0}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_untested_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"type": "service", "page": 2, "page_size": 5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"page":2`) {
		t.Errorf("expected page 2, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"page_size":5`) {
		t.Errorf("expected page_size 5, got: %s", r.Content)
	}
}

func TestListUntestedAssetsNoTask(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 0)

	tool, _ := reg.Lookup("list_untested_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error when taskID is 0")
	}
	if !strings.Contains(r.Content, "requires a task context") {
		t.Errorf("expected task context error, got: %s", r.Content)
	}
}

func TestListUntestedAssetsStoreError(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{untestedErr: fmt.Errorf("timeout")}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_untested_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "timeout") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestListUntestedAssetsInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_untested_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestListUntestedAssetsDefaultPagination(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{listAssets: []Asset{}, listTotal: 0}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_untested_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"page":1`) {
		t.Errorf("expected default page 1, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"page_size":10`) {
		t.Errorf("expected default page_size 10, got: %s", r.Content)
	}
}

func TestInsertAssetsOutOfScopeRedactsUserinfo(t *testing.T) {
	t.Parallel()
	ms := &assetMockStore{inScope: false, scopeReason: "not in scope"}
	reg := NewRegistry()
	RegisterAssetTools(reg, ms, 1)

	tool, _ := reg.Lookup("insert_assets")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"assets": [{"type": "service", "url": "https://admin:pa/ss@evil.com/x"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(r.Content, "out of scope (https://evil.com/x)") || strings.Contains(r.Content, "pa/ss") {
		t.Errorf("expected redacted out of scope error, got: %s", r.Content)
	}
}
