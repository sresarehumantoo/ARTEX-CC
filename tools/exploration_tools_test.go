package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type explorationMockStore struct {
	mockStore
	overviewErr   error
	addNodeID     int64
	addNodeErr    error
	nodeMap       map[int64]*Node
	listErr       error
	listNodes     []Node
	edgeErr       error
	proveErr      error
	constraintErr error
	goalsResult   *GoalsResult
	gotGoals      []GoalInput
	gotAppend     bool
	intentsResult *IntentsResult
	gotIntents    []IntentInput
}

func (m *explorationMockStore) AddGoals(ctx context.Context, explorationID int64, goals []GoalInput, appendGoals bool) (*GoalsResult, error) {
	m.gotGoals, m.gotAppend = goals, appendGoals
	if m.goalsResult != nil {
		return m.goalsResult, nil
	}
	return m.mockStore.AddGoals(ctx, explorationID, goals, appendGoals)
}

func newExplorationMockStore() *explorationMockStore {
	return &explorationMockStore{
		addNodeID: 10,
		nodeMap:   map[int64]*Node{},
	}
}

func (m *explorationMockStore) GraphOverview(_ context.Context, _ int64) (*Overview, error) {
	if m.overviewErr != nil {
		return nil, m.overviewErr
	}
	return &Overview{
		Task:          map[string]any{"goal": "test"},
		FactsTotal:    3,
		FindingsTotal: 1,
		Coverage:      map[string]any{},
	}, nil
}

func (m *explorationMockStore) AddNode(_ context.Context, _ int64, _ string, _ map[string]any, _ int, _, _ string, _ []int64) (int64, error) {
	return m.addNodeID, m.addNodeErr
}

func (m *explorationMockStore) AddIntents(_ context.Context, _ int64, intents []IntentInput) (*IntentsResult, error) {
	m.gotIntents = intents
	if m.intentsResult != nil {
		return m.intentsResult, nil
	}
	if m.addNodeErr != nil {
		return nil, m.addNodeErr
	}
	res := &IntentsResult{}
	for i := range intents {
		res.IDs = append(res.IDs, m.addNodeID+int64(i))
	}
	return res, nil
}

func (m *explorationMockStore) NodeDetail(_ context.Context, id int64) (*Node, error) {
	n, ok := m.nodeMap[id]
	if !ok {
		return nil, nil
	}
	return n, nil
}

func (m *explorationMockStore) ListNodes(_ context.Context, _ int64, _ string, _ int) ([]Node, error) {
	return m.listNodes, m.listErr
}

func (m *explorationMockStore) AddEdge(_ context.Context, _ int64, _ string, _ int64) error {
	return m.edgeErr
}

func (m *explorationMockStore) ProveGoal(_ context.Context, _, _ int64) error {
	return m.proveErr
}

func (m *explorationMockStore) AddConstraint(_ context.Context, _ int64, _, _, _ string) (int64, error) {
	if m.constraintErr != nil {
		return 0, m.constraintErr
	}
	return 1, nil
}

func TestAddIntentHappyPath(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, ok := reg.Lookup("add_intent")
	if !ok {
		t.Fatal("add_intent not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"summary": "test for XSS in search param"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "intent created: 10") {
		t.Errorf("expected intent created message, got: %s", r.Content)
	}
}

func TestAddIntentBatch(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"intents": [
			{"summary": "test XSS"},
			{"summary": "test SQLi"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"ids"`) {
		t.Errorf("expected ids array for batch, got: %s", r.Content)
	}
}

func TestAddIntentEmptySummary(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"summary": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty summary")
	}
	if !strings.Contains(r.Content, "summary must not be empty") {
		t.Errorf("expected summary required, got: %s", r.Content)
	}
}

func TestAddIntentInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAddIntentWithParents(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[5] = &Node{ID: 5, Kind: KindFact, State: "confirmed"}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"summary": "follow up on port scan",
		"parent_ids": [5]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestAddIntentInvalidParentKind(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[5] = &Node{ID: 5, Kind: KindIntent, State: "open"}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_intent")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"summary": "test",
		"parent_ids": [5]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for intent parent")
	}
	if !strings.Contains(r.Content, "can only anchor on fact/finding") {
		t.Errorf("expected anchor validation error, got: %s", r.Content)
	}
}

func TestRecordFactHappyPath(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[7] = &Node{ID: 7, Kind: KindIntent, State: "running"}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, ok := reg.Lookup("record_fact")
	if !ok {
		t.Fatal("record_fact not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"facts": [{"summary": "port 80 open", "intent_id": 7, "confidence": "observed", "source": "tool_output"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "fact created") {
		t.Errorf("expected fact created message, got: %s", r.Content)
	}
}

func TestRecordFactEmptyArray(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("record_fact")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"facts": []}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty facts")
	}
	if !strings.Contains(r.Content, "non-empty") {
		t.Errorf("expected non-empty error, got: %s", r.Content)
	}
}

func TestRecordFactEmptySummary(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("record_fact")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"facts": [{"summary": ""}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty summary")
	}
}

func TestRecordFactInvalidSource(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("record_fact")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"facts": [{"summary": "test", "source": "bogus"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid source")
	}
	if !strings.Contains(r.Content, "not valid") {
		t.Errorf("expected source validation error, got: %s", r.Content)
	}
}

func TestRecordFactInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("record_fact")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRecordFactDefaultSource(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("record_fact")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"facts": [{"summary": "port closed", "evidence": "nmap output", "detail": "extra detail"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success with default source, got error: %s", r.Content)
	}
}

func TestListFactsHappyPath(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.listNodes = []Node{
		{ID: 1, Kind: KindFact, State: "confirmed", Origin: "worker", Payload: map[string]any{"summary": "port open", "source": "tool_output"}},
	}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, ok := reg.Lookup("list_facts")
	if !ok {
		t.Fatal("list_facts not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"port open"`) {
		t.Errorf("expected fact summary, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"total":1`) {
		t.Errorf("expected total, got: %s", r.Content)
	}
}

func TestListFactsStoreError(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.listErr = fmt.Errorf("timeout")
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_facts")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
}

func TestListFactsInvalidJSON(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("list_facts")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNodeDetailHappyPath(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[42] = &Node{ID: 42, Kind: KindFact, State: "confirmed", Payload: map[string]any{"summary": "test"}}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, ok := reg.Lookup("node_detail")
	if !ok {
		t.Fatal("node_detail not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"id": 42}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"id":42`) {
		t.Errorf("expected id 42, got: %s", r.Content)
	}
}

func TestNodeDetailNotFound(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("node_detail")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"id": 999}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for nonexistent node")
	}
	if !strings.Contains(r.Content, "not found") {
		t.Errorf("expected not found, got: %s", r.Content)
	}
}

func TestNodeDetailMissingID(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("node_detail")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing id")
	}
	if !strings.Contains(r.Content, "id is required") {
		t.Errorf("expected id required, got: %s", r.Content)
	}
}

func TestAddHintHappyPath(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, ok := reg.Lookup("add_hint")
	if !ok {
		t.Fatal("add_hint not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": "try testing the admin panel"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "hint created") {
		t.Errorf("expected hint created message, got: %s", r.Content)
	}
}

func TestAddHintBatch(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_hint")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"hints": [
			{"text": "try admin panel"},
			{"text": "check for SSRF"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestAddHintEmptyText(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("add_hint")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty text")
	}
}

func TestSetGoalsHappyPath(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, ok := reg.Lookup("set_goals")
	if !ok {
		t.Fatal("set_goals not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": "find SQL injection"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	var out struct {
		Created []GoalRef `json:"created"`
	}
	if err := json.Unmarshal([]byte(r.Content), &out); err != nil {
		t.Fatalf("decode %s: %v", r.Content, err)
	}
	if len(out.Created) != 1 || out.Created[0].Text != "find SQL injection" {
		t.Errorf("created = %+v", out.Created)
	}
}

func TestSetGoalsAlreadySet(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.goalsResult = &GoalsResult{
		Created:  []GoalRef{},
		Skipped:  []SkippedGoal{{Text: "get RCE", Reason: GoalSkipAlreadySet}},
		Existing: []GoalRef{{ID: 7, Text: "dump the database", State: "open"}},
	}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_goals")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"goals": [{"text": "  get RCE "}, {"text": ""}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if ms.gotAppend {
		t.Error("append passed as true, want false")
	}
	if len(ms.gotGoals) != 1 || ms.gotGoals[0].Text != "get RCE" {
		t.Errorf("store got goals %+v, want the one trimmed non-empty goal", ms.gotGoals)
	}
	var out struct {
		Created  []GoalRef         `json:"created"`
		Skipped  []SkippedGoal     `json:"skipped"`
		Existing []GoalRef         `json:"existing_goals"`
		Errors   map[string]string `json:"errors"`
		Note     string            `json:"note"`
	}
	if err := json.Unmarshal([]byte(r.Content), &out); err != nil {
		t.Fatalf("decode %s: %v", r.Content, err)
	}
	if len(out.Created) != 0 || len(out.Skipped) != 1 || len(out.Existing) != 1 || out.Existing[0].ID != 7 {
		t.Errorf("unexpected result: %s", r.Content)
	}
	if out.Errors["1"] == "" {
		t.Errorf("expected an error for the empty goal at index 1: %s", r.Content)
	}
	if !strings.Contains(out.Note, "append: true") {
		t.Errorf("note should explain append, got %q", out.Note)
	}
}

func TestSetGoalsAppendPassedThrough(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.goalsResult = &GoalsResult{
		Created: []GoalRef{{ID: 12, Text: "read /etc/passwd", State: "open"}},
		Skipped: []SkippedGoal{{Text: "Dump the database.", Reason: GoalSkipDuplicate, ExistingID: 7}},
	}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_goals")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"append": true, "goals": [{"text": "read /etc/passwd"}, {"text": "Dump the database."}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ms.gotAppend {
		t.Error("append not passed to the store")
	}
	if strings.Contains(r.Content, "existing_goals") {
		t.Errorf("existing_goals should only appear when nothing was added: %s", r.Content)
	}
	if !strings.Contains(r.Content, `"existing_id":7`) || !strings.Contains(r.Content, `"reason":"duplicate"`) {
		t.Errorf("expected the duplicate skip in the result: %s", r.Content)
	}
}

func TestNormalizeGoalText(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Dump the database":         "dump the database",
		"  dump   THE\tdatabase.  ": "dump the database",
		"Get RCE...":                "get rce",
	}
	for in, want := range cases {
		if got := NormalizeGoalText(in); got != want {
			t.Errorf("NormalizeGoalText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetGoalsWithVulnclass(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_goals")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": "find XSS", "vulnclass": "xss"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestSetGoalsEmptyText(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_goals")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty text")
	}
}

func TestSetGoalsBatch(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_goals")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"goals": [
			{"text": "find XSS"},
			{"text": "find SQLi"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestSetConstraintsHappyPath(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, ok := reg.Lookup("set_constraints")
	if !ok {
		t.Fatal("set_constraints not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": "do not brute force", "type": "deny"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "constraint created") {
		t.Errorf("expected constraint created, got: %s", r.Content)
	}
}

func TestSetConstraintsDefaultType(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_constraints")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": "no DoS"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success with default type, got error: %s", r.Content)
	}
}

func TestSetConstraintsInvalidType(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_constraints")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": "test", "type": "maybe"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(r.Content, "must be allow or deny") {
		t.Errorf("expected type validation, got: %s", r.Content)
	}
}

func TestSetConstraintsEmptyText(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_constraints")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"text": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty text")
	}
}

func TestSetConstraintsBatch(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("set_constraints")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"constraints": [
			{"text": "no brute force", "type": "deny"},
			{"text": "SQLi allowed", "type": "allow"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestProveGoalHappyPath(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[1] = &Node{ID: 1, Kind: KindGoal, State: "open"}
	ms.nodeMap[2] = &Node{ID: 2, Kind: KindFinding, State: "confirmed"}
	ms.listNodes = []Node{{ID: 1, Kind: KindGoal, State: "met"}}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, ok := reg.Lookup("prove_goal")
	if !ok {
		t.Fatal("prove_goal not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"goal_id": 1,
		"evidence_id": 2,
		"reason": "finding proves the goal"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "goal 1 marked met") {
		t.Errorf("expected goal met message, got: %s", r.Content)
	}
}

func TestProveGoalWithObservedFact(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[1] = &Node{ID: 1, Kind: KindGoal, State: "open"}
	ms.nodeMap[2] = &Node{ID: 2, Kind: KindFact, State: "confirmed", Payload: map[string]any{"confidence": "observed"}}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("prove_goal")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"goal_id": 1,
		"evidence_id": 2
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success for observed fact, got error: %s", r.Content)
	}
}

func TestProveGoalFactNotObserved(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[1] = &Node{ID: 1, Kind: KindGoal, State: "open"}
	ms.nodeMap[2] = &Node{ID: 2, Kind: KindFact, State: "confirmed", Payload: map[string]any{"confidence": "inferred"}}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("prove_goal")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"goal_id": 1,
		"evidence_id": 2
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for non-observed fact")
	}
	if !strings.Contains(r.Content, "confidence") {
		t.Errorf("expected confidence error, got: %s", r.Content)
	}
}

func TestProveGoalMissingIDs(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("prove_goal")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing IDs")
	}
	if !strings.Contains(r.Content, "required") {
		t.Errorf("expected required error, got: %s", r.Content)
	}
}

func TestProveGoalNotAGoal(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[1] = &Node{ID: 1, Kind: KindFact, State: "confirmed"}
	ms.nodeMap[2] = &Node{ID: 2, Kind: KindFinding, State: "confirmed"}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("prove_goal")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"goal_id": 1,
		"evidence_id": 2
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for non-goal node")
	}
	if !strings.Contains(r.Content, "must be a goal") {
		t.Errorf("expected goal validation, got: %s", r.Content)
	}
}

func TestProveGoalNotAFactOrFinding(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[1] = &Node{ID: 1, Kind: KindGoal, State: "open"}
	ms.nodeMap[2] = &Node{ID: 2, Kind: KindIntent, State: "done"}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("prove_goal")
	r, err := tool.Run(context.Background(), json.RawMessage(`{
		"goal_id": 1,
		"evidence_id": 2
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for non-fact/finding evidence")
	}
	if !strings.Contains(r.Content, "must be a fact/finding") {
		t.Errorf("expected evidence validation, got: %s", r.Content)
	}
}

func TestProveGoalDeadEvidence(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[1] = &Node{ID: 1, Kind: KindGoal, State: "open"}
	ms.nodeMap[2] = &Node{ID: 2, Kind: KindFinding, State: "dismissed"}
	ms.proveErr = ErrDeadEvidence
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("prove_goal")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"goal_id": 1, "evidence_id": 2}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError || !strings.Contains(r.Content, "dismissed or merged") {
		t.Errorf("expected dead evidence refusal, got: %s", r.Content)
	}
}

func TestGraphOverviewStoreError(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.overviewErr = fmt.Errorf("db down")
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)

	tool, _ := reg.Lookup("graph_overview")
	r, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error on store failure")
	}
	if !strings.Contains(r.Content, "db down") {
		t.Errorf("expected store error, got: %s", r.Content)
	}
}

func TestAddIntentSingleDuplicateReportsExisting(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.intentsResult = &IntentsResult{
		IDs:     []int64{0},
		Skipped: []SkippedIntent{{Index: 0, Summary: "Test XSS", ExistingID: 4, State: "running"}},
	}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)
	tool, _ := reg.Lookup("add_intent")

	r, _ := tool.Run(context.Background(), json.RawMessage(`{"summary": "Test XSS", "asset_ids": [3], "priority": 7}`))
	if r.IsError {
		t.Fatalf("duplicate should not be an error: %s", r.Content)
	}
	var out struct {
		IDs     []int64         `json:"ids"`
		Skipped []SkippedIntent `json:"skipped"`
		Note    string          `json:"note"`
	}
	if err := json.Unmarshal([]byte(r.Content), &out); err != nil {
		t.Fatalf("want JSON, got %s", r.Content)
	}
	if len(out.IDs) != 1 || out.IDs[0] != 0 || len(out.Skipped) != 1 || out.Skipped[0].ExistingID != 4 || out.Note == "" {
		t.Errorf("got %+v", out)
	}
	if len(ms.gotIntents) != 1 || ms.gotIntents[0].Priority != 7 || fmt.Sprint(ms.gotIntents[0].AssetIDs) != "[3]" {
		t.Errorf("store got %+v", ms.gotIntents)
	}
}

func TestAddIntentBatchMapsIndexesAndListsOpen(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.nodeMap[5] = &Node{ID: 5, Kind: KindIntent, State: "open"}
	ms.intentsResult = &IntentsResult{
		IDs:     []int64{20, 0, 0},
		Skipped: []SkippedIntent{{Index: 1, Summary: "b", ExistingID: 8, State: "open"}},
		Errors:  map[int]string{2: "asset_id 99 does not exist"},
		Open:    []IntentRef{{ID: 9, Summary: "planned elsewhere", Priority: 5}},
	}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)
	tool, _ := reg.Lookup("add_intent")

	r, _ := tool.Run(context.Background(), json.RawMessage(`{"intents": [
		{"summary": "bad parent", "parent_ids": [5]},
		{"summary": "a"},
		{"summary": "b"},
		{"summary": "c", "asset_ids": [99]}
	]}`))
	if r.IsError {
		t.Fatalf("unexpected error: %s", r.Content)
	}
	var out struct {
		IDs         []int64           `json:"ids"`
		Errors      map[string]string `json:"errors"`
		Skipped     []SkippedIntent   `json:"skipped"`
		OpenIntents []IntentRef       `json:"open_intents"`
		Note        string            `json:"note"`
	}
	if err := json.Unmarshal([]byte(r.Content), &out); err != nil {
		t.Fatalf("want JSON, got %s", r.Content)
	}
	if fmt.Sprint(out.IDs) != "[0 20 0 0]" {
		t.Errorf("ids = %v, want [0 20 0 0]", out.IDs)
	}
	if !strings.Contains(out.Errors["0"], "can only anchor on fact/finding") || out.Errors["3"] != "asset_id 99 does not exist" {
		t.Errorf("errors = %v", out.Errors)
	}
	if len(out.Skipped) != 1 || out.Skipped[0].Index != 2 {
		t.Errorf("skipped = %+v, want caller index 2", out.Skipped)
	}
	if len(out.OpenIntents) != 1 || out.OpenIntents[0].ID != 9 || !strings.Contains(out.Note, "open_intents") {
		t.Errorf("open_intents = %+v, note = %q", out.OpenIntents, out.Note)
	}
	if len(ms.gotIntents) != 3 {
		t.Errorf("store got %d intents, want the 3 valid ones", len(ms.gotIntents))
	}
}

func TestAddIntentSingleListsOpenIntents(t *testing.T) {
	t.Parallel()
	ms := newExplorationMockStore()
	ms.intentsResult = &IntentsResult{IDs: []int64{21}, Open: []IntentRef{{ID: 9, Summary: "x", Priority: 5}}}
	reg := NewRegistry()
	RegisterExplorationTools(reg, ms, 1)
	tool, _ := reg.Lookup("add_intent")

	r, _ := tool.Run(context.Background(), json.RawMessage(`{"summary": "new"}`))
	if r.IsError || !strings.Contains(r.Content, `"ids":[21]`) || !strings.Contains(r.Content, `"open_intents"`) {
		t.Errorf("got %s", r.Content)
	}
}

func TestIntentKey(t *testing.T) {
	t.Parallel()
	same := [][2]string{
		{"Test the /login form for SQL injection.", "test the login form, for SQL-injection"},
		{"  Enumerate   SMB shares ", "enumerate smb shares!"},
	}
	for _, p := range same {
		if IntentKey(p[0], nil) != IntentKey(p[1], nil) {
			t.Errorf("IntentKey(%q) != IntentKey(%q)", p[0], p[1])
		}
	}
	if IntentKey("Enumerate ports", []int64{2, 1, 2}) != IntentKey("enumerate ports", []int64{1, 2}) {
		t.Error("asset order or repeats changed the key")
	}
	different := [][2]string{
		{"Test login for SQLi", "Test search for SQLi"},
		{"Scan port 80", "Scan port 8080"},
	}
	for _, p := range different {
		if IntentKey(p[0], nil) == IntentKey(p[1], nil) {
			t.Errorf("IntentKey(%q) == IntentKey(%q)", p[0], p[1])
		}
	}
	if IntentKey("Enumerate ports", []int64{1}) == IntentKey("Enumerate ports", nil) {
		t.Error("asset ids ignored")
	}
}
