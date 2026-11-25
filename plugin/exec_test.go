package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestExec_StdoutCapped(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "chatty", fmt.Sprintf(`#!/bin/sh
head -c %d /dev/zero
`, MaxOutputBytes+1))

	_, err := Exec(context.Background(), bin, Invocation{Tool: "x", Input: json.RawMessage(`{}`)}, 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("more than %d bytes", MaxOutputBytes)) {
		t.Fatalf("err = %v, want stdout cap error", err)
	}
}

func TestExec_StdoutCapKillsEndlessWriter(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "endless", `#!/bin/sh
exec cat /dev/zero
`)

	start := time.Now()
	_, err := Exec(context.Background(), bin, Invocation{Tool: "x", Input: json.RawMessage(`{}`)}, 30*time.Second)
	if err == nil || !strings.Contains(err.Error(), "bytes to stdout") {
		t.Fatalf("err = %v, want stdout cap error", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("took %s; the plugin should be killed once the cap is hit", elapsed)
	}
}

func TestExec_StderrQuoteCapped(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "noisy", `#!/bin/sh
head -c 1048576 /dev/zero | tr '\0' e >&2
exit 3
`)

	_, err := Exec(context.Background(), bin, Invocation{Tool: "x", Input: json.RawMessage(`{}`)}, 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), "more bytes)") {
		t.Fatalf("err = %v, want truncated stderr", err)
	}
	if len(err.Error()) > 2*MaxErrorTextBytes {
		t.Errorf("error is %d bytes, want it capped near %d", len(err.Error()), MaxErrorTextBytes)
	}
}

func TestExec_BadJSONQuoteCapped(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "garbage", `#!/bin/sh
head -c 1048576 /dev/zero | tr '\0' x
`)

	_, err := Exec(context.Background(), bin, Invocation{Tool: "x", Input: json.RawMessage(`{}`)}, 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), "parsing plugin output") || !strings.Contains(err.Error(), "more bytes)") {
		t.Fatalf("err = %v, want truncated parse error", err)
	}
	if len(err.Error()) > 2*MaxErrorTextBytes {
		t.Errorf("error is %d bytes, want it capped near %d", len(err.Error()), MaxErrorTextBytes)
	}
}

func TestExec_Success(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "echo-plugin", `#!/bin/sh
# read stdin, extract the "name" field, greet
INPUT=$(cat)
NAME=$(echo "$INPUT" | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -z "$NAME" ]; then
  NAME="world"
fi
cat <<EOF
{"content":[{"type":"text","text":"hello $NAME"}]}
EOF
`)

	inv := Invocation{
		Tool:  "greet",
		Input: json.RawMessage(`{"name":"alice"}`),
	}
	result, err := Exec(context.Background(), bin, inv, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Error("expected IsError=false")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
	if result.Content[0].Text != "hello alice" {
		t.Errorf("text = %q, want %q", result.Content[0].Text, "hello alice")
	}
}

func TestExec_ErrorResult(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "err-plugin", `#!/bin/sh
cat <<'EOF'
{"content":[{"type":"text","text":"something went wrong"}],"isError":true}
EOF
`)

	inv := Invocation{Tool: "fail", Input: json.RawMessage(`{}`)}
	result, err := Exec(context.Background(), bin, inv, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected IsError=true")
	}
}

func TestExec_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "crasher", `#!/bin/sh
echo "oh no" >&2
exit 1
`)

	inv := Invocation{Tool: "boom", Input: json.RawMessage(`{}`)}
	_, err := Exec(context.Background(), bin, inv, 10*time.Second)
	if err == nil {
		t.Fatal("expected error from non-zero exit")
	}
}

func TestExec_BadJSON(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "badjson", `#!/bin/sh
echo "not json at all"
`)

	inv := Invocation{Tool: "x", Input: json.RawMessage(`{}`)}
	_, err := Exec(context.Background(), bin, inv, 10*time.Second)
	if err == nil {
		t.Fatal("expected error for bad JSON output")
	}
}

func TestExec_Timeout(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "slow", `#!/bin/sh
trap 'exit 1' TERM
while true; do sleep 0.1; done
`)

	inv := Invocation{Tool: "x", Input: json.RawMessage(`{}`)}
	_, err := Exec(context.Background(), bin, inv, 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestExec_PassesInvocationJSON(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "mirror", `#!/bin/sh
INPUT=$(cat)
TOOL=$(echo "$INPUT" | grep -o '"tool":"[^"]*"' | head -1 | cut -d'"' -f4)
printf '{"content":[{"type":"text","text":"tool=%s"}]}' "$TOOL"
`)

	inv := Invocation{Tool: "mytool", Input: json.RawMessage(`{}`)}
	result, err := Exec(context.Background(), bin, inv, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "tool=mytool" {
		t.Errorf("got %q, want %q", result.Content[0].Text, "tool=mytool")
	}
}

func TestExec_MultiContent(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "multi", `#!/bin/sh
cat <<'EOF'
{"content":[{"type":"text","text":"line 1"},{"type":"text","text":"line 2"}]}
EOF
`)

	inv := Invocation{Tool: "x", Input: json.RawMessage(`{}`)}
	result, err := Exec(context.Background(), bin, inv, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 2 {
		t.Fatalf("expected 2 content items, got %d", len(result.Content))
	}
}

func TestExec_StructuredResults(t *testing.T) {
	dir := t.TempDir()
	bin := writePlugin(t, dir, "structured", `#!/bin/sh
cat <<'EOF'
{"content":[{"type":"text","text":"done"}],"assets":[{"type":"subdomain","domain":"a.example.com"}],"facts":[{"summary":"nginx 1.25"}],"findings":[{"vulnclass":"xss","severity":"low","summary":"reflected"}]}
EOF
`)

	result, err := Exec(context.Background(), bin, Invocation{Tool: "x", Input: json.RawMessage(`{}`)}, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 1 || len(result.Facts) != 1 || len(result.Findings) != 1 {
		t.Fatalf("got %d assets, %d facts, %d findings, want 1 each", len(result.Assets), len(result.Facts), len(result.Findings))
	}
	if string(result.Assets[0]) != `{"type":"subdomain","domain":"a.example.com"}` {
		t.Errorf("asset = %s", result.Assets[0])
	}
}
