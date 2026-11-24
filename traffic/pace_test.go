package traffic

import (
	"encoding/base64"
	"testing"
)

func TestIsPaceMarker(t *testing.T) {
	t.Parallel()

	encode := func(user, pass string) string {
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	}

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", "", false},
		{"no_basic_prefix", "Bearer token123", false},
		{"invalid_base64", "Basic !!!not-base64!!!", false},
		{"correct_user", encode(paceUser, "anypass"), true},
		{"correct_user_empty_pass", encode(paceUser, ""), true},
		{"wrong_user", encode("someone-else", "pass"), false},
		{"no_colon", "Basic " + base64.StdEncoding.EncodeToString([]byte("nocolon")), false},
		{"empty_user", encode("", "pass"), false},
		{"case_sensitive", encode("Artex-Pace", "pass"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isPaceMarker(tc.input)
			if got != tc.want {
				t.Fatalf("isPaceMarker(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
