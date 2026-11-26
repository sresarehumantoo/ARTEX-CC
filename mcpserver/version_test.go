package mcpserver

import (
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestWithVersionSetsServerVersion(t *testing.T) {
	if got := New(tools.NewRegistry()).version; got != "dev" {
		t.Fatalf("default version = %q, want dev", got)
	}
	if got := New(tools.NewRegistry(), WithVersion("0.7.0")).version; got != "0.7.0" {
		t.Fatalf("version = %q, want 0.7.0", got)
	}
}
