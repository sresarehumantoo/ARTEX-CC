package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var webDist embed.FS

func Handler() http.Handler {
	distFS, _ := fs.Sub(webDist, "dist")
	fileServer := http.FileServer(http.FS(distFS))
	return &spaHandler{fs: distFS, fileServer: fileServer}
}

type spaHandler struct {
	fs         fs.FS
	fileServer http.Handler
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(h.fs, path); err != nil {
		r.URL.Path = "/"
	}
	h.fileServer.ServeHTTP(w, r)
}
