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
	logs      *LogBuffer
	chat      *ChatStore
	runner    ChatRunner

	toolList []ToolInfo

	authRequired bool
	authMu       sync.Mutex
}

type Option func(*API)

func WithDataDir(dir string) Option {
	return func(a *API) { a.files = NewFileStore(dir) }
}

func WithRunner(r ChatRunner) Option {
	return func(a *API) { a.runner = r }
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
	if a.files != nil {
		a.chat = NewChatStore(a.files)
		if a.runner != nil {
			a.runner.Bind(a.chat, a.hub)
			if r, ok := a.runner.(*Runner); ok {
				r.SetSettings(a.llmSettings)
			}
		}
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
	a.registerLogsRoutes(mux)
	a.registerTasksRoutes(mux)
	if a.chat != nil {
		a.registerChatRoutes(mux)
		a.registerCommandsRoutes(mux)
		a.registerUsageRoutes(mux)
	}
	if a.files != nil {
		a.registerAgentsRoutes(mux)
		a.registerLLMSettingsRoutes(mux)
		a.registerInterceptRoutes(mux)
		a.registerMCPConfigRoutes(mux)
		a.registerSkillsRoutes(mux)
		a.registerWorkspaceRoutes(mux)
	}
	a.registerToolsRoutes(mux)
	a.registerAuthRoutes(mux)
	return corsMiddleware(a.authMiddleware(mux))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
