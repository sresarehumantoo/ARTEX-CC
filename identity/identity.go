// Package identity issues and verifies the signed client identity tokens
// carried in the X-Artex-Identity header. Tokens are signed with the data
// directory's identity.key, so every process serving that directory accepts
// the same tokens, and rotating the key revokes all of them.
package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/config"
)

const (
	DefaultTTL = 7 * 24 * time.Hour
	MaxTTL     = 30 * 24 * time.Hour

	prefix         = "ai1"
	domain         = "artex-identity-v1\x00"
	maxTokenLen    = 2048
	clockSkew      = 60 * time.Second
	reloadInterval = time.Second
)

var (
	ErrMalformed    = errors.New("malformed identity token")
	ErrBadSignature = errors.New("identity token signature mismatch")
	ErrExpired      = errors.New("identity token expired")
	ErrUnknownKey   = errors.New("identity token signed by an unknown key ID (rotated key or another data directory)")
	ErrInvalidName  = errors.New("identity name must match ^[a-z0-9][a-z0-9._/-]{0,63}$")
	ErrInvalidTTL   = errors.New("identity TTL must be positive and at most 30 days")
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]{0,63}$`)

var reservedPrefixes = []string{"chat/", "eval/"}

// ValidateName reports whether name is a usable identity name.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return nil
}

// Reserved reports whether name uses a prefix owned by a built-in issuer.
func Reserved(name string) bool {
	for _, p := range reservedPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// Claims is the verified content of a token.
type Claims struct {
	Name      string
	KeyID     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type payload struct {
	Name     string `json:"n"`
	KeyID    string `json:"k"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

// Signer issues and verifies tokens under the identity key of one data
// directory. It rereads the key file when its inode, mtime or size changes,
// checking at most once per second.
type Signer struct {
	path    string
	dataDir string
	now     func() time.Time

	mu        sync.Mutex
	key       []byte
	keyID     string
	info      os.FileInfo
	lastCheck time.Time
}

// NewSigner loads the identity key from dataDir, creating it if needed.
func NewSigner(dataDir string) (*Signer, error) {
	s := &Signer{
		path:    filepath.Join(dataDir, config.IdentityKeyFile),
		dataDir: dataDir,
		now:     time.Now,
	}
	if _, _, err := s.current(); err != nil {
		return nil, err
	}
	return s, nil
}

// KeyID returns the ID of key: the first 8 hex characters of its SHA-256.
func KeyID(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:4])
}

func (s *Signer) current() ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.key != nil && now.Sub(s.lastCheck) < reloadInterval {
		return s.key, s.keyID, nil
	}
	// The stat is taken before the read, so a change that lands in between
	// only causes one extra reload on the next check.
	info, err := os.Stat(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	if s.key != nil && info != nil && s.info != nil && os.SameFile(info, s.info) &&
		info.ModTime().Equal(s.info.ModTime()) && info.Size() == s.info.Size() {
		s.lastCheck = now
		return s.key, s.keyID, nil
	}
	key, err := config.IdentityKey(s.dataDir)
	if err != nil {
		return nil, "", err
	}
	s.key, s.keyID, s.info, s.lastCheck = key, KeyID(key), info, now
	return s.key, s.keyID, nil
}

// Issue returns a token for name that expires after ttl.
func (s *Signer) Issue(name string, ttl time.Duration) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if ttl <= 0 || ttl > MaxTTL {
		return "", fmt.Errorf("%w: %s", ErrInvalidTTL, ttl)
	}
	key, keyID, err := s.current()
	if err != nil {
		return "", err
	}
	now := s.now()
	body, err := json.Marshal(payload{
		Name:     name,
		KeyID:    keyID,
		IssuedAt: now.Unix(),
		Expires:  now.Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	p := base64.RawURLEncoding.EncodeToString(body)
	return prefix + "." + p + "." + base64.RawURLEncoding.EncodeToString(sign(key, p)), nil
}

// KeyID returns the ID of the signer's current key.
func (s *Signer) KeyID() (string, error) {
	_, keyID, err := s.current()
	return keyID, err
}

// Decode parses token without checking its signature or expiry. Its claims
// are unauthenticated and only fit for display; use Verify to trust them.
func Decode(token string) (Claims, error) {
	c, _, _, err := parse(token)
	return c, err
}

func parse(token string) (c Claims, signed string, mac []byte, err error) {
	if len(token) > maxTokenLen {
		return Claims{}, "", nil, fmt.Errorf("%w: longer than %d bytes", ErrMalformed, maxTokenLen)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return Claims{}, "", nil, fmt.Errorf("%w: want %s.<payload>.<mac>", ErrMalformed, prefix)
	}
	if parts[0] != prefix {
		return Claims{}, "", nil, fmt.Errorf("%w: unsupported version %q", ErrMalformed, parts[0])
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, "", nil, fmt.Errorf("%w: payload encoding", ErrMalformed)
	}
	mac, err = base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(mac) != sha256.Size {
		return Claims{}, "", nil, fmt.Errorf("%w: mac encoding", ErrMalformed)
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Claims{}, "", nil, fmt.Errorf("%w: payload", ErrMalformed)
	}
	c = Claims{Name: p.Name, KeyID: p.KeyID, IssuedAt: time.Unix(p.IssuedAt, 0), ExpiresAt: time.Unix(p.Expires, 0)}
	return c, parts[1], mac, nil
}

// Verify checks token and returns its claims. Errors wrap ErrMalformed,
// ErrUnknownKey, ErrBadSignature or ErrExpired.
func (s *Signer) Verify(token string) (Claims, error) {
	c, signed, mac, err := parse(token)
	if err != nil {
		return Claims{}, err
	}
	key, keyID, err := s.current()
	if err != nil {
		return Claims{}, err
	}
	if c.KeyID != keyID {
		return Claims{}, ErrUnknownKey
	}
	if !hmac.Equal(mac, sign(key, signed)) {
		return Claims{}, ErrBadSignature
	}
	if err := ValidateName(c.Name); err != nil {
		return Claims{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if s.now().After(c.ExpiresAt.Add(clockSkew)) {
		return Claims{}, ErrExpired
	}
	return c, nil
}

func sign(key []byte, payload string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(domain))
	m.Write([]byte(payload))
	return m.Sum(nil)
}
