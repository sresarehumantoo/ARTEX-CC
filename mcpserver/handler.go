package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func convertTool(t tools.Tool) mcp.Tool {
	schema, _ := json.Marshal(t.Schema)
	tool := mcp.NewToolWithRawSchema(t.Name, t.Description, schema)
	tool.Annotations.ReadOnlyHint = mcp.ToBoolPtr(t.ReadOnly)
	return tool
}

// stdioSessionID stands in for mcp-go's constant "stdio" session ID so that
// separate stdio server processes sharing one database hold distinct claims.
var stdioSessionID = "stdio-" + randomID()[:8]

func sessionKey(cs server.ClientSession) string {
	if id := cs.SessionID(); id != "stdio" {
		return id
	}
	return stdioSessionID
}

func callerSessionID(ctx context.Context) string {
	if cs := server.ClientSessionFromContext(ctx); cs != nil {
		return sessionKey(cs)
	}
	return ""
}

func makeHandlerWithNotifier(t tools.Tool, notifier ToolNotifier) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Info("tool called", "tool", t.Name)
		ctx = tools.WithSessionID(ctx, callerSessionID(ctx))
		raw, err := json.Marshal(req.GetRawArguments())
		if err != nil {
			return mcp.NewToolResultError("invalid arguments: " + err.Error()), nil
		}

		result, err := t.Run(ctx, json.RawMessage(raw))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if result.IsError {
			return mcp.NewToolResultError(result.Content), nil
		}
		if notifier != nil && !t.ReadOnly {
			notifier(t.Name, raw)
		}
		return mcp.NewToolResultText(result.Content), nil
	}
}
