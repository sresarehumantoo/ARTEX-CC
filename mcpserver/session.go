package mcpserver

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session holds per-client state for one MCP connection. Each client that
// initializes a session gets its own Session instance. The Context key-value
// store lets tool handlers maintain client-specific state across calls.
type Session struct {
	ID        string `json:"-"`
	CreatedAt time.Time

	mu         sync.Mutex
	lastActive time.Time
	context    map[string]string
}

func newSession(id string) *Session {
	now := time.Now()
	return &Session{
		ID:         id,
		CreatedAt:  now,
		lastActive: now,
		context:    make(map[string]string),
	}
}

// Get returns a value from the session's key-value context.
func (s *Session) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.context[key]
}

// Set stores a value in the session's key-value context.
func (s *Session) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.context[key] = value
}

func (s *Session) snapshot() SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := make(map[string]string, len(s.context))
	for k, v := range s.context {
		ctx[k] = v
	}
	return SessionInfo{
		ID:         s.ID,
		CreatedAt:  s.CreatedAt,
		LastActive: s.lastActive,
		Context:    ctx,
	}
}

// SessionInfo is the serializable snapshot of a Session.
type SessionInfo struct {
	ID         string            `json:"-"`
	CreatedAt  time.Time         `json:"created_at"`
	LastActive time.Time         `json:"last_active"`
	Context    map[string]string `json:"context"`
}

// SessionManager tracks all active client sessions. It is safe for
// concurrent use from multiple goroutines.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionManager returns an initialized SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]*Session)}
}

// Create allocates a new session with a random ID.
func (m *SessionManager) Create() *Session {
	id := randomID()
	s := newSession(id)
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s
}

// GetOrCreate returns the session for id, creating one if it does not exist.
// This is the primary path for mcp-go integration: the transport allocates
// session IDs, and we shadow them with our own Session objects.
func (m *SessionManager) GetOrCreate(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		return s
	}
	s := newSession(id)
	m.sessions[id] = s
	return s
}

// Get returns the session for id, or nil if not found.
func (m *SessionManager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// Remove deletes a session. Returns true if the session existed.
func (m *SessionManager) Remove(id string) bool {
	m.mu.Lock()
	_, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	return ok
}

// List returns a snapshot of all active sessions.
func (m *SessionManager) List() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s.snapshot())
	}
	return out
}

// Count returns the number of active sessions.
func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
