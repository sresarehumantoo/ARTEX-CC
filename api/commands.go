package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

const commandOutputPreview = 200

type CommandEntry struct {
	Seq               int64           `json:"seq"`
	ConversationID    string          `json:"conversation_id"`
	ConversationTitle string          `json:"conversation_title"`
	ToolName          string          `json:"tool_name"`
	Input             json.RawMessage `json:"input"`
	OutputPreview     string          `json:"output_preview"`
	IsError           bool            `json:"is_error"`
	Time              string          `json:"time"`
}

type CommandStats struct {
	Total          int            `json:"total"`
	ByTool         map[string]int `json:"by_tool"`
	ByConversation map[string]int `json:"by_conversation"`
	ErrorCount     int            `json:"error_count"`
}

func (a *API) registerCommandsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/commands", a.handleCommands)
	mux.HandleFunc("GET /api/commands/stats", a.handleCommandStats)
}

func (a *API) scanCommands() ([]CommandEntry, error) {
	convs, err := a.chat.List()
	if err != nil {
		return nil, err
	}
	out := []CommandEntry{}
	for _, c := range convs {
		events, err := a.chat.Events(c.ID, 0)
		if err != nil {
			continue
		}
		results := make(map[string]ChatEvent)
		for _, ev := range events {
			if ev.Kind == ChatKindToolResult && ev.ToolID != "" {
				results[ev.ToolID] = ev
			}
		}
		for _, ev := range events {
			if ev.Kind != ChatKindToolUse {
				continue
			}
			entry := CommandEntry{
				Seq:               ev.Seq,
				ConversationID:    c.ID,
				ConversationTitle: c.Title,
				ToolName:          ev.ToolName,
				Input:             ev.Input,
				Time:              ev.Time,
			}
			if len(entry.Input) == 0 {
				entry.Input = json.RawMessage("{}")
			}
			if res, ok := results[ev.ToolID]; ok {
				entry.IsError = res.IsError
				entry.OutputPreview = truncateRunes(res.Text, commandOutputPreview)
			}
			out = append(out, entry)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func (a *API) handleCommands(w http.ResponseWriter, r *http.Request) {
	entries, err := a.scanCommands()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	q := r.URL.Query()
	tool, conv := q.Get("tool"), q.Get("conv")
	var errFilter *bool
	if v := q.Get("error"); v == "true" || v == "false" {
		b := v == "true"
		errFilter = &b
	}
	filtered := entries[:0]
	for _, e := range entries {
		if tool != "" && !strings.HasPrefix(e.ToolName, tool) {
			continue
		}
		if conv != "" && e.ConversationID != conv {
			continue
		}
		if errFilter != nil && e.IsError != *errFilter {
			continue
		}
		filtered = append(filtered, e)
	}
	limit := parseIntParam(r, "limit", 100)
	offset := parseIntParam(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := len(filtered)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	writeJSON(w, http.StatusOK, filtered[offset:end])
}

func (a *API) handleCommandStats(w http.ResponseWriter, _ *http.Request) {
	entries, err := a.scanCommands()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stats := CommandStats{
		Total:          len(entries),
		ByTool:         map[string]int{},
		ByConversation: map[string]int{},
	}
	for _, e := range entries {
		stats.ByTool[e.ToolName]++
		stats.ByConversation[e.ConversationID]++
		if e.IsError {
			stats.ErrorCount++
		}
	}
	writeJSON(w, http.StatusOK, stats)
}
