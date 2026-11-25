package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// sessionIDPattern matches IDs from mcp-go's InsecureStatefulSessionIdManager.
var sessionIDPattern = regexp.MustCompile(`mcp-session-[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// identityTokenPattern matches X-Artex-Identity tokens (ai1.<payload>.<mac>).
var identityTokenPattern = regexp.MustCompile(`ai1\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)

const redactedIdentityToken = "ai1.[redacted]"

func redactSecrets(s string) string {
	s = identityTokenPattern.ReplaceAllLiteralString(s, redactedIdentityToken)
	return sessionIDPattern.ReplaceAllStringFunc(s, tools.OwnerLabel)
}

// sessionRedactingHandler replaces raw MCP session IDs in mcp-go's transport
// logs with their holder labels, since a logged ID can be replayed as
// Mcp-Session-Id, and removes identity tokens for the same reason. It resolves slog.Default at each record rather than at
// construction, so records still reach the web log tee installed later.
type sessionRedactingHandler struct {
	wrap []func(slog.Handler) slog.Handler
}

func newSessionRedactingLogger() *slog.Logger {
	return slog.New(&sessionRedactingHandler{})
}

func (h *sessionRedactingHandler) next() slog.Handler {
	next := slog.Default().Handler()
	for _, w := range h.wrap {
		next = w(next)
	}
	return next
}

func (h *sessionRedactingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next().Enabled(ctx, l)
}

func (h *sessionRedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, redactSecrets(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(redactAttr(a))
		return true
	})
	return h.next().Handle(ctx, out)
}

func (h *sessionRedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = redactAttr(a)
	}
	return h.with(func(n slog.Handler) slog.Handler { return n.WithAttrs(redacted) })
}

func (h *sessionRedactingHandler) WithGroup(name string) slog.Handler {
	return h.with(func(n slog.Handler) slog.Handler { return n.WithGroup(name) })
}

func (h *sessionRedactingHandler) with(w func(slog.Handler) slog.Handler) slog.Handler {
	return &sessionRedactingHandler{wrap: append(append([]func(slog.Handler) slog.Handler(nil), h.wrap...), w)}
}

func redactAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		// mcp-go logs the GET rejection's session header verbatim, so a
		// session attribute is labeled whatever its format.
		if (a.Key == "session" || a.Key == "sessionID") && v.String() != "" {
			return slog.String(a.Key, tools.OwnerLabel(v.String()))
		}
		return slog.String(a.Key, redactSecrets(v.String()))
	case slog.KindGroup:
		group := v.Group()
		redacted := make([]slog.Attr, len(group))
		for i, g := range group {
			redacted[i] = redactAttr(g)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(redacted...)}
	case slog.KindAny:
		s := fmt.Sprint(v.Any())
		if r := redactSecrets(s); r != s {
			return slog.String(a.Key, r)
		}
	}
	return slog.Attr{Key: a.Key, Value: v}
}
