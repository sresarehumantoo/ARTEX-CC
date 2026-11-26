package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
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

const (
	eventFindingAdded    = "finding_added"
	eventFindingsChanged = "findings_changed"
	eventAssetAdded      = "asset_added"
	eventGraphChanged    = "graph_changed"
	eventTrafficChanged  = "traffic_changed"
	eventStoreChanged    = "store_changed"
	eventRefresh         = "refresh"
)

// changeEvents maps each mutating tool to the SSE event naming what it
// changed. Tools not listed here, such as plugin tools, get eventRefresh.
var changeEvents = map[string]string{
	"report_finding":       eventFindingAdded,
	"merge_findings":       eventFindingsChanged,
	"dismiss_finding":      eventFindingsChanged,
	"bind_finding_traffic": eventFindingsChanged,

	"insert_assets": eventAssetAdded,

	"add_intent":      eventGraphChanged,
	"record_fact":     eventGraphChanged,
	"add_hint":        eventGraphChanged,
	"set_goals":       eventGraphChanged,
	"set_constraints": eventGraphChanged,
	"prove_goal":      eventGraphChanged,
	"start_intent":    eventGraphChanged,
	"complete_intent": eventGraphChanged,
	"cancel_intent":   eventGraphChanged,

	"traffic_replay": eventTrafficChanged,
	"traffic_clear":  eventTrafficChanged,

	"add_company_scope":      eventStoreChanged,
	"add_task_scope":         eventStoreChanged,
	"save_task_credential":   eventStoreChanged,
	"record_coverage":        eventStoreChanged,
	"log_activity":           eventStoreChanged,
	"save_evidence":          eventStoreChanged,
	"capture_command_output": eventStoreChanged,
	"goal_met":               eventStoreChanged,
	"update_engagement":      eventStoreChanged,
}

func changeEvent(toolName string) string {
	if ev, ok := changeEvents[toolName]; ok {
		return ev
	}
	return eventRefresh
}

// changeWindow is how long NotifyChange collects changes before broadcasting,
// so a burst of tool calls (local or from other processes via LISTEN) becomes
// one event per kind rather than one per call.
const changeWindow = 250 * time.Millisecond

type changeBatcher struct {
	mu      sync.Mutex
	window  time.Duration
	pending map[string]int
	timer   *time.Timer
}

// NotifyChange records that toolName changed the store. It never blocks on
// I/O: events are coalesced and broadcast without payload after a short
// window, and clients refetch what each event invalidates.
func (a *API) NotifyChange(toolName string) {
	b := &a.changes
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		b.pending = make(map[string]int)
	}
	b.pending[changeEvent(toolName)]++
	if b.timer == nil {
		window := b.window
		if window == 0 {
			window = changeWindow
		}
		b.timer = time.AfterFunc(window, a.flushChanges)
	}
}

func (a *API) flushChanges() {
	b := &a.changes
	b.mu.Lock()
	pending := b.pending
	b.pending = nil
	b.timer = nil
	b.mu.Unlock()

	events := make([]string, 0, len(pending))
	for ev := range pending {
		events = append(events, ev)
	}
	sort.Strings(events)
	for _, ev := range events {
		a.Broadcast(ev, map[string]any{"count": pending[ev]})
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
