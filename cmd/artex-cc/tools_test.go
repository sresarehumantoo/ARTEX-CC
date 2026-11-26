package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
)

func TestWriteToolsJSON(t *testing.T) {
	var buf bytes.Buffer
	writeToolsJSON(&buf, mcpserver.Catalog())

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != len(mcpserver.Catalog()) {
		t.Fatalf("got %d entries, want %d", len(got), len(mcpserver.Catalog()))
	}
	for _, key := range []string{"name", "description", "category", "inputSchema", "readOnly", "requiresProxy"} {
		if _, ok := got[0][key]; !ok {
			t.Errorf("entry missing %q", key)
		}
	}
}

func TestWriteToolsText(t *testing.T) {
	var buf bytes.Buffer
	writeToolsText(&buf, mcpserver.Catalog())
	out := buf.String()

	for _, want := range []string{"Exploration graph (", "Traffic (", "graph_overview", "[proxy]", "65 tools"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestSummarize(t *testing.T) {
	if got := summarize("First sentence. Second sentence.", 72); got != "First sentence." {
		t.Errorf("got %q", got)
	}
	if got := summarize(strings.Repeat("x", 100), 10); got != "xxxxxxx..." {
		t.Errorf("got %q", got)
	}
}
