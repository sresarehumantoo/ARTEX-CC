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

const maxWorkspaceUploadBytes = 256 << 20

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

func (a *API) workspacePath(w http.ResponseWriter, r *http.Request, allowRoot bool) (string, bool) {
	rel := r.URL.Query().Get("path")
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") {
		writeError(w, http.StatusBadRequest, "invalid path")
		return "", false
	}
	rel = path.Clean(filepath.ToSlash(rel))
	if rel == "." && !allowRoot {
		writeError(w, http.StatusBadRequest, "path required")
		return "", false
	}
	full, err := a.files.resolve(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return "", false
	}
	return full, true
}

func workspaceError(w http.ResponseWriter, err error) {
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (a *API) handleWorkspaceList(w http.ResponseWriter, r *http.Request) {
	full, ok := a.workspacePath(w, r, true)
	if !ok {
		return
	}
	entries, err := os.ReadDir(full)
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
	full, ok := a.workspacePath(w, r, false)
	if !ok {
		return
	}
	f, err := os.Open(full)
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
	switch strings.ToLower(filepath.Ext(full)) {
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
	full, ok := a.workspacePath(w, r, false)
	if !ok {
		return
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		workspaceError(w, err)
		return
	}
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
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
	full, ok := a.workspacePath(w, r, false)
	if !ok {
		return
	}
	if err := os.Remove(full); err != nil {
		workspaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleWorkspaceMkdir(w http.ResponseWriter, r *http.Request) {
	full, ok := a.workspacePath(w, r, false)
	if !ok {
		return
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		workspaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
