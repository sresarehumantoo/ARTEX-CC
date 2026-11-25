package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestHandlerPassesSessionIDToTools(t *testing.T) {
	t.Parallel()
	var got string
	h := makeHandlerWithNotifier(tools.Tool{
		Name: "whoami",
		Run: func(ctx context.Context, _ json.RawMessage) (tools.Result, error) {
			got = tools.SessionID(ctx)
			return tools.Result{}, nil
		},
	}, nil)
	srv := server.NewMCPServer("t", "0")

	cases := []struct {
		session server.ClientSession
		check   func(string) bool
	}{
		{nil, func(id string) bool { return id == "" }},
		{server.NewInProcessSession("http-abc", nil), func(id string) bool { return id == "http-abc" }},
		{server.NewInProcessSession("stdio", nil), func(id string) bool {
			return id == stdioSessionID && strings.HasPrefix(id, "stdio-") && len(id) > len("stdio-")
		}},
	}
	for _, c := range cases {
		ctx := context.Background()
		if c.session != nil {
			ctx = srv.WithContext(ctx, c.session)
		}
		if _, err := h(ctx, mcp.CallToolRequest{}); err != nil {
			t.Fatal(err)
		}
		if !c.check(got) {
			t.Errorf("session %v: tool saw session ID %q", c.session, got)
		}
	}
}
