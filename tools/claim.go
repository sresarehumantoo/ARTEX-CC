package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DefaultIntentLease is the claim lease when ARTEX_INTENT_LEASE is unset.
const DefaultIntentLease = 30 * time.Minute

// IntentLease is how long a start_intent claim stays exclusive before another
// session may take the intent over. Calling start_intent again renews it.
// serve sets it from ARTEX_INTENT_LEASE before registering tools, because
// start_intent's description states it.
var IntentLease = DefaultIntentLease

// ParseIntentLease parses an ARTEX_INTENT_LEASE value. Only whole minutes are
// accepted since agents are told the lease in minutes.
func ParseIntentLease(v string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("intent lease %q: %w", v, err)
	}
	if d < time.Minute || d%time.Minute != 0 {
		return 0, fmt.Errorf("intent lease %q must be a whole number of minutes, at least 1m", v)
	}
	return d, nil
}

const ownerArgDesc = "stable label identifying you as a claim holder, e.g. a random hex string chosen once and passed on every claim call. " +
	"Required when the transport has no session (MCP HTTP protocol 2026-07-28); otherwise optional, and it splits one session into separate holders"

// ErrNoClaimOwner means a claim operation has neither an MCP session nor an
// owner argument to identify the caller.
var ErrNoClaimOwner = errors.New("this connection has no MCP session to own intent claims: " +
	"pass owner, a stable label you pick once (e.g. a random hex string), on start_intent, complete_intent, cancel_intent and list_changes")

type sessionIDKey struct{}

// WithSessionID returns a context carrying the calling MCP session ID.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, id)
}

// SessionID returns the calling MCP session ID, or "" if the call has none.
func SessionID(ctx context.Context) string {
	id, _ := ctx.Value(sessionIDKey{}).(string)
	return id
}

// ClaimOwner resolves who holds claims for this call. A session alone keeps
// its ID, so claims from before the owner argument still match. An owner
// argument is scoped under the session when there is one, so it can only
// subdivide a session, never impersonate another one.
func ClaimOwner(ctx context.Context, owner string) (string, error) {
	session := SessionID(ctx)
	owner = strings.TrimSpace(owner)
	switch {
	case session != "" && owner != "":
		return session + "/owner:" + owner, nil
	case session != "":
		return session, nil
	case owner != "":
		return "owner:" + owner, nil
	default:
		return "", ErrNoClaimOwner
	}
}

// OwnerLabel is the form of a claim owner shown to agents. It is a hash so
// that a session ID or owner string seen in one client's output cannot be
// replayed by another to take over its claims.
func OwnerLabel(owner string) string {
	if owner == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(owner))
	return "h-" + hex.EncodeToString(sum[:6])
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
	return fmt.Sprintf("intent %d is claimed by %s until %s (in %s); pick another intent",
		e.NodeID, OwnerLabel(e.Owner), e.ExpiresAt.UTC().Format(time.RFC3339),
		time.Until(e.ExpiresAt).Round(time.Second))
}
