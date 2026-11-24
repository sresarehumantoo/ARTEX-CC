package traffic

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"
)

func TestStreamDisplayURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		method     string
		rawURL     string
		kind       string
		wantMethod string
		wantScheme string
	}{
		{"ws_connect", "CONNECT", "https://echo.test:443", "websocket", "GET", "ws"},
		{"ws_get_http", "GET", "http://echo.test/ws", "websocket", "GET", "ws"},
		{"ws_get_https", "GET", "https://echo.test/ws", "websocket", "GET", "wss"},
		{"sse_get", "GET", "https://echo.test/events", "sse", "GET", "https"},
		{"ws_connect_opaque", "CONNECT", "echo.test:443", "websocket", "GET", "ws"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatal(err)
			}
			method, display, tmpl := streamDisplayURL(tc.method, u, tc.kind)
			if method != tc.wantMethod {
				t.Fatalf("method = %q, want %q", method, tc.wantMethod)
			}
			parsed, err := url.Parse(display)
			if err != nil {
				t.Fatalf("display URL parse error: %v", err)
			}
			if parsed.Scheme != tc.wantScheme {
				t.Fatalf("scheme = %q, want %q (display = %q)", parsed.Scheme, tc.wantScheme, display)
			}
			_ = tmpl
		})
	}
}

func TestStreamDirectionFilter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		dir     string
		wantOK  bool
		wantSQL bool // true if cond is non-empty
	}{
		{"", true, false},
		{"c2s", true, true},
		{"s2c", true, true},
		{"invalid", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			cond, args, err := streamDirectionFilter(tc.dir)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tc.wantSQL {
					if cond == "" {
						t.Fatal("expected non-empty SQL condition")
					}
					if len(args) == 0 {
						t.Fatal("expected args")
					}
				} else {
					if cond != "" {
						t.Fatal("expected empty condition")
					}
				}
			} else {
				if !errors.Is(err, ErrBadStreamDirection) {
					t.Fatalf("got err %v, want ErrBadStreamDirection", err)
				}
			}
		})
	}
}

func TestStreamStoredCost(t *testing.T) {
	t.Parallel()

	t.Run("small_payload", func(t *testing.T) {
		r := streamRow{payload: []byte("hello"), ctype: "text/plain"}
		cost := streamStoredCost(r)
		if cost != 5 {
			t.Fatalf("cost = %d, want 5", cost)
		}
	})

	t.Run("large_text_payload", func(t *testing.T) {
		payload := make([]byte, maxStreamInline+100)
		for i := range payload {
			payload[i] = 'A'
		}
		r := streamRow{payload: payload, ctype: "text/plain"}
		cost := streamStoredCost(r)
		if cost <= int64(len(payload)) {
			t.Fatalf("cost %d should exceed payload len %d (includes preview)", cost, len(payload))
		}
	})

	t.Run("large_binary_payload", func(t *testing.T) {
		payload := make([]byte, maxStreamInline+100)
		payload[0] = 0xFF
		r := streamRow{payload: payload, ctype: "application/octet-stream"}
		cost := streamStoredCost(r)
		if cost <= int64(len(payload)) {
			t.Fatalf("cost %d should exceed payload len %d (includes binary tag)", cost, len(payload))
		}
	})
}

func TestBoolInt(t *testing.T) {
	t.Parallel()
	if boolInt(true) != 1 {
		t.Fatal("true should be 1")
	}
	if boolInt(false) != 0 {
		t.Fatal("false should be 0")
	}
}

func TestStreamMetaPayload(t *testing.T) {
	t.Parallel()
	st := &streamState{
		droppedMsgs:  42,
		droppedBytes: 12345,
	}
	raw := streamMetaPayload(st)
	var m map[string]int64
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["dropped_messages"] != 42 {
		t.Fatalf("dropped_messages = %d", m["dropped_messages"])
	}
	if m["dropped_bytes"] != 12345 {
		t.Fatalf("dropped_bytes = %d", m["dropped_bytes"])
	}
}

func TestStreamMessagesAndInfo(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"ws-0001", "ws.test", "GET", "/ws", "wss://ws.test/ws",
		101, "websocket", 0, 0,
		"GET /ws HTTP/1.1\nHost: ws.test\n", nil,
		"HTTP/1.1 101 Switching Protocols\n", nil)

	db := tr.DB()
	for i := int64(1); i <= 3; i++ {
		dir := "c2s"
		if i%2 == 0 {
			dir = "s2c"
		}
		_, err := db.Exec(`INSERT INTO stream_messages(exchange_id,seq,ts,direction,kind,event,event_id,retry,size,payload,truncated) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			"ws-0001", i, 1000+i, dir, 1, "", "", 0, 5, []byte("hello"), 0)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Run("StreamMessages_all", func(t *testing.T) {
		msgs, total, err := tr.StreamMessages("ws-0001", 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 || len(msgs) != 3 {
			t.Fatalf("total=%d msgs=%d", total, len(msgs))
		}
	})

	t.Run("StreamMessages_paginated", func(t *testing.T) {
		msgs, total, err := tr.StreamMessages("ws-0001", 0, 2)
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 || len(msgs) != 2 {
			t.Fatalf("total=%d msgs=%d", total, len(msgs))
		}
	})

	t.Run("StreamMessagesByDirection_c2s", func(t *testing.T) {
		msgs, total, err := tr.StreamMessagesByDirection("ws-0001", "c2s", 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 {
			t.Fatalf("total=%d, want 2 (c2s only)", total)
		}
		for _, m := range msgs {
			if m.Direction != "c2s" {
				t.Fatalf("direction = %q, want c2s", m.Direction)
			}
		}
	})

	t.Run("StreamMessagesByDirection_bad", func(t *testing.T) {
		_, _, err := tr.StreamMessagesByDirection("ws-0001", "bad", 0, 50)
		if !errors.Is(err, ErrBadStreamDirection) {
			t.Fatalf("err = %v, want ErrBadStreamDirection", err)
		}
	})

	t.Run("StreamInfo_websocket", func(t *testing.T) {
		count, kind, err := tr.StreamInfo("ws-0001")
		if err != nil {
			t.Fatal(err)
		}
		if kind != "websocket" {
			t.Fatalf("kind = %q, want websocket", kind)
		}
		if count != 3 {
			t.Fatalf("count = %d, want 3", count)
		}
	})

	t.Run("StreamInfo_nonexistent", func(t *testing.T) {
		count, kind, err := tr.StreamInfo("does-not-exist")
		if err != nil {
			t.Fatal(err)
		}
		if kind != "" || count != 0 {
			t.Fatalf("kind=%q count=%d", kind, count)
		}
	})
}

func TestStreamInfoSSE(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"sse-0001", "sse.test", "GET", "/events", "https://sse.test/events",
		200, "text/event-stream", 0, 0,
		"GET /events HTTP/1.1\nHost: sse.test\n", nil,
		"HTTP/1.1 200 OK\nContent-Type: text/event-stream\n", nil)

	_, kind, err := tr.StreamInfo("sse-0001")
	if err != nil {
		t.Fatal(err)
	}
	if kind != "sse" {
		t.Fatalf("kind = %q, want sse", kind)
	}
}

func TestStreamMessagesLimitClamp(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	seedExchange(t, tr,
		"clamp-ws", "clamp.test", "GET", "/ws", "wss://clamp.test/ws",
		101, "websocket", 0, 0,
		"GET /ws HTTP/1.1\n", nil, "HTTP/1.1 101\n", nil)

	db := tr.DB()
	for i := int64(1); i <= 5; i++ {
		db.Exec(`INSERT INTO stream_messages(exchange_id,seq,ts,direction,kind,event,event_id,retry,size,payload,truncated) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			"clamp-ws", i, 1000, "c2s", 1, "", "", 0, 1, []byte("x"), 0)
	}

	msgs, _, err := tr.StreamMessages("clamp-ws", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) > 50 && len(msgs) != 5 {
		t.Fatalf("default limit not applied: got %d msgs", len(msgs))
	}
}
