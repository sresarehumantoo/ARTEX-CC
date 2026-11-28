package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// SecurityHeaders sets the response headers that are safe on every response,
// API and web UI alike.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// sameOriginWrites refuses state-changing requests sent by a browser from
// another origin. Handlers decode JSON whatever the Content-Type, so a
// text/plain form POST needs no preflight and must be stopped here. Requests
// without Origin or Sec-Fetch-Site come from non-browser clients and pass. An
// Origin whose host is in allowed passes even when it differs from Host, for
// a reverse proxy that rewrites Host.
func sameOriginWrites(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if !sameOrigin(r, allowed) {
				writeError(w, http.StatusForbidden, "cross-origin request refused")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request, allowed []string) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "cross-site", "same-site":
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && (strings.EqualFold(u.Host, r.Host) || HostAllowed(u.Host, allowed))
}

// HostAllowed reports whether hostport matches an entry of allowed, ignoring
// case. An entry without a port matches its host on any port; an entry with
// a port matches only that port.
func HostAllowed(hostport string, allowed []string) bool {
	host, port := splitHostPort(hostport)
	for _, a := range allowed {
		ah, ap := splitHostPort(a)
		if strings.EqualFold(ah, host) && (ap == "" || ap == port) {
			return true
		}
	}
	return false
}

func splitHostPort(s string) (host, port string) {
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	return strings.Trim(s, "[]"), ""
}

// LoopbackHost reports whether r's Host header is localhost or a loopback IP.
func LoopbackHost(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
