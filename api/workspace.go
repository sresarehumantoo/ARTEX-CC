package api

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	workspaceDir            = "workspace"
	maxWorkspaceUploadBytes = 256 << 20
)

type workspaceEntry struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

func (a *API) registerWorkspaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workspace", a.handleWorkspaceList)
	mux.HandleFunc("GET /api/workspace/file", a.handleWorkspaceRead)
	mux.HandleFunc("PUT /api/workspace/file", a.handleWorkspaceWrite)
	mux.HandleFunc("DELETE /api/workspace/file", a.handleWorkspaceDelete)
	mux.HandleFunc("POST /api/workspace/mkdir", a.handleWorkspaceMkdir)
}

func (a *API) workspacePath(w http.ResponseWriter, r *http.Request, allowRoot bool) (*os.Root, string, bool) {
	rel := r.URL.Query().Get("path")
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") {
		writeError(w, http.StatusBadRequest, "invalid path")
		return nil, "", false
	}
	rel = path.Clean(filepath.ToSlash(rel))
	if rel == "." && !allowRoot {
		writeError(w, http.StatusBadRequest, "path required")
		return nil, "", false
	}
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return nil, "", false
	}
	if err := os.MkdirAll(a.workspace, 0o700); err != nil {
		workspaceError(w, err)
		return nil, "", false
	}
	root, err := os.OpenRoot(a.workspace)
	if err != nil {
		workspaceError(w, err)
		return nil, "", false
	}
	return root, filepath.FromSlash(rel), true
}

// os.Root does not export the error it returns for a symlink that leaves the root.
func escapesRoot(err error) bool {
	for ; err != nil; err = errors.Unwrap(err) {
		if err.Error() == "path escapes from parent" {
			return true
		}
	}
	return false
}

func workspaceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, "not found")
	case escapesRoot(err):
		writeError(w, http.StatusBadRequest, "invalid path")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (a *API) handleWorkspaceList(w http.ResponseWriter, r *http.Request) {
	root, rel, ok := a.workspacePath(w, r, true)
	if !ok {
		return
	}
	defer root.Close()
	dir, err := root.Open(rel)
	if err != nil {
		workspaceError(w, err)
		return
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		workspaceError(w, err)
		return
	}
	out := make([]workspaceEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		entry := workspaceEntry{Name: e.Name(), Type: "file", Size: info.Size(), Modified: info.ModTime()}
		if info.IsDir() {
			entry.Type = "dir"
			entry.Size = 0
		}
		out = append(out, entry)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleWorkspaceRead(w http.ResponseWriter, r *http.Request) {
	root, rel, ok := a.workspacePath(w, r, false)
	if !ok {
		return
	}
	defer root.Close()
	f, err := root.Open(rel)
	if err != nil {
		workspaceError(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		workspaceError(w, err)
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is a directory")
		return
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".json", ".jsonl":
		w.Header().Set("Content-Type", "application/json")
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	if r.URL.Query().Get("download") != "" {
		name := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(info.Name())
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	_, _ = io.Copy(w, f)
}

func (a *API) handleWorkspaceWrite(w http.ResponseWriter, r *http.Request) {
	root, rel, ok := a.workspacePath(w, r, false)
	if !ok {
		return
	}
	defer root.Close()
	if err := root.MkdirAll(filepath.Dir(rel), 0o700); err != nil {
		workspaceError(w, err)
		return
	}
	f, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		workspaceError(w, err)
		return
	}
	_, err = io.Copy(f, http.MaxBytesReader(w, r.Body, maxWorkspaceUploadBytes))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		workspaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleWorkspaceDelete(w http.ResponseWriter, r *http.Request) {
	root, rel, ok := a.workspacePath(w, r, false)
	if !ok {
		return
	}
	defer root.Close()
	if err := root.Remove(rel); err != nil {
		workspaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleWorkspaceMkdir(w http.ResponseWriter, r *http.Request) {
	root, rel, ok := a.workspacePath(w, r, false)
	if !ok {
		return
	}
	defer root.Close()
	if err := root.MkdirAll(rel, 0o700); err != nil {
		workspaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
