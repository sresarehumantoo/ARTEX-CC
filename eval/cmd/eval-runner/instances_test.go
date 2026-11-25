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
