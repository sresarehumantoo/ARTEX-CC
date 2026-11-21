package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
)

func (s *Server) ServeHTTP(ctx context.Context, addr string) error {
	httpServer := server.NewStreamableHTTPServer(s.mcp)

	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("listening on %s", addr)
		errCh <- httpServer.Start(addr)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		return httpServer.Shutdown(context.Background())
	}
}
