package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func (a *API) registerEngagementRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/engagement", a.handleEngagement)
	mux.HandleFunc("GET /api/graph", a.handleGraph)
	mux.HandleFunc("GET /api/search", a.handleSearch)
}

func (a *API) handleEngagement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	overview, err := a.store.GraphOverview(ctx, a.explorationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	desc, goal, _ := a.store.Root(ctx, a.explorationID)

	// Flatten the overview into the top-level response so the frontend
	// can read fields like goals, open_intents, etc. directly.
	flat := map[string]any{}
	b, _ := json.Marshal(overview)
	json.Unmarshal(b, &flat)
	flat["task_id"] = a.taskID
	flat["exploration_id"] = a.explorationID
	flat["description"] = desc
	flat["goal"] = goal
	writeJSON(w, http.StatusOK, flat)
}

func (a *API) handleGraph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodes, err := a.store.ListNodes(ctx, a.explorationID, "", 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	edges, err := a.store.Edges(ctx, a.explorationID, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if nodes == nil {
		nodes = []tools.Node{}
	}
	if edges == nil {
		edges = []tools.Edge{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

type searchResult struct {
	Type     string `json:"type"`
	ID       any    `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	URL      string `json:"url"`
}

func (a *API) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": []any{}})
		return
	}

	ctx := r.Context()
	ql := strings.ToLower(q)
	var results []searchResult

	findings, err := a.store.ListFindings(ctx, a.explorationID)
	if err == nil {
		n := 0
		for _, f := range findings {
			if n >= 5 {
				break
			}
			if strings.Contains(strings.ToLower(f.VulnClass), ql) || strings.Contains(strings.ToLower(f.Summary), ql) {
				results = append(results, searchResult{
					Type:     "finding",
					ID:       f.ID,
					Title:    f.VulnClass,
					Subtitle: f.Severity + " — " + truncate(f.Summary, 60),
					URL:      "/findings?id=" + strconv.FormatInt(f.ID, 10),
				})
				n++
			}
		}
	}

	assets, _, err := a.store.ListAssets(ctx, tools.AssetFilter{
		TaskID: a.taskID,
		DSL:    q,
		Limit:  5,
	})
	if err == nil {
		for _, asset := range assets {
			results = append(results, searchResult{
				Type:     "asset",
				ID:       asset.ID,
				Title:    asset.Label,
				Subtitle: asset.Type,
				URL:      "/assets?id=" + strconv.FormatInt(asset.ID, 10),
			})
		}
	}

	nodes, err := a.store.ListNodes(ctx, a.explorationID, "", 0)
	if err == nil {
		n := 0
		for _, nd := range nodes {
			if n >= 5 {
				break
			}
			label, _ := nd.Payload["label"].(string)
			summary, _ := nd.Payload["summary"].(string)
			if strings.Contains(strings.ToLower(label), ql) || strings.Contains(strings.ToLower(summary), ql) {
				subtitle := nd.Kind
				if nd.State != "" {
					subtitle += " — " + nd.State
				}
				results = append(results, searchResult{
					Type:     "node",
					ID:       nd.ID,
					Title:    label,
					Subtitle: subtitle,
					URL:      "/graph?node=" + strconv.FormatInt(nd.ID, 10),
				})
				n++
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
