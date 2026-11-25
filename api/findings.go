package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func (a *API) registerFindingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/findings", a.handleFindings)
	mux.HandleFunc("GET /api/findings/{id}", a.handleFindingDetail)
	mux.HandleFunc("POST /api/findings/{id}/dismiss", a.handleDismissFinding)
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
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, findings)
}

func (a *API) handleFindingDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid finding id")
		return
	}
	findings, err := a.store.ListFindings(r.Context(), a.explorationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
			writeJSON(w, http.StatusOK, result)
			return
		}
	}
	writeError(w, http.StatusNotFound, "finding not found")
}

func (a *API) handleDismissFinding(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid finding id")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := a.store.DismissFinding(r.Context(), id, body.Reason); err != nil {
		slog.Error("dismiss finding", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	a.Broadcast(eventFindingsChanged, map[string]any{"action": "dismiss", "finding_id": id})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
