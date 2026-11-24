package traffic

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/traffic/rewrite"
)

func TestDoneClosed(t *testing.T) {
	t.Parallel()

	t.Run("nil_channel", func(t *testing.T) {
		if doneClosed(nil) {
			t.Fatal("nil channel should return false")
		}
	})

	t.Run("closed_channel", func(t *testing.T) {
		ch := make(chan struct{})
		close(ch)
		if !doneClosed(ch) {
			t.Fatal("closed channel should return true")
		}
	})

	t.Run("open_channel", func(t *testing.T) {
		ch := make(chan struct{})
		if doneClosed(ch) {
			t.Fatal("open channel should return false")
		}
	})
}

func TestSnapTableReserveAndTake(t *testing.T) {
	t.Parallel()
	st := newSnapTable()

	done := make(chan struct{})
	k := flowKey{1}
	e := &reqSnap{done: done, cost: 100, rewritten: true, method: "GET",
		url: &url.URL{Path: "/"}, header: http.Header{}}

	if !st.reserve(k, e) {
		t.Fatal("reserve should succeed")
	}

	st.mu.Lock()
	entries, tombs := st.entries, st.tombs
	st.mu.Unlock()
	if entries != 1 || tombs != 0 {
		t.Fatalf("entries=%d tombs=%d", entries, tombs)
	}

	snap := st.take(k)
	if snap == nil {
		t.Fatal("take returned nil")
	}
	if snap.method != "GET" {
		t.Fatalf("method = %q", snap.method)
	}

	st.mu.Lock()
	entries, tombs = st.entries, st.tombs
	st.mu.Unlock()
	if entries != 0 || tombs != 0 {
		t.Fatalf("after take: entries=%d tombs=%d", entries, tombs)
	}
}

func TestSnapTableTakeNonexistent(t *testing.T) {
	t.Parallel()
	st := newSnapTable()
	if st.take(flowKey{99}) != nil {
		t.Fatal("take of nonexistent key should return nil")
	}
}

func TestSnapTableTakeNilTable(t *testing.T) {
	t.Parallel()
	var st *snapTable
	if st.take(flowKey{1}) != nil {
		t.Fatal("take on nil table should return nil")
	}
}

func TestSnapTableReserveNilTable(t *testing.T) {
	t.Parallel()
	var st *snapTable
	if st.reserve(flowKey{1}, &reqSnap{cost: 1}) {
		t.Fatal("reserve on nil table should fail")
	}
}

func TestSnapTableReserveFull(t *testing.T) {
	t.Parallel()
	st := newSnapTable()
	st.maxEntries = 2
	st.maxBytes = 10000

	done1 := make(chan struct{})
	done2 := make(chan struct{})
	done3 := make(chan struct{})

	st.reserve(flowKey{1}, &reqSnap{done: done1, cost: 10, method: "GET", url: &url.URL{}, header: http.Header{}})
	st.reserve(flowKey{2}, &reqSnap{done: done2, cost: 10, method: "GET", url: &url.URL{}, header: http.Header{}})

	ok := st.reserve(flowKey{3}, &reqSnap{done: done3, cost: 10, method: "GET", url: &url.URL{}, header: http.Header{}})
	if ok {
		t.Fatal("reserve should fail when full and no finished flows to sweep")
	}

	st.mu.Lock()
	tombs := st.tombs
	st.mu.Unlock()
	if tombs < 1 {
		t.Fatalf("expected at least 1 tombstone, got %d", tombs)
	}
}

func TestSnapTableSweepFinished(t *testing.T) {
	t.Parallel()
	st := newSnapTable()

	done := make(chan struct{})
	close(done)
	st.reserve(flowKey{1}, &reqSnap{done: done, cost: 10, method: "GET", url: &url.URL{}, header: http.Header{}})

	st.mu.Lock()
	entries := st.entries
	st.mu.Unlock()
	if entries != 1 {
		t.Fatalf("entries = %d before sweep", entries)
	}

	st.mu.Lock()
	st.sweepLocked(time.Now())
	entries = st.entries
	st.mu.Unlock()
	if entries != 0 {
		t.Fatalf("entries = %d after sweep, want 0", entries)
	}
}

func TestSnapTableTick(t *testing.T) {
	t.Parallel()
	st := newSnapTable()

	done := make(chan struct{})
	close(done)
	st.reserve(flowKey{1}, &reqSnap{done: done, cost: 10, method: "GET", url: &url.URL{}, header: http.Header{}})

	st.requestSweep()
	st.now = func() time.Time { return time.Now().Add(2 * sweepDebounce) }
	st.tick()

	st.mu.Lock()
	entries := st.entries
	st.mu.Unlock()
	if entries != 0 {
		t.Fatalf("entries = %d after tick, want 0", entries)
	}
}

func TestSnapTableTickEmpty(t *testing.T) {
	t.Parallel()
	st := newSnapTable()
	st.tick()
}

func TestReqSnapRequestRewritten(t *testing.T) {
	t.Parallel()

	t.Run("entry_rewritten", func(t *testing.T) {
		s := &reqSnap{rewritten: true}
		if !s.requestRewritten() {
			t.Fatal("should be true")
		}
	})

	t.Run("entry_not_rewritten", func(t *testing.T) {
		s := &reqSnap{rewritten: false}
		if s.requestRewritten() {
			t.Fatal("should be false")
		}
	})

	t.Run("tombstone_original_unavailable", func(t *testing.T) {
		s := &reqSnap{tomb: true, reason: reasonOriginalUnavailable}
		if !s.requestRewritten() {
			t.Fatal("should be true for original unavailable tombstone")
		}
	})

	t.Run("tombstone_capacity", func(t *testing.T) {
		s := &reqSnap{tomb: true, reason: reasonSnapshotCapacity}
		if s.requestRewritten() {
			t.Fatal("should be false for capacity tombstone")
		}
	})
}

func TestReqSnapAppliedRuleIDs(t *testing.T) {
	t.Parallel()

	t.Run("tombstone_with_ids", func(t *testing.T) {
		s := &reqSnap{tomb: true, ruleIDs: []int64{10, 20}}
		ids := s.appliedRuleIDs()
		if len(ids) != 2 || ids[0] != 10 || ids[1] != 20 {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("entry_with_lines", func(t *testing.T) {
		s := &reqSnap{
			lines: []rewrite.TraceLine{
				{RuleID: 5, Outcome: rewrite.OutcomeApplied},
				{RuleID: 7, Outcome: rewrite.OutcomeSkipped},
				{RuleID: 5, Outcome: rewrite.OutcomeApplied},
			},
		}
		ids := s.appliedRuleIDs()
		if len(ids) != 1 || ids[0] != 5 {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("entry_no_applied", func(t *testing.T) {
		s := &reqSnap{
			lines: []rewrite.TraceLine{
				{RuleID: 1, Outcome: rewrite.OutcomeSkipped},
			},
		}
		ids := s.appliedRuleIDs()
		if len(ids) != 0 {
			t.Fatalf("ids = %v, want empty", ids)
		}
	})
}

func TestTakeReplayNonce(t *testing.T) {
	t.Parallel()

	t.Run("nil_header", func(t *testing.T) {
		nonce, present := takeReplayNonce(nil)
		if present || nonce != "" {
			t.Fatalf("nonce=%q present=%v", nonce, present)
		}
	})

	t.Run("no_nonce_header", func(t *testing.T) {
		h := http.Header{"X-Other": {"v"}}
		nonce, present := takeReplayNonce(h)
		if present || nonce != "" {
			t.Fatalf("nonce=%q present=%v", nonce, present)
		}
	})

	t.Run("one_nonce", func(t *testing.T) {
		h := http.Header{}
		h.Set(replayNonceHeader, "abc123")
		nonce, present := takeReplayNonce(h)
		if !present || nonce != "abc123" {
			t.Fatalf("nonce=%q present=%v", nonce, present)
		}
		if h.Get(replayNonceHeader) != "" {
			t.Fatal("header should be removed")
		}
	})

	t.Run("multiple_nonces", func(t *testing.T) {
		h := http.Header{replayNonceHeader: {"a", "b"}}
		nonce, present := takeReplayNonce(h)
		if !present {
			t.Fatal("present should be true")
		}
		if nonce != "" {
			t.Fatalf("multiple values should return empty nonce, got %q", nonce)
		}
	})
}

func TestAppliedRules(t *testing.T) {
	t.Parallel()

	t.Run("none", func(t *testing.T) {
		rewritten, ids := appliedRules(nil)
		if rewritten || len(ids) != 0 {
			t.Fatalf("rewritten=%v ids=%v", rewritten, ids)
		}
	})

	t.Run("one_applied", func(t *testing.T) {
		lines := []rewrite.TraceLine{
			{RuleID: 42, Outcome: rewrite.OutcomeApplied},
		}
		rewritten, ids := appliedRules(lines)
		if !rewritten || len(ids) != 1 || ids[0] != 42 {
			t.Fatalf("rewritten=%v ids=%v", rewritten, ids)
		}
	})

	t.Run("dedup", func(t *testing.T) {
		lines := []rewrite.TraceLine{
			{RuleID: 1, Outcome: rewrite.OutcomeApplied},
			{RuleID: 1, Outcome: rewrite.OutcomeApplied},
			{RuleID: 2, Outcome: rewrite.OutcomeApplied},
		}
		rewritten, ids := appliedRules(lines)
		if !rewritten || len(ids) != 2 {
			t.Fatalf("rewritten=%v ids=%v", rewritten, ids)
		}
	})

	t.Run("applied_zero_id", func(t *testing.T) {
		lines := []rewrite.TraceLine{
			{RuleID: 0, Outcome: rewrite.OutcomeApplied},
		}
		rewritten, ids := appliedRules(lines)
		if !rewritten {
			t.Fatal("should be rewritten")
		}
		if len(ids) != 0 {
			t.Fatalf("zero rule id should not be in ids: %v", ids)
		}
	})
}

func TestRunTestHook(t *testing.T) {
	t.Parallel()

	t.Run("nil_no_panic", func(t *testing.T) {
		testRuleHook.Store(nil)
		runTestHook("test")
	})

	t.Run("set_hook", func(t *testing.T) {
		var called string
		fn := func(stage string) { called = stage }
		testRuleHook.Store(&fn)
		defer testRuleHook.Store(nil)
		runTestHook("my-stage")
		if called != "my-stage" {
			t.Fatalf("called = %q", called)
		}
	})
}

func TestRulesIsBinary(t *testing.T) {
	t.Parallel()
	testRuleHook.Store(nil)

	if rulesIsBinary("text/html", []byte("<html>")) {
		t.Fatal("text/html should not be binary")
	}
	if !rulesIsBinary("image/png", []byte{0x89, 0x50}) {
		t.Fatal("image/png should be binary")
	}
}
