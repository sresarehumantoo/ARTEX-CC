package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/evidence"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/SREsAreHumanToo/ARTEX-CC/traffic"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolNotifier is called after a mutating tool completes successfully, with
// the arguments it was called with.
type ToolNotifier func(toolName string, args json.RawMessage)

// Server is an MCP protocol server wrapping the tool registry.
type Server struct {
	mcp      *server.MCPServer
	registry *tools.Registry
	sessions *SessionManager

	identities identitySeen

	store         tools.Store
	engMu         sync.RWMutex
	taskID        int64
	explorationID int64
	traffic       *traffic.Traffic
	evidence      *evidence.Store
	notifier      ToolNotifier
	version       string
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

// WithVersion returns an Option that sets the version reported to MCP clients.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

// New creates a Server with its tools registered and optional engagement context.
func New(registry *tools.Registry, opts ...Option) *Server {
	s := &Server{
		registry: registry,
		sessions: NewSessionManager(),
		version:  "dev",
	}
	for _, opt := range opts {
		opt(s)
	}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(_ context.Context, cs server.ClientSession) {
		s.sessions.GetOrCreate(sessionKey(cs))
		slog.Debug("session registered", "session", tools.OwnerLabel(sessionKey(cs)))
	})
	hooks.AddOnUnregisterSession(func(_ context.Context, cs server.ClientSession) {
		s.sessions.Remove(sessionKey(cs))
		slog.Debug("session unregistered", "session", tools.OwnerLabel(sessionKey(cs)))
	})

	s.mcp = server.NewMCPServer(
		"artex",
		s.version,
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

// ActiveRegistry returns the active engagement together with the registry
// whose tools are bound to it, read under one lock so they always agree.
func (s *Server) ActiveRegistry() (taskID, explorationID int64, registry *tools.Registry) {
	s.engMu.RLock()
	defer s.engMu.RUnlock()
	return s.taskID, s.explorationID, s.registry
}

// Sessions returns the session manager for external inspection.
func (s *Server) Sessions() *SessionManager { return s.sessions }

// MCPServer returns the underlying mcp-go server for use as an http.Handler.
func (s *Server) MCPServer() *server.MCPServer { return s.mcp }

func (s *Server) listSessionsTool() tools.Tool {
	return tools.Tool{
		Name: "list_sessions",
		Description: "List all active MCP client sessions connected to this server. " +
			"Each session shows its holder label, the same label claimed_by shows for claims it takes without an owner argument, " +
			"and self is true on your own session. " +
			"identities lists the X-Artex-Identity names that made requests to this server process within the claim lease, " +
			"with self true on yours; clients on MCP HTTP protocol 2026-07-28 have no session and appear only there.",
		Category: tools.CategoryEngagement,
		ReadOnly: true,
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Run: func(ctx context.Context, _ json.RawMessage) (tools.Result, error) {
			caller := tools.SessionID(ctx)
			callerIdentity := tools.IdentityName(ctx)
			type identityEntry struct {
				Name       string    `json:"name"`
				Self       bool      `json:"self"`
				LastActive time.Time `json:"last_active"`
			}
			seen := s.identities.active(time.Now().Add(-tools.IntentLease))
			identities := make([]identityEntry, len(seen))
			for i, id := range seen {
				identities[i] = identityEntry{
					Name:       id.Name,
					Self:       callerIdentity != "" && id.Name == callerIdentity,
					LastActive: id.LastActive,
				}
			}
			type entry struct {
				Label      string            `json:"label"`
				Self       bool              `json:"self"`
				CreatedAt  time.Time         `json:"created_at"`
				LastActive time.Time         `json:"last_active"`
				Context    map[string]string `json:"context"`
			}
			sessions := s.sessions.List()
			entries := make([]entry, len(sessions))
			for i, info := range sessions {
				entries[i] = entry{
					Label:      tools.OwnerLabel(info.ID),
					Self:       caller != "" && info.ID == caller,
					CreatedAt:  info.CreatedAt,
					LastActive: info.LastActive,
					Context:    info.Context,
				}
			}
			out, _ := json.Marshal(map[string]any{
				"count":      len(entries),
				"sessions":   entries,
				"identities": identities,
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
	if tools.RequireIdentity {
		b.WriteString(requireIdentityGuide)
	}

	taskID, explorationID := s.ActiveEngagement()
	desc, goal, _ := s.store.Root(ctx, explorationID)
	constraints, _ := s.store.ListConstraints(ctx, explorationID)

	b.WriteString("\n## Current engagement\n\n")
	fmt.Fprintf(&b, "Goal: %s\n", goal)
	fmt.Fprintf(&b, "Description: %s\n", desc)
	fmt.Fprintf(&b, "Task ID: %d\n", taskID)
	fmt.Fprintf(&b, "Exploration ID: %d\n", explorationID)
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

2. PLAN: Based on the overview, decide what needs doing next. If the overview
   shows no goals, call set_goals once; to add a goal later, call set_goals with
   append: true. Create intents with add_intent for each direction to explore.
   Each intent is one sentence describing what to test/verify/exploit.

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

## Several agents on one engagement

When other sessions (separate Claude Code processes or HTTP clients) work the
same engagement, coordinate through the graph:
- Goals are set once per engagement. If set_goals returns existing_goals,
  another session set them first: adopt those goals instead of adding your
  own.
- add_intent skips an intent whose summary (ignoring case, punctuation and
  spacing) and asset_ids match an open or running intent, and returns its
  existing_id under skipped: start_intent that one. It also lists
  open_intents that nobody holds; if one covers work you just planned, work
  that one and cancel_intent yours.
- Call start_intent before working an intent. If it fails because another
  session holds the claim, pick a different intent; do not wait or retry.
- Claims last 30 minutes and renew whenever you pass the intent's id as
  intent_id to record_fact, report_finding or record_coverage, so record
  results as you go. If you work longer than that without recording
  anything for it, call start_intent on the same intent again to renew it.
  If you claimed with an owner argument, pass the same owner to those calls
  too, or they do not renew your claim.
- Finish with complete_intent, or cancel_intent if it is no longer worth
  doing, so the claim is released. Both are refused on an intent another
  session holds; leave it to its owner. On an open intent both claim it
  first, so the intent records who closed it (completed_by, cancelled_by).
  That is no substitute for start_intent: work you do without claiming
  looks free to every other session, which may do it too.
- Claims belong to your identity when your client sends an X-Artex-Identity
  header, otherwise to your MCP session. In the web UI chat your identity is
  chat/<conversation id>, sent for you. An identity keeps its claims across
  reconnects and server restarts. If
  start_intent says the connection has no session (HTTP clients on protocol
  2026-07-28 without an identity), pick an owner once, a random hex string,
  and pass it as owner on every start_intent, complete_intent, cancel_intent
  and list_changes call. With an identity or a session, owner is optional
  and makes a separate holder inside it, so subagents of one process can
  each pass their own; keep passing the same value.
- claimed_by, the session field of list_changes and claim errors show a
  holder label: your identity name (plus a hashed suffix when you pass
  owner), or a hashed label otherwise, never a session ID or owner. Compare
  claimed_by with the session field of list_changes to recognize your
  claims. list_sessions shows each session's label (what its claims carry
  without an owner) and the identities active on this server, and marks
  yours with self.
- Intents left running from before claims existed have no lease and can be
  started by any session right away.
- Between rounds, poll list_changes with the cursor from your last call (the
  first cursor is changes_cursor in graph_overview) instead of re-reading
  graph_overview. It returns new facts, findings, intents with their claim
  holder, goals, hints and assets, plus my_claims: the intents you hold and
  when each expires. Re-read graph_overview when you run out of work, since
  intents whose claims lapse reopen without showing up as changes.
- Before report_finding, check list_findings (or the findings in
  list_changes) for the same vulnclass on the same asset. If another session
  already reported it, add to that finding instead of filing a new one. If
  report_finding returns possible_duplicates and one is the same issue, call
  merge_findings right away.
- When every open intent is claimed by another session, do not wait or run
  sleep. Poll list_changes once more and take any new or reopened intent; if
  there is none, add intents for untested directions, or finish: complete
  your own intents, prove goals that have evidence, and stop.

Subagents inside one Claude Code process share its MCP session, so their
claims are yours unless each passes its own owner; assign them distinct
intents yourself.

## Changes to the target

- Before you modify, create or delete anything on a target (file, registry
  key, service, scheduled task, account, config), call record_target_change
  with its original fingerprint (sha256:<hex> of a file, absent for
  something you create) and where you keep the backup.
- If record_target_change returns a warning, another live change already
  exists on that resource and what you see is not the original. Restore to
  the returned baseline, never to your own backup, and coordinate with the
  holders in existing_changes.
- Before you finish, restore what you changed and call
  record_target_restore with the fingerprint you observe afterwards. Only a
  match with the baseline counts as restored.
- Never report a host restored or cleaned up while graph_overview or
  list_changes shows unrestored_target_changes.

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

const requireIdentityGuide = `
## Identity required for claims

This server requires a verified identity for intent claims. start_intent,
complete_intent and cancel_intent are refused, and list_changes omits
my_claims, unless your client sends an X-Artex-Identity header. An owner
argument or an MCP session does not count. If you get that refusal, stop
claiming and ask the operator to configure your MCP client with a token from
artex-cc identity issue (--format claude-json prints a ready config). The
other tools work without an identity.
`
