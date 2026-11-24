package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func (a *API) registerAssetsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/assets", a.handleAssets)
	mux.HandleFunc("GET /api/assets/{id}", a.handleAssetDetail)
	mux.HandleFunc("GET /api/assets/{id}/findings", a.handleAssetFindings)
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"assets": assets,
		"total":  total,
	})
}

func (a *API) handleAssetDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid asset id")
		return
	}
	filter := tools.AssetFilter{TaskID: a.taskID, ID: id, Limit: 1}
	assets, _, err := a.store.ListAssets(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(assets) == 0 {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	writeJSON(w, http.StatusOK, assets[0])
}

func (a *API) handleAssetFindings(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid asset id")
		return
	}
	q, ok := a.store.(dbQuerier)
	if !ok {
		writeJSON(w, http.StatusOK, []any{})
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
		writeError(w, http.StatusInternalServerError, err.Error())
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
			writeError(w, http.StatusInternalServerError, err.Error())
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
	writeJSON(w, http.StatusOK, out)
}
