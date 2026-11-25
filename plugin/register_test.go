package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestDiscoverAndRegister(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "demo", `#!/bin/sh
if [ "$1" = "--manifest" ]; then
  cat <<'EOF'
{"name":"demo","description":"demo plugin","version":"1.0.0","tools":[{"name":"ping","description":"pings","schema":{"type":"object","properties":{"target":{"type":"string"}}}},{"name":"scan","description":"scans","schema":{"type":"object"}}]}
EOF
  exit 0
fi
INPUT=$(cat)
TOOL=$(echo "$INPUT" | grep -o '"tool":"[^"]*"' | head -1 | cut -d'"' -f4)
printf '{"content":[{"type":"text","text":"ran %s"}]}' "$TOOL"
`)

	reg := tools.NewRegistry()
	if err := DiscoverAndRegister(reg, dir, 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}

	allTools := reg.Tools()
	if len(allTools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(allTools))
	}

	ping, ok := reg.Lookup("demo_ping")
	if !ok {
		t.Fatal("tool demo_ping not found")
	}
	if ping.Description != "pings" {
		t.Errorf("description = %q, want %q", ping.Description, "pings")
	}

	scan, ok := reg.Lookup("demo_scan")
	if !ok {
		t.Fatal("tool demo_scan not found")
	}
	if scan.Description != "scans" {
		t.Errorf("description = %q, want %q", scan.Description, "scans")
	}
}

func TestDiscoverAndRegister_ToolExecution(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "greeter", `#!/bin/sh
if [ "$1" = "--manifest" ]; then
  echo '{"name":"greeter","description":"greets","version":"1.0.0","tools":[{"name":"hi","description":"say hi","schema":{"type":"object","properties":{"who":{"type":"string"}}}}]}'
  exit 0
fi
INPUT=$(cat)
WHO=$(echo "$INPUT" | grep -o '"who":"[^"]*"' | head -1 | cut -d'"' -f4)
printf '{"content":[{"type":"text","text":"hi %s"}]}' "$WHO"
`)

	reg := tools.NewRegistry()
	if err := DiscoverAndRegister(reg, dir, 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}

	tool, ok := reg.Lookup("greeter_hi")
	if !ok {
		t.Fatal("tool greeter_hi not found")
	}

	result, err := tool.Run(t.Context(), json.RawMessage(`{"who":"bob"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "hi bob" {
		t.Errorf("content = %q, want %q", result.Content, "hi bob")
	}
	if result.IsError {
		t.Error("expected IsError=false")
	}
}

func TestDiscoverAndRegister_MissingDir(t *testing.T) {
	reg := tools.NewRegistry()
	if err := DiscoverAndRegister(reg, "/nonexistent/dir/xyz", 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	if len(reg.Tools()) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(reg.Tools()))
	}
}

func TestDiscoverAndRegister_BadSchemaSkipped(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "badschema", `#!/bin/sh
echo '{"name":"bs","version":"1.0.0","tools":[{"name":"ok","description":"ok","schema":{"type":"object"}},{"name":"bad","description":"bad","schema":"not an object"}]}'
`)

	reg := tools.NewRegistry()
	if err := DiscoverAndRegister(reg, dir, 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Lookup("bs_ok"); !ok {
		t.Error("tool bs_ok should be registered")
	}
	if _, ok := reg.Lookup("bs_bad"); ok {
		t.Error("tool bs_bad should NOT be registered (bad schema)")
	}
}

func TestDiscoverAndRegister_EmptySchema(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "noschem", `#!/bin/sh
echo '{"name":"ns","version":"1.0.0","tools":[{"name":"bare","description":"no schema"}]}'
`)

	reg := tools.NewRegistry()
	if err := DiscoverAndRegister(reg, dir, 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	tool, ok := reg.Lookup("ns_bare")
	if !ok {
		t.Fatal("tool ns_bare not found")
	}
	if tool.Schema == nil {
		t.Fatal("expected default schema, got nil")
	}
	if tool.Schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", tool.Schema["type"])
	}
}

func TestDiscoverAndRegister_MultiContent(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "multi", `#!/bin/sh
if [ "$1" = "--manifest" ]; then
  echo '{"name":"multi","version":"1.0.0","tools":[{"name":"lines","description":"multi line","schema":{"type":"object"}}]}'
  exit 0
fi
cat <<'EOF'
{"content":[{"type":"text","text":"line 1"},{"type":"text","text":"line 2"}]}
EOF
`)

	reg := tools.NewRegistry()
	if err := DiscoverAndRegister(reg, dir, 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	tool, _ := reg.Lookup("multi_lines")
	result, err := tool.Run(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "line 1\nline 2" {
		t.Errorf("content = %q, want %q", result.Content, "line 1\nline 2")
	}
}

// graphStore implements only the Store methods the persistence path reaches;
// any other call panics on the nil embedded interface.
type graphStore struct {
	tools.Store
	outOfScope string
	assets     []tools.AssetInput
	facts      []map[string]any
	findings   []tools.FindingInput
}

func (s *graphStore) IsInScope(_ context.Context, _ int64, target string) (bool, string, string, error) {
	if target == s.outOfScope {
		return false, "no matching rule", "", nil
	}
	return true, "matched", "*.example.com", nil
}

func (s *graphStore) InsertAsset(_ context.Context, _ int64, item tools.AssetInput) (int64, error) {
	s.assets = append(s.assets, item)
	return int64(len(s.assets)), nil
}

func (s *graphStore) AddNode(_ context.Context, _ int64, kind string, payload map[string]any, _ int, _, _ string, _ []int64) (int64, error) {
	if kind == tools.KindFact {
		s.facts = append(s.facts, payload)
	}
	return 100, nil
}

func (s *graphStore) FindPotentialDuplicates(context.Context, int64, string, string, []int64) ([]tools.Finding, error) {
	return nil, nil
}

func (s *graphStore) AddFinding(_ context.Context, f tools.FindingInput) (int64, int64, error) {
	s.findings = append(s.findings, f)
	return int64(len(s.findings)), 200, nil
}

func registerWithStore(t *testing.T, store tools.Store, name, output string) tools.Tool {
	t.Helper()
	dir := t.TempDir()
	writePlugin(t, dir, name, `#!/bin/sh
if [ "$1" = "--manifest" ]; then
  echo '{"name":"`+name+`","version":"1.0.0","tools":[{"name":"run","description":"run","schema":{"type":"object"}}]}'
  exit 0
fi
cat <<'EOF'
`+output+`
EOF
`)
	reg := tools.NewRegistry()
	tools.RegisterAll(reg, store, 1, 1)
	if err := DiscoverAndRegister(reg, dir, 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	tool, ok := reg.Lookup(name + "_run")
	if !ok {
		t.Fatalf("tool %s_run not found", name)
	}
	return tool
}

func TestDiscoverAndRegister_StructuredResults(t *testing.T) {
	store := &graphStore{outOfScope: "evil.test"}
	tool := registerWithStore(t, store, "recon", `{"content":[{"type":"text","text":"scan complete"}],
"assets":[{"type":"subdomain","domain":"a.example.com"},{"type":"ip","ip":"10.0.0.5"},{"type":"root_domain","domain":"evil.test"}],
"facts":[{"summary":"nginx 1.25 on a.example.com","source":"tool_output"}],
"findings":[{"vulnclass":"xss","severity":"low","summary":"reflected q param","evidence":"<script>"}]}`)

	result, err := tool.Run(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}
	want := "scan complete\n" +
		"inserted 2 assets, 1 fact, 1 finding; skipped 1 out of scope\n" +
		"skipped asset 2: out of scope (evil.test): no matching rule"
	if result.Content != want {
		t.Errorf("content =\n%s\nwant\n%s", result.Content, want)
	}
	if len(store.assets) != 2 || store.assets[0].Domain != "a.example.com" || store.assets[1].IP != "10.0.0.5" {
		t.Errorf("assets = %+v", store.assets)
	}
	if len(store.facts) != 1 || store.facts[0]["summary"] != "nginx 1.25 on a.example.com" || store.facts[0]["source"] != "tool_output" {
		t.Errorf("facts = %+v", store.facts)
	}
	if len(store.findings) != 1 || store.findings[0].VulnClass != "xss" || store.findings[0].TaskID != 1 || store.findings[0].ExplorationID != 1 {
		t.Errorf("findings = %+v", store.findings)
	}
}

func TestDiscoverAndRegister_InvalidItemsReported(t *testing.T) {
	store := &graphStore{}
	tool := registerWithStore(t, store, "sloppy", `{"content":[{"type":"text","text":"ok"}],
"assets":[{"domain":"no-type.example.com"}],
"facts":[{"summary":""}],
"findings":[{"vulnclass":"sqli","severity":"urgent","summary":"x"},{"vulnclass":"sqli","severity":"high","summary":"blind"}]}`)

	result, err := tool.Run(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("invalid items must not fail the call: %s", result.Content)
	}
	for _, s := range []string{
		"inserted 0 assets, 0 facts, 1 finding",
		"failed asset 0: type is required",
		"failed fact 0: summary must not be empty",
		`failed finding 0: severity "urgent" is not valid`,
	} {
		if !strings.Contains(result.Content, s) {
			t.Errorf("content missing %q:\n%s", s, result.Content)
		}
	}
	if len(store.assets) != 0 || len(store.facts) != 0 || len(store.findings) != 1 {
		t.Errorf("persisted %d assets, %d facts, %d findings", len(store.assets), len(store.facts), len(store.findings))
	}
}

func TestDiscoverAndRegister_ErrorResultNotPersisted(t *testing.T) {
	store := &graphStore{}
	tool := registerWithStore(t, store, "broken", `{"content":[{"type":"text","text":"target unreachable"}],"isError":true,
"assets":[{"type":"subdomain","domain":"a.example.com"}]}`)

	result, err := tool.Run(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || result.Content != "target unreachable" {
		t.Errorf("result = %+v", result)
	}
	if len(store.assets) != 0 {
		t.Errorf("error result persisted %d assets", len(store.assets))
	}
}

func TestDiscoverAndRegister_TooManyItemsRejected(t *testing.T) {
	store := &graphStore{}
	assets := make([]string, MaxResultItems+1)
	for i := range assets {
		assets[i] = fmt.Sprintf(`{"type":"subdomain","domain":"h%d.example.com"}`, i)
	}
	tool := registerWithStore(t, store, "flood", `{"content":[{"type":"text","text":"done"}],"assets":[`+strings.Join(assets, ",")+`]}`)

	result, err := tool.Run(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("done\npersisted nothing: result has %d structured items, max %d", MaxResultItems+1, MaxResultItems)
	if result.Content != want {
		t.Errorf("content =\n%s\nwant\n%s", result.Content, want)
	}
	if len(store.assets) != 0 {
		t.Errorf("persisted %d assets, want 0", len(store.assets))
	}
}

func TestDiscoverAndRegister_ProblemLinesCapped(t *testing.T) {
	store := &graphStore{}
	facts := make([]string, MaxProblemLines+5)
	for i := range facts {
		facts[i] = `{"summary":""}`
	}
	tool := registerWithStore(t, store, "noisy", `{"content":[{"type":"text","text":"done"}],"facts":[`+strings.Join(facts, ",")+`]}`)

	result, err := tool.Run(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(result.Content, "\n")
	if len(lines) != MaxProblemLines+3 {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), MaxProblemLines+3, result.Content)
	}
	if lines[1] != "inserted 0 assets, 0 facts, 0 findings" {
		t.Errorf("summary = %q", lines[1])
	}
	if last := lines[len(lines)-1]; last != "... and 5 more problems" {
		t.Errorf("last line = %q", last)
	}
}

type cancelingStore struct {
	graphStore
	cancel context.CancelFunc
}

func (s *cancelingStore) InsertAsset(ctx context.Context, taskID int64, item tools.AssetInput) (int64, error) {
	s.cancel()
	return s.graphStore.InsertAsset(ctx, taskID, item)
}

func TestPersistResults_StopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	store := &cancelingStore{cancel: cancel}
	reg := tools.NewRegistry()
	tools.RegisterAll(reg, store, 1, 1)

	summary := persistResults(ctx, reg, Result{
		Assets: []json.RawMessage{
			json.RawMessage(`{"type":"subdomain","domain":"a.example.com"}`),
			json.RawMessage(`{"type":"subdomain","domain":"b.example.com"}`),
		},
		Facts: []json.RawMessage{json.RawMessage(`{"summary":"x"}`)},
	})
	want := "inserted 1 asset, 0 facts, 0 findings; stopped (context canceled) with 2 items not attempted"
	if summary != want {
		t.Errorf("summary =\n%s\nwant\n%s", summary, want)
	}
	if len(store.assets) != 1 {
		t.Errorf("persisted %d assets, want 1", len(store.assets))
	}
}
