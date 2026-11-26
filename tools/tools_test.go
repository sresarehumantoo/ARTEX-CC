package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type mockStore struct{}

func (m *mockStore) InsertAsset(_ context.Context, _ int64, _ AssetInput) (int64, error) {
	return 1, nil
}

func (m *mockStore) ListAssets(_ context.Context, _ AssetFilter) ([]Asset, int, error) {
	return nil, 0, nil
}

func (m *mockStore) ListUntestedAssets(_ context.Context, _ int64, _ string, _, _ int) ([]Asset, int, error) {
	return nil, 0, nil
}

func (m *mockStore) ListCompanies(_ context.Context) ([]Company, error) {
	return nil, nil
}

func (m *mockStore) GetOrCreateCompany(_ context.Context, _ string, _ string) (int64, error) {
	return 1, nil
}

func (m *mockStore) AddCompanyScope(_ context.Context, _ int64, _ []string, _ string) (int, int, int, []string, error) {
	return 0, 0, 0, nil, nil
}

func (m *mockStore) AddFinding(_ context.Context, _ FindingInput) (int64, int64, error) {
	return 1, 0, nil
}

func (m *mockStore) ListFindings(_ context.Context, _ int64) ([]Finding, error) {
	return nil, nil
}

func (m *mockStore) FindingExists(_ context.Context, _ int64) (bool, error) {
	return true, nil
}

func (m *mockStore) GraphOverview(_ context.Context, _ int64) (*Overview, error) {
	return &Overview{
		Task:          map[string]any{"goal": "test"},
		FactsTotal:    3,
		FindingsTotal: 1,
		Coverage:      map[string]any{},
	}, nil
}

func (m *mockStore) AddNode(_ context.Context, _ int64, _ string, _ map[string]any, _ int, _, _ string, _ []int64) (int64, error) {
	return 10, nil
}

func (m *mockStore) UpdateNodeState(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockStore) ProveGoal(_ context.Context, _, _ int64) error {
	return nil
}

func (m *mockStore) ClaimIntent(_ context.Context, id int64, owner string, lease time.Duration) (*IntentClaim, error) {
	return &IntentClaim{NodeID: id, Owner: owner, ExpiresAt: time.Now().Add(lease)}, nil
}

func (m *mockStore) RenewIntentClaims(_ context.Context, _ []int64, _ string, _ time.Duration) error {
	return nil
}

func (m *mockStore) ListChanges(_ context.Context, _, _ int64, _ ChangeCursor, _ int) ([]Change, error) {
	return nil, nil
}

func (m *mockStore) ListClaims(_ context.Context, _ int64, _ string) ([]HeldClaim, error) {
	return nil, nil
}

func (m *mockStore) RecordTargetChange(_ context.Context, _ int64, _ TargetChangeInput) (*TargetChangeRecord, error) {
	return nil, nil
}

func (m *mockStore) RecordTargetRestore(_ context.Context, _ int64, _ []int64, _, _ string) ([]TargetRestoreOutcome, error) {
	return nil, nil
}

func (m *mockStore) ListTargetChanges(_ context.Context, _ int64, _ bool) ([]TargetChange, error) {
	return nil, nil
}

func (m *mockStore) ListNodes(_ context.Context, _ int64, _ string, _ int) ([]Node, error) {
	return nil, nil
}

func (m *mockStore) NodeDetail(_ context.Context, _ int64) (*Node, error) {
	return &Node{ID: 1, Kind: "fact", State: "confirmed"}, nil
}

func (m *mockStore) AddEdge(_ context.Context, _ int64, _ string, _ int64) error {
	return nil
}

func (m *mockStore) Frontier(_ context.Context, _ int64, _ int) ([]Node, error) {
	return nil, nil
}

func (m *mockStore) CountFinishedIntents(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

func (m *mockStore) CountOpenIntents(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

func (m *mockStore) OriginFactID(_ context.Context, _ int64) (int64, error) {
	return 1, nil
}

func (m *mockStore) AddGoals(_ context.Context, _ int64, goals []GoalInput, _ bool) (*GoalsResult, error) {
	res := &GoalsResult{Created: []GoalRef{}}
	for i, g := range goals {
		res.Created = append(res.Created, GoalRef{ID: int64(100 + i), Text: g.Text, State: "open"})
	}
	return res, nil
}

func (m *mockStore) AddIntents(_ context.Context, _ int64, intents []IntentInput) (*IntentsResult, error) {
	res := &IntentsResult{}
	for i := range intents {
		res.IDs = append(res.IDs, int64(10+i))
	}
	return res, nil
}

func (m *mockStore) Edges(_ context.Context, _ int64, _ int) ([]Edge, error) {
	return nil, nil
}

func (m *mockStore) Root(_ context.Context, _ int64) (string, string, error) {
	return "desc", "goal", nil
}

func (m *mockStore) AddConstraint(_ context.Context, _ int64, _, _, _ string) (int64, error) {
	return 1, nil
}

func (m *mockStore) ListConstraints(_ context.Context, _ int64) ([]Constraint, error) {
	return nil, nil
}

func (m *mockStore) AddTaskScope(_ context.Context, _ int64, _, _, _, _ string) error {
	return nil
}

func (m *mockStore) ListTaskScope(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}

func (m *mockStore) ListScopeRules(_ context.Context, _ int64) ([]ScopeRule, error) {
	return nil, nil
}

func (m *mockStore) IsInScope(_ context.Context, _ int64, _ string) (bool, string, string, error) {
	return true, "mock: always in scope", "", nil
}

func (m *mockStore) UpdateNodeFields(_ context.Context, _ int64, _ string, _ map[string]any) error {
	return nil
}

func (m *mockStore) ExportEngagementData(_ context.Context, _ int64) (*EngagementExport, error) {
	return nil, nil
}

func (m *mockStore) ImportEngagementData(_ context.Context, _ *EngagementExport) (int64, int64, error) {
	return 1, 1, nil
}

func (m *mockStore) SaveCredential(_ context.Context, _ int64, _ CredentialInput) (int64, error) {
	return 1, nil
}

func (m *mockStore) ListCredentials(_ context.Context, _ int64) ([]Credential, error) {
	return nil, nil
}

func (m *mockStore) RecordCoverage(_ context.Context, _ int64, _ CoverageEntry) error {
	return nil
}

func (m *mockStore) CreateEngagement(_ context.Context, _, _ string) (int64, int64, error) {
	return 1, 1, nil
}

func (m *mockStore) ListEngagements(_ context.Context) ([]EngagementSummary, error) {
	return nil, nil
}

func (m *mockStore) GetEngagement(_ context.Context, _ int64) (*EngagementDetail, error) {
	return nil, nil
}

func (m *mockStore) ListFindingsForReport(_ context.Context, _ int64) ([]ReportFinding, error) {
	return nil, nil
}

func (m *mockStore) BindFindingTraffic(_ context.Context, _ int64, _, _, _ string, _ int) error {
	return nil
}

func (m *mockStore) ListFindingTraffic(_ context.Context, _ int64) ([]TrafficBinding, error) {
	return nil, nil
}

func (m *mockStore) FindingTrafficCount(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

func (m *mockStore) CompleteEngagement(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockStore) UpdateEngagement(_ context.Context, _ int64, _, _ *string) error {
	return nil
}

func (m *mockStore) DeleteEngagement(_ context.Context, _ int64) error {
	return nil
}

func (m *mockStore) EngagementActivity(_ context.Context, _ int64, _ int) ([]TimelineEntry, error) {
	return nil, nil
}

func (m *mockStore) LogActivity(_ context.Context, _ int64, _ ActivityEntry) (int64, error) {
	return 1, nil
}

func (m *mockStore) ListActivity(_ context.Context, _ int64, _ ActivityFilter) ([]Activity, error) {
	return nil, nil
}

func (m *mockStore) FindPotentialDuplicates(_ context.Context, _ int64, _, _ string, _ []int64) ([]Finding, error) {
	return nil, nil
}

func (m *mockStore) MergeFindings(_ context.Context, _ int64, _ []int64) (*MergedFinding, error) {
	return nil, nil
}

func (m *mockStore) DismissFinding(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockStore) ListCoverageGaps(_ context.Context, _ int64, _ int64) (*CoverageGaps, error) {
	return nil, nil
}

var _ Store = (*mockStore)(nil)

func TestRegistryLookup(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	reg.Register(Tool{
		Name: "alpha",
		Run: func(_ context.Context, _ json.RawMessage) (Result, error) {
			return Result{Content: "a"}, nil
		},
	})
	reg.Register(Tool{
		Name: "beta",
		Run: func(_ context.Context, _ json.RawMessage) (Result, error) {
			return Result{Content: "b"}, nil
		},
	})

	t.Run("found_alpha", func(t *testing.T) {
		tool, ok := reg.Lookup("alpha")
		if !ok {
			t.Fatal("expected alpha to be found")
		}
		if tool.Name != "alpha" {
			t.Fatalf("expected name alpha, got %s", tool.Name)
		}
	})

	t.Run("found_beta", func(t *testing.T) {
		tool, ok := reg.Lookup("beta")
		if !ok {
			t.Fatal("expected beta to be found")
		}
		if tool.Name != "beta" {
			t.Fatalf("expected name beta, got %s", tool.Name)
		}
	})

	t.Run("not_found_gamma", func(t *testing.T) {
		_, ok := reg.Lookup("gamma")
		if ok {
			t.Fatal("expected gamma to not be found")
		}
	})
}

func TestInsertAssetsToolSchema(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterAll(reg, &mockStore{}, 1, 1)

	tool, ok := reg.Lookup("insert_assets")
	if !ok {
		t.Fatal("insert_assets tool not found")
	}

	assets, ok := tool.Schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing or wrong type")
	}
	assetsProp, ok := assets["assets"].(map[string]any)
	if !ok {
		t.Fatal("assets property missing or wrong type")
	}
	if assetsProp["type"] != "array" {
		t.Fatalf("expected assets type=array, got %v", assetsProp["type"])
	}
}

func TestGraphOverviewToolCallsMock(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterAll(reg, &mockStore{}, 1, 1)

	tool, ok := reg.Lookup("graph_overview")
	if !ok {
		t.Fatal("graph_overview tool not found")
	}

	result, err := tool.Run(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	if !strings.Contains(result.Content, `"facts"`) {
		t.Fatalf("expected content to contain facts, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, `"findings_total"`) {
		t.Fatalf("expected content to contain findings_total, got: %s", result.Content)
	}
}
