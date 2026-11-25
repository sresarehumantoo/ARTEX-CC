package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

func (s *Server) ServeHTTP(ctx context.Context, addr string, middleware ...func(http.Handler) http.Handler) error {
	httpSrv := &http.Server{Addr: addr}
	streamable := server.NewStreamableHTTPServer(s.mcp, server.WithStreamableHTTPServer(httpSrv))
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
