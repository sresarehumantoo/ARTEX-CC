package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

// StreamableHTTP builds the HTTP transport for this server with session IDs
// redacted from its logs.
func (s *Server) StreamableHTTP(opts ...server.StreamableHTTPOption) *server.StreamableHTTPServer {
	return server.NewStreamableHTTPServer(s.mcp, append(opts, server.WithStreamableHTTPLogger(newSessionRedactingLogger()))...)
}

func (s *Server) ServeHTTP(ctx context.Context, addr string, middleware ...func(http.Handler) http.Handler) error {
	httpSrv := &http.Server{Addr: addr}
	streamable := s.StreamableHTTP(server.WithStreamableHTTPServer(httpSrv))
	var handler http.Handler = streamable
	for _, m := range middleware {
		handler = m(handler)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	httpSrv.Handler = mux

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http listener started", "addr", addr)
		errCh <- streamable.Start(addr)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		return streamable.Shutdown(context.Background())
	}
}
