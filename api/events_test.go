package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
)

func TestChangeEvent(t *testing.T) {
	cases := map[string]string{
		"report_finding":         eventFindingAdded,
		"merge_findings":         eventFindingsChanged,
		"dismiss_finding":        eventFindingsChanged,
		"bind_finding_traffic":   eventFindingsChanged,
		"insert_assets":          eventAssetAdded,
		"add_intent":             eventGraphChanged,
		"record_fact":            eventGraphChanged,
		"add_hint":               eventGraphChanged,
		"set_goals":              eventGraphChanged,
		"set_constraints":        eventGraphChanged,
		"prove_goal":             eventGraphChanged,
		"start_intent":           eventGraphChanged,
		"complete_intent":        eventGraphChanged,
		"cancel_intent":          eventGraphChanged,
		"traffic_replay":         eventTrafficChanged,
		"traffic_clear":          eventTrafficChanged,
		"add_company_scope":      eventStoreChanged,
		"add_task_scope":         eventStoreChanged,
		"save_task_credential":   eventStoreChanged,
		"record_coverage":        eventStoreChanged,
		"log_activity":           eventStoreChanged,
		"save_evidence":          eventStoreChanged,
		"capture_command_output": eventStoreChanged,
		"goal_met":               eventStoreChanged,
		"update_engagement":      eventStoreChanged,
		"ingest_scanner_output":  eventRefresh,
		"ingest_masscan":         eventRefresh,
		"import_engagement":      eventRefresh,
		"create_engagement":      eventRefresh,
		"delete_engagement":      eventRefresh,
		"switch_engagement":      eventRefresh,
		"nmap_scan":              eventRefresh,
		"":                       eventRefresh,
	}
	for tool, want := range cases {
		if got := changeEvent(tool); got != want {
			t.Errorf("changeEvent(%q) = %q, want %q", tool, got, want)
		}
	}
}

func TestChangeEvents_MatchCatalog(t *testing.T) {
	names := map[string]bool{}
	for _, e := range mcpserver.Catalog() {
		names[e.Name] = true
		if _, ok := changeEvents[e.Name]; ok && e.ReadOnly {
			t.Errorf("read-only tool %s has a change event", e.Name)
		}
	}
	for name := range changeEvents {
		if !names[name] {
			t.Errorf("changeEvents names unknown tool %s", name)
		}
	}
}

func recvEvents(t *testing.T, ch <-chan []byte, wait time.Duration) map[string]float64 {
	t.Helper()
	got := map[string]float64{}
	deadline := time.After(wait)
	for {
		select {
		case msg := <-ch:
			var p struct {
				Type string         `json:"type"`
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(msg, &p); err != nil {
				t.Fatalf("decode event: %v", err)
			}
			if _, dup := got[p.Type]; dup {
				t.Errorf("event %s broadcast more than once", p.Type)
			}
			count, _ := p.Data["count"].(float64)
			got[p.Type] = count
		case <-deadline:
			return got
		}
	}
}

func TestNotifyChange_CoalescesBurst(t *testing.T) {
	a := newTestAPI(&mockStore{})
	a.changes.window = 20 * time.Millisecond
	ch, unsub := a.hub.Subscribe(globalTopic)
	defer unsub()

	for range 3 {
		a.NotifyChange("report_finding")
	}
	for range 50 {
		a.NotifyChange("start_intent")
		a.NotifyChange("some_plugin_tool")
	}

	got := recvEvents(t, ch, 200*time.Millisecond)
	want := map[string]float64{eventFindingAdded: 3, eventGraphChanged: 50, eventRefresh: 50}
	if len(got) != len(want) {
		t.Fatalf("got events %v, want %v", got, want)
	}
	for ev, n := range want {
		if got[ev] != n {
			t.Errorf("%s count = %v, want %v", ev, got[ev], n)
		}
	}
}

func TestNotifyChange_NewWindowAfterFlush(t *testing.T) {
	a := newTestAPI(&mockStore{})
	a.changes.window = 10 * time.Millisecond
	ch, unsub := a.hub.Subscribe(globalTopic)
	defer unsub()

	for i := range 2 {
		a.NotifyChange("insert_assets")
		if got := recvEvents(t, ch, 100*time.Millisecond); len(got) != 1 || got[eventAssetAdded] != 1 {
			t.Fatalf("window %d: got %v", i, got)
		}
	}
}

func TestNotifyChange_DoesNotReadStore(t *testing.T) {
	a := newTestAPI(nil)
	a.changes.window = time.Millisecond
	ch, unsub := a.hub.Subscribe(globalTopic)
	defer unsub()

	a.NotifyChange("add_intent")
	if got := recvEvents(t, ch, 50*time.Millisecond); got[eventGraphChanged] != 1 {
		t.Fatalf("got %v", got)
	}
}
