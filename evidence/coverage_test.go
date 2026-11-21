package evidence

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilenameEdgeCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", "unnamed"},
		{".", "unnamed"},
		{"..", "unnamed"},
		{"normal.txt", "normal.txt"},
		{"path/to/file.txt", "file.txt"},
		{"back\\slash.txt", "back_slash.txt"},
		{"null\x00byte.txt", "null_byte.txt"},
	}
	for _, c := range cases {
		got := sanitizeFilename(c.in)
		if got != c.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSaveCreateDirError(t *testing.T) {
	t.Parallel()
	s := &Store{dir: "/dev/null/impossible"}
	_, err := s.Save(context.Background(), 1, 1, "other", "f.txt", strings.NewReader("x"))
	if err == nil {
		t.Error("expected error when directory creation fails")
	}
}

func TestSaveCopyError(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	r := &errReader{err: io.ErrUnexpectedEOF}
	_, err := s.Save(context.Background(), 1, 1, "other", "f.txt", r)
	if err == nil {
		t.Error("expected error on copy failure")
	}
}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadDataFileMissing(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 1, 1, "other", "f.txt", strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}

	dataPath := filepath.Join(s.dir, "1", ev.ID+"_f.txt")
	os.Remove(dataPath)

	_, err = s.Read(ctx, ev.ID)
	if err == nil {
		t.Error("expected error when data file is missing but meta exists")
	}
}

func TestDeleteNotFound(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	err := s.Delete(context.Background(), "ev_nonexistent")
	if err == nil {
		t.Error("expected error when deleting nonexistent evidence")
	}
}

func TestLocateCorruptMeta(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 1, 1, "other", "f.txt", strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}

	metaPath := filepath.Join(s.dir, "1", ev.ID+"_f.txt.meta.json")
	os.WriteFile(metaPath, []byte("{invalid json"), 0644)

	_, err = s.Get(ctx, ev.ID)
	if err == nil {
		t.Error("expected error for corrupt metadata")
	}
}

func TestListCorruptMetaSkipped(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 1, 1, "other", "good.txt", strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}

	badMeta := filepath.Join(s.dir, "1", "evbad_corrupt.txt.meta.json")
	os.WriteFile(badMeta, []byte("{invalid"), 0644)

	items, err := s.List(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != ev.ID {
		t.Errorf("expected 1 valid item, got %d", len(items))
	}
}

func TestListUnreadableMetaSkipped(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 1, 1, "other", "good.txt", strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}

	badMeta := filepath.Join(s.dir, "1", "evbad_unreadable.txt.meta.json")
	os.Mkdir(badMeta, 0755)

	items, err := s.List(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != ev.ID {
		t.Errorf("expected 1 valid item, got %d", len(items))
	}
}

func TestLocateMetaReadError(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 1, 1, "other", "f.txt", strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}

	metaPath := filepath.Join(s.dir, "1", ev.ID+"_f.txt.meta.json")
	os.Remove(metaPath)
	os.Mkdir(metaPath, 0755)

	_, err = s.Get(ctx, ev.ID)
	if err == nil {
		t.Error("expected error when meta file is unreadable")
	}
}

func TestSaveMetadataVerified(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	ev, err := s.Save(ctx, 42, 7, "poc_output", "proof.txt", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}

	metaPath := filepath.Join(s.dir, "42", ev.ID+"_proof.txt.meta.json")
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	var meta Evidence
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Kind != "poc_output" {
		t.Errorf("kind = %q", meta.Kind)
	}
	if meta.Size != 5 {
		t.Errorf("size = %d", meta.Size)
	}
}
