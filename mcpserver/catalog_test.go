package mcpserver

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestCatalogCount(t *testing.T) {
	const want = 65
	if got := len(Catalog()); got != want {
		t.Fatalf("catalog has %d tools, want %d; update this count and the docs when adding or removing tools", got, want)
	}
}

func TestCatalogCategories(t *testing.T) {
	for _, e := range Catalog() {
		if !slices.Contains(tools.Categories, e.Category) {
			t.Errorf("tool %q has unknown category %q", e.Name, e.Category)
		}
	}
}

func TestCatalogEntries(t *testing.T) {
	seen := map[string]bool{}
	proxy := 0
	for _, e := range Catalog() {
		if seen[e.Name] {
			t.Errorf("duplicate tool %q", e.Name)
		}
		seen[e.Name] = true
		if e.Description == "" {
			t.Errorf("tool %q has no description", e.Name)
		}
		var schema map[string]any
		if err := json.Unmarshal(e.InputSchema, &schema); err != nil || schema["type"] != "object" {
			t.Errorf("tool %q has invalid input schema: %s", e.Name, e.InputSchema)
		}
		if e.RequiresProxy {
			proxy++
		}
	}
	if proxy != 12 {
		t.Errorf("%d tools require the proxy, want 12", proxy)
	}
}
