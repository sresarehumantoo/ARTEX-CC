package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseKillAfter(t *testing.T) {
	k, err := parseKillAfter("2:3m", 3)
	if err != nil || k.Instance != 2 || k.After != 3*time.Minute {
		t.Fatalf("got %+v, %v", k, err)
	}
	if k, err := parseKillAfter("", 3); k != nil || err != nil {
		t.Fatalf("empty: got %+v, %v", k, err)
	}
	for _, bad := range []string{"2", "4:1m", "0:1m", "x:1m", "2:soon"} {
		if _, err := parseKillAfter(bad, 3); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestBuildPrompt(t *testing.T) {
	sc := Scenario{Target: "http://localhost:8083", Goal: "find bugs"}
	single := buildPrompt(sc, 1, 1, "stdio")
	if strings.Contains(single, "instance") || strings.Contains(single, "owner") {
		t.Fatalf("single stdio prompt must stay unchanged: %s", single)
	}
	multi := buildPrompt(sc, 2, 3, "stdio")
	if !strings.HasPrefix(multi, single) || !strings.Contains(multi, "You are instance 2 of 3;") {
		t.Fatalf("multi prompt: %s", multi)
	}
	if strings.Contains(multi, "owner") {
		t.Fatalf("stdio prompt should not ask for an owner: %s", multi)
	}
	if http := buildPrompt(sc, 1, 3, "http"); !strings.Contains(http, "pass it as owner on every start_intent, complete_intent, cancel_intent and list_changes call") {
		t.Fatalf("http prompt: %s", http)
	}
}

func TestClaudeArgsLoadOnlyTheEvalMCPConfig(t *testing.T) {
	args := claudeArgs("/tmp/mcp.json", "claude-opus-5-5", "go")
	has := func(flag string) bool {
		for _, a := range args {
			if a == flag {
				return true
			}
		}
		return false
	}
	for _, flag := range []string{"--strict-mcp-config", "--mcp-config", "--dangerously-skip-permissions", "--print"} {
		if !has(flag) {
			t.Errorf("claude args %v missing %s", args, flag)
		}
	}
	if got := args[len(args)-2:]; got[0] != "-p" || got[1] != "go" {
		t.Errorf("prompt not last: %v", got)
	}
	if !has("--model") || has("") {
		t.Errorf("model handling wrong: %v", args)
	}
	if noModel := claudeArgs("/tmp/mcp.json", "", "go"); len(noModel) != len(args)-2 {
		t.Errorf("empty model should omit --model: %v", noModel)
	}
}

func TestCoverageEntries(t *testing.T) {
	for _, tc := range []struct {
		cov  map[string]any
		want int
	}{
		{map[string]any{"entries": 20, "gaps": 2}, 20},
		{map[string]any{"entries": float64(7)}, 7},
		{map[string]any{"total": 9}, 0},
		{nil, 0},
	} {
		if got := coverageEntries(tc.cov); got != tc.want {
			t.Errorf("coverageEntries(%v) = %d, want %d", tc.cov, got, tc.want)
		}
	}
}
