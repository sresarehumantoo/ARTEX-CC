package store

import (
	"regexp"
	"strings"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

const redactedPassword = "***"

var (
	urlPasswordParam = regexp.MustCompile(`([?&](?:ssl)?password=)[^&#]*`)
	kvPassword       = regexp.MustCompile(`(?i)(\b(?:ssl)?password\s*=\s*)('(?:\\.|[^'\\])*'|\S+)`)
)

// RedactDSN masks the password and sslpassword in a PostgreSQL connection
// string, in either URL form (userinfo, ?password= or ?sslpassword=) or
// key=value form, keeping the rest intact.
func RedactDSN(dsn string) string {
	start, _ := tools.URLAuthority(dsn)
	if start < 0 {
		return kvPassword.ReplaceAllString(dsn, "${1}"+redactedPassword)
	}
	if stripped, userinfo, found := tools.SplitURLUserinfo(dsn); found {
		if user, _, hasPass := strings.Cut(userinfo, ":"); hasPass {
			dsn = stripped[:start] + user + ":" + redactedPassword + "@" + stripped[start:]
		}
	}
	return urlPasswordParam.ReplaceAllString(dsn, "${1}"+redactedPassword)
}
