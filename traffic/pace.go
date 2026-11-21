package traffic

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	mproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
)

const paceWaitMax = 30 * time.Second

const paceUser = "artex-pace"

func isPaceMarker(proxyAuth string) bool {
	if proxyAuth == "" {
		return false
	}
	const prefix = "Basic "
	if !strings.HasPrefix(proxyAuth, prefix) {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(proxyAuth[len(prefix):])
	if err != nil {
		return false
	}
	user, _, _ := strings.Cut(string(decoded), ":")
	return user == paceUser
}

func (s *sink) Requestheaders(f *mproxy.Flow) {
	if f.Request == nil || f.ConnContext == nil || f.ConnContext.ClientConn == nil {
		return
	}
	id := f.ConnContext.ClientConn.Id
	if isPaceMarker(f.Request.Header.Get("Proxy-Authorization")) {
		s.paced.Store(id, struct{}{})
	}
	f.Request.Header.Del("Proxy-Authorization")
	if f.Request.Method == http.MethodConnect || f.Request.URL == nil {
		return
	}
}

func (s *sink) ClientDisconnected(c *mproxy.ClientConn) {
	if c != nil {
		s.paced.Delete(c.Id)
	}
	if s.t != nil {
		s.t.snaps.requestSweep()
	}
}
