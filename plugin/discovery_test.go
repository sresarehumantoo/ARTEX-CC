package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writePlugin(t *testing.T, dir, name, script string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscover_ValidPlugin(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "good-plugin", `#!/bin/sh
if [ "$1" = "--manifest" ]; then
  cat <<'EOF'
{"name":"good","description":"a good plugin","version":"1.0.0","tools":[{"name":"hello","description":"says hello","schema":{"type":"object","properties":{"name":{"type":"string"}}}}]}
EOF
  exit 0
fi
`)

	plugins, err := Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	p := plugins[0]
	if p.Manifest.Name != "good" {
		t.Errorf("name = %q, want %q", p.Manifest.Name, "good")
	}
	if p.Manifest.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", p.Manifest.Version, "1.0.0")
	}
	if len(p.Manifest.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(p.Manifest.Tools))
	}
	if p.Manifest.Tools[0].Name != "hello" {
		t.Errorf("tool name = %q, want %q", p.Manifest.Tools[0].Name, "hello")
	}
}

func TestDiscover_MissingDir(t *testing.T) {
	plugins, err := Discover(context.Background(), "/nonexistent/path/xyz")
	if err != nil {
		t.Fatal(err)
	}
	if plugins != nil {
		t.Fatalf("expected nil, got %v", plugins)
	}
}

func TestDiscover_SkipsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notexec")
	if err := os.WriteFile(path, []byte(`#!/bin/sh\necho '{}'`), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestDiscover_SkipsBadManifest(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "bad-json", `#!/bin/sh
echo "not json"
`)

	plugins, err := Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins (bad JSON), got %d", len(plugins))
	}
}

func TestDiscover_SkipsNoName(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "no-name", `#!/bin/sh
echo '{"description":"missing name","version":"1.0.0","tools":[]}'
`)

	plugins, err := Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins (no name), got %d", len(plugins))
	}
}

func TestDiscover_SkipsFailingBinary(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "fail-plugin", `#!/bin/sh
exit 1
`)

	plugins, err := Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins (exit 1), got %d", len(plugins))
	}
}

func TestDiscover_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	plugins, err := Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestDiscover_MultiplePlugins(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "alpha", `#!/bin/sh
echo '{"name":"alpha","version":"1.0.0","tools":[{"name":"a","description":"tool a"}]}'
`)
	writePlugin(t, dir, "beta", `#!/bin/sh
echo '{"name":"beta","version":"2.0.0","tools":[{"name":"b","description":"tool b"}]}'
`)

	plugins, err := Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
}
