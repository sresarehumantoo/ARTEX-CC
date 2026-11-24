package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const manifestTimeout = 10 * time.Second

type discovered struct {
	Path     string
	Manifest Manifest
}

// Discover scans dir for executable plugin binaries, calls each with
// --manifest, and returns the ones that respond with a valid manifest.
// If dir does not exist, it returns nil without error.
func Discover(ctx context.Context, dir string) ([]discovered, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading plugin dir: %w", err)
	}

	var plugins []discovered
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if !isExecutable(path) {
			continue
		}
		m, err := loadManifest(ctx, path)
		if err != nil {
			slog.Warn("skipping plugin", "path", path, "err", err)
			continue
		}
		plugins = append(plugins, discovered{Path: path, Manifest: m})
	}
	return plugins, nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}

func loadManifest(ctx context.Context, path string) (Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, manifestTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--manifest")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Manifest{}, fmt.Errorf("exec %s --manifest: %w (stderr: %s)", path, err, stderr.String())
	}

	var m Manifest
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		return Manifest{}, fmt.Errorf("parsing manifest from %s: %w", path, err)
	}
	if m.Name == "" {
		return Manifest{}, fmt.Errorf("manifest from %s has no name", path)
	}
	return m, nil
}
