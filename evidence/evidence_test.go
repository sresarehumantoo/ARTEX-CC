package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(t.TempDir())
}

func TestSaveAndGet(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 1, 10, "screenshot", "login.png", strings.NewReader("fake-png"))
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if ev.EngagementID != 1 {
		t.Fatalf("expected engagement_id=1, got %d", ev.EngagementID)
	}
	if ev.FindingID != 10 {
		t.Fatalf("expected finding_id=10, got %d", ev.FindingID)
	}
	if ev.Kind != "screenshot" {
		t.Fatalf("expected kind=screenshot, got %s", ev.Kind)
	}
	if ev.Filename != "login.png" {
		t.Fatalf("expected filename=login.png, got %s", ev.Filename)
	}
	if ev.Size != 8 {
		t.Fatalf("expected size=8, got %d", ev.Size)
	}

	got, err := s.Get(ctx, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != ev.ID {
		t.Fatalf("expected id=%s, got %s", ev.ID, got.ID)
	}
	if got.SHA256 != ev.SHA256 {
		t.Fatalf("sha256 mismatch: %s vs %s", ev.SHA256, got.SHA256)
	}
}

func TestSHA256Integrity(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	content := "proof of concept output data"
	ev, err := s.Save(ctx, 1, 5, "poc_output", "poc.txt", strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256([]byte(content))
	expected := hex.EncodeToString(h[:])
	if ev.SHA256 != expected {
		t.Fatalf("sha256 mismatch: expected %s, got %s", expected, ev.SHA256)
	}
}

func TestRead(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	content := "request-response data"
	ev, err := s.Save(ctx, 2, 3, "request_response", "req.txt", strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}

	rc, err := s.Read(ctx, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("expected %q, got %q", content, string(got))
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	for _, f := range []struct {
		engID, findID int64
		name          string
	}{
		{1, 10, "a.txt"},
		{1, 10, "b.txt"},
		{1, 20, "c.txt"},
		{2, 30, "d.txt"},
	} {
		if _, err := s.Save(ctx, f.engID, f.findID, "other", f.name, strings.NewReader("x")); err != nil {
			t.Fatal(err)
		}
	}

	all, err := s.List(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 items for engagement 1, got %d", len(all))
	}

	findID := int64(10)
	filtered, err := s.List(ctx, 1, &findID)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 items for finding 10, got %d", len(filtered))
	}

	other, err := s.List(ctx, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 {
		t.Fatalf("expected 1 item for engagement 2, got %d", len(other))
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 1, 1, "log_extract", "app.log", strings.NewReader("log data"))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(ctx, ev.ID); err != nil {
		t.Fatal(err)
	}

	_, err = s.Get(ctx, ev.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestInvalidKind(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	_, err := s.Save(context.Background(), 1, 1, "invalid_kind", "f.txt", strings.NewReader("x"))
	if err == nil {
		t.Fatal("expected error for invalid kind")
	}
}

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 1, 1, "other", "../../etc/passwd", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ev.Filename, "/") || strings.Contains(ev.Filename, "..") {
		t.Fatalf("filename not sanitized: %s", ev.Filename)
	}
}

func TestFileOnDisk(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 42, 7, "config_file", "web.conf", strings.NewReader("config data"))
	if err != nil {
		t.Fatal(err)
	}

	dataPath := filepath.Join(s.dir, "42", ev.ID+"_web.conf")
	if _, err := os.Stat(dataPath); err != nil {
		t.Fatalf("expected data file at %s: %v", dataPath, err)
	}
	metaPath := dataPath + ".meta.json"
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("expected metadata file at %s: %v", metaPath, err)
	}
}

func TestGetNotFound(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	_, err := s.Get(context.Background(), "ev_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestListEmptyEngagement(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	items, err := s.List(context.Background(), 999, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestValidKind(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"screenshot", "poc_output", "request_response", "config_file", "log_extract", "other"} {
		if !ValidKind(k) {
			t.Errorf("expected %q to be valid", k)
		}
	}
	if ValidKind("bogus") {
		t.Error("expected bogus to be invalid")
	}
}

func TestSaveIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	t.Parallel()
	s := testStore(t)
	ev, err := s.Save(context.Background(), 1, 1, "request_response", "req.txt", strings.NewReader("Cookie: session=x"))
	if err != nil {
		t.Fatal(err)
	}
	dataPath := filepath.Join(s.dir, "1", ev.ID+"_req.txt")
	for path, want := range map[string]os.FileMode{
		filepath.Join(s.dir, "1"): 0o700,
		dataPath:                  0o600,
		dataPath + ".meta.json":   0o600,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("%s mode %o, want %o", path, got, want)
		}
	}
}
