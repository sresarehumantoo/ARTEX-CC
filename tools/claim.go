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
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
)

// DefaultIntentLease is the claim lease when ARTEX_INTENT_LEASE is unset.
const DefaultIntentLease = 30 * time.Minute

// IntentLease is how long a start_intent claim stays exclusive before another
// session may take the intent over. Calling start_intent again renews it, and
// so does the holder recording a fact, finding or coverage against it.
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

// RequireIdentity makes claim operations refuse callers without a verified
// X-Artex-Identity. serve sets it from --require-identity or
// ARTEX_REQUIRE_IDENTITY before registering tools, because start_intent's
// description states it.
var RequireIdentity bool

const ownerArgDesc = "stable label identifying you as a claim holder, e.g. a random hex string chosen once and passed on every claim call. " +
	"Required when the transport has no session and no X-Artex-Identity header (MCP HTTP protocol 2026-07-28); otherwise optional, and it splits your session or identity into separate holders"

const renewOwnerArgDesc = "the owner you passed to start_intent, if you claimed with one, so this call renews that claim"

// ErrNoClaimOwner means a claim operation has neither an MCP session nor an
// owner argument to identify the caller.
var ErrNoClaimOwner = errors.New("this connection has no MCP session to own intent claims: " +
	"pass owner, a stable label you pick once (e.g. a random hex string), on start_intent, complete_intent, cancel_intent and list_changes")

// ErrIdentityRequired means RequireIdentity is on and the call carried no
// verified identity. A session or an owner argument does not count.
var ErrIdentityRequired = errors.New("this server requires a verified identity for intent claims (serve --require-identity or ARTEX_REQUIRE_IDENTITY): " +
	"configure your MCP client to send an X-Artex-Identity header with a token from " +
	"`artex-cc identity issue --data <dir> --name <name>` (add --format claude-json for a ready client config), then reconnect")

// CheckIdentity returns ErrIdentityRequired when RequireIdentity is on and
// the call has no verified identity.
func CheckIdentity(ctx context.Context) error {
	if RequireIdentity && IdentityName(ctx) == "" {
		return ErrIdentityRequired
	}
	return nil
}

// ErrDeadEvidence means prove_goal was given a dismissed or merged node.
var ErrDeadEvidence = errors.New("evidence node is dismissed or merged")

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

type identityKey struct{}

// WithIdentity returns a context carrying the caller's verified identity name.
func WithIdentity(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, identityKey{}, name)
}

// IdentityName returns the caller's verified identity name, or "" if the
// call carried no identity.
func IdentityName(ctx context.Context) string {
	name, _ := ctx.Value(identityKey{}).(string)
	return name
}

const identityHolderPrefix = "id:"

// ClaimOwner resolves who holds claims for this call. A verified identity
// takes precedence and leaves the session out, so the same identity keeps
// its claims across reconnects and protocol eras. Without one, a session
// alone keeps its ID, so claims from before the owner argument still match.
// An owner argument is scoped under the identity or session when there is
// one, so it can only subdivide that holder, never impersonate another one.
func ClaimOwner(ctx context.Context, owner string) (string, error) {
	if err := CheckIdentity(ctx); err != nil {
		return "", err
	}
	session := SessionID(ctx)
	name := IdentityName(ctx)
	owner = strings.TrimSpace(owner)
	switch {
	case name != "" && owner != "":
		return identityHolderPrefix + name + "/owner:" + owner, nil
	case name != "":
		return identityHolderPrefix + name, nil
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

// Label returns the identity name for identity holders, with the owner part
// hashed when there is one, and a keyed hash for every other holder. Names
// are not secret: knowing one does not let a client forge its token.
func (l *OwnerLabeler) Label(owner string) string {
	if owner == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(owner, identityHolderPrefix); ok {
		// Identity names cannot contain ':', so the first "/owner:" ends the name.
		if i := strings.Index(rest, "/owner:"); i >= 0 {
			return rest[:i] + "/" + l.hash(rest[i+1:])
		}
		return rest
	}
	return l.hash(owner)
}

func (l *OwnerLabeler) hash(s string) string {
	h := hmac.New(sha256.New, l.key)
	h.Write([]byte(s))
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
	return fmt.Sprintf("intent %d is claimed by %s until %s (in %s); pick another intent. "+
		"The claim renews while its holder records results against it, so do not wait for it to lapse",
		e.NodeID, OwnerLabel(e.Owner), e.ExpiresAt.UTC().Format(time.RFC3339),
		time.Until(e.ExpiresAt).Round(time.Second))
}

// renewHeldIntents extends the caller's claims on the intents a write was
// recorded against, so a holder that keeps recording results keeps its
// claim. The holder is resolved from owner as start_intent resolves it, so
// only the holder's own calls renew, and a failed renewal never fails the
// write.
func renewHeldIntents(ctx context.Context, store Store, owner string, intentIDs []int64) {
	var ids []int64
	for _, id := range intentIDs {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	holder, err := ClaimOwner(ctx, owner)
	if err != nil {
		return
	}
	if err := store.RenewIntentClaims(ctx, ids, holder, IntentLease); err != nil {
		slog.Warn("renew intent claims", "intents", ids, "err", err)
	}
}
