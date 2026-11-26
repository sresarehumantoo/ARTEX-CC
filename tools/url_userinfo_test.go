package tools

import "testing"

func TestSplitURLUserinfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, stripped, userinfo string
		found                  bool
	}{
		{"https://user:pass@host/x", "https://host/x", "user:pass", true},
		{"https://user@host", "https://host", "user", true},
		{"//u:p@host/x", "//host/x", "u:p", true},
		{"http://u:p@host:abc/", "http://host:abc/", "u:p", true},
		{"https://user:pa/ss@host/", "https://host/", "user:pa/ss", true},
		{"https://user:p#ss@host/", "https://host/", "user:p#ss", true},
		{"https://user:p?s@s@host/a", "https://host/a", "user:p?s@s", true},
		{"https://user:p/s@host/a b@c", "https://host/a b@c", "user:p/s", true},
		{"http://host:8080/path@x", "http://host:8080/path@x", "", false},
		{"http://host:8080?q@x", "http://host:8080?q@x", "", false},
		{"https://host/users/@me", "https://host/users/@me", "", false},
		{"https://host/a:b@c", "https://host/a:b@c", "", false},
		{"https://host/?next=http://u:p@other/", "https://host/?next=http://u:p@other/", "", false},
		{"example.com/?u=http://u:p@other/", "example.com/?u=http://u:p@other/", "", false},
		{"mailto:user@example.com", "mailto:user@example.com", "", false},
		{"", "", "", false},
	}
	for _, tt := range tests {
		stripped, userinfo, found := SplitURLUserinfo(tt.in)
		if stripped != tt.stripped || userinfo != tt.userinfo || found != tt.found {
			t.Errorf("SplitURLUserinfo(%q) = %q, %q, %v, want %q, %q, %v",
				tt.in, stripped, userinfo, found, tt.stripped, tt.userinfo, tt.found)
		}
	}
}
