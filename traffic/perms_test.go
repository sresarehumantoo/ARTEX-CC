package traffic

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode %o, want %o", path, got, want)
	}
}

func TestSpilledBlobIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	tr := openTestTraffic(t)
	sb := tr.spill(bytes.Repeat([]byte("a"), maxInlineBody+1), "text/plain")
	if sb.hash == "" {
		t.Fatal("body was not spilled")
	}
	bucket := filepath.Join(tr.dir, "_blobs", "sha256", sb.hash[:2])
	assertMode(t, bucket, 0o700)
	assertMode(t, filepath.Join(bucket, sb.hash+".bin"), 0o600)
	assertMode(t, filepath.Join(tr.dir, "_blobs"), 0o700)
	assertMode(t, filepath.Join(tr.dir, "_index"), 0o700)

	_, h := tr.spillRawDir(bytes.Repeat([]byte("b"), DefaultSOCKS5PreviewBytes+1), 0)
	if h == "" {
		t.Fatal("raw data was not spilled")
	}
	assertMode(t, filepath.Join(tr.dir, "_blobs", "sha256", h[:2], h+".bin"), 0o600)
}

func TestOpenTightensExistingPerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	dir := t.TempDir()
	bucket := filepath.Join(dir, "_blobs", "sha256", "ab")
	if err := os.MkdirAll(bucket, 0o755); err != nil {
		t.Fatal(err)
	}
	blob := filepath.Join(bucket, "ab.bin")
	if err := os.WriteFile(blob, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{filepath.Join(dir, "_blobs"), filepath.Join(dir, "_blobs", "sha256"), bucket} {
		if err := os.Chmod(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(blob, 0o644); err != nil {
		t.Fatal(err)
	}

	tr, err := Open(dir, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tr.Close() })

	assertMode(t, filepath.Join(dir, "_blobs"), 0o700)
	assertMode(t, filepath.Join(dir, "_blobs", "sha256"), 0o700)
	assertMode(t, bucket, 0o700)
	assertMode(t, blob, 0o600)
}
