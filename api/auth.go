package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	authFile       = "auth.json"
	sessionCookie  = "artex-session"
	sessionTTL     = 7 * 24 * time.Hour
	minPasswordLen = 8
)

type AuthConfig struct {
	PasswordHash string `json:"password_hash"`
	HMACKey      string `json:"hmac_key"`
}

func WithAuthRequired(required bool) Option {
	return func(a *API) { a.authRequired = required }
}

func (a *API) loadAuth() (AuthConfig, bool) {
	var cfg AuthConfig
	if a.files == nil {
		return cfg, false
	}
	if err := a.files.ReadJSON(authFile, &cfg); err != nil {
		return cfg, false
	}
	return cfg, cfg.PasswordHash != "" && cfg.HMACKey != ""
}

func signExpiry(key, expiry string) (string, error) {
	k, err := hex.DecodeString(key)
	if err != nil {
		return "", err
	}
	m := hmac.New(sha256.New, k)
	m.Write([]byte(expiry))
	return hex.EncodeToString(m.Sum(nil)), nil
}

func (a *API) setSession(w http.ResponseWriter, cfg AuthConfig) error {
	exp := time.Now().Add(sessionTTL)
	expiry := strconv.FormatInt(exp.Unix(), 10)
	sig, err := signExpiry(cfg.HMACKey, expiry)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    expiry + "." + sig,
		Path:     "/",
		Expires:  exp,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func validSession(r *http.Request, cfg AuthConfig) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	expiry, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || time.Now().Unix() >= unix {
		return false
	}
	want, err := signExpiry(cfg.HMACKey, expiry)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(want))
}

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.authRequired {
			next.ServeHTTP(w, r)
			return
		}
		switch r.URL.Path {
		case "/api/auth/status", "/api/auth/init", "/api/auth/login":
			next.ServeHTTP(w, r)
			return
		}
		cfg, ok := a.loadAuth()
		if !ok || !validSession(r, cfg) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		a.setSession(w, cfg) //nolint:errcheck
		next.ServeHTTP(w, r)
	})
}

func (a *API) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/status", a.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/init", a.handleAuthInit)
	mux.HandleFunc("POST /api/auth/login", a.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/change-password", a.handleAuthChangePassword)
	mux.HandleFunc("POST /api/auth/logout", a.handleAuthLogout)
}

func (a *API) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	_, initialized := a.loadAuth()
	writeJSON(w, http.StatusOK, map[string]bool{
		"initialized":   initialized,
		"auth_required": a.authRequired,
	})
}

type passwordBody struct {
	Password string `json:"password"`
}

func newAuthConfig(password string) (AuthConfig, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthConfig{}, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return AuthConfig{}, err
	}
	return AuthConfig{PasswordHash: string(hash), HMACKey: hex.EncodeToString(key)}, nil
}

func (a *API) handleAuthInit(w http.ResponseWriter, r *http.Request) {
	if a.files == nil {
		writeError(w, http.StatusServiceUnavailable, "data directory not configured")
		return
	}
	var body struct {
		Password   string `json:"password"`
		SetupToken string `json:"setup_token"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	if _, ok := a.loadAuth(); ok {
		writeError(w, http.StatusConflict, "already initialized")
		return
	}
	if a.authRequired && (a.setupToken == "" || subtle.ConstantTimeCompare([]byte(body.SetupToken), []byte(a.setupToken)) != 1) {
		writeError(w, http.StatusForbidden, "invalid setup token: the server prints it in its logs at startup while no password is set")
		return
	}
	if a.storeAuth(w, body.Password) {
		a.setupToken = ""
	}
}

// IssueSetupToken generates the one-time token that POST /api/auth/init
// requires, or returns "" when auth is off or a password is already set.
func (a *API) IssueSetupToken() (string, error) {
	if !a.authRequired || a.files == nil {
		return "", nil
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	if _, ok := a.loadAuth(); ok {
		return "", nil
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	a.setupToken = hex.EncodeToString(b)
	return a.setupToken, nil
}

func (a *API) storeAuth(w http.ResponseWriter, password string) bool {
	cfg, err := newAuthConfig(password)
	if err == nil {
		err = a.files.WriteJSON(authFile, cfg)
	}
	if err == nil {
		err = a.setSession(w, cfg)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return false
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	return true
}

func (a *API) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var body passwordBody
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg, ok := a.loadAuth()
	if !ok {
		writeError(w, http.StatusConflict, "not initialized")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(body.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}
	if err := a.setSession(w, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) handleAuthChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	cfg, ok := a.loadAuth()
	if !ok {
		writeError(w, http.StatusConflict, "not initialized")
		return
	}
	if a.authRequired && !validSession(r, cfg) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(body.CurrentPassword)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid current password")
		return
	}
	if len(body.NewPassword) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	a.storeAuth(w, body.NewPassword)
}

func (a *API) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
