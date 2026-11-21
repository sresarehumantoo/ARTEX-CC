package ingest

import (
	"strings"
	"testing"
)

func TestParseNmapXML(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<nmaprun>
  <host>
    <address addr="10.10.10.1" addrtype="ipv4"/>
    <hostnames>
      <hostname name="target.htb" type="user"/>
    </hostnames>
    <ports>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh" product="OpenSSH" version="8.9"/>
      </port>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http" product="nginx"/>
      </port>
      <port protocol="tcp" portid="443">
        <state state="open"/>
        <service name="https"/>
      </port>
      <port protocol="tcp" portid="3306">
        <state state="filtered"/>
        <service name="mysql"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	r, err := ParseNmap(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.Assets) == 0 {
		t.Fatal("expected assets, got none")
	}

	var hasIP, hasHostname, hasHTTP, hasHTTPS, hasSSH bool
	for _, a := range r.Assets {
		switch {
		case a.Type == "ip" && a.IP == "10.10.10.1":
			hasIP = true
		case a.Type == "subdomain" && a.Domain == "target.htb":
			hasHostname = true
		case a.Type == "service" && a.Port == 80 && strings.HasPrefix(a.URL, "http://"):
			hasHTTP = true
		case a.Type == "service" && a.Port == 443 && strings.HasPrefix(a.URL, "https://"):
			hasHTTPS = true
		case a.Type == "service" && a.Port == 22 && a.ServiceName == "ssh":
			hasSSH = true
		}
	}

	if !hasIP {
		t.Error("missing IP asset")
	}
	if !hasHostname {
		t.Error("missing hostname asset")
	}
	if !hasHTTP {
		t.Error("missing HTTP service asset")
	}
	if !hasHTTPS {
		t.Error("missing HTTPS service asset")
	}
	if !hasSSH {
		t.Error("missing SSH service asset")
	}

	for _, a := range r.Assets {
		if a.Type == "service" && a.Port == 3306 {
			t.Error("filtered port 3306 should not be included")
		}
	}
}

func TestParseNmapGreppable(t *testing.T) {
	t.Parallel()
	data := `Host: 10.10.10.5 ()	Ports: 22/open/tcp//ssh//OpenSSH 8.9/, 80/open/tcp//http//nginx/, 443/closed/tcp//https//	Ignored State: closed (997)
Host: 10.10.10.6 ()	Ports: 8080/open/tcp//http-proxy///`

	r, err := ParseNmap(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ipCount, svcCount int
	for _, a := range r.Assets {
		if a.Type == "ip" {
			ipCount++
		}
		if a.Type == "service" {
			svcCount++
		}
	}
	if ipCount != 2 {
		t.Errorf("expected 2 IP assets, got %d", ipCount)
	}
	if svcCount < 3 {
		t.Errorf("expected at least 3 service assets, got %d", svcCount)
	}
}

func TestParseNmapGreppableEmpty(t *testing.T) {
	t.Parallel()
	_, err := ParseNmap("# Nmap scan report\n# No hosts found")
	if err == nil {
		t.Error("expected error for empty greppable output")
	}
}

func TestParseNuclei(t *testing.T) {
	t.Parallel()
	data := `{"template-id":"cve-2021-44228","info":{"name":"Log4j RCE","severity":"critical","description":"Remote code execution via Log4Shell","tags":["cve","rce"]},"host":"https://target.htb","matched-at":"https://target.htb/api/login","matcher-name":"dns-callback","extracted-results":["${jndi:ldap://callback}"]}
{"template-id":"http-missing-security-headers","info":{"name":"Missing X-Frame-Options","severity":"info","tags":["header"]},"host":"https://target.htb","matched-at":"https://target.htb"}`

	r, err := ParseNuclei(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 finding (info severity filtered), got %d", len(r.Findings))
	}

	f := r.Findings[0]
	if f.Severity != "critical" {
		t.Errorf("expected critical severity, got %s", f.Severity)
	}
	if f.VulnClass != "cve" {
		t.Errorf("expected vulnclass 'cve' (first tag), got %s", f.VulnClass)
	}
	if !strings.Contains(f.Evidence, "cve-2021-44228") {
		t.Error("evidence should contain template id")
	}

	if len(r.Assets) == 0 {
		t.Error("expected at least one service asset")
	}
}

func TestParseNucleiEmpty(t *testing.T) {
	t.Parallel()
	_, err := ParseNuclei("")
	if err == nil {
		t.Error("expected error for empty nuclei output")
	}
}

func TestParseHttpx(t *testing.T) {
	t.Parallel()
	data := `{"url":"https://target.htb","status_code":200,"content_length":1234,"title":"Dashboard","tech":["React","nginx"],"webserver":"nginx/1.18","host":"10.10.10.1","port":"443","scheme":"https"}
{"url":"http://target.htb:8080","status_code":302,"content_length":0,"title":"","tech":[],"webserver":"Apache","host":"10.10.10.1","port":"8080","scheme":"http"}`

	r, err := ParseHttpx(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.Assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(r.Assets))
	}

	a := r.Assets[0]
	if a.URL != "https://target.htb" {
		t.Errorf("expected URL https://target.htb, got %s", a.URL)
	}
	if a.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", a.StatusCode)
	}
	if a.PageTitle != "Dashboard" {
		t.Errorf("expected title Dashboard, got %s", a.PageTitle)
	}
	if len(a.Technologies) < 2 {
		t.Errorf("expected at least 2 technologies, got %d", len(a.Technologies))
	}
}

func TestParseHttpxDeduplicate(t *testing.T) {
	t.Parallel()
	data := `{"url":"https://target.htb","status_code":200}
{"url":"https://target.htb","status_code":200}`

	r, err := ParseHttpx(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Assets) != 1 {
		t.Errorf("expected 1 asset (deduplicated), got %d", len(r.Assets))
	}
}

func TestParseHttpxEmpty(t *testing.T) {
	t.Parallel()
	_, err := ParseHttpx("")
	if err == nil {
		t.Error("expected error for empty httpx output")
	}
}

func TestParseFfuf(t *testing.T) {
	t.Parallel()
	data := `{
  "config": {"url": "https://target.htb/FUZZ", "method": "GET"},
  "results": [
    {"status": 200, "length": 1234, "url": "https://target.htb/admin"},
    {"status": 301, "length": 0, "url": "https://target.htb/api"},
    {"status": 403, "length": 199, "url": "https://target.htb/secret"}
  ]
}`

	r, err := ParseFfuf(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.Assets) != 3 {
		t.Fatalf("expected 3 endpoint assets, got %d", len(r.Assets))
	}

	for _, a := range r.Assets {
		if a.Type != "endpoint" {
			t.Errorf("expected type endpoint, got %s", a.Type)
		}
		if a.Method != "GET" {
			t.Errorf("expected method GET, got %s", a.Method)
		}
	}
}

func TestParseFfufDefaultMethod(t *testing.T) {
	t.Parallel()
	data := `{"config": {"url": "https://t.htb/FUZZ"}, "results": [{"status": 200, "length": 100, "url": "https://t.htb/x"}]}`

	r, err := ParseFfuf(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Assets[0].Method != "GET" {
		t.Errorf("expected default method GET, got %s", r.Assets[0].Method)
	}
}

func TestParseFfufEmpty(t *testing.T) {
	t.Parallel()
	_, err := ParseFfuf(`{"config":{},"results":[]}`)
	if err == nil {
		t.Error("expected error for empty ffuf results")
	}
}

func TestParseFfufInvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := ParseFfuf("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
