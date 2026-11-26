package store

import "testing"

func TestRedactDSN(t *testing.T) {
	for _, tt := range []struct {
		name, in, want string
	}{
		{"url password", "postgres://artex:s3cret@db:5432/ccartex?sslmode=disable", "postgres://artex:***@db:5432/ccartex?sslmode=disable"},
		{"url no password", "postgres://localhost/ccartex", "postgres://localhost/ccartex"},
		{"url user only", "postgres://artex@localhost/ccartex", "postgres://artex@localhost/ccartex"},
		{"url encoded password", "postgres://artex:p%40ss%2Fw0rd@db/ccartex", "postgres://artex:***@db/ccartex"},
		{"url raw slash in password", "postgres://artex:pa/ss@db/ccartex", "postgres://artex:***@db/ccartex"},
		{"url query password", "postgresql://artex@db/ccartex?sslmode=disable&password=s3cret", "postgresql://artex@db/ccartex?sslmode=disable&password=***"},
		{"kv password", "host=db user=artex password=s3cret dbname=ccartex", "host=db user=artex password=*** dbname=ccartex"},
		{"kv quoted password", "host=db password='s3 cr\\'et' dbname=ccartex", "host=db password=*** dbname=ccartex"},
		{"kv spaced password", "host=db password = s3cret dbname=ccartex", "host=db password = *** dbname=ccartex"},
		{"kv no password", "dbname=ccartex sslmode=disable", "dbname=ccartex sslmode=disable"},
		{"empty", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactDSN(tt.in); got != tt.want {
				t.Errorf("RedactDSN(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
