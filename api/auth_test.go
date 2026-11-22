package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func authDo(h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
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
	w := authDo(h, "POST", "/api/auth/init", `{"password":"correct horse"}`)
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
