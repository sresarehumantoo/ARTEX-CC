package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func reopenRequest(h http.Handler, id int64, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/nodes/%d/reopen", id), strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandleReopenNode_ReasonRequired(t *testing.T) {
	h := newTestAPI(&mockStore{}).Handler()
	for _, body := range []string{`{}`, `{"reason":"  "}`} {
		rec := reopenRequest(h, 5, body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "reason is required") {
			t.Errorf("body %s: got %d %s, want 400 reason is required", body, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleReopenNode_Store(t *testing.T) {
	dsn := os.Getenv("ARTEX_TEST_PG")
	if dsn == "" {
		t.Skip("ARTEX_TEST_PG not set")
	}
	ctx := context.Background()
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	taskID, expID, err := s.CreateEngagement(ctx, "api-reopen goal", "api-reopen description")
	if err != nil {
		t.Fatal(err)
	}
	_, otherExp, err := s.CreateEngagement(ctx, "api-reopen other goal", "api-reopen other description")
	if err != nil {
		t.Fatal(err)
	}
	intent, err := s.AddNode(ctx, expID, tools.KindIntent, map[string]any{"summary": "test login"}, 0, "open", "agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimIntent(ctx, intent, "sess-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateNodeFields(ctx, intent, "done", map[string]any{"result": "nothing"}); err != nil {
		t.Fatal(err)
	}
	openIntent, err := s.AddNode(ctx, expID, tools.KindIntent, map[string]any{"summary": "open"}, 0, "open", "agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.AddNode(ctx, otherExp, tools.KindGoal, map[string]any{"text": "g"}, 0, "met", "system", nil)
	if err != nil {
		t.Fatal(err)
	}

	h := New(s, nil, taskID, expID).Handler()

	rec := reopenRequest(h, intent, `{"reason":"missed the second form"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen done intent: got %d %s", rec.Code, rec.Body.String())
	}
	var res tools.ReopenResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || res.ID != intent || res.Kind != tools.KindIntent {
		t.Errorf("response = %s (%v)", rec.Body.String(), err)
	}
	n, err := s.NodeDetail(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	if n.State != "open" || n.Payload["reopen_reason"] != "missed the second form" || n.Payload["reopened_by"] != "web-ui" {
		t.Errorf("node = %+v", n)
	}

	if rec := reopenRequest(h, openIntent, `{"reason":"why"}`); rec.Code != http.StatusConflict {
		t.Errorf("reopen open intent: got %d %s, want 409", rec.Code, rec.Body.String())
	}
	if rec := reopenRequest(h, foreign, `{"reason":"why"}`); rec.Code != http.StatusNotFound {
		t.Errorf("reopen node of another exploration: got %d %s, want 404", rec.Code, rec.Body.String())
	}
	if goal, _ := s.NodeDetail(ctx, foreign); goal.State != "met" {
		t.Errorf("foreign goal state = %q, want met", goal.State)
	}
}
