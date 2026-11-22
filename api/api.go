package api

import (
	"net/http"
	"sync"

	"github.com/SREsAreHumanToo/cc-artex/tools"
	"github.com/SREsAreHumanToo/cc-artex/traffic"
)

type API struct {
	store         tools.Store
	traffic       *traffic.Traffic
	taskID        int64
	explorationID int64

	files     *FileStore
	claudeBin string
	hub       *Hub

	authRequired bool
	authMu       sync.Mutex
}

type Option func(*API)

func WithDataDir(dir string) Option {
	return func(a *API) { a.files = NewFileStore(dir) }
}

func WithClaudeBin(path string) Option {
	return func(a *API) { a.claudeBin = path }
}

func New(store tools.Store, t *traffic.Traffic, taskID, explorationID int64, opts ...Option) *API {
	a := &API{
		store:         store,
		traffic:       t,
		taskID:        taskID,
		explorationID: explorationID,
		claudeBin:     "claude",
		hub:           NewHub(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	a.registerEngagementRoutes(mux)
	a.registerFindingsRoutes(mux)
	a.registerAssetsRoutes(mux)
	a.registerTrafficRoutes(mux)
	a.registerActivityRoutes(mux)
	a.registerCoverageRoutes(mux)
	a.registerCredentialsRoutes(mux)
	a.registerConstraintsRoutes(mux)
	a.registerHintsRoutes(mux)
	a.registerEventsRoutes(mux)
	a.registerAuthRoutes(mux)
	return corsMiddleware(a.authMiddleware(mux))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
