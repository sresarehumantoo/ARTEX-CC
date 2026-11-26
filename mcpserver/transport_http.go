package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

// StreamableHTTP builds the HTTP transport for this server with session IDs
// redacted from its logs.
func (s *Server) StreamableHTTP(opts ...server.StreamableHTTPOption) *server.StreamableHTTPServer {
	return server.NewStreamableHTTPServer(s.mcp, append(opts, server.WithStreamableHTTPLogger(newSessionRedactingLogger()))...)
}

func (s *Server) ServeHTTP(ctx context.Context, addr string, middleware ...func(http.Handler) http.Handler) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("http server: %w", err)
	}
	return s.Serve(ctx, ln, middleware...)
}

// Serve runs the HTTP transport on ln until ctx is canceled. It takes
// ownership of ln and closes it on return.
func (s *Server) Serve(ctx context.Context, ln net.Listener, middleware ...func(http.Handler) http.Handler) error {
	addr := ln.Addr().String()
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
		errCh <- httpSrv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		return streamable.Shutdown(context.Background())
	}
}
