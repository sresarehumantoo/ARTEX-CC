package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func authDo(h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "localhost:9400"
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func initBody(t *testing.T, a *API, password string) string {
	t.Helper()
	token, err := a.IssueSetupToken()
	if err != nil || token == "" {
		t.Fatalf("setup token %q: %v", token, err)
	}
	return `{"password":"` + password + `","setup_token":"` + token + `"}`
}

func sessionFrom(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func TestAuthFlow(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()), WithAuthRequired(true))
	h := a.Handler()

	if w := authDo(h, "GET", "/api/engagement", ""); w.Code != 401 {
		t.Fatalf("unauthenticated: %d", w.Code)
	}
	if w := authDo(h, "POST", "/api/auth/init", `{"password":"short"}`); w.Code != 400 {
		t.Fatalf("short password: %d", w.Code)
	}
	w := authDo(h, "POST", "/api/auth/init", initBody(t, a, "correct horse"))
	if w.Code != 200 {
		t.Fatalf("init: %d", w.Code)
	}
	cookie := sessionFrom(t, w)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags: %+v", cookie)
	}
	if w := authDo(h, "POST", "/api/auth/init", `{"password":"another one"}`); w.Code != 409 {
		t.Fatalf("re-init: %d", w.Code)
	}
	if w := authDo(h, "POST", "/api/auth/login", `{"password":"wrong password"}`); w.Code != 401 {
		t.Fatalf("bad login: %d", w.Code)
	}
	if w := authDo(h, "POST", "/api/auth/login", `{"password":"correct horse"}`); w.Code != 200 {
		t.Fatalf("login: %d", w.Code)
	}
	if w := authDo(h, "GET", "/api/auth/status", ""); !strings.Contains(w.Body.String(), `"initialized":true`) {
		t.Fatalf("status: %s", w.Body.String())
	}

	tampered := &http.Cookie{Name: sessionCookie, Value: strings.Replace(cookie.Value, ".", "0.", 1)}
	if w := authDo(h, "POST", "/api/auth/change-password", `{"current_password":"correct horse","new_password":"battery staple"}`, tampered); w.Code != 401 {
		t.Fatalf("tampered cookie: %d", w.Code)
	}
	if w := authDo(h, "POST", "/api/auth/change-password", `{"current_password":"nope nope","new_password":"battery staple"}`, cookie); w.Code != 401 {
		t.Fatalf("wrong current: %d", w.Code)
	}
	if w := authDo(h, "POST", "/api/auth/change-password", `{"current_password":"correct horse","new_password":"battery staple"}`, cookie); w.Code != 200 {
		t.Fatalf("change: %d", w.Code)
	}
	if w := authDo(h, "POST", "/api/auth/login", `{"password":"battery staple"}`); w.Code != 200 {
		t.Fatalf("login new: %d", w.Code)
	}
}

func TestAuthNotRequiredPassesThrough(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	w := authDo(a.Handler(), "GET", "/api/auth/status", "")
	if !strings.Contains(w.Body.String(), `"auth_required":false`) {
		t.Fatalf("status: %s", w.Body.String())
	}
}

func TestAuthLogout(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()), WithAuthRequired(true))
	h := a.Handler()

	w := authDo(h, "POST", "/api/auth/init", initBody(t, a, "correct horse"))
	if w.Code != 200 {
		t.Fatalf("init: %d", w.Code)
	}
	cookie := sessionFrom(t, w)

	w = authDo(h, "POST", "/api/auth/logout", "", cookie)
	if w.Code != 200 {
		t.Fatalf("logout: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("body: %s", w.Body.String())
	}
	var cleared *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			cleared = c
		}
	}
	if cleared == nil || cleared.MaxAge != -1 || cleared.Value != "" {
		t.Fatalf("session cookie not cleared: %+v", cleared)
	}
}

func TestAuthLogout_NoAuth(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	w := authDo(h, "POST", "/api/auth/logout", "")
	if w.Code != 200 {
		t.Fatalf("logout without auth: %d", w.Code)
	}
}

func TestAuthChangePassword_ShortNew(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()), WithAuthRequired(true))
	h := a.Handler()
	w := authDo(h, "POST", "/api/auth/init", initBody(t, a, "correct horse"))
	cookie := sessionFrom(t, w)
	w = authDo(h, "POST", "/api/auth/change-password", `{"current_password":"correct horse","new_password":"short"}`, cookie)
	if w.Code != 400 {
		t.Fatalf("short new password: %d", w.Code)
	}
}

func TestAuthChangePassword_BadBody(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()), WithAuthRequired(true))
	h := a.Handler()
	w := authDo(h, "POST", "/api/auth/init", initBody(t, a, "correct horse"))
	cookie := sessionFrom(t, w)
	w = authDo(h, "POST", "/api/auth/change-password", "{invalid", cookie)
	if w.Code != 400 {
		t.Fatalf("bad body: %d", w.Code)
	}
}

func TestAuthChangePassword_NotInitialized(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	w := authDo(h, "POST", "/api/auth/change-password", `{"current_password":"x","new_password":"longpassword"}`)
	if w.Code != 409 {
		t.Fatalf("not initialized: %d", w.Code)
	}
}

func TestAuthLogin_BadBody(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	w := authDo(h, "POST", "/api/auth/login", "{invalid")
	if w.Code != 400 {
		t.Fatalf("bad body: %d", w.Code)
	}
}

func TestAuthLogin_NotInitialized(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	w := authDo(h, "POST", "/api/auth/login", `{"password":"anything"}`)
	if w.Code != 409 {
		t.Fatalf("not initialized: %d", w.Code)
	}
}

func TestAuthInitSetupToken(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()), WithAuthRequired(true))
	h := a.Handler()
	if w := authDo(h, "POST", "/api/auth/init", `{"password":"correct horse"}`); w.Code != http.StatusForbidden {
		t.Fatalf("no token issued: %d", w.Code)
	}
	token, err := a.IssueSetupToken()
	if err != nil || len(token) != 32 {
		t.Fatalf("token %q: %v", token, err)
	}
	for _, body := range []string{
		`{"password":"correct horse"}`,
		`{"password":"correct horse","setup_token":""}`,
		`{"password":"correct horse","setup_token":"` + strings.Repeat("0", 32) + `"}`,
		`{"password":"correct horse","setup_token":"` + token[:31] + `"}`,
	} {
		if w := authDo(h, "POST", "/api/auth/init", body); w.Code != http.StatusForbidden {
			t.Fatalf("%s: %d", body, w.Code)
		}
	}
	if w := authDo(h, "GET", "/api/auth/status", ""); !strings.Contains(w.Body.String(), `"initialized":false`) {
		t.Fatalf("status: %s", w.Body.String())
	}

	r := httptest.NewRequest("POST", "/api/auth/init", strings.NewReader(`{"password":"correct horse","setup_token":"`+token+`"}`))
	r.Host = "artex.example:9400"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("right token from a remote Host: %d %s", w.Code, w.Body)
	}
	if a.setupToken != "" {
		t.Fatal("setup token still active after init")
	}
	if tok, err := a.IssueSetupToken(); err != nil || tok != "" {
		t.Fatalf("token issued after init: %q %v", tok, err)
	}

	if err := a.files.Delete(authFile); err != nil {
		t.Fatal(err)
	}
	if w := authDo(h, "POST", "/api/auth/init", `{"password":"another one","setup_token":"`+token+`"}`); w.Code != http.StatusForbidden {
		t.Fatalf("reused token: %d", w.Code)
	}
}
