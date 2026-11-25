package store

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewChangeNodeID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool string
		args string
		want int64
	}{
		{"start_intent", `{"node_id": 42}`, 42},
		{"complete_intent", `{"node_id": "17", "result": "done"}`, 17},
		{"cancel_intent", `{}`, 0},
		{"update_intent", `not json`, 0},
		{"insert_node", `{"node_id": 9}`, 0},
	}
	for _, c := range cases {
		got := NewChange("o", c.tool, 1, 2, json.RawMessage(c.args))
		if got.NodeID != c.want {
			t.Errorf("%s %s: NodeID = %d, want %d", c.tool, c.args, got.NodeID, c.want)
		}
		if got.Tool != c.tool || got.TaskID != 1 || got.ExplorationID != 2 || got.Origin != "o" {
			t.Errorf("%s: unexpected change %+v", c.tool, got)
		}
	}
}

func TestChangeEncodeRoundTrip(t *testing.T) {
	t.Parallel()
	in := Change{Tool: "start_intent", TaskID: 3, ExplorationID: 4, NodeID: 5, Origin: "abc"}
	payload, err := EncodeChange(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecodeChange(payload)
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}

func TestEncodeChangeRejectsOversize(t *testing.T) {
	t.Parallel()
	if _, err := EncodeChange(Change{Tool: strings.Repeat("x", 8000)}); err == nil {
		t.Fatal("expected error for oversize payload")
	}
}

func TestChangeRelevant(t *testing.T) {
	t.Parallel()
	c := Change{Tool: "insert_node", ExplorationID: 7, Origin: "remote"}
	if !c.Relevant("local", 7) {
		t.Error("remote change on active exploration should be relevant")
	}
	if c.Relevant("remote", 7) {
		t.Error("own echo should not be relevant")
	}
	if c.Relevant("local", 8) {
		t.Error("change on another exploration should not be relevant")
	}
}

func TestListenChangesReceivesRemoteOnly(t *testing.T) {
	s := testDB(t)
	dsn := os.Getenv("ARTEX_TEST_PG")
	_, expID := testEngagement(t, s, "notify")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	got := make(chan Change, 64)
	go ListenChanges(ctx, dsn, "listener", func() int64 { return expID }, func(c Change) { got <- c })

	self := Change{Tool: "self_echo", ExplorationID: expID, Origin: "listener"}
	other := Change{Tool: "other_engagement", ExplorationID: expID + 1_000_000, Origin: "remote"}
	remote := Change{Tool: "start_intent", ExplorationID: expID, NodeID: 11, Origin: "remote"}

	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		for _, c := range []Change{self, other, remote} {
			if err := s.PublishChange(ctx, c); err != nil {
				t.Fatalf("PublishChange: %v", err)
			}
		}
		select {
		case c := <-got:
			if c != remote {
				t.Fatalf("listener received %+v, want only %+v", c, remote)
			}
			return
		case <-tick.C:
		case <-ctx.Done():
			t.Fatal("no change received before timeout")
		}
	}
}

func TestListenChangesReconnects(t *testing.T) {
	s := testDB(t)
	dsn := os.Getenv("ARTEX_TEST_PG")
	_, expID := testEngagement(t, s, "notify-reconnect")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	got := make(chan Change, 64)
	go ListenChanges(ctx, dsn, "listener", func() int64 { return expID }, func(c Change) { got <- c })

	waitFor := func(c Change) {
		t.Helper()
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			if err := s.PublishChange(ctx, c); err != nil {
				t.Fatalf("PublishChange: %v", err)
			}
			select {
			case r := <-got:
				if r == c {
					return
				}
			case <-tick.C:
			case <-ctx.Done():
				t.Fatalf("change %+v not received before timeout", c)
			}
		}
	}

	waitFor(Change{Tool: "before", ExplorationID: expID, Origin: "remote"})
	var killed int
	err := s.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE pg_terminate_backend(pid)) FROM pg_stat_activity
		WHERE datname = current_database() AND query = 'LISTEN `+ChangeChannel+`' AND pid <> pg_backend_pid()`).Scan(&killed)
	if err != nil {
		t.Fatalf("terminating listener: %v", err)
	}
	if killed == 0 {
		t.Fatal("found no listener connection to terminate")
	}
	waitFor(Change{Tool: "after", ExplorationID: expID, Origin: "remote"})
}
