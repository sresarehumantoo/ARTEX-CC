package mcpserver

import (
	"log"
	"os"

	"github.com/SREsAreHumanToo/cc-artex/tools"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcp      *server.MCPServer
	registry *tools.Registry
	logger   *log.Logger
}

type Option func(*Server)

func WithLogger(l *log.Logger) Option {
	return func(s *Server) {
		s.logger = l
	}
}

func New(registry *tools.Registry, opts ...Option) *Server {
	s := &Server{
		registry: registry,
		logger:   log.New(os.Stderr, "mcpserver: ", log.LstdFlags),
	}
	for _, opt := range opts {
		opt(s)
	}

	s.mcp = server.NewMCPServer(
		"cc-artex",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	for _, t := range registry.Tools() {
		s.mcp.AddTool(convertTool(t), makeHandler(t))
	}

	return s
}

