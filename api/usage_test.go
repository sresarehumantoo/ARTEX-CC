package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newUsageTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func seedConversationWithUsage(t *testing.T, a *API, model string, inputTokens, outputTokens int, cost float64) string {
	t.Helper()
	conv, err := a.chat.Create(CreateConversationRequest{Title: "test conv", Model: model})
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.chat.AppendEvent(conv.ID, ChatEvent{
		Kind: ChatKindResult,
		Text: "done",
		Usage: &ChatUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			CostUSD:      cost,
			DurationMS:   1000,
			NumTurns:     1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return conv.ID
}

func TestHandleUsage_Empty(t *testing.T) {
	srv := newUsageTestServer(t)
	code, body := agentsDo(t, srv, "GET", "/api/usage", "")
	var out []UsageSummary
	json.Unmarshal(body, &out)
	if code != 200 || len(out) != 0 {
		t.Fatalf("code=%d len=%d", code, len(out))
	}
}

func TestHandleUsage_WithData(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	seedConversationWithUsage(t, a, "sonnet", 100, 50, 0.01)
	seedConversationWithUsage(t, a, "opus", 200, 100, 0.05)

	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	code, body := agentsDo(t, srv, "GET", "/api/usage", "")
	var out []UsageSummary
	json.Unmarshal(body, &out)
	if code != 200 || len(out) != 2 {
		t.Fatalf("code=%d len=%d body=%s", code, len(out), body)
	}
	if out[0].TotalInputTokens == 0 || out[0].ResultCount != 1 {
		t.Fatalf("first summary: %+v", out[0])
	}
}

func TestHandleUsage_FilterByConv(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	id := seedConversationWithUsage(t, a, "sonnet", 100, 50, 0.01)
	seedConversationWithUsage(t, a, "opus", 200, 100, 0.05)

	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	code, body := agentsDo(t, srv, "GET", "/api/usage?conv="+id, "")
	var out []UsageSummary
	json.Unmarshal(body, &out)
	if code != 200 || len(out) != 1 {
		t.Fatalf("code=%d len=%d body=%s", code, len(out), body)
	}
	if out[0].ConversationID != id {
		t.Fatalf("wrong conv: %s", out[0].ConversationID)
	}
}

func TestHandleUsageTotals_Empty(t *testing.T) {
	srv := newUsageTestServer(t)
	code, body := agentsDo(t, srv, "GET", "/api/usage/totals", "")
	var totals UsageTotals
	json.Unmarshal(body, &totals)
	if code != 200 || totals.ConversationCount != 0 || len(totals.ByModel) != 0 {
		t.Fatalf("code=%d totals=%+v", code, totals)
	}
}

func TestHandleUsageTotals_WithData(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	seedConversationWithUsage(t, a, "sonnet", 100, 50, 0.01)
	seedConversationWithUsage(t, a, "opus", 200, 100, 0.05)
	seedConversationWithUsage(t, a, "", 50, 25, 0.005)

	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	code, body := agentsDo(t, srv, "GET", "/api/usage/totals", "")
	var totals UsageTotals
	json.Unmarshal(body, &totals)
	if code != 200 {
		t.Fatalf("code=%d body=%s", code, body)
	}
	if totals.ConversationCount != 3 {
		t.Fatalf("conversation_count: want 3, got %d", totals.ConversationCount)
	}
	if totals.TotalInputTokens != 350 || totals.TotalOutputTokens != 175 {
		t.Fatalf("totals: input=%d output=%d", totals.TotalInputTokens, totals.TotalOutputTokens)
	}
	if _, ok := totals.ByModel["sonnet"]; !ok {
		t.Fatal("missing sonnet in by_model")
	}
	if _, ok := totals.ByModel["default"]; !ok {
		t.Fatal("missing 'default' for empty model")
	}
}

func TestHandleUsage_NonResultEventsSkipped(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	conv, _ := a.chat.Create(CreateConversationRequest{Title: "text only"})
	a.chat.AppendEvent(conv.ID, ChatEvent{Kind: ChatKindText, Text: "hello"})

	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	code, body := agentsDo(t, srv, "GET", "/api/usage", "")
	var out []UsageSummary
	json.Unmarshal(body, &out)
	if code != 200 || len(out) != 0 {
		t.Fatalf("code=%d len=%d (non-result events should be skipped)", code, len(out))
	}
}

func TestHandleUsage_NoChatStore(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/usage", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want 404 (no chat store means route not registered), got %d", rec.Code)
	}
}
