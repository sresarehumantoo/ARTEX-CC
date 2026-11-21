package enrich

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Fact is one piece of reconnaissance intelligence tied to a parent asset.
type Fact struct {
	Parent string `json:"parent"`
	Text   string `json:"text"`
	Source string `json:"source"`
}

// Asset is a discovered asset produced by an enrichment probe.
type Asset struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Result is the combined output of an enrichment probe: facts and new assets.
type Result struct {
	Facts  []Fact  `json:"facts,omitempty"`
	Assets []Asset `json:"assets,omitempty"`
}

// EnrichDomain collects DNS records and reverse lookups for a domain.
func EnrichDomain(ctx context.Context, domain string) (*Result, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")

	r := &Result{}
	resolver := net.DefaultResolver

	lookupRecords(ctx, resolver, domain, r)
	lookupReverseDNSForDomain(ctx, resolver, domain, r)

	return r, nil
}

func lookupRecords(ctx context.Context, resolver *net.Resolver, domain string, r *Result) {
	if addrs, err := resolver.LookupHost(ctx, domain); err == nil {
		for _, addr := range addrs {
			ip := net.ParseIP(addr)
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				r.Facts = append(r.Facts, Fact{Parent: domain, Text: fmt.Sprintf("A %s", addr), Source: "dns"})
			} else {
				r.Facts = append(r.Facts, Fact{Parent: domain, Text: fmt.Sprintf("AAAA %s", addr), Source: "dns"})
			}
			r.Assets = append(r.Assets, Asset{Type: "ip", Value: addr})
		}
	}

	if cname, err := resolver.LookupCNAME(ctx, domain); err == nil {
		cname = strings.TrimSuffix(cname, ".")
		if !strings.EqualFold(cname, domain) {
			r.Facts = append(r.Facts, Fact{Parent: domain, Text: fmt.Sprintf("CNAME %s", cname), Source: "dns"})
			r.Assets = append(r.Assets, Asset{Type: "subdomain", Value: cname})
		}
	}

	if mxs, err := resolver.LookupMX(ctx, domain); err == nil {
		for _, mx := range mxs {
			host := strings.TrimSuffix(mx.Host, ".")
			r.Facts = append(r.Facts, Fact{Parent: domain, Text: fmt.Sprintf("MX %s (pref %d)", host, mx.Pref), Source: "dns"})
		}
	}

	if nss, err := resolver.LookupNS(ctx, domain); err == nil {
		for _, ns := range nss {
			host := strings.TrimSuffix(ns.Host, ".")
			r.Facts = append(r.Facts, Fact{Parent: domain, Text: fmt.Sprintf("NS %s", host), Source: "dns"})
		}
	}

	if txts, err := resolver.LookupTXT(ctx, domain); err == nil {
		for _, txt := range txts {
			r.Facts = append(r.Facts, Fact{Parent: domain, Text: fmt.Sprintf("TXT %s", txt), Source: "dns"})
		}
	}
}

func lookupReverseDNSForDomain(ctx context.Context, resolver *net.Resolver, domain string, r *Result) {
	addrs, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		return
	}
	for _, addr := range addrs {
		names, err := resolver.LookupAddr(ctx, addr)
		if err != nil {
			continue
		}
		for _, name := range names {
			name = strings.TrimSuffix(name, ".")
			r.Facts = append(r.Facts, Fact{Parent: domain, Text: fmt.Sprintf("reverse-dns %s -> %s", addr, name), Source: "reverse-dns"})
		}
	}
}

// EnrichIP collects reverse DNS records for an IP address.
func EnrichIP(ctx context.Context, ip string) (*Result, error) {
	ip = strings.TrimSpace(ip)
	if net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	r := &Result{}
	resolver := net.DefaultResolver

	names, err := resolver.LookupAddr(ctx, ip)
	if err == nil {
		for _, name := range names {
			name = strings.TrimSuffix(name, ".")
			r.Facts = append(r.Facts, Fact{Parent: ip, Text: fmt.Sprintf("PTR %s", name), Source: "reverse-dns"})
			r.Assets = append(r.Assets, Asset{Type: "subdomain", Value: name})
		}
	}

	return r, nil
}

// EnrichService probes an HTTP(S) service for technology fingerprints and TLS details.
func EnrichService(ctx context.Context, host string, port int, scheme string) (*Result, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("port must be 1-65535")
	}
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme == "" {
		if port == 443 || port == 8443 {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	r := &Result{}
	parent := fmt.Sprintf("%s:%d", host, port)

	probeHTTP(ctx, host, port, scheme, parent, r)

	if scheme == "https" {
		probeTLS(ctx, host, port, parent, r)
	}

	return r, nil
}

var titleRE = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)

func probeHTTP(ctx context.Context, host string, port int, scheme, parent string, r *Result) {
	addr := fmt.Sprintf("%s:%d", host, port)
	url := fmt.Sprintf("%s://%s", scheme, addr)
	if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		url = fmt.Sprintf("%s://%s", scheme, host)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; enricher/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("HTTP probe failed: %s", err), Source: "http"})
		return
	}
	defer resp.Body.Close()

	r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("status %d", resp.StatusCode), Source: "http"})

	if resp.Request.URL.String() != url {
		r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("redirect -> %s", resp.Request.URL), Source: "http"})
	}

	fingerprint(resp.Header, parent, r)

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if m := titleRE.FindSubmatch(body); len(m) > 1 {
		title := strings.TrimSpace(string(m[1]))
		if title != "" {
			r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("title: %s", title), Source: "http"})
		}
	}

	extractTechFromHTML(string(body), parent, r)
}

var techHeaders = []string{
	"Server",
	"X-Powered-By",
	"X-AspNet-Version",
	"X-AspNetMvc-Version",
	"X-Generator",
	"X-Drupal-Cache",
	"X-Varnish",
	"Via",
}

func fingerprint(h http.Header, parent string, r *Result) {
	for _, name := range techHeaders {
		if v := h.Get(name); v != "" {
			r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("header %s: %s", name, v), Source: "http"})
		}
	}
}

var metaGeneratorRE = regexp.MustCompile(`(?i)<meta\s[^>]*name=["']generator["'][^>]*content=["']([^"']+)["']`)

func extractTechFromHTML(body, parent string, r *Result) {
	if m := metaGeneratorRE.FindStringSubmatch(body); len(m) > 1 {
		r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("generator: %s", strings.TrimSpace(m[1])), Source: "http"})
	}
}

func probeTLS(ctx context.Context, host string, port int, parent string, r *Result) {
	dialer := &tls.Dialer{
		Config: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		},
		NetDialer: &net.Dialer{Timeout: 5 * time.Second},
	}

	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return
	}

	cert := state.PeerCertificates[0]
	r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("TLS CN=%s", cert.Subject.CommonName), Source: "tls"})
	r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("TLS issuer=%s", cert.Issuer.CommonName), Source: "tls"})
	r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("TLS expires=%s", cert.NotAfter.Format(time.DateOnly)), Source: "tls"})

	for _, san := range cert.DNSNames {
		r.Facts = append(r.Facts, Fact{Parent: parent, Text: fmt.Sprintf("TLS SAN=%s", san), Source: "tls"})
		if !strings.Contains(san, "*") {
			r.Assets = append(r.Assets, Asset{Type: "subdomain", Value: san})
		}
	}
}
