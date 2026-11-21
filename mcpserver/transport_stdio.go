package mcpserver

import (
	"context"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

func (s *Server) ServeStdio(ctx context.Context) error {
	stdio := server.NewStdioServer(s.mcp)
	stdio.SetErrorLogger(s.logger)
	return stdio.Listen(ctx, os.Stdin, os.Stdout)
}
