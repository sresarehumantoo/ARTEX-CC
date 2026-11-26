package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrepareDataDirTightensOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "data")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	files := map[string]os.FileMode{
		".artex.json":                       0o644,
		"auth.json":                         0o644,
		"auth.json.tmp":                     0o644,
		"README.md":                         0o644,
		"src/main.go":                       0o644,
		"scripts/run.sh":                    0o755,
		"conversations/c1/events.jsonl":     0o644,
		"evidence/1/ev1_req.txt":            0o644,
		"workspace/run.sh":                  0o755,
		"traffic/_ca/mitmproxy-ca-cert.pem": 0o644,
		"traffic/_index/legacy":             0o644,
	}
	dirs := map[string]os.FileMode{
		"":                 0o755,
		"conversations":    0o755,
		"conversations/c1": 0o755,
		"evidence":         0o755,
		"evidence/1":       0o755,
		"workspace":        0o755,
		"src":              0o755,
		"scripts":          0o755,
		"traffic":          0o755,
		"traffic/_ca":      0o755,
		"traffic/_index":   0o755,
	}
	for p := range files {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(outside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "conversations", "c1", "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(dir, "skills")); err != nil {
		t.Fatal(err)
	}
	chmodAll(t, dir, files)
	chmodAll(t, dir, dirs)
	if err := os.Chmod(outside, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := PrepareDataDir(dir); err != nil {
		t.Fatal(err)
	}

	for p, want := range map[string]os.FileMode{
		"":                                  0o755,
		".artex.json":                       0o600,
		"auth.json":                         0o600,
		"auth.json.tmp":                     0o600,
		"README.md":                         0o644,
		"src":                               0o755,
		"src/main.go":                       0o644,
		"scripts":                           0o755,
		"scripts/run.sh":                    0o755,
		"conversations":                     0o700,
		"conversations/c1":                  0o700,
		"conversations/c1/events.jsonl":     0o600,
		"evidence":                          0o700,
		"evidence/1":                        0o700,
		"evidence/1/ev1_req.txt":            0o600,
		"workspace":                         0o700,
		"workspace/run.sh":                  0o700,
		"traffic":                           0o755,
		"traffic/_ca":                       0o755,
		"traffic/_ca/mitmproxy-ca-cert.pem": 0o644,
		"traffic/_index/legacy":             0o644,
		permsMarkerFile:                     0o600,
	} {
		assertPerm(t, filepath.Join(dir, p), want)
	}
	assertPerm(t, outside, 0o644)
	assertPerm(t, filepath.Dir(outside), 0o755)

	if err := os.Chmod(filepath.Join(dir, "auth.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PrepareDataDir(dir); err != nil {
		t.Fatal(err)
	}
	assertPerm(t, filepath.Join(dir, "auth.json"), 0o644)
}

func TestPrepareDataDirLeavesAForeignDirAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	files := map[string]os.FileMode{
		"workspace/site/index.html": 0o644,
		"skills/notes.md":           0o644,
		"evidence/report.pdf":       0o644,
	}
	dirs := map[string]os.FileMode{
		"":               0o755,
		"workspace":      0o755,
		"workspace/site": 0o755,
		"skills":         os.ModeSetgid | 0o775,
		"evidence":       0o755,
	}
	for p := range files {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	chmodAll(t, dir, files)
	chmodAll(t, dir, dirs)

	if err := PrepareDataDir(dir); err != nil {
		t.Fatal(err)
	}

	for p, want := range files {
		assertPerm(t, filepath.Join(dir, p), want)
	}
	for p, want := range dirs {
		info, err := os.Stat(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode() & (os.ModePerm | os.ModeSetgid); got != want {
			t.Errorf("%s mode %v, want %v", p, got, want)
		}
	}
	assertPerm(t, filepath.Join(dir, permsMarkerFile), 0o600)
}

func TestPrepareDataDirTightensAnEarlierDataDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	t.Parallel()
	for _, artexFile := range []string{".artex.json", "owner-label.key"} {
		dir := t.TempDir()
		files := map[string]os.FileMode{
			artexFile:                   0o644,
			"workspace/site/index.html": 0o644,
		}
		for p := range files {
			if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, p), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		chmodAll(t, dir, files)
		chmodAll(t, dir, map[string]os.FileMode{"workspace": 0o755, "workspace/site": 0o755})

		if err := PrepareDataDir(dir); err != nil {
			t.Fatal(err)
		}

		for p, want := range map[string]os.FileMode{
			artexFile:                   0o600,
			"workspace":                 0o700,
			"workspace/site":            0o700,
			"workspace/site/index.html": 0o600,
			permsMarkerFile:             0o600,
		} {
			assertPerm(t, filepath.Join(dir, p), want)
		}
	}
}

func TestPrepareDataDirChmodsOnlyARootItCreates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "data")
	if err := PrepareDataDir(dir); err != nil {
		t.Fatal(err)
	}
	assertPerm(t, dir, 0o711)
	assertPerm(t, filepath.Join(dir, permsMarkerFile), 0o600)

	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, permsMarkerFile)); err != nil {
		t.Fatal(err)
	}
	if err := PrepareDataDir(dir); err != nil {
		t.Fatal(err)
	}
	assertPerm(t, dir, 0o700)
}

func chmodAll(t *testing.T, dir string, modes map[string]os.FileMode) {
	t.Helper()
	for p, mode := range modes {
		if err := os.Chmod(filepath.Join(dir, p), mode); err != nil {
			t.Fatal(err)
		}
	}
}

func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode %o, want %o", path, got, want)
	}
}
