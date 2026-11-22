package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTrafficPatternsRegistered(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_patterns")
	if !ok {
		t.Fatal("traffic_patterns not found")
	}
	if !tool.ReadOnly {
		t.Error("expected traffic_patterns to be read-only")
	}
}

func TestTrafficPatternsInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_patterns")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTrafficPatternsEmptyHost(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_patterns")
	if !ok {
		t.Fatal("traffic_patterns not found")
	}
	_, err := tool.Run(context.Background(), json.RawMessage(`{"host": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTrafficAnalysisToolsAllRegistered(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	expected := []string{
		"traffic_stats", "traffic_timeline", "traffic_patterns",
	}
	for _, name := range expected {
		if _, ok := reg.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}
