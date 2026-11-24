package api

import (
	"net/http"
	"strconv"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func (a *API) registerActivityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/activity", a.handleActivity)
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, activity)
}
