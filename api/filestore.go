package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type FileStore struct {
	root string
	mu   sync.Mutex
}

func NewFileStore(root string) *FileStore {
	return &FileStore{root: root}
}

func (fs *FileStore) resolve(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute path not allowed: %q", path)
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return "", fmt.Errorf("path traversal not allowed: %q", path)
		}
	}
	return filepath.FromSlash(path), nil
}

// openRoot confines every operation to the data directory, so a symlink
// inside it cannot redirect a read or write elsewhere.
func (fs *FileStore) openRoot(create bool) (*os.Root, error) {
	if create {
		if err := os.MkdirAll(fs.root, 0o700); err != nil { //nolint:gosec // fs.root is the operator's --data directory
			return nil, err
		}
	}
	return os.OpenRoot(fs.root)
}

func (fs *FileStore) ReadJSON(path string, v any) error {
	p, err := fs.resolve(path)
	if err != nil {
		return err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	root, err := fs.openRoot(false)
	if err != nil {
		return err
	}
	defer root.Close()
	data, err := root.ReadFile(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (fs *FileStore) WriteJSON(path string, v any) error {
	p, err := fs.resolve(path)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	root, err := fs.openRoot(true)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := root.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := root.Rename(tmp, p); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	return nil
}

func (fs *FileStore) AppendJSONL(path string, v any) error {
	p, err := fs.resolve(path)
	if err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	root, err := fs.openRoot(true)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := root.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ReadJSONL returns the lines after the first `after` lines. A missing file yields no lines.
func (fs *FileStore) ReadJSONL(path string, after int64) ([]json.RawMessage, error) {
	p, err := fs.resolve(path)
	if err != nil {
		return nil, err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	root, err := fs.openRoot(false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []json.RawMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 16<<20)
	var n int64
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		n++
		if n <= after {
			continue
		}
		out = append(out, json.RawMessage(append([]byte(nil), line...)))
	}
	return out, sc.Err()
}

func (fs *FileStore) ListDir(path string) ([]string, error) {
	p, err := fs.resolve(path)
	if err != nil {
		return nil, err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	root, err := fs.openRoot(false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.Open(p)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

func (fs *FileStore) Delete(path string) error {
	p, err := fs.resolve(path)
	if err != nil {
		return err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	root, err := fs.openRoot(false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(p)
}
