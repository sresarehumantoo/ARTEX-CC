package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoadEngagement(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := EngagementConfig{
		TaskID:        42,
		ExplorationID: 7,
		Goal:          "test goal",
		Description:   "test description",
	}
	if err := SaveEngagement(dir, cfg); err != nil {
		t.Fatalf("SaveEngagement: %v", err)
	}

	got, err := LoadEngagement(dir)
	if err != nil {
		t.Fatalf("LoadEngagement: %v", err)
	}
	if got.TaskID != cfg.TaskID {
		t.Errorf("TaskID = %d, want %d", got.TaskID, cfg.TaskID)
	}
	if got.ExplorationID != cfg.ExplorationID {
		t.Errorf("ExplorationID = %d, want %d", got.ExplorationID, cfg.ExplorationID)
	}
	if got.Goal != cfg.Goal {
		t.Errorf("Goal = %q, want %q", got.Goal, cfg.Goal)
	}
	if got.Description != cfg.Description {
		t.Errorf("Description = %q, want %q", got.Description, cfg.Description)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg, err := LoadEngagement(dir)
	if err != nil {
		t.Fatalf("LoadEngagement on empty dir: %v", err)
	}
	if cfg.TaskID != 0 || cfg.ExplorationID != 0 {
		t.Errorf("expected zero config from missing file; got %+v", cfg)
	}
}

func TestRoundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	original := EngagementConfig{
		TaskID:        100,
		ExplorationID: 200,
		Goal:          "full roundtrip",
		Description:   "verify all fields survive",
	}
	if err := SaveEngagement(dir, original); err != nil {
		t.Fatalf("SaveEngagement: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, engagementFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("saved file is empty")
	}

	loaded, err := LoadEngagement(dir)
	if err != nil {
		t.Fatalf("LoadEngagement: %v", err)
	}
	if loaded != original {
		t.Errorf("roundtrip mismatch:\n  got:  %+v\n  want: %+v", loaded, original)
	}
}

func TestSaveEngagementOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	if err := SaveEngagement(dir, EngagementConfig{TaskID: 1}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, engagementFile))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode %o, want 600", got)
	}
}
