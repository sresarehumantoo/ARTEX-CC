package tools

import (
	"context"
	"fmt"
	"time"
)

// IntentLease is how long a start_intent claim stays exclusive before another
// session may take the intent over. Calling start_intent again renews it.
const IntentLease = 30 * time.Minute

// LocalSessionID is the claim owner used when a call carries no MCP session.
const LocalSessionID = "local"

type sessionIDKey struct{}

// WithSessionID returns a context carrying the calling MCP session ID.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, id)
}

// SessionID returns the calling MCP session ID, or LocalSessionID if none.
func SessionID(ctx context.Context) string {
	if id, _ := ctx.Value(sessionIDKey{}).(string); id != "" {
		return id
	}
	return LocalSessionID
}

// IntentClaim is the result of a successful ClaimIntent.
type IntentClaim struct {
	NodeID    int64
	Owner     string
	ExpiresAt time.Time
	Renewed   bool
}

// ClaimConflictError means another session holds a live claim on the intent.
type ClaimConflictError struct {
	NodeID    int64
	Owner     string
	ExpiresAt time.Time
}

func (e *ClaimConflictError) Error() string {
	return fmt.Sprintf("intent %d is claimed by session %s until %s (in %s); pick another intent",
		e.NodeID, e.Owner, e.ExpiresAt.UTC().Format(time.RFC3339),
		time.Until(e.ExpiresAt).Round(time.Second))
}
