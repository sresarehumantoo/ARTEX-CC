package api

import (
	"net/http"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func (a *API) registerTasksRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tasks", a.handleTasks)
	mux.HandleFunc("GET /api/tasks/{id}/goals", a.handleTaskGoals)
}

type taskGetter interface {
	GetTask(id int64) (*store.Task, error)
}

type taskStats struct {
	Findings    int `json:"findings"`
	Assets      int `json:"assets"`
	CoveragePct int `json:"coverage_pct"`
}

type taskSummary struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt string    `json:"created_at,omitempty"`
	Stats     taskStats `json:"stats"`
}

func (a *API) handleTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	overview, err := a.store.GraphOverview(ctx, a.explorationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, goal, _ := a.store.Root(ctx, a.explorationID)

	_, assetTotal, err := a.store.ListAssets(ctx, tools.AssetFilter{TaskID: a.taskID, Limit: 1})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	t := taskSummary{
		ID:     a.taskID,
		Title:  goal,
		Status: "running",
		Stats: taskStats{
			Findings:    overview.FindingsTotal,
			Assets:      assetTotal,
			CoveragePct: a.coveragePct(),
		},
	}
	if tg, ok := a.store.(taskGetter); ok {
		if task, err := tg.GetTask(a.taskID); err == nil && task != nil {
			t.CreatedAt = task.CreatedAt.Format(time.RFC3339)
			if task.Status != "" {
				t.Status = task.Status
			}
			if t.Title == "" {
				t.Title = task.Name
			}
		}
	}
	writeJSON(w, http.StatusOK, []taskSummary{t})
}

func (a *API) coveragePct() int {
	cs, ok := a.store.(coverageStatter)
	if !ok {
		return 0
	}
	stats, err := cs.CoverageStats(a.taskID)
	if err != nil {
		return 0
	}
	total := len(stats.Entries) + len(stats.Unexamined)
	if total == 0 {
		return 0
	}
	return len(stats.Entries) * 100 / total
}

func (a *API) handleTaskGoals(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok || id != a.taskID {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	ctx := r.Context()
	overview, err := a.store.GraphOverview(ctx, a.explorationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(overview.Goals) > 0 {
		writeJSON(w, http.StatusOK, overview.Goals)
		return
	}
	_, goal, _ := a.store.Root(ctx, a.explorationID)
	writeJSON(w, http.StatusOK, []map[string]any{{"id": 0, "label": goal, "state": "open"}})
}
