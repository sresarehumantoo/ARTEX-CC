package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func (a *API) registerConstraintsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/constraints", a.handleConstraints)
	mux.HandleFunc("POST /api/constraints", a.handleAddConstraint)
}

func (a *API) handleConstraints(w http.ResponseWriter, r *http.Request) {
	constraints, err := a.store.ListConstraints(r.Context(), a.explorationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, constraints)
}

func (a *API) handleAddConstraint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" || body.Kind == "" {
		writeError(w, http.StatusBadRequest, "kind and text are required")
		return
	}
	id, err := a.store.AddConstraint(r.Context(), a.explorationID, body.Kind, body.Text, "web-ui")
	if err != nil {
		slog.Error("add constraint", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	a.Broadcast(eventGraphChanged, map[string]any{"action": "add_constraint", "id": id})
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}
