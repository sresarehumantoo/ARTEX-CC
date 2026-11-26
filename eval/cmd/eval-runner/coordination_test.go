package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

var t0 = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

// transcript builds a stream-json file in the shape claude --print
// --output-format stream-json --verbose writes.
type transcript struct {
	lines []string
	n     int
}

func (b *transcript) add(v any) {
	data, _ := json.Marshal(v)
	b.lines = append(b.lines, string(data))
}

func (b *transcript) call(at time.Time, tool string, input any, result string, isError bool) *transcript {
	b.n++
	id := fmt.Sprintf("toolu_%d", b.n)
	name := tool
	if tool != "Bash" {
		name = "mcp__artex__" + tool
	}
	b.add(map[string]any{
		"type":      "assistant",
		"timestamp": at.Add(-time.Second).Format(time.RFC3339Nano),
		"message": map[string]any{
			"id":      fmt.Sprintf("msg_%d", b.n),
			"content": []any{map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}},
			"usage":   map[string]any{"input_tokens": 10, "output_tokens": 5, "cache_read_input_tokens": 100},
		},
	})
	b.add(map[string]any{
		"type":      "user",
		"timestamp": at.Format(time.RFC3339Nano),
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "tool_result", "tool_use_id": id, "is_error": isError,
				"content": []any{map[string]any{"type": "text", "text": result}},
			}},
		},
	})
	return b
}

func (b *transcript) claim(at time.Time, intent int64, holder string, lease time.Duration) *transcript {
	return b.call(at, "start_intent", map[string]any{"node_id": intent},
		fmt.Sprintf("intent %d is now running; claimed by %s until %s", intent, holder, at.Add(lease).UTC().Format(time.RFC3339)), false)
}

func (b *transcript) conflict(at time.Time, intent int64, holder string) *transcript {
	return b.call(at, "start_intent", map[string]any{"node_id": intent},
		fmt.Sprintf("intent %d is claimed by %s until %s (in 25m0s); pick another intent", intent, holder, at.Add(25*time.Minute).Format(time.RFC3339)), true)
}

func (b *transcript) start(at time.Time, intent int64) *transcript {
	return b.call(at, "start_intent", map[string]any{"node_id": intent}, "temporary failure", true)
}

func (b *transcript) other(at time.Time) *transcript {
	return b.call(at, "graph_overview", map[string]any{}, `{"goals":[]}`, false)
}

func (b *transcript) result(cost float64) *transcript {
	b.add(map[string]any{
		"type": "result", "subtype": "success", "total_cost_usd": cost,
		"usage": map[string]any{"input_tokens": 7, "output_tokens": 3, "cache_creation_input_tokens": 50, "cache_read_input_tokens": 900},
	})
	return b
}

func (b *transcript) parse(t *testing.T, instance int) *Transcript {
	t.Helper()
	path := filepath.Join(t.TempDir(), fmt.Sprintf("instance-%d.jsonl", instance))
	if err := os.WriteFile(path, []byte(strings.Join(b.lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tr, err := parseTranscript(instance, path)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func at(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }

func TestParseTranscript(t *testing.T) {
	b := &transcript{}
	b.claim(at(1), 10, "h-aaa", 30*time.Minute)
	// A second line for the same API message must not count its usage twice.
	b.add(map[string]any{"type": "assistant", "message": map[string]any{
		"id": "msg_1", "content": []any{map[string]any{"type": "text", "text": "ok"}},
		"usage": map[string]any{"input_tokens": 10, "output_tokens": 5, "cache_read_input_tokens": 100},
	}})
	b.add(map[string]any{"type": "user", "timestamp": at(2).Format(time.RFC3339Nano), "message": map[string]any{
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "missing", "content": "x"}},
	}})
	b.lines = append(b.lines, `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"cut`)
	tr := b.parse(t, 1)

	if len(tr.Calls) != 1 || tr.Calls[0].Tool != "start_intent" || !tr.Calls[0].At.Equal(at(1)) {
		t.Fatalf("calls: %+v", tr.Calls)
	}
	if !strings.HasPrefix(tr.Calls[0].Result, "intent 10 is now running") {
		t.Fatalf("result text: %q", tr.Calls[0].Result)
	}
	if tr.HasResult || tr.Usage.InputTokens != 10 || tr.Usage.CacheReadTokens != 100 {
		t.Fatalf("usage without a result message: %+v", tr)
	}

	tr = b.result(1.25).parse(t, 1)
	if !tr.HasResult || tr.CostUSD != 1.25 || tr.Usage.OutputTokens != 3 || tr.Usage.CacheCreationTokens != 50 {
		t.Fatalf("usage from the result message: %+v", tr)
	}
}

func TestResultTextString(t *testing.T) {
	if got := resultText(json.RawMessage(`"plain"`)); got != "plain" {
		t.Fatalf("got %q", got)
	}
}

func TestDoubleClaims(t *testing.T) {
	one := (&transcript{}).
		claim(at(0), 10, "h-aaa", 30*time.Minute).
		claim(at(0), 12, "h-aaa", 2*time.Minute).
		claim(at(0), 13, "h-aaa", 30*time.Minute).
		call(at(1), "complete_intent", map[string]any{"node_id": 13}, "intent 13 marked done", false).
		call(at(1), "start_intent", map[string]any{"node_id": 10},
			fmt.Sprintf("intent 10 is now running; renewed by h-aaa until %s", at(31).Format(time.RFC3339)), false)
	two := (&transcript{}).
		claim(at(5), 10, "h-bbb", 30*time.Minute).
		claim(at(3), 12, "h-bbb", 30*time.Minute).
		claim(at(4), 13, "h-bbb", 30*time.Minute)

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{one.parse(t, 1), two.parse(t, 2)}})
	if len(c.DoubleClaims) != 1 {
		t.Fatalf("want one double claim, got %+v", c.DoubleClaims)
	}
	d := c.DoubleClaims[0]
	if d.IntentID != 10 || d.FirstInstance != 1 || d.SecondInstance != 2 || d.SecondHolder != "h-bbb" {
		t.Fatalf("double claim: %+v", d)
	}
	if c.Instances[0].ClaimsTaken != 4 || c.Instances[1].ClaimsTaken != 3 {
		t.Fatalf("claims taken: %+v", c.Instances)
	}
}

func TestDoubleClaimsUsesLeaseWhenExpiryUnparsable(t *testing.T) {
	one := (&transcript{}).call(at(0), "start_intent", map[string]any{"node_id": 10},
		"intent 10 is now running; claimed by h-aaa until soon", false)
	two := (&transcript{}).claim(at(3), 10, "h-bbb", time.Minute)

	in := CoordinationInput{Transcripts: []*Transcript{one.parse(t, 1), two.parse(t, 2)}, Lease: 2 * time.Minute}
	if c := scoreCoordination(in); len(c.DoubleClaims) != 0 {
		t.Fatalf("claim after a 2m lease is not a double claim: %+v", c.DoubleClaims)
	}
	in.Lease = 0
	if c := scoreCoordination(in); len(c.DoubleClaims) != 1 {
		t.Fatalf("default 30m lease should flag it: %+v", c.DoubleClaims)
	}
}

func TestConflictHandling(t *testing.T) {
	movedOn := (&transcript{}).
		conflict(at(1), 20, "h-aaa").
		other(at(1)).
		call(at(2), "start_intent", map[string]any{"node_id": "21"}, "intent 21 is not open", true)
	mixed := (&transcript{}).
		conflict(at(1), 20, "h-aaa").
		other(at(1)).other(at(1)).other(at(1)).other(at(1)).other(at(1)).
		claim(at(2), 21, "h-ccc", 30*time.Minute).
		conflict(at(3), 22, "h-aaa").
		conflict(at(3), 22, "h-aaa").
		conflict(at(4), 30, "h-aaa").
		call(at(4), "complete_intent", map[string]any{"node_id": 30}, "intent 30 marked done", false).
		conflict(at(5), 31, "h-aaa").
		call(at(5), "report_finding", map[string]any{"intent_id": 31, "vulnclass": "sqli"}, `{"finding_id":1,"node_id":40}`, false)

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{movedOn.parse(t, 2), mixed.parse(t, 3)}})
	if c.Conflicts != 6 || c.ConflictsHandled != 3 || c.ConflictsHandledPc != 50 {
		t.Fatalf("conflicts %d handled %d (%.0f%%)", c.Conflicts, c.ConflictsHandled, c.ConflictsHandledPc)
	}
	want := []ConflictRef{{Instance: 3, Call: 7, IntentID: 22, Holder: "h-aaa"}, {Instance: 3, Call: 9, IntentID: 30, Holder: "h-aaa"}, {Instance: 3, Call: 11, IntentID: 31, Holder: "h-aaa"}}
	if len(c.UnhandledConflicts) != len(want) {
		t.Fatalf("unhandled: %+v", c.UnhandledConflicts)
	}
	for k, u := range want {
		if c.UnhandledConflicts[k] != u {
			t.Fatalf("unhandled[%d] = %+v, want %+v (all: %+v)", k, c.UnhandledConflicts[k], u, c.UnhandledConflicts)
		}
	}
	if c.Instances[1].Conflicts != 5 {
		t.Fatalf("per instance conflicts: %+v", c.Instances)
	}
}

func TestRefusedCompletions(t *testing.T) {
	b := (&transcript{}).
		call(at(1), "complete_intent", map[string]any{"node_id": 10},
			"intent 10 is claimed by h-aaa until 2026-11-25T12:30:00Z (in 29m0s); pick another intent", true).
		call(at(1), "cancel_intent", map[string]any{"node_id": 11},
			"intent 11 is claimed by h-aaa until 2026-11-25T12:30:00Z (in 29m0s); pick another intent", true).
		call(at(2), "complete_intent", map[string]any{"node_id": 12}, "intent 12 marked done", false)

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{b.parse(t, 1)}})
	if c.RefusedCompletions != 2 || c.Conflicts != 0 {
		t.Fatalf("refused %d conflicts %d", c.RefusedCompletions, c.Conflicts)
	}
}

func finding(at time.Time, findingID, nodeID int64) (time.Time, string, any, string, bool) {
	return at, "report_finding", map[string]any{"vulnclass": "x", "severity": "high", "summary": "s"},
		fmt.Sprintf(`{"finding_id":%d,"node_id":%d}`, findingID, nodeID), false
}

func TestDuplicateFindings(t *testing.T) {
	one := (&transcript{}).call(finding(at(1), 1, 100)).call(finding(at(1), 3, 102)).call(finding(at(1), 5, 104))
	two := (&transcript{}).call(finding(at(2), 2, 101)).call(finding(at(2), 4, 103)).call(finding(at(2), 6, 105))
	findings := []EngagementFinding{
		{ID: 1, VulnClass: "SQL Injection", AssetIDs: []int64{5}},
		{ID: 2, VulnClass: "sql_injection", AssetIDs: []int64{5}, MergedInto: 1},
		{ID: 3, VulnClass: "ssrf", AssetIDs: []int64{5}},
		{ID: 4, VulnClass: "SSRF", AssetIDs: []int64{5}},
		{ID: 5, VulnClass: "idor", AssetIDs: []int64{5}},
		{ID: 6, VulnClass: "idor", AssetIDs: []int64{6}},
		{ID: 7, VulnClass: "ssrf", AssetIDs: []int64{5}},
	}

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{one.parse(t, 1), two.parse(t, 2)}, Findings: findings})
	df := c.DuplicateFindings
	if df.Raw != 3 || df.AfterMerges != 2 || len(df.Groups) != 2 {
		t.Fatalf("duplicates: %+v", df)
	}
	if g := df.Groups[0]; g.VulnClass != "sqlinjection" || g.Live != 1 || len(g.Instances) != 2 {
		t.Fatalf("sqli group: %+v", g)
	}
}

func bash(at time.Time, cmd string) (time.Time, string, any, string, bool) {
	return at, "Bash", map[string]any{"command": cmd, "description": "scan"}, "ok", false
}

func TestRedundantReconAndListChanges(t *testing.T) {
	one := (&transcript{}).
		call(bash(at(1), "nmap  -sV -p- localhost")).
		call(bash(at(2), "curl -s http://localhost:8083/")).
		call(at(3), "list_changes", map[string]any{"since": "c1.fact.1"}, `{"count":0}`, false).
		call(at(4), "list_changes", map[string]any{"since": "c1.fact.1"}, `{"count":0}`, false)
	two := (&transcript{}).
		call(bash(at(1), "nmap -sV -p- localhost\n")).
		call(bash(at(2), "nmap -sV -p- localhost")).
		call(at(3), "list_changes", map[string]any{"since": "c1.fact.1"}, `{"count":0}`, false)
	three := (&transcript{}).call(bash(at(1), "curl -s http://localhost:8083/admin"))

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{one.parse(t, 1), two.parse(t, 2), three.parse(t, 3)}})
	rr := c.RedundantRecon
	if rr.SharedCommands != 1 || rr.RedundantRuns != 1 || rr.Examples[0].Runs != 3 || rr.Examples[0].Command != "nmap -sV -p- localhost" {
		t.Fatalf("redundant recon: %+v", rr)
	}
	if c.Instances[0].ListChanges != 2 || c.Instances[1].ListChanges != 1 || c.Instances[2].ListChanges != 0 {
		t.Fatalf("list_changes: %+v", c.Instances)
	}
	if c.Instances[1].BashCommands != 2 {
		t.Fatalf("bash commands: %+v", c.Instances[1])
	}
}

func TestCrossInstanceParents(t *testing.T) {
	one := (&transcript{}).
		call(at(1), "add_intent", map[string]any{"summary": "probe"}, "intent created: 10", false).
		call(at(2), "record_fact", map[string]any{"facts": []any{map[string]any{"summary": "port 8083 open", "intent_id": 10}}}, "fact created: 200", false)
	two := (&transcript{}).
		call(at(3), "add_intent", map[string]any{"intents": []any{map[string]any{"summary": "a"}, map[string]any{"summary": "b"}}},
			`{"ids":[300,0,301],"errors":{"1":"summary must not be empty"}}`, false).
		call(at(4), "record_fact", map[string]any{"facts": []any{map[string]any{"summary": "x", "intent_id": 10}}}, "fact created: 201", false).
		call(finding(at(5), 9, 202))
	edges := []tools.Edge{
		{From: 10, Rel: tools.RelYields, To: 200},
		{From: 200, Rel: tools.RelDerivedFrom, To: 300},
		{From: 10, Rel: tools.RelYields, To: 201},
		{From: 300, Rel: tools.RelYields, To: 202},
		{From: 1, Rel: tools.RelDerivedFrom, To: 10},
	}

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{one.parse(t, 1), two.parse(t, 2)}, Edges: edges})
	cp := c.CrossParents
	if cp.Total != 2 || cp.ByKind["intent"] != 1 || cp.ByKind["fact"] != 1 {
		t.Fatalf("cross parents: %+v", cp)
	}
	if c.Instances[1].CrossParents != 2 || c.Instances[1].NodesCreated != 4 {
		t.Fatalf("instance 2: %+v", c.Instances[1])
	}
}

func TestTakeoverAfterKill(t *testing.T) {
	killed := (&transcript{}).
		claim(at(0), 30, "h-bbb", 2*time.Minute).
		claim(at(0), 31, "h-bbb", 2*time.Minute).
		claim(at(0), 32, "h-bbb", 2*time.Minute).
		call(at(0), "complete_intent", map[string]any{"node_id": 32}, "intent 32 marked done", false).
		call(at(0), "record_fact", map[string]any{"facts": []any{
			map[string]any{"summary": "a", "intent_id": 30},
			map[string]any{"summary": "b", "intent_id": 30},
		}}, `{"ids":[400,401]}`, false)
	survivor := (&transcript{}).
		conflict(at(1), 30, "h-bbb").
		claim(at(1), 33, "h-ccc", 2*time.Minute).
		claim(at(3), 30, "h-ccc", 2*time.Minute)
	edges := []tools.Edge{{From: 30, Rel: tools.RelYields, To: 400}}

	c := scoreCoordination(CoordinationInput{
		Transcripts: []*Transcript{killed.parse(t, 2), survivor.parse(t, 3)},
		Edges:       edges,
		Kills:       map[int]time.Time{2: at(1)},
	})
	if len(c.Takeovers) != 2 {
		t.Fatalf("takeovers: %+v", c.Takeovers)
	}
	tk := c.Takeovers[0]
	if tk.IntentID != 30 || tk.TakenBy != 3 || tk.TakenAt == nil || !tk.TakenAt.Equal(at(3)) || !tk.AfterLeaseLapsed {
		t.Fatalf("takeover of 30: %+v", tk)
	}
	if tk.FactsRecorded != 2 || tk.FactsAttached != 1 {
		t.Fatalf("facts: %+v", tk)
	}
	if missed := c.Takeovers[1]; missed.IntentID != 31 || missed.TakenAt != nil {
		t.Fatalf("intent 31 was never taken over: %+v", missed)
	}
	if len(c.DoubleClaims) != 0 {
		t.Fatalf("takeover after the lease is not a double claim: %+v", c.DoubleClaims)
	}
	if !strings.Contains(c.Summary(), "intent 31 held by killed instance 2 was never taken over") {
		t.Fatalf("summary:\n%s", c.Summary())
	}
}

func TestUsageTotals(t *testing.T) {
	done := (&transcript{}).other(at(1)).result(2.5)
	killed := (&transcript{}).other(at(1)).other(at(2))

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{done.parse(t, 1), killed.parse(t, 2)}})
	if !c.Instances[0].ResultMessage || c.Instances[1].ResultMessage {
		t.Fatalf("result flags: %+v", c.Instances)
	}
	if c.Instances[1].Usage.InputTokens != 20 || c.Instances[1].Usage.CacheReadTokens != 200 {
		t.Fatalf("killed instance usage: %+v", c.Instances[1].Usage)
	}
	if c.Total.CostUSD != 2.5 || c.Total.Usage.InputTokens != 27 || c.Total.Usage.OutputTokens != 13 {
		t.Fatalf("totals: %+v", c.Total)
	}
}

func TestRealStreamJSON(t *testing.T) {
	tr, err := parseTranscript(1, filepath.Join("testdata", "claude-stream.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Calls) != 4 || tr.Calls[1].Tool != "add_intent" || tr.Calls[1].Result != "intent created: 4" {
		t.Fatalf("calls: %+v", tr.Calls)
	}
	if !tr.Calls[3].IsError || !strings.Contains(tr.Calls[3].Result, "pick another intent") {
		t.Fatalf("conflict result: %+v", tr.Calls[3])
	}
	if !tr.HasResult || tr.CostUSD <= 0 || tr.Usage.OutputTokens == 0 {
		t.Fatalf("usage: %+v", tr)
	}

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{tr}})
	if c.Instances[0].ClaimsTaken != 1 || c.Conflicts != 1 || c.ConflictsHandled != 1 || c.Instances[0].NodesCreated != 1 {
		t.Fatalf("scored: %+v", c)
	}
}

func TestSummary(t *testing.T) {
	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{(&transcript{}).other(at(1)).parse(t, 1)}})
	s := c.Summary()
	for _, want := range []string{"Concurrent double claims: 0 (pass)", "Concurrent work on one intent: 0 (pass)", "Work outside a claimed intent: 0 calls", "no conflicts", "Duplicate findings: 0 raw"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary lacks %q:\n%s", want, s)
		}
	}
}

func TestConflictRetryAfterLeaseLapse(t *testing.T) {
	expiresSoon := func(b *transcript, at time.Time, until time.Time) *transcript {
		return b.call(at, "start_intent", map[string]any{"node_id": 50},
			fmt.Sprintf("intent 50 is claimed by h-aaa until %s (in 1s); pick another intent", until.UTC().Format(time.RFC3339)), true)
	}
	takeover := expiresSoon(&transcript{}, at(1), at(1).Add(time.Second)).
		other(at(1).Add(time.Second)).
		claim(at(1).Add(3*time.Second), 50, "h-bbb", time.Minute)
	tooSoon := expiresSoon(&transcript{}, at(1), at(1).Add(time.Minute)).
		claim(at(1).Add(2*time.Second), 50, "h-ccc", time.Minute)

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{takeover.parse(t, 1), tooSoon.parse(t, 2)}})
	if c.Conflicts != 2 || c.ConflictsHandled != 1 {
		t.Fatalf("conflicts %d handled %d: %+v", c.Conflicts, c.ConflictsHandled, c.UnhandledConflicts)
	}
	if len(c.UnhandledConflicts) != 1 || c.UnhandledConflicts[0].Instance != 2 {
		t.Fatalf("unhandled: %+v", c.UnhandledConflicts)
	}
}

func TestHolderAttribution(t *testing.T) {
	for _, tc := range []struct {
		name   string
		holder func(instance int) string
	}{
		{"identity", func(i int) string { return fmt.Sprintf("eval/20261125-120000/i%d", i) }},
		{"identity with owner", func(i int) string { return fmt.Sprintf("eval/20261125-120000/i%d/h-0a1b2c", i) }},
		{"hashed", func(i int) string { return []string{"", "h-aaa", "h-bbb"}[i] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			one := (&transcript{}).
				claim(at(1), 10, tc.holder(1), 30*time.Minute).
				conflict(at(2), 11, tc.holder(2)).
				start(at(2), 11)
			two := (&transcript{}).
				claim(at(1), 11, tc.holder(2), 30*time.Minute).
				conflict(at(2), 10, tc.holder(1)).
				start(at(2), 10).
				claim(at(3), 10, tc.holder(2), 30*time.Minute)

			c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{one.parse(t, 1), two.parse(t, 2)}})
			if c.Instances[0].ClaimsTaken != 1 || c.Instances[1].ClaimsTaken != 2 {
				t.Fatalf("claims not parsed: %+v", c.Instances)
			}
			if c.Conflicts != 2 || len(c.UnhandledConflicts) != 2 {
				t.Fatalf("conflicts %d, unhandled %+v", c.Conflicts, c.UnhandledConflicts)
			}
			for _, u := range c.UnhandledConflicts {
				if want := 3 - u.Instance; u.HolderInstance != want || u.Holder != tc.holder(want) {
					t.Errorf("conflict from instance %d attributed to %d (%s), want %d", u.Instance, u.HolderInstance, u.Holder, want)
				}
			}
			if len(c.DoubleClaims) != 1 {
				t.Fatalf("double claims: %+v", c.DoubleClaims)
			}
			d := c.DoubleClaims[0]
			if d.FirstInstance != 1 || d.FirstHolder != tc.holder(1) || d.SecondInstance != 2 || d.SecondHolder != tc.holder(2) {
				t.Errorf("double claim: %+v", d)
			}
			if got := c.Instances[1].Holders; len(got) != 1 || got[0] != tc.holder(2) {
				t.Errorf("instance 2 holders: %v", got)
			}
			if c.HolderMismatches != 0 {
				t.Errorf("holder mismatches: %d", c.HolderMismatches)
			}
			if s := c.Summary(); !strings.Contains(s, "held by "+tc.holder(1)+" (instance 1)") {
				t.Errorf("summary lacks attributed conflict:\n%s", s)
			}
		})
	}
}

func TestHolderMismatch(t *testing.T) {
	b := (&transcript{}).claim(at(1), 10, "eval/r/i2", 30*time.Minute)
	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{b.parse(t, 1)}})
	if c.HolderMismatches != 1 || !strings.Contains(c.Summary(), "Claims under another instance's identity: 1") {
		t.Fatalf("mismatch not reported: %+v", c)
	}
}

func TestEvalHolderInstance(t *testing.T) {
	for label, want := range map[string]int{
		"eval/20261125-120000/i3":           3,
		"eval/20261125-120000/i12/h-abcdef": 12,
		"eval/r/i3x":                        0,
		"h-abcdef":                          0,
		"chat/i3":                           0,
		"ops/eval/r/i3":                     0,
	} {
		if got := evalHolderInstance(label); got != want {
			t.Errorf("evalHolderInstance(%q) = %d, want %d", label, got, want)
		}
	}
}

func TestResultCostIsRunningTotal(t *testing.T) {
	tr := (&transcript{}).other(at(1)).result(3.48).other(at(2)).result(3.78).other(at(3)).result(5.36).parse(t, 1)
	if tr.CostUSD != 5.36 {
		t.Fatalf("cost = %v, want the last running total 5.36", tr.CostUSD)
	}
	if tr.Usage.OutputTokens != 9 || tr.Usage.CacheReadTokens != 2700 {
		t.Fatalf("usage should sum per-turn result usage: %+v", tr.Usage)
	}
}

func fact(at time.Time, id int64, intents ...int64) (time.Time, string, any, string, bool) {
	var facts []any
	for _, i := range intents {
		facts = append(facts, map[string]any{"summary": "s", "intent_id": i})
	}
	return at, "record_fact", map[string]any{"facts": facts}, fmt.Sprintf("fact created: %d", id), false
}

func intentFinding(at time.Time, intent, findingID int64) (time.Time, string, any, string, bool) {
	return at, "report_finding", map[string]any{"vulnclass": "x", "severity": "high", "summary": "s", "intent_id": intent},
		fmt.Sprintf(`{"finding_id":%d,"node_id":%d}`, findingID, findingID+1000), false
}

func finish(at time.Time, tool string, intent int64, result string) (time.Time, string, any, string, bool) {
	return at, tool, map[string]any{"node_id": intent}, result, false
}

func TestUnclaimedWork(t *testing.T) {
	claimed := (&transcript{}).
		call(bash(at(0), "nmap -sV winforge")).
		call(at(1), "add_intent", map[string]any{"summary": "a"}, "intent created: 10", false).
		claim(at(2), 10, "h-aaa", 30*time.Minute).
		call(bash(at(3), "whoami")).
		call(fact(at(4), 500, 10)).
		call(finish(at(5), "complete_intent", 10, "intent 10 marked done")).
		call(bash(at(6), "cat notes.txt")).
		call(at(7), "report_finding", map[string]any{"vulnclass": "x"}, "summary is required", true).
		other(at(8))
	unclaimed := (&transcript{}).
		call(bash(at(2), "icacls C:\\ProgramData")).
		call(at(3), "record_coverage", map[string]any{"surface": "s", "risk_area": "r", "outcome": "reported"}, "recorded", false).
		call(at(3), "insert_assets", map[string]any{"assets": []any{}}, `{"ids":[7]}`, false).
		call(finish(at(4), "complete_intent", 11, "intent 11 marked done")).
		call(finish(at(4), "cancel_intent", 12, "intent 12 cancelled")).
		call(finish(at(4), "complete_intent", 12, "intent 12 cancelled")).
		call(at(5), "complete_intent", map[string]any{"node_id": 14},
			"intent 14 is claimed by h-aaa until 2026-11-25T12:30:00Z (in 25m0s); pick another intent", true).
		claim(at(10), 13, "h-bbb", 2*time.Minute).
		call(bash(at(11), "type flag.txt")).
		call(bash(at(13), "type flag2.txt"))

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{claimed.parse(t, 1), unclaimed.parse(t, 2)}})
	want := []UnclaimedWork{
		{WorkCalls: 3, Calls: 1, Bash: 1},
		{WorkCalls: 5, Calls: 4, Bash: 2, Records: 2, Finishes: []int64{11, 12}},
	}
	for i, w := range want {
		if got := c.Instances[i].Unclaimed; fmt.Sprint(got) != fmt.Sprint(w) {
			t.Errorf("instance %d unclaimed work %+v, want %+v", i+1, got, w)
		}
	}
	if c.UnclaimedWork != 5 || c.UnclaimedFinishes != 2 {
		t.Errorf("totals: %d calls, %d finishes", c.UnclaimedWork, c.UnclaimedFinishes)
	}
	s := c.Summary()
	for _, line := range []string{
		"Work outside a claimed intent: 5 calls, 2 unclaimed intents completed or canceled",
		"  instance 1: 1 of 3 work calls (1 bash, 0 artex records)\n",
		"  instance 2: 4 of 5 work calls (2 bash, 2 artex records), finished unclaimed intents [11 12]",
	} {
		if !strings.Contains(s, line) {
			t.Errorf("summary lacks %q:\n%s", line, s)
		}
	}

	bootstrapOnly := (&transcript{}).call(bash(at(1), "nmap winforge")).call(fact(at(2), 1))
	c = scoreCoordination(CoordinationInput{Transcripts: []*Transcript{bootstrapOnly.parse(t, 1)}})
	if u := c.Instances[0].Unclaimed; u.WorkCalls != 0 || c.UnclaimedWork != 0 {
		t.Errorf("work before any intent exists counted: %+v", u)
	}
}

func TestUnclaimedFinishSkipsOwnDuplicateCancel(t *testing.T) {
	one := (&transcript{}).
		call(addIntent(at(0), 20)).
		call(addIntent(at(1), 21)).
		call(addIntent(at(1), 22)).
		call(finish(at(2), "cancel_intent", 21, "intent 21 cancelled")).
		call(finish(at(2), "cancel_intent", 30, "intent 30 cancelled")).
		call(finish(at(3), "complete_intent", 22, "intent 22 marked done"))
	two := (&transcript{}).
		call(addIntent(at(0), 30)).
		call(finish(at(2), "cancel_intent", 20, "intent 20 cancelled")).
		call(finish(at(3), "cancel_intent", 31, "intent 31 cancelled"))

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{one.parse(t, 1), two.parse(t, 2)}})
	want := []UnclaimedWork{
		{Finishes: []int64{30, 22}, OwnCancels: []int64{21}},
		{Finishes: []int64{20, 31}},
	}
	for i, w := range want {
		if got := c.Instances[i].Unclaimed; fmt.Sprint(got) != fmt.Sprint(w) {
			t.Errorf("instance %d unclaimed work %+v, want %+v", i+1, got, w)
		}
	}
	if c.UnclaimedFinishes != 4 || c.OwnCancels != 1 {
		t.Errorf("totals: %d finishes, %d own cancels", c.UnclaimedFinishes, c.OwnCancels)
	}
	s := c.Summary()
	for _, line := range []string{
		"Work outside a claimed intent: 0 calls, 4 unclaimed intents completed or canceled, 1 own duplicates canceled",
		"  instance 1: 0 of 0 work calls (0 bash, 0 artex records), finished unclaimed intents [30 22], canceled own duplicates [21]\n",
		"  instance 2: 0 of 0 work calls (0 bash, 0 artex records), finished unclaimed intents [20 31]\n",
	} {
		if !strings.Contains(s, line) {
			t.Errorf("summary lacks %q:\n%s", line, s)
		}
	}

	onlyOwn := (&transcript{}).
		call(addIntent(at(0), 40)).
		call(finish(at(1), "cancel_intent", 40, "intent 40 cancelled"))
	c = scoreCoordination(CoordinationInput{Transcripts: []*Transcript{onlyOwn.parse(t, 1)}})
	if c.UnclaimedFinishes != 0 || c.OwnCancels != 1 ||
		!strings.Contains(c.Summary(), "  instance 1: 0 of 0 work calls (0 bash, 0 artex records), canceled own duplicates [40]\n") {
		t.Errorf("own duplicate cancel only:\n%s", c.Summary())
	}
}

func TestConcurrentIntentWork(t *testing.T) {
	one := (&transcript{}).
		claim(at(0), 40, "h-aaa", time.Minute).
		claim(at(0), 41, "h-aaa", time.Minute).
		call(fact(at(1), 500, 40, 41)).
		call(fact(at(8), 501, 40))
	two := (&transcript{}).
		claim(at(3), 40, "h-bbb", time.Minute).
		call(intentFinding(at(4), 40, 1)).
		call(finish(at(5), "complete_intent", 40, "intent 40 marked done")).
		claim(at(3), 41, "h-bbb", time.Minute).
		call(fact(at(4), 502, 41)).
		call(finish(at(5), "complete_intent", 41, "intent 41 marked done")).
		claim(at(10), 42, "h-bbb", 30*time.Minute).
		call(fact(at(12), 503, 42)).
		call(finish(at(13), "complete_intent", 42, "intent 42 marked done"))
	three := (&transcript{}).
		call(intentFinding(at(11), 42, 2)).
		call(at(12), "report_finding", map[string]any{"vulnclass": "x", "intent_id": 41}, "summary is required", true).
		claim(at(20), 43, "h-ccc", time.Minute)

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{one.parse(t, 1), two.parse(t, 2), three.parse(t, 3)}})
	if len(c.DoubleClaims) != 0 {
		t.Fatalf("claims never overlapped: %+v", c.DoubleClaims)
	}
	want := []ConcurrentWork{
		{IntentID: 40, FirstInstance: 1, FirstRecords: 2, SecondInstance: 2, SecondRecords: 1, From: at(3), To: at(5)},
		{IntentID: 42, FirstInstance: 2, FirstRecords: 1, SecondInstance: 3, SecondRecords: 1, From: at(11), To: at(11)},
	}
	if len(c.ConcurrentWork) != len(want) {
		t.Fatalf("concurrent work: %+v", c.ConcurrentWork)
	}
	for i, w := range want {
		if g := c.ConcurrentWork[i]; g.IntentID != w.IntentID || g.FirstInstance != w.FirstInstance || g.FirstRecords != w.FirstRecords ||
			g.SecondInstance != w.SecondInstance || g.SecondRecords != w.SecondRecords || !g.From.Equal(w.From) || !g.To.Equal(w.To) {
			t.Errorf("concurrent work[%d] = %+v, want %+v", i, g, w)
		}
	}
	if s := c.Summary(); !strings.Contains(s, "Concurrent work on one intent: 2 (FAIL)") ||
		!strings.Contains(s, "  intent 40: instance 1 (2 records) and instance 2 (1 records) interleaved from") {
		t.Errorf("summary:\n%s", s)
	}
}

func coverage(at time.Time, intent int64) (time.Time, string, any, string, bool) {
	return at, "record_coverage", map[string]any{"surface": "s", "risk_area": "r", "outcome": "no_issue_found", "intent_id": intent}, "recorded", false
}

func addIntent(at time.Time, id int64) (time.Time, string, any, string, bool) {
	return at, "add_intent", map[string]any{"summary": "a"}, fmt.Sprintf("intent created: %d", id), false
}

func TestImplicitRenewalKeepsWorkClaimed(t *testing.T) {
	const lease = 10 * time.Minute
	holder := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-aaa", lease).
		call(fact(at(6), 500, 10)).
		call(bash(at(12), "whoami")).
		call(coverage(at(13), 10)).
		call(bash(at(20), "type flag.txt")).
		call(intentFinding(at(21), 10, 1)).
		call(bash(at(28), "dir"))
	survivor := (&transcript{}).claim(at(32), 10, "h-bbb", lease)
	in := CoordinationInput{
		Transcripts: []*Transcript{holder.parse(t, 1), survivor.parse(t, 2)},
		Kills:       map[int]time.Time{1: at(29)},
		Lease:       lease,
	}

	old := scoreCoordination(in)
	// Without implicit renewal the claim lapses at at(10), but the holder keeps
	// working intent 10 over the shell right up to the kill, so the survivor's
	// later claim is still a takeover of work it was holding.
	if old.ImplicitRenewal || old.ImplicitRenewals != 0 || old.Instances[0].Unclaimed.Calls != 5 || len(old.Takeovers) != 1 {
		t.Fatalf("pre-renewal scoring changed: renewal %t (%d), unclaimed %+v, takeovers %+v",
			old.ImplicitRenewal, old.ImplicitRenewals, old.Instances[0].Unclaimed, old.Takeovers)
	}
	if tk := old.Takeovers[0]; tk.IntentID != 10 || tk.TakenBy != 2 || !tk.AfterLeaseLapsed {
		t.Fatalf("pre-renewal takeover: %+v", tk)
	}
	if !strings.Contains(old.Summary(), "Claim replay: start_intent only") {
		t.Errorf("summary:\n%s", old.Summary())
	}

	in.Renewal = true
	c := scoreCoordination(in)
	if !c.ImplicitRenewal || c.ImplicitRenewals != 3 {
		t.Fatalf("renewals: %t %d", c.ImplicitRenewal, c.ImplicitRenewals)
	}
	if u := c.Instances[0].Unclaimed; u.Calls != 0 || u.WorkCalls != 6 {
		t.Fatalf("work after renewals counted unclaimed: %+v", u)
	}
	if len(c.Takeovers) != 1 {
		t.Fatalf("takeovers: %+v", c.Takeovers)
	}
	if tk := c.Takeovers[0]; tk.IntentID != 10 || !tk.LeaseExpires.Equal(at(31)) || tk.TakenBy != 2 || !tk.AfterLeaseLapsed {
		t.Fatalf("takeover: %+v", tk)
	}
	if len(c.DoubleClaims) != 0 {
		t.Fatalf("double claims: %+v", c.DoubleClaims)
	}
	if !strings.Contains(c.Summary(), "Claim replay: start_intent plus 3 implicit renewals") {
		t.Errorf("summary:\n%s", c.Summary())
	}

	early := (&transcript{}).claim(at(25), 10, "h-bbb", lease)
	in.Transcripts[1] = early.parse(t, 2)
	in.Kills = nil
	c = scoreCoordination(in)
	if len(c.DoubleClaims) != 1 || !c.DoubleClaims[0].FirstExpires.Equal(at(31)) {
		t.Fatalf("claim inside the renewed lease is a double claim: %+v", c.DoubleClaims)
	}
}

func TestRenewalDetectedFromConflictText(t *testing.T) {
	holder := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-aaa", 10*time.Minute).
		call(fact(at(8), 500, 10)).
		call(bash(at(12), "whoami"))
	other := (&transcript{}).call(at(1), "start_intent", map[string]any{"node_id": 10},
		fmt.Sprintf("intent 10 is claimed by h-aaa until %s (in 9m0s); pick another intent. %s, so do not wait for it to lapse",
			at(10).Format(time.RFC3339), renewalConflictText), true)

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{holder.parse(t, 1), other.parse(t, 2)}, Lease: 10 * time.Minute})
	if !c.ImplicitRenewal || c.ImplicitRenewals != 1 || c.Instances[0].Unclaimed.Calls != 0 {
		t.Fatalf("renewal not detected: %t %d %+v", c.ImplicitRenewal, c.ImplicitRenewals, c.Instances[0].Unclaimed)
	}
}

func TestRenewalHolderOnly(t *testing.T) {
	const lease = 10 * time.Minute
	holder := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-aaa", lease).
		call(bash(at(10).Add(30*time.Second), "whoami"))
	other := (&transcript{}).
		claim(at(0), 11, "h-bbb", lease).
		call(fact(at(8), 500, 10)).
		call(coverage(at(8), 10)).
		claim(at(11), 10, "h-bbb", lease)

	c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{holder.parse(t, 1), other.parse(t, 2)}, Lease: lease, Renewal: true})
	if c.ImplicitRenewals != 0 {
		t.Fatalf("another instance's records renewed the claim: %d", c.ImplicitRenewals)
	}
	if len(c.DoubleClaims) != 0 {
		t.Fatalf("claim after the holder's lease lapsed is not a double claim: %+v", c.DoubleClaims)
	}
	if u := c.Instances[0].Unclaimed; u.Calls != 1 {
		t.Fatalf("holder's work after its lapse: %+v", u)
	}
}

func TestRenewalRevivesLapsedClaim(t *testing.T) {
	const lease = 10 * time.Minute
	holder := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-aaa", lease).
		call(bash(at(12), "whoami")).
		call(fact(at(13), 500, 10)).
		call(bash(at(14), "dir"))
	other := (&transcript{}).
		call(at(5), "start_intent", map[string]any{"node_id": 10},
			fmt.Sprintf("intent 10 is claimed by h-aaa until %s (in 5m0s); pick another intent", at(10).Format(time.RFC3339)), true).
		other(at(6)).
		call(at(15), "start_intent", map[string]any{"node_id": 10},
			fmt.Sprintf("intent 10 is claimed by h-aaa until %s (in 8m0s); pick another intent", at(23).Format(time.RFC3339)), true)
	in := CoordinationInput{Transcripts: []*Transcript{holder.parse(t, 1), other.parse(t, 2)}, Lease: lease}

	old := scoreCoordination(in)
	if old.Instances[0].Unclaimed.Calls != 3 || old.Conflicts != 2 || old.ConflictsHandled != 2 {
		t.Fatalf("pre-renewal: unclaimed %+v, conflicts %d handled %d", old.Instances[0].Unclaimed, old.Conflicts, old.ConflictsHandled)
	}

	in.Renewal = true
	c := scoreCoordination(in)
	if c.ImplicitRenewals != 1 {
		t.Fatalf("lapsed claim not revived: %d", c.ImplicitRenewals)
	}
	if u := c.Instances[0].Unclaimed; u.Calls != 1 || u.Bash != 1 {
		t.Fatalf("only the bash during the lapse is unclaimed: %+v", u)
	}
	if c.Conflicts != 2 || c.ConflictsHandled != 1 || len(c.UnhandledConflicts) != 1 || c.UnhandledConflicts[0].Call != 0 {
		t.Fatalf("retry into the revived claim is mishandling: %d/%d %+v", c.ConflictsHandled, c.Conflicts, c.UnhandledConflicts)
	}

	sniped := (&transcript{}).claim(at(15), 10, "h-bbb", lease)
	in.Transcripts[1] = sniped.parse(t, 2)
	if c := scoreCoordination(in); len(c.DoubleClaims) != 1 || c.DoubleClaims[0].SecondInstance != 2 {
		t.Fatalf("claim of a revived intent is a double claim: %+v", c.DoubleClaims)
	}
}

func TestNoRevivalAfterTakeover(t *testing.T) {
	const lease = 10 * time.Minute
	holder := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-aaa", lease).
		call(fact(at(12), 500, 10)).
		call(bash(at(13), "whoami"))
	taker := (&transcript{}).
		claim(at(11), 10, "h-bbb", lease).
		call(fact(at(17), 501, 10)).
		claim(at(19), 10, "h-bbb", lease)

	c := scoreCoordination(CoordinationInput{
		Transcripts: []*Transcript{holder.parse(t, 1), taker.parse(t, 2)},
		Lease:       lease,
		Renewal:     true,
		Kills:       map[int]time.Time{1: at(15)},
	})
	if c.ImplicitRenewals != 1 {
		t.Fatalf("only the taker's record renews: %d", c.ImplicitRenewals)
	}
	if len(c.DoubleClaims) != 0 {
		t.Fatalf("double claims: %+v", c.DoubleClaims)
	}
	if u := c.Instances[0].Unclaimed; u.Calls != 2 {
		t.Fatalf("old holder's work after the takeover: %+v", u)
	}
	if len(c.Takeovers) != 0 {
		t.Fatalf("killed instance no longer held the intent: %+v", c.Takeovers)
	}
	if len(c.ConcurrentWork) != 1 {
		t.Fatalf("old holder recording against the taken intent: %+v", c.ConcurrentWork)
	}
}

func TestRenewalHalfLeaseSkip(t *testing.T) {
	const lease = 10 * time.Minute
	for _, tc := range []struct {
		name     string
		recordAt time.Time
		renewals int
		doubles  int
		lapsed   int
	}{
		{"more than half left", at(3), 0, 0, 1},
		{"less than half left", at(6), 1, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			holder := (&transcript{}).
				call(addIntent(at(0), 10)).
				claim(at(0), 10, "h-aaa", lease).
				call(fact(tc.recordAt, 500, 10)).
				call(bash(at(10).Add(30*time.Second), "whoami"))
			other := (&transcript{}).claim(at(11), 10, "h-bbb", lease)

			c := scoreCoordination(CoordinationInput{Transcripts: []*Transcript{holder.parse(t, 1), other.parse(t, 2)}, Lease: lease, Renewal: true})
			if c.ImplicitRenewals != tc.renewals || len(c.DoubleClaims) != tc.doubles || c.Instances[0].Unclaimed.Calls != tc.lapsed {
				t.Fatalf("renewals %d, double claims %+v, unclaimed %+v", c.ImplicitRenewals, c.DoubleClaims, c.Instances[0].Unclaimed)
			}
		})
	}
}

func TestServerRenewsClaims(t *testing.T) {
	t.Setenv(fakeEnv, "1")
	for desc, want := range map[string]bool{
		"The claim is exclusive for 30 minutes and renews whenever you pass this intent as intent_id to record_fact. ": true,
		"The claim is exclusive for 30 minutes; call again to renew it on long work. ":                                 false,
	} {
		catalog, _ := json.Marshal([]map[string]string{{"name": "record_fact"}, {"name": "start_intent", "description": desc}})
		t.Setenv(fakeCatalogEnv, string(catalog))
		if got := serverRenewsClaims(context.Background(), os.Args[0]); got != want {
			t.Errorf("serverRenewsClaims with %q = %t", desc, got)
		}
	}
	if serverRenewsClaims(context.Background(), filepath.Join(t.TempDir(), "missing")) {
		t.Error("a missing binary does not renew")
	}
}

func TestTakeoverLapsedShortlyBeforeKill(t *testing.T) {
	const lease = 10 * time.Minute
	// The killed holder renewed its claim through an ARTEX record, then worked
	// over the shell while the lease lapsed a short while before the kill.
	killed := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-bbb", lease).
		call(fact(at(6), 500, 10)).
		call(bash(at(12), "systemctl restart svc")).
		call(bash(at(16).Add(20*time.Second), "sleep 60"))
	// Lease renews to at(16); the kill lands 30s after it lapses.
	killedAt := at(16).Add(30 * time.Second)
	survivor := (&transcript{}).
		other(at(17)).
		claim(at(25), 10, "h-ccc", lease).
		call(finish(at(27), "complete_intent", 10, "intent 10 marked done"))

	c := scoreCoordination(CoordinationInput{
		Transcripts: []*Transcript{killed.parse(t, 2), survivor.parse(t, 3)},
		Kills:       map[int]time.Time{2: killedAt},
		Lease:       lease,
		Renewal:     true,
	})
	if len(c.Takeovers) != 1 {
		t.Fatalf("takeovers: %+v", c.Takeovers)
	}
	tk := c.Takeovers[0]
	if tk.IntentID != 10 || tk.KilledInstance != 2 || tk.TakenBy != 3 || tk.TakenAt == nil || !tk.TakenAt.Equal(at(25)) || !tk.AfterLeaseLapsed {
		t.Fatalf("takeover: %+v", tk)
	}
	if !tk.LeaseExpires.Equal(at(16)) {
		t.Fatalf("lease expiry should reflect the implicit renewal: %+v", tk)
	}
	if len(c.DoubleClaims) != 0 {
		t.Fatalf("a claim after the lapse is not a double claim: %+v", c.DoubleClaims)
	}
}

func TestTakeoverStillHeldAtKill(t *testing.T) {
	const lease = 10 * time.Minute
	killed := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-bbb", lease)
	survivor := (&transcript{}).
		other(at(5)).
		claim(at(12), 10, "h-ccc", lease)

	c := scoreCoordination(CoordinationInput{
		Transcripts: []*Transcript{killed.parse(t, 2), survivor.parse(t, 3)},
		Kills:       map[int]time.Time{2: at(5)},
		Lease:       lease,
	})
	if len(c.Takeovers) != 1 {
		t.Fatalf("takeovers: %+v", c.Takeovers)
	}
	if tk := c.Takeovers[0]; tk.IntentID != 10 || tk.TakenBy != 3 || tk.TakenAt == nil || !tk.TakenAt.Equal(at(12)) || !tk.AfterLeaseLapsed {
		t.Fatalf("takeover: %+v", tk)
	}
	if len(c.DoubleClaims) != 0 {
		t.Fatalf("double claims: %+v", c.DoubleClaims)
	}
}

func TestTakeoverLapsedNeverReclaimed(t *testing.T) {
	const lease = 2 * time.Minute
	// The holder was working the intent right up to the kill, its lease lapsed,
	// but nobody ever reclaims it: a lapsed candidate is reported only when it
	// was actually taken over.
	killed := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-bbb", lease).
		call(bash(at(2).Add(30*time.Second), "whoami"))
	survivor := (&transcript{}).other(at(1)).other(at(4))

	c := scoreCoordination(CoordinationInput{
		Transcripts: []*Transcript{killed.parse(t, 2), survivor.parse(t, 3)},
		Kills:       map[int]time.Time{2: at(3)},
		Lease:       lease,
	})
	if len(c.Takeovers) != 0 {
		t.Fatalf("a lapsed intent nobody reclaimed is not reported: %+v", c.Takeovers)
	}
}

func TestTakeoverLapsedAfterGoingIdle(t *testing.T) {
	const lease = 2 * time.Minute
	// The holder stopped working the intent well before the kill; a later claim
	// after the kill is not a takeover of work it was holding at the kill.
	killed := (&transcript{}).
		call(addIntent(at(0), 10)).
		claim(at(0), 10, "h-bbb", lease).
		call(bash(at(1), "whoami"))
	survivor := (&transcript{}).other(at(1)).claim(at(12), 10, "h-ccc", lease)

	c := scoreCoordination(CoordinationInput{
		Transcripts: []*Transcript{killed.parse(t, 2), survivor.parse(t, 3)},
		Kills:       map[int]time.Time{2: at(10)},
		Lease:       lease,
	})
	if len(c.Takeovers) != 0 {
		t.Fatalf("an intent gone idle long before the kill is not a takeover: %+v", c.Takeovers)
	}
}
