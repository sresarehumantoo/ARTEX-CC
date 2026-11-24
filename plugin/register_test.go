package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/tools"
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
	if err := DiscoverAndRegister(reg, dir, 10*time.Second); err != nil {
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
	if err := DiscoverAndRegister(reg, dir, 10*time.Second); err != nil {
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
	if err := DiscoverAndRegister(reg, "/nonexistent/dir/xyz", 10*time.Second); err != nil {
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
	if err := DiscoverAndRegister(reg, dir, 10*time.Second); err != nil {
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
	if err := DiscoverAndRegister(reg, dir, 10*time.Second); err != nil {
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
	if err := DiscoverAndRegister(reg, dir, 10*time.Second); err != nil {
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
