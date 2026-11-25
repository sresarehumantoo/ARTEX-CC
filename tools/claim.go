package tools

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
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

// OwnerLabeler turns claim owners into the labels shown to agents. Labels
// are keyed hashes so that a session ID or owner string can be neither read
// from another client's output and replayed, nor guessed from its label.
type OwnerLabeler struct {
	key []byte
}

func NewOwnerLabeler(key []byte) *OwnerLabeler {
	return &OwnerLabeler{key: bytes.Clone(key)}
}

func (l *OwnerLabeler) Label(owner string) string {
	if owner == "" {
		return ""
	}
	h := hmac.New(sha256.New, l.key)
	h.Write([]byte(owner))
	return "h-" + hex.EncodeToString(h.Sum(nil)[:6])
}

// ownerLabeler is process-wide because labels are rendered in tools, store
// and mcpserver alike, including inside ClaimConflictError.Error. It starts
// with a random key so a process that never calls SetOwnerLabeler still
// hides owners; serve installs the data directory's key at startup.
var ownerLabeler atomic.Pointer[OwnerLabeler]

func init() {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	ownerLabeler.Store(NewOwnerLabeler(key))
}

// SetOwnerLabeler replaces the labeler OwnerLabel uses.
func SetOwnerLabeler(l *OwnerLabeler) {
	ownerLabeler.Store(l)
}

// OwnerLabel is the form of a claim owner or session ID shown in tool
// results, logs and exports.
func OwnerLabel(owner string) string {
	return ownerLabeler.Load().Label(owner)
}

// IntentClaim is the result of a successful ClaimIntent.
type IntentClaim struct {
	NodeID    int64
	Owner     string `json:"-"`
	ExpiresAt time.Time
	Renewed   bool
}

// ClaimConflictError means another session holds a live claim on the intent.
type ClaimConflictError struct {
	NodeID    int64
	Owner     string `json:"-"`
	ExpiresAt time.Time
}

func (e *ClaimConflictError) Error() string {
	return fmt.Sprintf("intent %d is claimed by %s until %s (in %s); pick another intent",
		e.NodeID, OwnerLabel(e.Owner), e.ExpiresAt.UTC().Format(time.RFC3339),
		time.Until(e.ExpiresAt).Round(time.Second))
}
