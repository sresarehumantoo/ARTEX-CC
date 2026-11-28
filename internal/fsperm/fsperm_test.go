package fsperm

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTightenSkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(dir, "tree")
	if err := os.MkdirAll(filepath.Join(tree, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "sub", "file.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tree, "link")); err != nil {
		t.Fatal(err)
	}
	for path, perm := range map[string]os.FileMode{
		tree:                                   0o755,
		filepath.Join(tree, "sub"):             0o755,
		filepath.Join(tree, "sub", "file.txt"): 0o644,
		outside:                                0o644,
	} {
		if err := os.Chmod(path, perm); err != nil {
			t.Fatal(err)
		}
	}
	if !Tighten(tree) {
		t.Fatal("Tighten reported failure")
	}
	for path, want := range map[string]os.FileMode{
		tree:                                   0o700,
		filepath.Join(tree, "sub"):             0o700,
		filepath.Join(tree, "sub", "file.txt"): 0o600,
		outside:                                0o644,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode %o, want %o", path, got, want)
		}
	}

	link := filepath.Join(dir, "toplink")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if !Tighten(link) || mustPerm(t, outside) != 0o644 {
		t.Fatal("Tighten followed a top-level symlink")
	}
}

func mustPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
