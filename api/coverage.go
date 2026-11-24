package api

import (
	"net/http"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
)

func (a *API) registerCoverageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/coverage", a.handleCoverage)
	mux.HandleFunc("GET /api/coverage/gaps", a.handleCoverageGaps)
}

type coverageStatter interface {
	CoverageStats(taskID int64) (*store.CoverageStats, error)
}

func (a *API) handleCoverage(w http.ResponseWriter, r *http.Request) {
	cs, ok := a.store.(coverageStatter)
	if !ok {
		writeJSON(w, http.StatusOK, store.CoverageStats{
			Entries:    []store.CoverageEntry{},
			Gaps:       []store.CoverageGap{},
			Unexamined: []string{},
		})
		return
	}
	stats, err := cs.CoverageStats(a.taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *API) handleCoverageGaps(w http.ResponseWriter, r *http.Request) {
	gaps, err := a.store.ListCoverageGaps(r.Context(), a.taskID, a.explorationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gaps)
}
