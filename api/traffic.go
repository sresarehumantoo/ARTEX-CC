package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/SREsAreHumanToo/ARTEX-CC/traffic"
)

func (a *API) registerTrafficRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/traffic", a.handleTraffic)
	mux.HandleFunc("GET /api/traffic/{id}", a.handleTrafficDetail)
}

func (a *API) handleTraffic(w http.ResponseWriter, r *http.Request) {
	if a.traffic == nil {
		writeJSON(w, http.StatusOK, map[string]any{"rows": []any{}, "total": 0})
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rows":  exchanges,
		"total": total,
	})
}

func (a *API) handleTrafficDetail(w http.ResponseWriter, r *http.Request) {
	if a.traffic == nil {
		writeError(w, http.StatusNotFound, "traffic proxy not enabled")
		return
	}
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "missing exchange id")
		return
	}
	req, resp, err := a.traffic.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "exchange not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       id,
		"request":  req,
		"response": resp,
	})
}
