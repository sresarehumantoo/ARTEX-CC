package mcpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/config"
	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
)

func signedToken(key []byte, p map[string]any) string {
	body, _ := json.Marshal(p)
	payload := base64.RawURLEncoding.EncodeToString(body)
	m := hmac.New(sha256.New, key)
	m.Write([]byte("artex-identity-v1\x00"))
	m.Write([]byte(payload))
	return "ai1." + payload + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func TestIdentityRejectionLogged(t *testing.T) {
	dir := t.TempDir()
	signer, err := identity.NewSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	key, err := config.IdentityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	kid := identity.KeyID(key)
	valid, err := signer.Issue("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid, ".")
	foreignSigner, err := identity.NewSigner(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	foreign, _ := foreignSigner.Issue("alice", time.Hour)
	foreignKID, _ := foreignSigner.KeyID()
	now := time.Now()
	expired := signedToken(key, map[string]any{"n": "alice", "k": kid, "iat": now.Add(-2 * time.Hour).Unix(), "exp": now.Add(-time.Hour).Unix()})
	otherKey := make([]byte, 32)
	forged := signedToken(otherKey, map[string]any{"n": "alice", "k": kid, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	badName := signedToken(key, map[string]any{"n": "Evil " + valid, "k": "x\nlevel=ERROR", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})

	cases := []struct {
		name, token, reason, wantName, wantKID string
	}{
		{"expired", expired, "expired", "alice", kid},
		{"unknown key", foreign, "unknown_key", "alice", foreignKID},
		{"bad signature", forged, "bad_signature", "alice", kid},
		{"malformed", parts[0] + "." + parts[1], "malformed", `""`, `""`},
		{"invalid claimed name", badName, "unknown_key", `""`, `""`},
	}
	h := (&Server{}).IdentityMiddleware(signer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler ran for a rejected identity")
	}))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureDefaultLog(t)
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Header.Set(IdentityHeader, tc.token)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("HTTP %d, want 400", rec.Code)
			}
			out := buf.String()
			if n := strings.Count(out, "\n"); n != 1 {
				t.Fatalf("%d log lines, want 1: %s", n, out)
			}
			for _, want := range []string{"level=WARN", `msg="identity rejected"`, "reason=" + tc.reason, "name=" + tc.wantName + " ", "key_id=" + tc.wantKID + " "} {
				if !strings.Contains(out, want) {
					t.Errorf("log %q lacks %q", out, want)
				}
			}
			tokenParts := strings.Split(tc.token, ".")
			for _, secret := range append([]string{tc.token, valid}, tokenParts[1:]...) {
				if strings.Contains(out, secret) {
					t.Errorf("log %q contains token material %q", out, secret)
				}
			}
		})
	}

	buf := captureDefaultLog(t)
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header[IdentityHeader] = []string{valid, valid}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if out := buf.String(); rec.Code != http.StatusBadRequest || strings.Count(out, "\n") != 1 || !strings.Contains(out, "reason=header") || strings.Contains(out, parts[2]) {
		t.Errorf("repeated header: HTTP %d, log %q", rec.Code, out)
	}
}
