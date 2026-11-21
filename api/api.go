package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/tools"
	"github.com/SREsAreHumanToo/cc-artex/traffic"
)

type API struct {
	store         tools.Store
	traffic       *traffic.Traffic
	taskID        int64
	explorationID int64

	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

func New(store tools.Store, t *traffic.Traffic, taskID, explorationID int64) *API {
	return &API{
		store:         store,
		traffic:       t,
		taskID:        taskID,
		explorationID: explorationID,
		subscribers:   make(map[chan []byte]struct{}),
	}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/engagement", a.handleEngagement)
	mux.HandleFunc("GET /api/graph", a.handleGraph)
	mux.HandleFunc("GET /api/findings", a.handleFindings)
	mux.HandleFunc("GET /api/findings/{id}", a.handleFindingDetail)
	mux.HandleFunc("GET /api/assets", a.handleAssets)
	mux.HandleFunc("GET /api/assets/{id}", a.handleAssetDetail)
	mux.HandleFunc("GET /api/traffic", a.handleTraffic)
	mux.HandleFunc("GET /api/traffic/{id}", a.handleTrafficDetail)
	mux.HandleFunc("GET /api/activity", a.handleActivity)
	mux.HandleFunc("GET /api/events", a.handleSSE)
	return corsMiddleware(mux)
}

// Broadcast sends an event to all connected SSE clients.
func (a *API) Broadcast(eventType string, data any) {
	payload, err := json.Marshal(map[string]any{"type": eventType, "data": data})
	if err != nil {
		return
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for ch := range a.subscribers {
		select {
		case ch <- payload:
		default:
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("json encode failed", "err", err)
	}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

func parseIntParam(r *http.Request, name string, def int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseID(r *http.Request) (int64, bool) {
	s := r.PathValue("id")
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func (a *API) handleEngagement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	overview, err := a.store.GraphOverview(ctx, a.explorationID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	desc, goal, _ := a.store.Root(ctx, a.explorationID)

	// Flatten the overview into the top-level response so the frontend
	// can read fields like goals, open_intents, etc. directly.
	flat := map[string]any{}
	b, _ := json.Marshal(overview)
	json.Unmarshal(b, &flat)
	flat["task_id"] = a.taskID
	flat["exploration_id"] = a.explorationID
	flat["description"] = desc
	flat["goal"] = goal
	jsonResponse(w, http.StatusOK, flat)
}

func (a *API) handleGraph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodes, err := a.store.ListNodes(ctx, a.explorationID, "", 0)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	edges, err := a.store.Edges(ctx, a.explorationID, 0)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

func (a *API) handleFindings(w http.ResponseWriter, r *http.Request) {
	findings, err := a.store.ListFindings(r.Context(), a.explorationID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, findings)
}

func (a *API) handleFindingDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid finding id")
		return
	}
	findings, err := a.store.ListFindings(r.Context(), a.explorationID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, f := range findings {
		if f.ID == id {
			bindings, _ := a.store.ListFindingTraffic(r.Context(), id)
			result := map[string]any{}
			b, _ := json.Marshal(f)
			json.Unmarshal(b, &result)
			result["traffic_bindings"] = bindings
			jsonResponse(w, http.StatusOK, result)
			return
		}
	}
	jsonError(w, http.StatusNotFound, "finding not found")
}

func (a *API) handleAssets(w http.ResponseWriter, r *http.Request) {
	filter := tools.AssetFilter{
		TaskID: a.taskID,
		DSL:    r.URL.Query().Get("dsl"),
		Type:   r.URL.Query().Get("type"),
		Limit:  parseIntParam(r, "limit", 50),
		Offset: parseIntParam(r, "offset", 0),
	}
	assets, total, err := a.store.ListAssets(r.Context(), filter)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"assets": assets,
		"total":  total,
	})
}

func (a *API) handleAssetDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid asset id")
		return
	}
	filter := tools.AssetFilter{TaskID: a.taskID, ID: id, Limit: 1}
	assets, _, err := a.store.ListAssets(r.Context(), filter)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(assets) == 0 {
		jsonError(w, http.StatusNotFound, "asset not found")
		return
	}
	jsonResponse(w, http.StatusOK, assets[0])
}

func (a *API) handleTraffic(w http.ResponseWriter, r *http.Request) {
	if a.traffic == nil {
		jsonResponse(w, http.StatusOK, map[string]any{"rows": []any{}, "total": 0})
		return
	}
	q := traffic.PageQuery{
		Host:   r.URL.Query().Get("host"),
		Method: r.URL.Query().Get("method"),
		Query:  r.URL.Query().Get("q"),
		Body:   r.URL.Query().Get("body"),
		Path:   r.URL.Query().Get("path"),
		Status: r.URL.Query().Get("status"),
		Sort:   r.URL.Query().Get("sort"),
		Order:  r.URL.Query().Get("order"),
	}
	if v := r.URL.Query().Get("resp_min"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.RespMin = n
		} else {
			q.RespMin = -1
		}
	} else {
		q.RespMin = -1
	}
	if v := r.URL.Query().Get("resp_max"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.RespMax = n
		} else {
			q.RespMax = -1
		}
	} else {
		q.RespMax = -1
	}
	page := parseIntParam(r, "page", 0)
	size := parseIntParam(r, "size", 50)
	exchanges, total, err := a.traffic.Page(q, page, size)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"rows":  exchanges,
		"total": total,
	})
}

func (a *API) handleTrafficDetail(w http.ResponseWriter, r *http.Request) {
	if a.traffic == nil {
		jsonError(w, http.StatusNotFound, "traffic proxy not enabled")
		return
	}
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		jsonError(w, http.StatusBadRequest, "missing exchange id")
		return
	}
	req, resp, err := a.traffic.Get(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "exchange not found")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"id":       id,
		"request":  req,
		"response": resp,
	})
}

func (a *API) handleActivity(w http.ResponseWriter, r *http.Request) {
	filter := tools.ActivityFilter{
		Limit: parseIntParam(r, "limit", 50),
		Kind:  r.URL.Query().Get("kind"),
	}
	if v := r.URL.Query().Get("node_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.NodeID = n
		}
	}
	activity, err := a.store.ListActivity(r.Context(), a.explorationID, filter)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, activity)
}

func (a *API) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan []byte, 32)
	a.mu.Lock()
	a.subscribers[ch] = struct{}{}
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.subscribers, ch)
		a.mu.Unlock()
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			w.Write([]byte("data: "))
			w.Write(msg)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-ticker.C:
			w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()
		}
	}
}
