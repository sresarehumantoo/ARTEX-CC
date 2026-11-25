package mcpserver

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// ConversationHeader carries a web chat conversation's identity on its MCP
// requests, as "<conversation id>.<mac>". Only tokens minted by the
// ConversationSigner of the running server verify.
const ConversationHeader = "X-Artex-Conversation"

// ConversationSigner mints and verifies conversation tokens. Its key lives
// only in memory, so holding the server's MCP bearer token is not enough to
// claim as another conversation.
type ConversationSigner struct {
	key []byte
}

func NewConversationSigner() (*ConversationSigner, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return &ConversationSigner{key: key}, nil
}

func (s *ConversationSigner) mac(id string) string {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(id))
	return hex.EncodeToString(h.Sum(nil))
}

// Token returns the ConversationHeader value for conversation id.
func (s *ConversationSigner) Token(id string) string {
	return id + "." + s.mac(id)
}

func (s *ConversationSigner) verify(token string) (string, bool) {
	i := strings.LastIndexByte(token, '.')
	if i <= 0 {
		return "", false
	}
	id, mac := token[:i], token[i+1:]
	return id, hmac.Equal([]byte(mac), []byte(s.mac(id)))
}

// Middleware rejects requests whose ConversationHeader does not verify and
// tags verified ones so their tool calls claim as that conversation when the
// transport gives them no MCP session.
func (s *ConversationSigner) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(ConversationHeader)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, ok := s.verify(token)
		if !ok {
			http.Error(w, "invalid "+ConversationHeader, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), conversationKey{}, id)))
	})
}

type conversationKey struct{}

func conversationOwner(ctx context.Context) string {
	if id, _ := ctx.Value(conversationKey{}).(string); id != "" {
		return "conversation:" + id
	}
	return ""
}
