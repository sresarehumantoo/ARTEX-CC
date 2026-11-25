package mcpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/mark3labs/mcp-go/server"
)

func TestConversationHeaderOwnsClaims(t *testing.T) {
	t.Parallel()
	st := &claimStore{
		nodes: map[int64]*tools.Node{
			10: {ID: 10, Kind: tools.KindIntent, State: "open"},
			11: {ID: 11, Kind: tools.KindIntent, State: "open"},
		},
		claims: map[int64]tools.IntentClaim{},
	}
	reg := tools.NewRegistry()
	tools.RegisterIntentTools(reg, st, 1)
	srv := mcpserver.New(reg)
	signer, err := mcpserver.NewConversationSigner()
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(signer.Middleware(server.NewStreamableHTTPServer(srv.MCPServer())))
	t.Cleanup(ts.Close)
	url := ts.URL + "/mcp"

	as := func(token string) http.Header {
		return http.Header{mcpserver.ConversationHeader: {token}}
	}
	call := func(h http.Header, tool string, node int64) (string, bool) {
		t.Helper()
		resp := postModern(t, url, h, tool, map[string]any{"node_id": node})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: HTTP %d", tool, resp.StatusCode)
		}
		return decodeToolResult(t, tool, resp)
	}
	convA, convB := as(signer.Token("conv-a")), as(signer.Token("conv-b"))

	if out, isErr := call(convA, "start_intent", 10); isErr || !strings.Contains(out, "claimed by") {
		t.Fatalf("conversation A first claim without owner = %q", out)
	}
	if out, isErr := call(convB, "start_intent", 11); isErr || !strings.Contains(out, "claimed by") {
		t.Fatalf("conversation B first claim without owner = %q", out)
	}
	if a, b := st.owner(10), st.owner(11); a == "" || b == "" || a == b {
		t.Fatalf("owners A=%q B=%q, want distinct non-empty", a, b)
	}
	if out, isErr := call(convB, "start_intent", 10); !isErr || !strings.Contains(out, "claimed by") {
		t.Fatalf("conversation B on A's intent = %q, want claim conflict", out)
	}
	if out, isErr := call(convA, "start_intent", 10); isErr || !strings.Contains(out, "renewed") {
		t.Fatalf("conversation A renewal = %q", out)
	}

	other, err := mcpserver.NewConversationSigner()
	if err != nil {
		t.Fatal(err)
	}
	macA := signer.Token("conv-a")[len("conv-a."):]
	forged := []string{
		"conv-a",
		"conv-a.",
		"conv-a.deadbeef",
		"conv-b." + macA,
		other.Token("conv-a"),
	}
	for _, token := range forged {
		resp := postModern(t, url, as(token), "complete_intent", map[string]any{"node_id": 10})
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("forged token %q: HTTP %d, want 401", token, resp.StatusCode)
		}
	}
	if got := st.owner(10); got == "" {
		t.Fatal("conversation A lost its claim to a forged token")
	}

	if out, isErr := call(nil, "start_intent", 10); !isErr || !strings.Contains(out, "owner") {
		t.Fatalf("start_intent without header or owner = %q, want refusal naming owner", out)
	}
}
