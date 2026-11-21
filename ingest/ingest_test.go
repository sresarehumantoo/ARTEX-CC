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

func TestNmapXMLMultipleHosts(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<nmaprun>
  <host>
    <address addr="10.10.10.1" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh"/>
      </port>
    </ports>
  </host>
  <host>
    <address addr="10.10.10.2" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http"/>
      </port>
    </ports>
  </host>
  <host>
    <address addr="10.10.10.3" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="443">
        <state state="open"/>
        <service name="https"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	r, err := ParseNmap(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ips := map[string]bool{}
	for _, a := range r.Assets {
		if a.Type == "ip" {
			ips[a.IP] = true
		}
	}
	for _, want := range []string{"10.10.10.1", "10.10.10.2", "10.10.10.3"} {
		if !ips[want] {
			t.Errorf("missing IP asset %s", want)
		}
	}
}

func TestNmapXMLNoOpenPorts(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<nmaprun>
  <host>
    <address addr="10.10.10.99" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="22">
        <state state="filtered"/>
        <service name="ssh"/>
      </port>
      <port protocol="tcp" portid="80">
        <state state="closed"/>
        <service name="http"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	r, err := ParseNmap(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range r.Assets {
		if a.Type == "service" {
			t.Errorf("expected no service assets for host with no open ports, got port %d", a.Port)
		}
	}
	var foundIP bool
	for _, a := range r.Assets {
		if a.Type == "ip" && a.IP == "10.10.10.99" {
			foundIP = true
		}
	}
	if !foundIP {
		t.Error("expected IP asset even when no ports are open")
	}
}

func TestNmapXMLUDPPorts(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<nmaprun>
  <host>
    <address addr="10.10.10.5" addrtype="ipv4"/>
    <ports>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain"/>
      </port>
      <port protocol="udp" portid="161">
        <state state="open"/>
        <service name="snmp"/>
      </port>
      <port protocol="udp" portid="500">
        <state state="open|filtered"/>
        <service name="isakmp"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	r, err := ParseNmap(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var udpServices []Asset
	for _, a := range r.Assets {
		if a.Type == "service" && a.Proto == "udp" {
			udpServices = append(udpServices, a)
		}
	}
	if len(udpServices) != 2 {
		t.Errorf("expected 2 open UDP services (domain, snmp), got %d", len(udpServices))
	}
	names := map[string]bool{}
	for _, s := range udpServices {
		names[s.ServiceName] = true
	}
	if !names["domain"] {
		t.Error("missing DNS (domain) UDP service")
	}
	if !names["snmp"] {
		t.Error("missing SNMP UDP service")
	}
}

func TestNmapXMLServiceVersion(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<nmaprun>
  <host>
    <address addr="10.10.10.7" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh" product="OpenSSH" version="9.3p1"/>
      </port>
      <port protocol="tcp" portid="3306">
        <state state="open"/>
        <service name="mysql" product="MySQL" version="8.0.35"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	r, err := ParseNmap(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sshFound, mysqlFound bool
	for _, a := range r.Assets {
		if a.Type == "service" && a.Port == 22 && a.ServiceName == "ssh" {
			sshFound = true
		}
		if a.Type == "service" && a.Port == 3306 && a.ServiceName == "mysql" {
			mysqlFound = true
		}
	}
	if !sshFound {
		t.Error("missing SSH service with version detection")
	}
	if !mysqlFound {
		t.Error("missing MySQL service with version detection")
	}
}

func TestNmapXMLEmpty(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?><nmaprun></nmaprun>`

	r, err := ParseNmap(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Assets) != 0 {
		t.Errorf("expected 0 assets for empty scan, got %d", len(r.Assets))
	}
}

func TestNmapXMLEncodingIssues(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<nmaprun>
  <host>
    <address addr="10.10.10.8" addrtype="ipv4"/>
    <hostnames>
      <hostname name="host-with-special.example.com" type="user"/>
    </hostnames>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http" product="Apache httpd" version="2.4.52"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	r, err := ParseNmap(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Assets) == 0 {
		t.Fatal("expected assets from XML with encoding declaration")
	}
	var hasDomain bool
	for _, a := range r.Assets {
		if a.Type == "subdomain" && a.Domain == "host-with-special.example.com" {
			hasDomain = true
		}
	}
	if !hasDomain {
		t.Error("missing hostname asset from encoded XML")
	}
}

func TestNmapXMLNoAddress(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<nmaprun>
  <host>
    <address addr="aa:bb:cc:dd:ee:ff" addrtype="mac"/>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	r, err := ParseNmap(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Assets) != 0 {
		t.Errorf("expected 0 assets for host with only MAC address, got %d", len(r.Assets))
	}
}

func TestNucleiMultipleSeverities(t *testing.T) {
	t.Parallel()
	data := `{"template-id":"cve-2023-0001","info":{"name":"Critical RCE","severity":"critical","tags":["cve"]},"host":"https://a.htb","matched-at":"https://a.htb/rce"}
{"template-id":"sqli-detect","info":{"name":"SQL Injection","severity":"high","tags":["sqli"]},"host":"https://b.htb","matched-at":"https://b.htb/search"}
{"template-id":"open-redirect","info":{"name":"Open Redirect","severity":"medium","tags":["redirect"]},"host":"https://c.htb","matched-at":"https://c.htb/go"}
{"template-id":"info-disclosure","info":{"name":"Version Disclosed","severity":"low","tags":["info-disc"]},"host":"https://d.htb","matched-at":"https://d.htb/"}
{"template-id":"tech-detect","info":{"name":"Nginx Detected","severity":"info","tags":["tech"]},"host":"https://e.htb","matched-at":"https://e.htb/"}`

	r, err := ParseNuclei(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.Findings) != 4 {
		t.Fatalf("expected 4 findings (info filtered), got %d", len(r.Findings))
	}

	sevs := map[string]bool{}
	for _, f := range r.Findings {
		sevs[f.Severity] = true
	}
	for _, want := range []string{"critical", "high", "medium", "low"} {
		if !sevs[want] {
			t.Errorf("missing severity %s", want)
		}
	}
}

func TestNucleiMatcherAndExtracted(t *testing.T) {
	t.Parallel()
	data := `{"template-id":"creds-detect","info":{"name":"Credential Found","severity":"high","tags":["creds"]},"host":"https://target.htb","matched-at":"https://target.htb/config","matcher-name":"aws-key","extracted-results":["AKIA1234567890ABCDEF","secret/path"]}`

	r, err := ParseNuclei(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(r.Findings))
	}
	f := r.Findings[0]
	if !strings.Contains(f.Evidence, "matcher: aws-key") {
		t.Errorf("evidence should contain matcher name, got: %s", f.Evidence)
	}
	if !strings.Contains(f.Evidence, "extracted: AKIA1234567890ABCDEF, secret/path") {
		t.Errorf("evidence should contain extracted results, got: %s", f.Evidence)
	}
}

func TestNucleiMalformedLine(t *testing.T) {
	t.Parallel()
	data := `{"template-id":"xss-detect","info":{"name":"Reflected XSS","severity":"high","tags":["xss"]},"host":"https://ok.htb","matched-at":"https://ok.htb/search"}
{this is not valid json at all
{"template-id":"sqli","info":{"name":"SQLi","severity":"medium","tags":["sqli"]},"host":"https://ok2.htb","matched-at":"https://ok2.htb/api"}`

	r, err := ParseNuclei(data)
	if err != nil {
		t.Fatalf("should not error on malformed lines: %v", err)
	}
	if len(r.Findings) != 2 {
		t.Errorf("expected 2 findings (malformed line skipped), got %d", len(r.Findings))
	}
}

func TestNucleiUnknownSeverity(t *testing.T) {
	t.Parallel()
	data := `{"template-id":"custom-check","info":{"name":"Custom","severity":"weird","tags":["custom"]},"host":"https://t.htb","matched-at":"https://t.htb/"}`

	r, err := ParseNuclei(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(r.Findings))
	}
	if r.Findings[0].Severity != "low" {
		t.Errorf("unknown severity should default to low, got %s", r.Findings[0].Severity)
	}
}

func TestHttpxMalformedLine(t *testing.T) {
	t.Parallel()
	data := `{"url":"https://good.htb","status_code":200,"title":"OK"}
this is garbage
{"url":"https://also-good.htb","status_code":301,"title":"Redirect"}`

	r, err := ParseHttpx(data)
	if err != nil {
		t.Fatalf("should not error on malformed lines: %v", err)
	}
	if len(r.Assets) != 2 {
		t.Errorf("expected 2 assets (malformed line skipped), got %d", len(r.Assets))
	}
}

func TestHttpxWebServerTechDedup(t *testing.T) {
	t.Parallel()
	data := `{"url":"https://target.htb","status_code":200,"tech":["nginx"],"webserver":"nginx"}`

	r, err := ParseHttpx(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(r.Assets))
	}
	a := r.Assets[0]
	count := 0
	for _, t := range a.Technologies {
		if strings.EqualFold(t, "nginx") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("webserver should be deduplicated with tech; nginx appeared %d times", count)
	}
}

func TestHttpxTechnologiesArray(t *testing.T) {
	t.Parallel()
	data := `{"url":"https://app.htb","status_code":200,"tech":["React","Webpack","Node.js"],"webserver":"Express","title":"App"}`

	r, err := ParseHttpx(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := r.Assets[0]
	if len(a.Technologies) != 4 {
		t.Errorf("expected 4 technologies (3 tech + Express webserver), got %d: %v", len(a.Technologies), a.Technologies)
	}
}

func TestHttpxMissingURL(t *testing.T) {
	t.Parallel()
	data := `{"status_code":200,"title":"no url"}`
	_, err := ParseHttpx(data)
	if err == nil {
		t.Error("expected error when no entries have a URL")
	}
}

func TestFfufMultipleStatusCodes(t *testing.T) {
	t.Parallel()
	data := `{
  "config": {"url": "https://t.htb/FUZZ", "method": "POST"},
  "results": [
    {"status": 200, "length": 500, "url": "https://t.htb/login"},
    {"status": 301, "length": 0, "url": "https://t.htb/old"},
    {"status": 403, "length": 199, "url": "https://t.htb/admin"},
    {"status": 500, "length": 50, "url": "https://t.htb/error"}
  ]
}`

	r, err := ParseFfuf(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Assets) != 4 {
		t.Fatalf("expected 4 assets, got %d", len(r.Assets))
	}
	codes := map[int]bool{}
	for _, a := range r.Assets {
		codes[a.StatusCode] = true
		if a.Method != "POST" {
			t.Errorf("expected method POST, got %s", a.Method)
		}
	}
	for _, code := range []int{200, 301, 403, 500} {
		if !codes[code] {
			t.Errorf("missing result with status code %d", code)
		}
	}
}

func TestFfufResultWithEmptyURL(t *testing.T) {
	t.Parallel()
	data := `{
  "config": {"url": "https://t.htb/FUZZ", "method": "GET"},
  "results": [
    {"status": 200, "length": 100, "url": "https://t.htb/valid"},
    {"status": 200, "length": 50, "url": ""}
  ]
}`

	r, err := ParseFfuf(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Assets) != 1 {
		t.Errorf("expected 1 asset (empty URL skipped), got %d", len(r.Assets))
	}
}

func TestParseMasscanXML(t *testing.T) {
	t.Parallel()
	data := `<?xml version="1.0"?>
<nmaprun scanner="masscan">
  <host>
    <address addr="10.0.0.1" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="80"><state state="open"/><service name="http"/></port>
      <port protocol="tcp" portid="443"><state state="open"/><service name="https"/></port>
      <port protocol="tcp" portid="22"><state state="open"/><service name="ssh"/></port>
    </ports>
  </host>
  <host>
    <address addr="10.0.0.2" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="8080"><state state="open"/><service name="http-proxy"/></port>
    </ports>
  </host>
</nmaprun>`

	results, err := ParseMasscanXML(strings.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result set, got %d", len(results))
	}
	r := results[0]

	var hasIP1, hasIP2, hasHTTP, hasHTTPS, hasSSH, hasProxy bool
	for _, a := range r.Assets {
		switch {
		case a.Type == "ip" && a.IP == "10.0.0.1":
			hasIP1 = true
		case a.Type == "ip" && a.IP == "10.0.0.2":
			hasIP2 = true
		case a.Type == "service" && a.Port == 80 && strings.HasPrefix(a.URL, "http://"):
			hasHTTP = true
		case a.Type == "service" && a.Port == 443 && strings.HasPrefix(a.URL, "https://"):
			hasHTTPS = true
		case a.Type == "service" && a.Port == 22 && a.ServiceName == "ssh":
			hasSSH = true
		case a.Type == "service" && a.Port == 8080 && strings.Contains(a.URL, "http://"):
			hasProxy = true
		}
	}

	if !hasIP1 {
		t.Error("missing IP asset 10.0.0.1")
	}
	if !hasIP2 {
		t.Error("missing IP asset 10.0.0.2")
	}
	if !hasHTTP {
		t.Error("missing HTTP service on port 80")
	}
	if !hasHTTPS {
		t.Error("missing HTTPS service on port 443")
	}
	if !hasSSH {
		t.Error("missing SSH service on port 22")
	}
	if !hasProxy {
		t.Error("missing HTTP proxy service on port 8080")
	}
}

func TestParseMasscanXMLEmpty(t *testing.T) {
	t.Parallel()
	data := `<?xml version="1.0"?><nmaprun scanner="masscan"></nmaprun>`
	_, err := ParseMasscanXML(strings.NewReader(data))
	if err == nil {
		t.Error("expected error for empty masscan XML")
	}
}

func TestParseMasscanJSON(t *testing.T) {
	t.Parallel()
	data := `[
{"ip": "10.0.0.1", "timestamp": "1234567890", "ports": [{"port": 80, "proto": "tcp", "status": "open", "service": {"name": "http"}}]},
{"ip": "10.0.0.1", "timestamp": "1234567891", "ports": [{"port": 443, "proto": "tcp", "status": "open", "service": {"name": "https"}}]},
{"ip": "10.0.0.2", "timestamp": "1234567892", "ports": [{"port": 22, "proto": "tcp", "status": "open", "service": {"name": "ssh"}}]}
]`

	results, err := ParseMasscanJSON(strings.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result set, got %d", len(results))
	}
	r := results[0]

	ips := map[string]bool{}
	var svcCount int
	for _, a := range r.Assets {
		if a.Type == "ip" {
			ips[a.IP] = true
		}
		if a.Type == "service" {
			svcCount++
		}
	}
	if !ips["10.0.0.1"] {
		t.Error("missing IP 10.0.0.1")
	}
	if !ips["10.0.0.2"] {
		t.Error("missing IP 10.0.0.2")
	}
	if svcCount != 3 {
		t.Errorf("expected 3 service assets, got %d", svcCount)
	}
}

func TestParseMasscanJSONEmpty(t *testing.T) {
	t.Parallel()
	_, err := ParseMasscanJSON(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty masscan JSON")
	}
}

func TestParseMasscanJSONSingleLine(t *testing.T) {
	t.Parallel()
	data := `{"ip": "192.168.1.1", "timestamp": "1700000000", "ports": [{"port": 8443, "proto": "tcp", "status": "open", "service": {"name": "https"}}]}`

	results, err := ParseMasscanJSON(strings.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := results[0]

	var found bool
	for _, a := range r.Assets {
		if a.Type == "service" && a.Port == 8443 && strings.HasPrefix(a.URL, "https://") {
			found = true
		}
	}
	if !found {
		t.Error("expected HTTPS service on port 8443")
	}
}

func TestParseMasscanXMLNoService(t *testing.T) {
	t.Parallel()
	data := `<?xml version="1.0"?>
<nmaprun scanner="masscan">
  <host>
    <address addr="10.0.0.5" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="9999"><state state="open"/></port>
    </ports>
  </host>
</nmaprun>`

	results, err := ParseMasscanXML(strings.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := results[0]

	var svc *Asset
	for _, a := range r.Assets {
		if a.Type == "service" {
			svc = &a
		}
	}
	if svc == nil {
		t.Fatal("expected a service asset")
	}
	if svc.ServiceName != "unknown-9999" {
		t.Errorf("expected service name unknown-9999, got %s", svc.ServiceName)
	}
}
