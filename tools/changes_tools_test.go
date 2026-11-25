package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type changesMockStore struct {
	mockStore
	changes   []Change
	claims    []HeldClaim
	gotAfter  ChangeCursor
	gotLimit  int
	gotTask   int64
	gotOwner  string
	callCount int
}

func (m *changesMockStore) ListChanges(_ context.Context, _, taskID int64, after ChangeCursor, limit int) ([]Change, error) {
	m.callCount++
	m.gotAfter, m.gotLimit, m.gotTask = after, limit, taskID
	if len(m.changes) > limit {
		return m.changes[:limit], nil
	}
	return m.changes, nil
}

func (m *changesMockStore) ListClaims(_ context.Context, _ int64, owner string) ([]HeldClaim, error) {
	m.gotOwner = owner
	return m.claims, nil
}

func runListChanges(t *testing.T, m Store, ctx context.Context, in string) (Result, map[string]any) {
	t.Helper()
	tool := makeListChanges(m, 7, 1)
	r, err := tool.Run(ctx, json.RawMessage(in))
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError {
		return r, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(r.Content), &out); err != nil {
		t.Fatalf("decode %q: %v", r.Content, err)
	}
	return r, out
}

func TestChangeCursorRoundTrip(t *testing.T) {
	t.Parallel()
	c := ChangeCursor{At: time.UnixMicro(1764000000123456), Kind: "intent", ID: 42}
	got, err := ParseChangeCursor(c.String())
	if err != nil {
		t.Fatal(err)
	}
	if !got.At.Equal(c.At) || got.Kind != c.Kind || got.ID != c.ID {
		t.Errorf("round trip = %+v, want %+v", got, c)
	}

	ts, err := ParseChangeCursor("2026-11-25T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if ts.Kind != "" || ts.ID != 0 || !ts.At.Equal(time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("timestamp cursor = %+v", ts)
	}

	for _, bad := range []string{"yesterday", "c1.intent", "cX.intent.1", "1.intent.2"} {
		if _, err := ParseChangeCursor(bad); err == nil {
			t.Errorf("ParseChangeCursor(%q) succeeded, want error", bad)
		}
	}
}

func TestListChangesRequiresSince(t *testing.T) {
	t.Parallel()
	m := &changesMockStore{}
	for _, in := range []string{`{}`, `{"since": "soon"}`} {
		r, _ := runListChanges(t, m, context.Background(), in)
		if !r.IsError {
			t.Errorf("input %s: want error, got %s", in, r.Content)
		}
	}
	if m.callCount != 0 {
		t.Errorf("store called %d times on bad input", m.callCount)
	}
}

func TestListChangesGroupsAndAdvancesCursor(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	m := &changesMockStore{
		changes: []Change{
			{Kind: "finding", Cursor: ChangeCursor{At: at, Kind: "finding", ID: 3}, Fields: map[string]any{"id": 3}},
			{Kind: "intent", Cursor: ChangeCursor{At: at.Add(time.Second), Kind: "intent", ID: 9}, Fields: map[string]any{"id": 9, "claimed_by": OwnerLabel("stdio-b")}},
		},
		claims: []HeldClaim{{NodeID: 11, ExpiresAt: at.Add(30 * time.Minute)}},
	}
	ctx := WithSessionID(context.Background(), "stdio-a")
	_, out := runListChanges(t, m, ctx, `{"since": "2026-11-25T09:00:00Z"}`)

	if m.gotLimit != defaultChangesLimit+1 {
		t.Errorf("store limit = %d, want %d", m.gotLimit, defaultChangesLimit+1)
	}
	if m.gotTask != 7 || m.gotOwner != "stdio-a" {
		t.Errorf("task = %d owner = %q, want 7 stdio-a", m.gotTask, m.gotOwner)
	}
	if !m.gotAfter.At.Equal(at.Add(-time.Hour)) {
		t.Errorf("after = %v", m.gotAfter)
	}
	if out["session"] != OwnerLabel("stdio-a") || out["truncated"] != false || out["count"] != float64(2) {
		t.Errorf("header = %v", out)
	}
	if want := m.changes[1].Cursor.String(); out["cursor"] != want {
		t.Errorf("cursor = %v, want %s", out["cursor"], want)
	}
	if f, _ := out["findings"].([]any); len(f) != 1 {
		t.Errorf("findings = %v", out["findings"])
	}
	if i, _ := out["intents"].([]any); len(i) != 1 {
		t.Errorf("intents = %v", out["intents"])
	}
	claims, _ := out["my_claims"].([]any)
	if len(claims) != 1 || claims[0].(map[string]any)["node_id"] != float64(11) {
		t.Errorf("my_claims = %v", out["my_claims"])
	}
}

func TestListChangesTruncates(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	m := &changesMockStore{}
	for i := range 4 {
		id := int64(i + 1)
		m.changes = append(m.changes, Change{Kind: "asset", Cursor: ChangeCursor{At: at, Kind: "asset", ID: id}, Fields: map[string]any{"id": id}})
	}
	_, out := runListChanges(t, m, context.Background(), `{"since": "`+ChangeCursor{At: at.Add(-time.Minute)}.String()+`", "limit": 3}`)

	if out["truncated"] != true || out["count"] != float64(3) {
		t.Fatalf("out = %v", out)
	}
	if want := m.changes[2].Cursor.String(); out["cursor"] != want {
		t.Errorf("cursor = %v, want %s", out["cursor"], want)
	}
	if note, _ := out["note"].(string); !strings.Contains(note, "call list_changes again") {
		t.Errorf("note = %q", note)
	}
}

func TestListChangesNoChangesKeepsCursor(t *testing.T) {
	t.Parallel()
	since := ChangeCursor{At: time.UnixMicro(1764000000000001), Kind: "fact", ID: 5}.String()
	m := &changesMockStore{}
	_, out := runListChanges(t, m, context.Background(), `{"since": "`+since+`", "limit": 9999}`)
	if out["cursor"] != since || out["count"] != float64(0) {
		t.Errorf("out = %v, want cursor %s", out, since)
	}
	if m.gotLimit != maxChangesLimit+1 {
		t.Errorf("store limit = %d, want %d", m.gotLimit, maxChangesLimit+1)
	}
}

func TestListChangesOwnerArgument(t *testing.T) {
	t.Parallel()
	m := &changesMockStore{}
	_, out := runListChanges(t, m, context.Background(), `{"since": "2026-11-25T09:00:00Z", "owner": "f00d"}`)
	if m.gotOwner != "owner:f00d" || out["session"] != OwnerLabel("owner:f00d") {
		t.Errorf("owner = %q, session = %v", m.gotOwner, out["session"])
	}

	m = &changesMockStore{}
	_, out = runListChanges(t, m, context.Background(), `{"since": "2026-11-25T09:00:00Z"}`)
	if _, ok := out["my_claims"]; ok || out["session"] != nil {
		t.Errorf("sessionless call without owner returned claims: %v", out)
	}
	if msg, _ := out["claims_error"].(string); !strings.Contains(msg, "pass owner") {
		t.Errorf("claims_error = %v", out["claims_error"])
	}
}
