package config

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestIdentityKeyPersistedAndSeparate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	key, err := IdentityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("key length %d, want 32", len(key))
	}
	info, err := os.Stat(filepath.Join(dir, IdentityKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file mode %o, want 600", perm)
	}
	again, err := IdentityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, again) {
		t.Error("key changed across loads")
	}
	label, err := OwnerLabelKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(key, label) {
		t.Error("identity key equals owner label key")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("data dir holds %d entries, want the two key files", len(entries))
	}
}

func TestIdentityKeyConcurrentFirstStart(t *testing.T) {
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
			keys[i], errs[i] = IdentityKey(dir)
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

func TestIdentityKeyRejectsCorruptFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, IdentityKeyFile), []byte("abcd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := IdentityKey(dir); err == nil {
		t.Error("corrupt key file accepted")
	}
}
