package mcpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// IdentityHeader carries a signed client identity token on MCP requests.
const IdentityHeader = "X-Artex-Identity"

// retiredConversationHeader is the pre-identity web chat header. Its tokens
// were signed with a per-process key, so none can still be valid.
const retiredConversationHeader = "X-Artex-Conversation"

var identityRejections = []struct {
	err    error
	reason string
}{
	{identity.ErrMalformed, "malformed"},
	{identity.ErrUnknownKey, "unknown_key"},
	{identity.ErrBadSignature, "bad_signature"},
	{identity.ErrExpired, "expired"},
}

var keyIDPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)

// IdentityMiddleware verifies IdentityHeader and puts the verified name on
// the request context, where mcp-go passes it on to tool handlers in both
// protocol eras. Requests without the header pass through unchanged. A
// header that is present but does not verify, including an empty or repeated
// one, gets 400 so a misconfigured client fails loudly instead of claiming
// anonymously, and the rejection is logged. Not 401 or 403: Claude Code reads
// a 401 as an OAuth challenge and caches a 403 as "needs authentication",
// and both hide the reason. A request carrying the retired
// X-Artex-Conversation header
// gets 400, so a stale web chat process stops instead of moving its claims
// to another holder. Mount it after bearer auth.
func (s *Server) IdentityMiddleware(signer *identity.Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := r.Header[http.CanonicalHeaderKey(retiredConversationHeader)]; ok {
				http.Error(w, retiredConversationHeader+" is no longer supported: start a new web chat conversation", http.StatusBadRequest)
				return
			}
			values, present := r.Header[http.CanonicalHeaderKey(IdentityHeader)]
			if !present {
				next.ServeHTTP(w, r)
				return
			}
			if len(values) != 1 || values[0] == "" {
				slog.Warn("identity rejected", "reason", "header", "headers", len(values), "remote", r.RemoteAddr)
				http.Error(w, "invalid "+IdentityHeader+": want exactly one non-empty token", http.StatusBadRequest)
				return
			}
			claims, err := signer.Verify(values[0])
			if err != nil {
				for _, reject := range identityRejections {
					if errors.Is(err, reject.err) {
						name, keyID := claimedIdentity(values[0])
						slog.Warn("identity rejected", "reason", reject.reason, "name", name, "key_id", keyID, "remote", r.RemoteAddr)
						http.Error(w, "invalid "+IdentityHeader+": "+err.Error(), http.StatusBadRequest)
						return
					}
				}
				slog.Error("identity verification failed", "err", err)
				http.Error(w, "identity verification unavailable", http.StatusInternalServerError)
				return
			}
			s.identities.touch(claims.Name)
			next.ServeHTTP(w, r.WithContext(tools.WithIdentity(r.Context(), claims.Name)))
		})
	}
}

// claimedIdentity returns the unverified name and key ID of token for
// logging, each empty unless it has the shape a signer would issue, so a
// crafted payload cannot put arbitrary text in the log.
func claimedIdentity(token string) (name, keyID string) {
	c, err := identity.Decode(token)
	if err != nil {
		return "", ""
	}
	if identity.ValidateName(c.Name) == nil {
		name = c.Name
	}
	if keyIDPattern.MatchString(c.KeyID) {
		keyID = c.KeyID
	}
	return name, keyID
}

// identitySeen records when each verified identity last made a request, for
// list_sessions.
type identitySeen struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func (s *identitySeen) touch(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		s.last = map[string]time.Time{}
	}
	s.last[name] = time.Now()
}

type seenIdentity struct {
	Name       string
	LastActive time.Time
}

// active returns identities seen since cutoff, most recent first, and
// forgets older ones.
func (s *identitySeen) active(cutoff time.Time) []seenIdentity {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []seenIdentity{}
	for name, at := range s.last {
		if at.Before(cutoff) {
			delete(s.last, name)
			continue
		}
		out = append(out, seenIdentity{Name: name, LastActive: at})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastActive.Equal(out[j].LastActive) {
			return out[i].LastActive.After(out[j].LastActive)
		}
		return out[i].Name < out[j].Name
	})
	return out
}
