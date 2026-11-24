package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/SREsAreHumanToo/cc-artex/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func convertTool(t tools.Tool) mcp.Tool {
	schema, _ := json.Marshal(t.Schema)
	tool := mcp.NewToolWithRawSchema(t.Name, t.Description, schema)
	tool.Annotations.ReadOnlyHint = mcp.ToBoolPtr(t.ReadOnly)
	return tool
}

func makeHandlerWithNotifier(t tools.Tool, notifier ToolNotifier) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Info("tool called", "tool", t.Name)
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
			notifier(t.Name)
		}
		return mcp.NewToolResultText(result.Content), nil
	}
}
