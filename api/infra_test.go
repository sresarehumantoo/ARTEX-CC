package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecodeBody(t *testing.T) {
	var v struct{ A int }
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"A":3}`))
	if err := decodeBody(r, &v); err != nil || v.A != 3 {
		t.Fatalf("decodeBody: v=%+v err=%v", v, err)
	}
	big := `{"A":"` + strings.Repeat("x", maxBodyBytes) + `"}`
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	if err := decodeBody(r, &v); err == nil {
		t.Fatal("want error for oversized body")
	}
}

func TestPathID(t *testing.T) {
	mux := http.NewServeMux()
	var got int64
	var gotErr error
	mux.HandleFunc("GET /x/{n}", func(w http.ResponseWriter, r *http.Request) {
		got, gotErr = pathID(r, "n")
	})
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x/7", nil))
	if gotErr != nil || got != 7 {
		t.Fatalf("pathID: got (%d, %v)", got, gotErr)
	}
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x/abc", nil))
	if gotErr == nil {
		t.Fatal("want error for non-numeric id")
	}
}

func TestFileStore_JSON(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	type doc struct{ Name string }
	if err := fs.WriteJSON("a/b.json", doc{"x"}); err != nil {
		t.Fatal(err)
	}
	var d doc
	if err := fs.ReadJSON("a/b.json", &d); err != nil || d.Name != "x" {
		t.Fatalf("ReadJSON: %+v %v", d, err)
	}
	names, err := fs.ListDir("a")
	if err != nil || len(names) != 1 || names[0] != "b.json" {
		t.Fatalf("ListDir: %v %v", names, err)
	}
	if err := fs.Delete("a/b.json"); err != nil {
		t.Fatal(err)
	}
	if err := fs.ReadJSON("a/b.json", &d); err == nil {
		t.Fatal("want error after delete")
	}
}

func TestFileStore_JSONL(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	for i := 0; i < 3; i++ {
		if err := fs.AppendJSONL("log/c.jsonl", map[string]int{"n": i}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := fs.ReadJSONL("log/c.jsonl", 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("all: %d %v", len(all), err)
	}
	rest, err := fs.ReadJSONL("log/c.jsonl", 2)
	if err != nil || len(rest) != 1 || string(rest[0]) != `{"n":2}` {
		t.Fatalf("rest: %v %v", rest, err)
	}
	none, err := fs.ReadJSONL("missing.jsonl", 0)
	if err != nil || len(none) != 0 {
		t.Fatalf("missing: %v %v", none, err)
	}
}

func TestFileStore_Traversal(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	for _, p := range []string{"../x", "a/../../x", "/etc/passwd"} {
		if err := fs.WriteJSON(p, 1); err == nil {
			t.Errorf("WriteJSON(%q): want error", p)
		}
		if err := fs.Delete(p); err == nil {
			t.Errorf("Delete(%q): want error", p)
		}
	}
}

func TestHub(t *testing.T) {
	h := NewHub()
	a, unsubA := h.Subscribe("t")
	b, unsubB := h.Subscribe("other")
	defer unsubB()

	h.Publish("t", []byte("hi"))
	select {
	case m := <-a:
		if string(m) != "hi" {
			t.Fatalf("got %q", m)
		}
	case <-time.After(time.Second):
		t.Fatal("no message")
	}
	select {
	case <-b:
		t.Fatal("other topic received message")
	default:
	}

	unsubA()
	h.Publish("t", []byte("again"))
	select {
	case <-a:
		t.Fatal("received after unsubscribe")
	default:
	}
}

func TestHub_NonBlocking(t *testing.T) {
	h := NewHub()
	h.Subscribe("t")
	for i := 0; i < subscriberBuffer+5; i++ {
		h.Publish("t", []byte("x"))
	}
}
