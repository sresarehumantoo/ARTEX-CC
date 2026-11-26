package tools

import (
	"strings"
	"unicode"
)

// URLAuthority returns the bounds of the authority in raw, which starts after
// "scheme://" or a leading "//". start is -1 when raw has no authority.
func URLAuthority(raw string) (start, end int) {
	switch {
	case strings.HasPrefix(raw, "//"):
		start = 2
	default:
		i := strings.Index(raw, "://")
		if i < 0 || strings.ContainsAny(raw[:i], "/?#@") {
			return -1, -1
		}
		start = i + 3
	}
	end = len(raw)
	if j := strings.IndexAny(raw[start:], "/?#"); j >= 0 {
		end = start + j
	}
	return start, end
}

// SplitURLUserinfo removes the userinfo from raw and returns it separately.
// A password may contain a raw /, ? or #, so when the authority holds no @ the
// text up to the last @ is still taken as userinfo if it reads as
// user:password and not as host:port followed by a path.
func SplitURLUserinfo(raw string) (stripped, userinfo string, found bool) {
	start, end := URLAuthority(raw)
	if start < 0 {
		return raw, "", false
	}
	at := strings.LastIndex(raw[start:end], "@")
	if at < 0 {
		at = delimitedUserinfoEnd(raw[start:])
		if at < 0 {
			return raw, "", false
		}
	}
	return raw[:start] + raw[start+at+1:], raw[start : start+at], true
}

// RedactURLUserinfo returns raw without its userinfo.
func RedactURLUserinfo(raw string) string {
	stripped, _, _ := SplitURLUserinfo(raw)
	return stripped
}

func delimitedUserinfoEnd(rest string) int {
	if i := strings.IndexFunc(rest, unicode.IsSpace); i >= 0 {
		rest = rest[:i]
	}
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return -1
	}
	user, pass, ok := strings.Cut(rest[:at], ":")
	if !ok || strings.ContainsAny(user, "/?#@") || isPortPrefix(pass) {
		return -1
	}
	return at
}

func isPortPrefix(s string) bool {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n > 0 && (n == len(s) || strings.IndexByte("/?#", s[n]) >= 0)
}
