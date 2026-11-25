package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/SREsAreHumanToo/ARTEX-CC/evidence"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/SREsAreHumanToo/ARTEX-CC/traffic"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolNotifier is called after a mutating tool completes successfully.
type ToolNotifier func(toolName string)

// Server is an MCP protocol server wrapping the tool registry.
type Server struct {
	mcp      *server.MCPServer
	registry *tools.Registry
	sessions *SessionManager

	store         tools.Store
	engMu         sync.RWMutex
	taskID        int64
	explorationID int64
	traffic       *traffic.Traffic
	evidence      *evidence.Store
	notifier      ToolNotifier
}

// Option is a functional option for configuring a Server.
type Option func(*Server)

// WithEngagement returns an Option that binds the server to a store and engagement.
func WithEngagement(store tools.Store, taskID, explorationID int64) Option {
	return func(s *Server) {
		s.store = store
		s.taskID = taskID
		s.explorationID = explorationID
	}
}

// WithTraffic returns an Option that provides the traffic proxy for re-registration on engagement switch.
func WithTraffic(t *traffic.Traffic) Option {
	return func(s *Server) { s.traffic = t }
}

// WithEvidence returns an Option that provides the evidence store for re-registration on engagement switch.
func WithEvidence(es *evidence.Store) Option {
	return func(s *Server) { s.evidence = es }
}

// WithNotifier returns an Option that installs a callback invoked after each
// mutating (non-read-only) tool completes successfully.
func WithNotifier(fn ToolNotifier) Option {
	return func(s *Server) { s.notifier = fn }
}

// New creates a Server with its tools registered and optional engagement context.
func New(registry *tools.Registry, opts ...Option) *Server {
	s := &Server{
		registry: registry,
		sessions: NewSessionManager(),
	}
	for _, opt := range opts {
		opt(s)
	}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(_ context.Context, cs server.ClientSession) {
		s.sessions.GetOrCreate(sessionKey(cs))
		slog.Debug("session registered", "id", sessionKey(cs))
	})
	hooks.AddOnUnregisterSession(func(_ context.Context, cs server.ClientSession) {
		s.sessions.Remove(sessionKey(cs))
		slog.Debug("session unregistered", "id", sessionKey(cs))
	})

	s.mcp = server.NewMCPServer(
		"artex",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithHooks(hooks),
	)

	for _, t := range registry.Tools() {
		s.addTool(t)
	}

	s.addTool(s.listSessionsTool())

	if s.store != nil && s.explorationID > 0 {
		s.registerEngagementResource()
		s.addTool(s.switchEngagementTool())
	}

	return s
}

func (s *Server) addTool(t tools.Tool) {
	s.mcp.AddTool(convertTool(t), makeHandlerWithNotifier(t, s.notifier))
}

// ActiveEngagement returns the engagement the server is bound to now, which
// switch_engagement can change at runtime.
func (s *Server) ActiveEngagement() (taskID, explorationID int64) {
	s.engMu.RLock()
	defer s.engMu.RUnlock()
	return s.taskID, s.explorationID
}

// Sessions returns the session manager for external inspection.
func (s *Server) Sessions() *SessionManager { return s.sessions }

// MCPServer returns the underlying mcp-go server for use as an http.Handler.
func (s *Server) MCPServer() *server.MCPServer { return s.mcp }

func (s *Server) listSessionsTool() tools.Tool {
	return tools.Tool{
		Name:        "list_sessions",
		Description: "List all active MCP client sessions connected to this server.",
		Category:    tools.CategoryEngagement,
		ReadOnly:    true,
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Run: func(_ context.Context, _ json.RawMessage) (tools.Result, error) {
			sessions := s.sessions.List()
			out, _ := json.Marshal(map[string]any{
				"count":    len(sessions),
				"sessions": sessions,
			})
			return tools.Result{Content: string(out)}, nil
		},
	}
}

// SessionFromContext returns the application-level Session for the client
// that sent the current request. It bridges mcp-go's transport session
// to our SessionManager. Returns nil if the context has no session.
func (s *Server) SessionFromContext(ctx context.Context) *Session {
	cs := server.ClientSessionFromContext(ctx)
	if cs == nil {
		return nil
	}
	return s.sessions.Get(sessionKey(cs))
}

const engagementURI = "artex://engagement"

func (s *Server) registerEngagementResource() {
	res := mcp.NewResource(
		engagementURI,
		"ARTEX Orchestration Guide",
		mcp.WithResourceDescription("Penetration test orchestration guide with current engagement context"),
		mcp.WithMIMEType("text/plain"),
	)
	s.mcp.AddResource(res, s.handleReadEngagement)
}

func (s *Server) handleReadEngagement(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	slog.Debug("engagement resource read")
	var b strings.Builder
	b.WriteString(orchestrationGuide)

	desc, goal, _ := s.store.Root(ctx, s.explorationID)
	constraints, _ := s.store.ListConstraints(ctx, s.explorationID)

	b.WriteString("\n## Current engagement\n\n")
	fmt.Fprintf(&b, "Goal: %s\n", goal)
	fmt.Fprintf(&b, "Description: %s\n", desc)
	fmt.Fprintf(&b, "Task ID: %d\n", s.taskID)
	fmt.Fprintf(&b, "Exploration ID: %d\n", s.explorationID)
	b.WriteString("\nConstraints:\n")
	if len(constraints) == 0 {
		b.WriteString("None set.\n")
	} else {
		for _, c := range constraints {
			fmt.Fprintf(&b, "- [%s] %s\n", c.Kind, c.Text)
		}
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      engagementURI,
			MIMEType: "text/plain",
			Text:     b.String(),
		},
	}, nil
}

const orchestrationGuide = `# ARTEX Penetration Test Orchestration

You are running an authorized penetration test through ARTEX. The exploration
graph is your task board. Every piece of state lives there, not in your context.

## Workflow

1. ORIENT: Call graph_overview to see goals, open/running/done intents, facts,
   findings, coverage. This is the single source of truth.

2. PLAN: Based on the overview, decide what needs doing next. Create intents
   with add_intent for each direction to explore. Each intent is one sentence
   describing what to test/verify/exploit.

3. EXECUTE: For each open intent, do the work: run tools, test the target,
   record what you find. Use record_fact for conclusions, insert_assets for
   new assets, report_finding for confirmed vulnerabilities.

4. LOOP: After completing work, call graph_overview again. New facts may
   reveal new directions. Create more intents. Continue until all goals are met
   or all directions are exhausted.

## Parallel execution with subagents

Dispatch subagents for independent intents. Each subagent:
- Receives one intent (by id and summary) in its prompt
- Has access to the same MCP tools
- Calls record_fact/report_finding/insert_assets to write results into the graph
- Returns a one-line summary when done

The coordinator (you) reads graph_overview after subagents finish to see their
results and plan the next round.

Pattern:
  1. graph_overview -> identify 2-4 open intents
  2. Launch one Agent per intent (parallel, in one message)
  3. When agents finish, graph_overview again
  4. Plan next intents based on new facts
  5. Repeat

## Recovery from context exhaustion

The exploration graph preserves ALL state. If you start a fresh session or
your context fills up:
  1. Call graph_overview. It shows everything: goals, what has been tried,
     what is open, what was found.
  2. start_intent claims an intent for your session for 30 minutes. Running
     intents show claimed_by and claim_expires_at; leave those to their
     owner. Once a claim expires the intent is listed as open again with
     claim_expired, and any session may start it.
  3. Continue from where the graph says you left off.

You never lose work. Facts, findings, and assets are persisted the moment they
are recorded.

## When to stop

Call prove_goal when a finding or fact proves a goal met. When all goals are
met, the engagement is complete. If all directions are exhausted without
meeting goals, report what was tested and what remains unclear.

## Hands-free mode

For continuous autonomous operation, the user can run:
  /loop 5m Continue the pentest: call graph_overview, assess progress, dispatch subagents for open intents, prove goals when evidence exists.
`
