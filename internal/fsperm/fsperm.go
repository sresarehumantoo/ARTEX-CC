// Package fsperm restricts files written by earlier versions to their owner.
package fsperm

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
)

// TightenOnce restricts every entry of dir except those named in skip, then
// writes marker so later calls return at once. The marker is written only
// after a pass that reached and restricted everything.
func TightenOnce(dir, marker string, skip ...string) {
	if _, err := os.Stat(marker); err == nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Warn("failed to restrict file permissions", "path", dir, "err", err)
		return
	}
	ok := true
	for _, e := range entries {
		if !slices.Contains(skip, e.Name()) && !Tighten(filepath.Join(dir, e.Name())) {
			ok = false
		}
	}
	if ok {
		if err := os.WriteFile(marker, nil, 0o600); err != nil {
			slog.Warn("failed to record permission tightening", "path", marker, "err", err)
		}
	}
}

// Tighten removes group and other access from one tree, skipping symlinks and
// special files. It reports whether every entry was reached and restricted.
// The walk runs inside an os.Root on the parent directory, so an entry swapped
// for a symlink mid-walk cannot redirect a chmod outside it.
func Tighten(path string) bool {
	parent, base := filepath.Dir(path), filepath.Base(path)
	root, err := os.OpenRoot(parent)
	if err != nil {
		slog.Warn("failed to restrict file permissions", "path", path, "err", err)
		return false
	}
	defer root.Close()
	if info, err := root.Lstat(base); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return true
	}
	ok := true
	_ = fs.WalkDir(root.FS(), base, func(rel string, d fs.DirEntry, err error) error {
		rel = filepath.FromSlash(rel)
		if err != nil {
			slog.Warn("failed to restrict file permissions", "path", filepath.Join(parent, rel), "err", err)
			ok = false
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			slog.Warn("failed to restrict file permissions", "path", filepath.Join(parent, rel), "err", err)
			ok = false
			return nil
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			if err := root.Chmod(rel, perm&^0o077); err != nil {
				slog.Warn("failed to restrict file permissions", "path", filepath.Join(parent, rel), "err", err)
				ok = false
			}
		}
		return nil
	})
	return ok
}
