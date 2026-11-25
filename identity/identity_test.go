package identity

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/config"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTestSigner(t *testing.T, dir string) (*Signer, *clock) {
	t.Helper()
	s, err := NewSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	s.now = c.now
	return s, c
}

func issue(t *testing.T, s *Signer, name string, ttl time.Duration) string {
	t.Helper()
	tok, err := s.Issue(name, ttl)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func writeKey(t *testing.T, dir string) {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	tmp := filepath.Join(dir, "rotate.tmp")
	if err := os.WriteFile(tmp, []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, config.IdentityKeyFile)); err != nil {
		t.Fatal(err)
	}
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	t.Parallel()
	s, c := newTestSigner(t, t.TempDir())
	tok := issue(t, s, "chat/abc-123", DefaultTTL)
	if !strings.HasPrefix(tok, "ai1.") {
		t.Fatalf("token %q lacks ai1 prefix", tok)
	}
	claims, err := s.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Name != "chat/abc-123" {
		t.Errorf("name %q", claims.Name)
	}
	if !claims.IssuedAt.Equal(c.now()) || !claims.ExpiresAt.Equal(c.now().Add(DefaultTTL)) {
		t.Errorf("times %v %v", claims.IssuedAt, claims.ExpiresAt)
	}
	key, err := config.IdentityKey(s.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if claims.KeyID != KeyID(key) || len(claims.KeyID) != 8 {
		t.Errorf("key ID %q, want %q", claims.KeyID, KeyID(key))
	}
}

func TestIdentityKeyIsNotLabelKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, _ := newTestSigner(t, dir)
	label, err := config.OwnerLabelKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if KeyID(label) == s.keyID {
		t.Error("signer uses the owner label key")
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	t.Parallel()
	s, _ := newTestSigner(t, t.TempDir())
	tok := issue(t, s, "alice", time.Hour)
	parts := strings.Split(tok, ".")

	flip := func(seg string) string {
		b := []byte(seg)
		if b[0] == 'A' {
			b[0] = 'B'
		} else {
			b[0] = 'A'
		}
		return string(b)
	}
	repayload := func(edit func(*payload)) string {
		body, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var p payload
		if err := json.Unmarshal(body, &p); err != nil {
			t.Fatal(err)
		}
		edit(&p)
		body, _ = json.Marshal(p)
		return base64.RawURLEncoding.EncodeToString(body)
	}
	otherKey := make([]byte, 32)
	_, _ = rand.Read(otherKey)

	cases := []struct {
		name  string
		token string
		want  error
	}{
		{"empty", "", ErrMalformed},
		{"truncated", tok[:len(tok)-10], ErrMalformed},
		{"two segments", parts[0] + "." + parts[1], ErrMalformed},
		{"four segments", tok + ".x", ErrMalformed},
		{"empty payload", "ai1.." + parts[2], ErrMalformed},
		{"empty mac", "ai1." + parts[1] + ".", ErrMalformed},
		{"wrong prefix", "ai0." + parts[1] + "." + parts[2], ErrMalformed},
		{"future version", "ai2." + parts[1] + "." + parts[2], ErrMalformed},
		{"oversized", "ai1." + strings.Repeat("A", 2100) + "." + parts[2], ErrMalformed},
		{"payload not base64", "ai1.!!!." + parts[2], ErrMalformed},
		{"payload not json", "ai1." + base64.RawURLEncoding.EncodeToString([]byte("nope")) + "." + parts[2], ErrMalformed},
		{"flipped mac", parts[0] + "." + parts[1] + "." + flip(parts[2]), ErrBadSignature},
		{"flipped payload", parts[0] + "." + flip(parts[1]) + "." + parts[2], ErrMalformed},
		{"edited name", "ai1." + repayload(func(p *payload) { p.Name = "bob" }) + "." + parts[2], ErrBadSignature},
		{"edited exp", "ai1." + repayload(func(p *payload) { p.Expires += 3600 }) + "." + parts[2], ErrBadSignature},
		{"mac from another key", "ai1." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(sign(otherKey, parts[1])), ErrBadSignature},
		{"other key ID", "ai1." + repayload(func(p *payload) { p.KeyID = "00000000" }) + "." + parts[2], ErrUnknownKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Verify(tc.token); !errors.Is(err, tc.want) {
				t.Errorf("err %v, want %v", err, tc.want)
			}
		})
	}
}

func TestVerifyRejectsFlippedPayloadByteWithValidEncoding(t *testing.T) {
	t.Parallel()
	s, _ := newTestSigner(t, t.TempDir())
	tok := issue(t, s, "alice", time.Hour)
	parts := strings.Split(tok, ".")
	body, _ := base64.RawURLEncoding.DecodeString(parts[1])
	body = []byte(strings.Replace(string(body), `"alice"`, `"alicf"`, 1))
	forged := "ai1." + base64.RawURLEncoding.EncodeToString(body) + "." + parts[2]
	if _, err := s.Verify(forged); !errors.Is(err, ErrBadSignature) {
		t.Errorf("err %v, want ErrBadSignature", err)
	}
}

func TestVerifyExpiry(t *testing.T) {
	t.Parallel()
	s, c := newTestSigner(t, t.TempDir())
	tok := issue(t, s, "alice", time.Hour)
	c.advance(time.Hour + 30*time.Second)
	if _, err := s.Verify(tok); err != nil {
		t.Errorf("within skew: %v", err)
	}
	c.advance(31 * time.Second)
	if _, err := s.Verify(tok); !errors.Is(err, ErrExpired) {
		t.Errorf("past skew: err %v, want ErrExpired", err)
	}
}

func TestVerifyRejectsOtherDataDir(t *testing.T) {
	t.Parallel()
	a, _ := newTestSigner(t, t.TempDir())
	b, _ := newTestSigner(t, t.TempDir())
	tok := issue(t, a, "alice", time.Hour)
	if _, err := b.Verify(tok); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("err %v, want ErrUnknownKey", err)
	}
}

func TestSignersShareDataDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, _ := newTestSigner(t, dir)
	b, _ := newTestSigner(t, dir)
	claims, err := b.Verify(issue(t, a, "alice", time.Hour))
	if err != nil || claims.Name != "alice" {
		t.Errorf("claims %+v err %v", claims, err)
	}
}

func TestRotationReload(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, c := newTestSigner(t, dir)
	old := issue(t, s, "alice", time.Hour)
	oldID := s.keyID

	writeKey(t, dir)
	if _, err := s.Verify(old); err != nil {
		t.Errorf("reload happened inside the check interval: %v", err)
	}
	c.advance(time.Second)
	if _, err := s.Verify(old); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("old token after rotation: err %v, want ErrUnknownKey", err)
	}
	fresh := issue(t, s, "alice", time.Hour)
	claims, err := s.Verify(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if claims.KeyID == oldID {
		t.Error("new token still uses the old key ID")
	}
	other, _ := newTestSigner(t, dir)
	if _, err := other.Verify(fresh); err != nil {
		t.Errorf("fresh signer on rotated dir: %v", err)
	}
}

func TestReloadRecreatesDeletedKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, c := newTestSigner(t, dir)
	old := issue(t, s, "alice", time.Hour)
	if err := os.Remove(filepath.Join(dir, config.IdentityKeyFile)); err != nil {
		t.Fatal(err)
	}
	c.advance(time.Second)
	if _, err := s.Verify(old); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("err %v, want ErrUnknownKey", err)
	}
	if _, err := os.Stat(filepath.Join(dir, config.IdentityKeyFile)); err != nil {
		t.Errorf("key not recreated: %v", err)
	}
}

func TestReloadRejectsCorruptKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, c := newTestSigner(t, dir)
	tok := issue(t, s, "alice", time.Hour)
	if err := os.WriteFile(filepath.Join(dir, config.IdentityKeyFile), []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.advance(time.Second)
	if _, err := s.Verify(tok); err == nil {
		t.Error("corrupt key file accepted on reload")
	}
}

func TestIssueTTLBounds(t *testing.T) {
	t.Parallel()
	s, _ := newTestSigner(t, t.TempDir())
	for _, ttl := range []time.Duration{0, -time.Second, MaxTTL + time.Second} {
		if _, err := s.Issue("alice", ttl); !errors.Is(err, ErrInvalidTTL) {
			t.Errorf("ttl %s: err %v, want ErrInvalidTTL", ttl, err)
		}
	}
	for _, ttl := range []time.Duration{time.Second, DefaultTTL, MaxTTL} {
		if _, err := s.Issue("alice", ttl); err != nil {
			t.Errorf("ttl %s: %v", ttl, err)
		}
	}
}

func TestValidateName(t *testing.T) {
	t.Parallel()
	good := []string{"a", "0", "alice", "eval/run-1/i3", "chat/0f3a", "a.b_c-d/e", "a" + strings.Repeat("b", 63)}
	bad := []string{"", "Alice", "-a", ".a", "/a", "_a", "a b", "a:b", "a\n", "é", "a" + strings.Repeat("b", 64)}
	for _, n := range good {
		if err := ValidateName(n); err != nil {
			t.Errorf("%q rejected: %v", n, err)
		}
	}
	for _, n := range bad {
		if err := ValidateName(n); !errors.Is(err, ErrInvalidName) {
			t.Errorf("%q: err %v, want ErrInvalidName", n, err)
		}
	}
	s, _ := newTestSigner(t, t.TempDir())
	if _, err := s.Issue("Bad Name", time.Hour); !errors.Is(err, ErrInvalidName) {
		t.Errorf("Issue accepted an invalid name: %v", err)
	}
}

func TestReserved(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"chat/1":   true,
		"eval/r/i": true,
		"chat":     false,
		"chatty/1": false,
		"alice":    false,
	} {
		if got := Reserved(name); got != want {
			t.Errorf("Reserved(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestConcurrentFirstStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const n = 8
	signers := make([]*Signer, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signers[i], errs[i] = NewSigner(dir)
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
	}
	tok, err := signers[0].Issue("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for i := range n {
		if _, err := signers[i].Verify(tok); err != nil {
			t.Fatalf("signer %d: %v", i, err)
		}
	}
}

func TestConcurrentVerifyDuringRotation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, c := newTestSigner(t, dir)
	tok := issue(t, s, "alice", time.Hour)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_, _ = s.Verify(tok)
				c.advance(100 * time.Millisecond)
			}
		}()
	}
	writeKey(t, dir)
	wg.Wait()
	c.advance(time.Second)
	if _, err := s.Verify(tok); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("err %v, want ErrUnknownKey", err)
	}
}

func TestDecodeSkipsSignatureAndExpiry(t *testing.T) {
	t.Parallel()
	s, c := newTestSigner(t, t.TempDir())
	tok := issue(t, s, "alice", time.Hour)
	c.advance(2 * time.Hour)
	if _, err := s.Verify(tok); !errors.Is(err, ErrExpired) {
		t.Fatalf("err %v, want ErrExpired", err)
	}
	parts := strings.Split(tok, ".")
	forged := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	got, err := Decode(forged)
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := s.KeyID()
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "alice" || got.KeyID != keyID || !got.ExpiresAt.Equal(got.IssuedAt.Add(time.Hour)) {
		t.Errorf("claims %+v", got)
	}
	if _, err := Decode("ai1.x"); !errors.Is(err, ErrMalformed) {
		t.Errorf("err %v, want ErrMalformed", err)
	}
}
