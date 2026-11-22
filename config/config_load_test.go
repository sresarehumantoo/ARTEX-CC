package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEngagementInvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, engagementFile), []byte("{bad json"), 0644)
	_, err := LoadEngagement(dir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadEngagementReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, engagementFile), 0755)
	_, err := LoadEngagement(dir)
	if err == nil {
		t.Error("expected error when file is a directory")
	}
}

func TestDefault(t *testing.T) {
	t.Parallel()
	d := Default()
	if d.Listen != "localhost:9400" {
		t.Errorf("Listen = %q, want localhost:9400", d.Listen)
	}
	if d.DataDir != "." {
		t.Errorf("DataDir = %q, want .", d.DataDir)
	}
}

func TestLoadOrDefaultMissing(t *testing.T) {
	t.Parallel()
	cfg, err := LoadOrDefault(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Listen != "localhost:9400" {
		t.Errorf("missing file should return default, got Listen=%q", cfg.Listen)
	}
}

func TestLoadOrDefaultValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"listen":":8080","pg_dsn":"postgres://localhost/test"}`), 0644)
	cfg, err := LoadOrDefault(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Listen != ":8080" {
		t.Errorf("Listen = %q, want :8080", cfg.Listen)
	}
	if cfg.PGDSN != "postgres://localhost/test" {
		t.Errorf("PGDSN = %q", cfg.PGDSN)
	}
	if cfg.DataDir != "." {
		t.Errorf("DataDir should keep default when not in JSON, got %q", cfg.DataDir)
	}
}

func TestLoadOrDefaultInvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte("{nope"), 0644)
	_, err := LoadOrDefault(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadOrDefaultReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.Mkdir(path, 0755)
	_, err := LoadOrDefault(path)
	if err == nil {
		t.Error("expected error when path is a directory")
	}
}

func TestLoadOrDefaultTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte("listen = \":8080\"\npg_dsn = \"postgres://localhost/test\"\n"), 0644)
	cfg, err := LoadOrDefault(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Listen != ":8080" {
		t.Errorf("Listen = %q, want :8080", cfg.Listen)
	}
	if cfg.PGDSN != "postgres://localhost/test" {
		t.Errorf("PGDSN = %q", cfg.PGDSN)
	}
	if cfg.DataDir != "." {
		t.Errorf("DataDir should keep default when not in TOML, got %q", cfg.DataDir)
	}
}

func TestLoadOrDefaultTOMLAllFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte(`listen = ":9000"
pg_dsn = "postgres://localhost/db"
proxy_addr = ":8788"
data_dir = "/var/data"
skill_dir = "/opt/skills"
`), 0644)
	cfg, err := LoadOrDefault(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Listen != ":9000" {
		t.Errorf("Listen = %q", cfg.Listen)
	}
	if cfg.PGDSN != "postgres://localhost/db" {
		t.Errorf("PGDSN = %q", cfg.PGDSN)
	}
	if cfg.ProxyAddr != ":8788" {
		t.Errorf("ProxyAddr = %q", cfg.ProxyAddr)
	}
	if cfg.DataDir != "/var/data" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	if cfg.SkillDir != "/opt/skills" {
		t.Errorf("SkillDir = %q", cfg.SkillDir)
	}
}

func TestLoadOrDefaultTOMLInvalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte("[invalid toml"), 0644)
	_, err := LoadOrDefault(path)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestLoadOrDefaultTOMLMissing(t *testing.T) {
	t.Parallel()
	cfg, err := LoadOrDefault(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Listen != "localhost:9400" {
		t.Errorf("missing TOML should return default, got Listen=%q", cfg.Listen)
	}
}
