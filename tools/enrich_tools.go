package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/SREsAreHumanToo/cc-artex/enrich"
)

func RegisterEnrichTools(reg *Registry, store Store, taskID int64) {
	reg.Register(makeEnrichDomain(store, taskID))
	reg.Register(makeEnrichIP(store, taskID))
	reg.Register(makeEnrichService(store, taskID))
}

func makeEnrichDomain(store Store, taskID int64) Tool {
	return Tool{
		Name: "enrich_domain",
		Description: "Run DNS enrichment on a domain: resolves A/AAAA/CNAME/MX/NS/TXT records and performs reverse DNS on discovered IPs. " +
			"Returns discovered facts and any new assets (IPs, subdomains from CNAMEs).",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"domain": str("domain or subdomain to enrich"),
		}, "domain"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Domain string `json:"domain"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Domain) == "" {
				return errResult("domain is required"), nil
			}
			if ok, reason, _, err := store.IsInScope(ctx, taskID, a.Domain); err != nil {
				return errResult("scope check failed: " + err.Error()), nil
			} else if !ok {
				return errResult(fmt.Sprintf("enrichment blocked: target is out of scope (%s)", reason)), nil
			}
			r, err := enrich.EnrichDomain(ctx, a.Domain)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(r)
		},
	}
}

func makeEnrichIP(store Store, taskID int64) Tool {
	return Tool{
		Name: "enrich_ip",
		Description: "Run reverse DNS on an IP address. Returns PTR records and any discovered hostnames as new assets.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"ip": str("IPv4 or IPv6 address"),
		}, "ip"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			ip := strings.TrimSpace(a.IP)
			if ip == "" {
				return errResult("ip is required"), nil
			}
			if net.ParseIP(ip) == nil {
				return errResult(fmt.Sprintf("invalid IP address: %s", ip)), nil
			}
			if ok, reason, _, err := store.IsInScope(ctx, taskID, ip); err != nil {
				return errResult("scope check failed: " + err.Error()), nil
			} else if !ok {
				return errResult(fmt.Sprintf("enrichment blocked: target is out of scope (%s)", reason)), nil
			}
			r, err := enrich.EnrichIP(ctx, ip)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(r)
		},
	}
}

func makeEnrichService(store Store, taskID int64) Tool {
	return Tool{
		Name: "enrich_service",
		Description: "Probe an HTTP(S) service for status code, redirect chain, server headers, page title, " +
			"TLS certificate info (CN, SAN, issuer, expiry), and technology fingerprinting from headers and HTML meta tags. " +
			"Scheme defaults to https for ports 443/8443, http otherwise.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"host":   str("hostname or IP"),
			"port":   intp("port number (1-65535)"),
			"scheme": str("http or https (auto-detected from port if omitted)"),
		}, "host", "port"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Host   string `json:"host"`
				Port   int    `json:"port"`
				Scheme string `json:"scheme"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Host) == "" {
				return errResult("host is required"), nil
			}
			if a.Port <= 0 || a.Port > 65535 {
				return errResult("port must be 1-65535"), nil
			}
			if ok, reason, _, err := store.IsInScope(ctx, taskID, a.Host); err != nil {
				return errResult("scope check failed: " + err.Error()), nil
			} else if !ok {
				return errResult(fmt.Sprintf("enrichment blocked: target is out of scope (%s)", reason)), nil
			}
			r, err := enrich.EnrichService(ctx, a.Host, a.Port, a.Scheme)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(r)
		},
	}
}
