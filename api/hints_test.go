package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestHandleAddHint_PersistsActiveHint(t *testing.T) {
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
	taskID, expID, err := s.CreateEngagement(ctx, "api-hint goal", "api-hint description")
	if err != nil {
		t.Fatal(err)
	}

	h := New(s, nil, taskID, expID).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/hints", strings.NewReader(`{"text":"check /admin"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	hints, err := s.ListNodes(ctx, expID, tools.KindHint, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != 1 || hints[0].State != tools.StateHintActive || hints[0].Payload["text"] != "check /admin" {
		t.Fatalf("hints = %+v, want one active hint", hints)
	}
}
