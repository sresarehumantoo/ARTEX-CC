package api

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileStoreWritesOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	root := t.TempDir()
	fs := NewFileStore(root)
	if err := fs.WriteJSON("settings/llm.json", map[string]string{"api_key": "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := fs.AppendJSONL("conversations/c1/events.jsonl", map[string]string{"text": "secret"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{
		"settings":                      0o700,
		"settings/llm.json":             0o600,
		"conversations":                 0o700,
		"conversations/c1":              0o700,
		"conversations/c1/events.jsonl": 0o600,
	} {
		info, err := os.Stat(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode %o, want %o", path, got, want)
		}
	}
}

func TestFileStoreRefusesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.json"), []byte(`{"k":"v"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	fs := NewFileStore(root)
	var v map[string]string
	if err := fs.ReadJSON("link/secret.json", &v); err == nil {
		t.Fatalf("read through symlink escaped the root: %v", v)
	}
	if err := fs.WriteJSON("link/new.json", v); err == nil {
		t.Fatal("write through symlink escaped the root")
	}
	if _, err := os.Stat(filepath.Join(outside, "new.json")); err == nil {
		t.Fatal("file created outside the root")
	}
}
