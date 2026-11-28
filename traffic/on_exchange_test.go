package traffic

import (
	"net/http"
	"net/url"
	"testing"
)

func TestOnExchangeGetsRecordedHost(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	var hosts []string
	tr.OnExchange(func(host string) { hosts = append(hosts, host) })

	u, _ := url.Parse("https://App.Example.test:8443/login")
	tr.recordViews(requestView{method: "GET", url: u, proto: "HTTP/1.1", header: http.Header{}},
		responseView{status: 200, header: http.Header{}}, nil)
	if len(hosts) != 1 || hosts[0] != "App.Example.test" {
		t.Fatalf("hook saw %v, want the recorded host", hosts)
	}

	tr.OnExchange(nil)
	tr.recordViews(requestView{method: "GET", url: u, proto: "HTTP/1.1", header: http.Header{}},
		responseView{status: 200, header: http.Header{}}, nil)
	if len(hosts) != 1 {
		t.Errorf("cleared hook still called: %v", hosts)
	}
}
