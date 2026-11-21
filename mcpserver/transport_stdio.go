package mcpserver

import (
	"context"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

func (s *Server) ServeStdio(ctx context.Context) error {
	stdio := server.NewStdioServer(s.mcp)
	stdio.SetErrorLogger(slog.NewLogLogger(slog.Default().Handler(), slog.LevelError))
	return stdio.Listen(ctx, os.Stdin, os.Stdout)
}
