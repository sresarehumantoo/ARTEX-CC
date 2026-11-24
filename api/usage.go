package api

import (
	"net/http"
	"sort"
)

type UsageSummary struct {
	ConversationID           string  `json:"conversation_id"`
	ConversationTitle        string  `json:"conversation_title"`
	Model                    string  `json:"model"`
	TotalInputTokens         int     `json:"total_input_tokens"`
	TotalOutputTokens        int     `json:"total_output_tokens"`
	TotalCacheReadTokens     int     `json:"total_cache_read_tokens"`
	TotalCacheCreationTokens int     `json:"total_cache_creation_tokens"`
	TotalCostUSD             float64 `json:"total_cost_usd"`
	TotalDurationMS          int64   `json:"total_duration_ms"`
	TotalTurns               int     `json:"total_turns"`
	ResultCount              int     `json:"result_count"`
	FirstTime                string  `json:"first_time"`
	LastTime                 string  `json:"last_time"`
}

type ModelUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type UsageTotals struct {
	TotalInputTokens         int                   `json:"total_input_tokens"`
	TotalOutputTokens        int                   `json:"total_output_tokens"`
	TotalCacheReadTokens     int                   `json:"total_cache_read_tokens"`
	TotalCacheCreationTokens int                   `json:"total_cache_creation_tokens"`
	TotalCostUSD             float64               `json:"total_cost_usd"`
	TotalDurationMS          int64                 `json:"total_duration_ms"`
	TotalTurns               int                   `json:"total_turns"`
	ConversationCount        int                   `json:"conversation_count"`
	ByModel                  map[string]ModelUsage `json:"by_model"`
}

func (a *API) registerUsageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/usage", a.handleUsage)
	mux.HandleFunc("GET /api/usage/totals", a.handleUsageTotals)
}

func (a *API) scanUsage(conv string) ([]UsageSummary, error) {
	convs, err := a.chat.List()
	if err != nil {
		return nil, err
	}
	out := []UsageSummary{}
	for _, c := range convs {
		if conv != "" && c.ID != conv {
			continue
		}
		events, err := a.chat.Events(c.ID, 0)
		if err != nil {
			continue
		}
		s := UsageSummary{ConversationID: c.ID, ConversationTitle: c.Title, Model: c.Model}
		for _, ev := range events {
			if ev.Kind != ChatKindResult || ev.Usage == nil {
				continue
			}
			u := ev.Usage
			s.TotalInputTokens += u.InputTokens
			s.TotalOutputTokens += u.OutputTokens
			s.TotalCacheReadTokens += u.CacheReadTokens
			s.TotalCacheCreationTokens += u.CacheCreationTokens
			s.TotalCostUSD += u.CostUSD
			s.TotalDurationMS += u.DurationMS
			s.TotalTurns += u.NumTurns
			s.ResultCount++
			if s.FirstTime == "" || ev.Time < s.FirstTime {
				s.FirstTime = ev.Time
			}
			if ev.Time > s.LastTime {
				s.LastTime = ev.Time
			}
		}
		if s.ResultCount > 0 {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastTime > out[j].LastTime })
	return out, nil
}

func (a *API) handleUsage(w http.ResponseWriter, r *http.Request) {
	out, err := a.scanUsage(r.URL.Query().Get("conv"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleUsageTotals(w http.ResponseWriter, _ *http.Request) {
	list, err := a.scanUsage("")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	t := UsageTotals{ByModel: map[string]ModelUsage{}, ConversationCount: len(list)}
	for _, s := range list {
		t.TotalInputTokens += s.TotalInputTokens
		t.TotalOutputTokens += s.TotalOutputTokens
		t.TotalCacheReadTokens += s.TotalCacheReadTokens
		t.TotalCacheCreationTokens += s.TotalCacheCreationTokens
		t.TotalCostUSD += s.TotalCostUSD
		t.TotalDurationMS += s.TotalDurationMS
		t.TotalTurns += s.TotalTurns
		m := s.Model
		if m == "" {
			m = "default"
		}
		mu := t.ByModel[m]
		mu.InputTokens += s.TotalInputTokens
		mu.OutputTokens += s.TotalOutputTokens
		mu.CostUSD += s.TotalCostUSD
		t.ByModel[m] = mu
	}
	writeJSON(w, http.StatusOK, t)
}
