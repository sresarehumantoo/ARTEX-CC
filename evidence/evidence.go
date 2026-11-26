package evidence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var validKinds = map[string]bool{
	"screenshot":       true,
	"poc_output":       true,
	"request_response": true,
	"config_file":      true,
	"log_extract":      true,
	"other":            true,
}

// ValidKind reports whether kind is a recognized evidence type.
func ValidKind(kind string) bool { return validKinds[kind] }

// Evidence is the metadata for one stored artifact.
type Evidence struct {
	ID           string `json:"id"`
	EngagementID int64  `json:"engagement_id"`
	FindingID    int64  `json:"finding_id"`
	Kind         string `json:"kind"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	CreatedAt    string `json:"created_at"`
}

// Store manages evidence artifacts on disk under a base directory.
type Store struct {
	dir string
}

// NewStore returns a Store rooted at dataDir/evidence.
func NewStore(dataDir string) *Store {
	return &Store{dir: filepath.Join(dataDir, "evidence")}
}

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ev" + hex.EncodeToString(b), nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		name = "unnamed"
	}
	return name
}

// Save writes an evidence artifact to disk and returns its metadata.
func (s *Store) Save(_ context.Context, engagementID, findingID int64, kind, filename string, data io.Reader) (Evidence, error) {
	if !validKinds[kind] {
		return Evidence{}, fmt.Errorf("invalid evidence kind %q", kind)
	}
	filename = sanitizeFilename(filename)

	id, err := generateID()
	if err != nil {
		return Evidence{}, fmt.Errorf("generate id: %w", err)
	}

	engDir := filepath.Join(s.dir, strconv.FormatInt(engagementID, 10))
	if err := os.MkdirAll(engDir, 0o700); err != nil {
		return Evidence{}, fmt.Errorf("create evidence directory: %w", err)
	}

	baseName := id + "_" + filename
	dataPath := filepath.Join(engDir, baseName)

	f, err := os.OpenFile(dataPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return Evidence{}, fmt.Errorf("create evidence file: %w", err)
	}

	h := sha256.New()
	w := io.MultiWriter(f, h)
	n, copyErr := io.Copy(w, data)
	if closeErr := f.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		os.Remove(dataPath)
		return Evidence{}, fmt.Errorf("write evidence: %w", copyErr)
	}

	ev := Evidence{
		ID:           id,
		EngagementID: engagementID,
		FindingID:    findingID,
		Kind:         kind,
		Filename:     filename,
		Size:         n,
		SHA256:       hex.EncodeToString(h.Sum(nil)),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	metaPath := dataPath + ".meta.json"
	metaData, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		os.Remove(dataPath)
		return Evidence{}, fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0o600); err != nil {
		os.Remove(dataPath)
		return Evidence{}, fmt.Errorf("write metadata: %w", err)
	}

	return ev, nil
}

// Get retrieves the metadata for an evidence artifact by ID.
func (s *Store) Get(_ context.Context, id string) (Evidence, error) {
	ev, _, err := s.locate(id)
	return ev, err
}

// Read opens the evidence artifact data for reading.
func (s *Store) Read(_ context.Context, id string) (io.ReadCloser, error) {
	ev, metaPath, err := s.locate(id)
	if err != nil {
		return nil, err
	}
	dataPath := strings.TrimSuffix(metaPath, ".meta.json")
	f, err := os.Open(dataPath)
	if err != nil {
		return nil, fmt.Errorf("evidence %s (%s): %w", ev.ID, ev.Filename, err)
	}
	return f, nil
}

// List returns evidence for an engagement, optionally filtered by finding.
func (s *Store) List(_ context.Context, engagementID int64, findingID *int64) ([]Evidence, error) {
	engDir := filepath.Join(s.dir, strconv.FormatInt(engagementID, 10))
	matches, err := filepath.Glob(filepath.Join(engDir, "*.meta.json"))
	if err != nil {
		return nil, err
	}

	var out []Evidence
	for _, m := range matches {
		raw, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		var ev Evidence
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		if findingID != nil && ev.FindingID != *findingID {
			continue
		}
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out, nil
}

// Delete removes an evidence artifact and its metadata from disk.
func (s *Store) Delete(_ context.Context, id string) error {
	_, metaPath, err := s.locate(id)
	if err != nil {
		return err
	}
	dataPath := strings.TrimSuffix(metaPath, ".meta.json")
	os.Remove(dataPath)
	os.Remove(metaPath)
	return nil
}

func (s *Store) locate(id string) (Evidence, string, error) {
	engDirs, err := filepath.Glob(filepath.Join(s.dir, "*"))
	if err != nil {
		return Evidence{}, "", fmt.Errorf("scan evidence directory: %w", err)
	}
	for _, d := range engDirs {
		pattern := filepath.Join(d, id+"_*.meta.json")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(matches) == 0 {
			continue
		}
		raw, err := os.ReadFile(matches[0])
		if err != nil {
			return Evidence{}, "", err
		}
		var ev Evidence
		if err := json.Unmarshal(raw, &ev); err != nil {
			return Evidence{}, "", err
		}
		return ev, matches[0], nil
	}
	return Evidence{}, "", fmt.Errorf("evidence %q not found", id)
}
