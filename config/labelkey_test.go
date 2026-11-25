package config

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestOwnerLabelKeyPersisted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	key, err := OwnerLabelKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("key length %d, want 32", len(key))
	}
	info, err := os.Stat(filepath.Join(dir, ownerLabelKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file mode %o, want 600", perm)
	}
	again, err := OwnerLabelKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, again) {
		t.Error("key changed across loads")
	}
	other, err := OwnerLabelKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(key, other) {
		t.Error("separate data directories got the same key")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("data dir holds %d entries, want only the key file", len(entries))
	}
}

func TestOwnerLabelKeyConcurrentFirstStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const n = 8
	keys := make([][]byte, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			keys[i], errs[i] = OwnerLabelKey(dir)
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if !bytes.Equal(keys[i], keys[0]) {
			t.Fatal("concurrent first starts got different keys")
		}
	}
}

func TestOwnerLabelKeyRejectsCorruptFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ownerLabelKeyFile), []byte("nothex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OwnerLabelKey(dir); err == nil {
		t.Error("corrupt key file accepted")
	}
}
