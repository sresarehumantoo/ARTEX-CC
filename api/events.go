package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const globalTopic = ""

func (a *API) registerEventsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events", a.handleSSE)
}

// Broadcast sends an event to all connected SSE clients.
func (a *API) Broadcast(eventType string, data any) {
	payload, err := json.Marshal(map[string]any{"type": eventType, "data": data})
	if err != nil {
		return
	}
	a.hub.Publish(globalTopic, payload)
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

	w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	ch, unsubscribe := a.hub.Subscribe(globalTopic)
	defer unsubscribe()

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
