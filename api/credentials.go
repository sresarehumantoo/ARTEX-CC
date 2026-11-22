package api

import (
	"net/http"
)

func (a *API) registerCredentialsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/credentials", a.handleCredentials)
}

func (a *API) handleCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := a.store.ListCredentials(r.Context(), a.taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, creds)
}
