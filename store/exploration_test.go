package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNodeJSONOmitsOwner(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(Node{ID: 1, Kind: "intent", State: "running", Owner: "mcp-session-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "mcp-session-secret") || strings.Contains(string(raw), `"owner"`) {
		t.Errorf("Node JSON exposes the raw owner: %s", raw)
	}
}

func TestUtf8Clean(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean string", "hello world", "hello world"},
		{"null bytes", "hello\x00world", "helloworld"},
		{"invalid utf8", "hello\xff\xfe", "hello�"},
		{"empty", "", ""},
		{"mixed", "\x00hi\xff", "hi�"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utf8Clean(tt.input)
			if got != tt.want {
				t.Errorf("utf8Clean(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestJsonbClean(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no nulls", `{"key":"value"}`, `{"key":"value"}`},
		{"with null escape", `{"key":"val\u0000ue"}`, `{"key":"value"}`},
		{"multiple null escapes", `{"a":"\u0000","b":"\u0000"}`, `{"a":"","b":""}`},
		{"normal escape", `{"a":"line\nbreak"}`, `{"a":"line\nbreak"}`},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(jsonbClean([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("jsonbClean(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNullID(t *testing.T) {
	t.Parallel()
	if nullID(0) != nil {
		t.Errorf("nullID(0) = %v, want nil", nullID(0))
	}
	if nullID(-1) != nil {
		t.Errorf("nullID(-1) = %v, want nil", nullID(-1))
	}
	v := nullID(42)
	if v != int64(42) {
		t.Errorf("nullID(42) = %v, want 42", v)
	}
}

func TestNodeToTools(t *testing.T) {
	t.Parallel()
	n := Node{
		ID:      1,
		Kind:    KindGoal,
		State:   "open",
		Origin:  "planner",
		Payload: json.RawMessage(`{"text":"find xss"}`),
	}
	tn := nodeToTools(n)
	if tn.ID != 1 {
		t.Errorf("ID = %d, want 1", tn.ID)
	}
	if tn.Kind != KindGoal {
		t.Errorf("Kind = %q, want %q", tn.Kind, KindGoal)
	}
	if tn.State != "open" {
		t.Errorf("State = %q, want %q", tn.State, "open")
	}
	if tn.Origin != "planner" {
		t.Errorf("Origin = %q, want %q", tn.Origin, "planner")
	}
	if tn.Payload["text"] != "find xss" {
		t.Errorf("Payload[text] = %v, want 'find xss'", tn.Payload["text"])
	}
}

func TestNodeToToolsEmptyPayload(t *testing.T) {
	t.Parallel()
	n := Node{ID: 2, Kind: KindFact, State: "confirmed"}
	tn := nodeToTools(n)
	if tn.Payload == nil {
		t.Error("Payload should be initialized to empty map, not nil")
	}
	if len(tn.Payload) != 0 {
		t.Errorf("Payload should be empty, got %v", tn.Payload)
	}
}
