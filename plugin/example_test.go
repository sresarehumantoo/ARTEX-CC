package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func runExamplePlugin(t *testing.T, cfg PluginConfig, input string) (tools.Result, *graphStore) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not on PATH")
	}
	t.Setenv("NO_PROXY", "example.com,127.0.0.1,localhost")
	t.Setenv("no_proxy", "example.com,127.0.0.1,localhost")

	store := &graphStore{}
	reg := tools.NewRegistry()
	tools.RegisterAll(reg, store, 1, 1)
	ctxFn := func(context.Context) (PluginConfig, *tools.Registry, error) { return cfg, reg, nil }
	if err := DiscoverAndRegister(reg, "../examples/plugins", 30*time.Second, ctxFn); err != nil {
		t.Fatal(err)
	}
	tool, ok := reg.Lookup("httpprobe_probe")
	if !ok {
		t.Fatal("httpprobe_probe not registered; is examples/plugins/httpprobe.py executable?")
	}
	result, err := tool.Run(t.Context(), json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	return result, store
}

func TestExamplePlugin_ProbesInScopeAndSkipsTheRest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "demo/1.0")
		fmt.Fprint(w, "<html><title>Demo app</title></html>")
	}))
	defer srv.Close()

	cfg := PluginConfig{
		Engagement: Engagement{ID: 1, ExplorationID: 1, Name: "example"},
		InScope:    []tools.ScopeRule{{Kind: "ip", Value: "127.0.0.1/32"}},
	}
	result, store := runExamplePlugin(t, cfg, `{"urls":["`+srv.URL+`/","http://out-of-scope.invalid/"]}`)
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}
	for _, s := range []string{
		srv.URL + "/ [200] [36] [Demo app] [demo/1.0]",
		"http://out-of-scope.invalid/ skipped: out of scope",
		"inserted 1 asset, 1 fact, 0 findings",
	} {
		if !strings.Contains(result.Content, s) {
			t.Errorf("content missing %q:\n%s", s, result.Content)
		}
	}
	if len(store.assets) != 1 {
		t.Fatalf("assets = %+v", store.assets)
	}
	a := store.assets[0]
	if a.Type != "service" || a.URL != srv.URL+"/" || a.PageTitle != "Demo app" || a.StatusCode == nil || *a.StatusCode != 200 {
		t.Errorf("asset = %+v", a)
	}
	if len(store.facts) != 1 || store.facts[0]["source"] != "tool_output" {
		t.Errorf("facts = %+v", store.facts)
	}
}

func TestExamplePlugin_RoutesThroughProxy(t *testing.T) {
	proxied := make(chan string, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied <- r.URL.String()
		fmt.Fprint(w, "<title>via proxy</title>")
	}))
	defer proxy.Close()

	cfg := PluginConfig{
		InScope: []tools.ScopeRule{{Kind: "root_domain", Value: "example.com"}},
		Proxy:   &Proxy{URL: proxy.URL},
	}
	result, store := runExamplePlugin(t, cfg, `{"urls":["http://app.example.com/login"]}`)
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}
	select {
	case got := <-proxied:
		if got != "http://app.example.com/login" {
			t.Errorf("proxy saw %q", got)
		}
	default:
		t.Error("request did not go through the proxy")
	}
	if len(store.assets) != 1 || store.assets[0].PageTitle != "via proxy" {
		t.Errorf("assets = %+v", store.assets)
	}
}

func TestExamplePlugin_SkipsNonHTTPSchemes(t *testing.T) {
	hit := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit <- struct{}{}
	}))
	defer srv.Close()

	cfg := PluginConfig{InScope: []tools.ScopeRule{{Kind: "ip", Value: "127.0.0.1/32"}}}
	input := `{"urls":["file://127.0.0.1/etc/hostname","ftp://127.0.0.1:1/","FTP://` + strings.TrimPrefix(srv.URL, "http://") + `/"]}`
	result, store := runExamplePlugin(t, cfg, input)
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}
	for _, s := range []string{
		"file://127.0.0.1/etc/hostname skipped: only http and https URLs are probed",
		"ftp://127.0.0.1:1/ skipped: only http and https URLs are probed",
	} {
		if !strings.Contains(result.Content, s) {
			t.Errorf("content missing %q:\n%s", s, result.Content)
		}
	}
	if strings.Contains(result.Content, "failed") {
		t.Errorf("a skipped URL was attempted:\n%s", result.Content)
	}
	select {
	case <-hit:
		t.Error("server was contacted")
	default:
	}
	if len(store.assets)+len(store.facts) != 0 {
		t.Errorf("persisted %d assets, %d facts", len(store.assets), len(store.facts))
	}
}

func TestExamplePlugin_BadInputIsErrorResult(t *testing.T) {
	cfg := PluginConfig{InScope: []tools.ScopeRule{{Kind: "ip", Value: "127.0.0.1/32"}}}
	for _, input := range []string{
		`{"urls":["http://127.0.0.1:1/"],"timeout":"soon"}`,
		`{"urls":["http://127.0.0.1:1/"],"timeout":0}`,
		`{"urls":["http://127.0.0.1:1/"],"timeout":-5}`,
		`{"urls":["http://127.0.0.1:1/"],"timeout":100000}`,
		`{"urls":["http://127.0.0.1:1/"],"timeout":true}`,
		`{"urls":"http://127.0.0.1:1/"}`,
		`{"urls":[1,2]}`,
		`{"urls":[]}`,
	} {
		result, store := runExamplePlugin(t, cfg, input)
		if !result.IsError {
			t.Errorf("%s: want error result, got %s", input, result.Content)
		}
		if len(store.assets)+len(store.facts) != 0 {
			t.Errorf("%s: persisted data", input)
		}
	}
}

func TestExamplePlugin_EmptyScopeProbesNothing(t *testing.T) {
	result, store := runExamplePlugin(t, PluginConfig{InScope: []tools.ScopeRule{}}, `{"urls":["http://127.0.0.1:1/"]}`)
	if !strings.Contains(result.Content, "skipped: out of scope") {
		t.Errorf("content = %s", result.Content)
	}
	if len(store.assets)+len(store.facts) != 0 {
		t.Errorf("persisted %d assets, %d facts", len(store.assets), len(store.facts))
	}
}
