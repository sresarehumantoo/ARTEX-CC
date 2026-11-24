package api

import (
	"net/http"
	"strings"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ReadOnly    bool           `json:"read_only"`
	Category    string         `json:"category"`
	Schema      map[string]any `json:"schema,omitempty"`
}

func WithToolList(list []tools.Tool) Option {
	return func(a *API) {
		a.toolList = make([]ToolInfo, 0, len(list))
		for _, t := range list {
			a.toolList = append(a.toolList, ToolInfo{
				Name:        t.Name,
				Description: t.Description,
				ReadOnly:    t.ReadOnly,
				Category:    toolCategory(t.Name),
				Schema:      t.Schema,
			})
		}
	}
}

func (a *API) registerToolsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tools", a.handleListTools)
}

func (a *API) handleListTools(w http.ResponseWriter, r *http.Request) {
	out := a.toolList
	if out == nil {
		out = []ToolInfo{}
	}
	writeJSON(w, http.StatusOK, out)
}

var categoryKeywords = []struct {
	category string
	keywords []string
}{
	{"traffic", []string{"traffic"}},
	{"evidence", []string{"evidence", "capture_command_output"}},
	{"findings", []string{"finding"}},
	{"assets", []string{"asset"}},
	{"engagement", []string{"engagement", "goal", "report", "scope", "company"}},
	{"coverage", []string{"coverage"}},
	{"credentials", []string{"credential"}},
	{"enrichment", []string{"enrich", "ingest"}},
	{"activity", []string{"activity"}},
	{"graph", []string{"graph", "intent", "fact", "node", "hint", "constraint"}},
}

var systemTools = map[string]bool{
	"Bash": true, "Read": true, "Write": true, "Edit": true, "Glob": true,
	"Grep": true, "WebFetch": true, "WebSearch": true, "Task": true,
}

func toolCategory(name string) string {
	if systemTools[name] {
		return "system"
	}
	if rest, ok := strings.CutPrefix(name, "mcp__"); ok {
		if _, tool, found := strings.Cut(rest, "__"); found {
			name = tool
		}
	}
	for _, c := range categoryKeywords {
		for _, k := range c.keywords {
			if strings.Contains(name, k) {
				return c.category
			}
		}
	}
	return "other"
}
