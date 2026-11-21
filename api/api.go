package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/store"
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
	mux.HandleFunc("GET /api/assets/{id}/findings", a.handleAssetFindings)
	mux.HandleFunc("GET /api/traffic", a.handleTraffic)
	mux.HandleFunc("GET /api/traffic/{id}", a.handleTrafficDetail)
	mux.HandleFunc("GET /api/search", a.handleSearch)
	mux.HandleFunc("GET /api/activity", a.handleActivity)
	mux.HandleFunc("GET /api/coverage", a.handleCoverage)
	mux.HandleFunc("GET /api/coverage/gaps", a.handleCoverageGaps)
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

func (a *API) NotifyChange(toolName string) {
	ctx := context.Background()
	switch {
	case strings.HasPrefix(toolName, "insert_node"),
		strings.HasPrefix(toolName, "add_hint"),
		strings.HasPrefix(toolName, "add_intent"),
		strings.HasPrefix(toolName, "record_fact"),
		strings.HasPrefix(toolName, "prove_goal"),
		strings.HasPrefix(toolName, "update_intent"):
		nodes, _ := a.store.ListNodes(ctx, a.explorationID, "", 0)
		edges, _ := a.store.Edges(ctx, a.explorationID, 0)
		a.Broadcast("full_refresh", map[string]any{"nodes": nodes, "edges": edges})
	case strings.HasPrefix(toolName, "add_finding"),
		strings.HasPrefix(toolName, "confirm_finding"),
		strings.HasPrefix(toolName, "report_finding"):
		a.Broadcast("finding_added", nil)
	case strings.HasPrefix(toolName, "insert_asset"):
		a.Broadcast("asset_added", nil)
	default:
		a.Broadcast("store_changed", nil)
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

type dbQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type findingWithTime struct {
	tools.Finding
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

func (a *API) findingTimestamps(ctx context.Context) map[int64]time.Time {
	q, ok := a.store.(dbQuerier)
	if !ok {
		return nil
	}
	rows, err := q.QueryContext(ctx,
		`SELECT f.id, f.created_at FROM findings f JOIN tasks t ON t.id = f.task_id WHERE t.exploration_id = $1`,
		a.explorationID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	m := map[int64]time.Time{}
	for rows.Next() {
		var id int64
		var ts time.Time
		if rows.Scan(&id, &ts) == nil {
			m[id] = ts
		}
	}
	return m
}

func (a *API) handleFindings(w http.ResponseWriter, r *http.Request) {
	findings, err := a.store.ListFindings(r.Context(), a.explorationID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	timestamps := a.findingTimestamps(r.Context())
	if timestamps != nil {
		result := make([]findingWithTime, len(findings))
		for i, f := range findings {
			result[i] = findingWithTime{Finding: f}
			if ts, ok := timestamps[f.ID]; ok {
				result[i].CreatedAt = &ts
			}
		}
		jsonResponse(w, http.StatusOK, result)
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
			timestamps := a.findingTimestamps(r.Context())
			if ts, ok := timestamps[f.ID]; ok {
				result["created_at"] = ts
			}
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

func (a *API) handleAssetFindings(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid asset id")
		return
	}
	q, ok := a.store.(dbQuerier)
	if !ok {
		jsonResponse(w, http.StatusOK, []any{})
		return
	}
	ctx := r.Context()
	assetIDJSON := "[" + strconv.FormatInt(id, 10) + "]"
	rows, err := q.QueryContext(ctx, `
		SELECT f.id, f.node_id, f.vulnclass, f.severity, f.summary, COALESCE(f.status,''),
		       f.merged_into, f.dismissed, f.dismiss_reason, f.created_at
		FROM findings f
		JOIN tasks t ON t.id = f.task_id
		WHERE t.exploration_id = $1
		  AND f.asset_ids @> $2::jsonb
		ORDER BY f.created_at DESC`, a.explorationID, assetIDJSON)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var out []findingWithTime
	for rows.Next() {
		var f findingWithTime
		var nodeID, mergedInto sql.NullInt64
		var ts time.Time
		if err := rows.Scan(&f.ID, &nodeID, &f.VulnClass, &f.Severity, &f.Summary, &f.Status,
			&mergedInto, &f.Dismissed, &f.DismissReason, &ts); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		f.NodeID = nodeID.Int64
		f.MergedInto = mergedInto.Int64
		f.CreatedAt = &ts
		out = append(out, f)
	}
	if out == nil {
		out = []findingWithTime{}
	}
	jsonResponse(w, http.StatusOK, out)
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

type searchResult struct {
	Type     string `json:"type"`
	ID       any    `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	URL      string `json:"url"`
}

func (a *API) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonResponse(w, http.StatusOK, map[string]any{"results": []any{}})
		return
	}

	ctx := r.Context()
	ql := strings.ToLower(q)
	var results []searchResult

	findings, err := a.store.ListFindings(ctx, a.explorationID)
	if err == nil {
		n := 0
		for _, f := range findings {
			if n >= 5 {
				break
			}
			if strings.Contains(strings.ToLower(f.VulnClass), ql) || strings.Contains(strings.ToLower(f.Summary), ql) {
				results = append(results, searchResult{
					Type:     "finding",
					ID:       f.ID,
					Title:    f.VulnClass,
					Subtitle: f.Severity + " — " + truncate(f.Summary, 60),
					URL:      "/findings?id=" + strconv.FormatInt(f.ID, 10),
				})
				n++
			}
		}
	}

	assets, _, err := a.store.ListAssets(ctx, tools.AssetFilter{
		TaskID: a.taskID,
		DSL:    q,
		Limit:  5,
	})
	if err == nil {
		for _, asset := range assets {
			results = append(results, searchResult{
				Type:     "asset",
				ID:       asset.ID,
				Title:    asset.Label,
				Subtitle: asset.Type,
				URL:      "/assets?id=" + strconv.FormatInt(asset.ID, 10),
			})
		}
	}

	nodes, err := a.store.ListNodes(ctx, a.explorationID, "", 0)
	if err == nil {
		n := 0
		for _, nd := range nodes {
			if n >= 5 {
				break
			}
			label, _ := nd.Payload["label"].(string)
			summary, _ := nd.Payload["summary"].(string)
			if strings.Contains(strings.ToLower(label), ql) || strings.Contains(strings.ToLower(summary), ql) {
				subtitle := nd.Kind
				if nd.State != "" {
					subtitle += " — " + nd.State
				}
				results = append(results, searchResult{
					Type:     "node",
					ID:       nd.ID,
					Title:    label,
					Subtitle: subtitle,
					URL:      "/graph?node=" + strconv.FormatInt(nd.ID, 10),
				})
				n++
			}
		}
	}

	jsonResponse(w, http.StatusOK, map[string]any{"results": results})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
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

type coverageStatter interface {
	CoverageStats(taskID int64) (*store.CoverageStats, error)
}

func (a *API) handleCoverage(w http.ResponseWriter, r *http.Request) {
	cs, ok := a.store.(coverageStatter)
	if !ok {
		jsonResponse(w, http.StatusOK, store.CoverageStats{
			Entries:    []store.CoverageEntry{},
			Gaps:       []store.CoverageGap{},
			Unexamined: []string{},
		})
		return
	}
	stats, err := cs.CoverageStats(a.taskID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, stats)
}

func (a *API) handleCoverageGaps(w http.ResponseWriter, r *http.Request) {
	gaps, err := a.store.ListCoverageGaps(r.Context(), a.taskID, a.explorationID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, gaps)
}
