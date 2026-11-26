package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func (a *API) registerHintsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/hints", a.handleAddHint)
}

func (a *API) handleAddHint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	id, err := tools.AddHint(r.Context(), a.store, a.explorationID, body.Text, "web-ui", nil)
	if err != nil {
		slog.Error("add hint", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	a.Broadcast("node_added", map[string]any{"id": id, "kind": "hint", "label": body.Text})
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}
