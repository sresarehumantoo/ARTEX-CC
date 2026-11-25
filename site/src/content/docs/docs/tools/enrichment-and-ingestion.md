---
title: Enrichment and ingestion
description: "DNS and HTTP enrichment, and importing nmap, nuclei, httpx, ffuf and masscan output."
sidebar:
  order: 6
---

## `enrich_domain`

Run DNS enrichment on a domain: resolves A/AAAA/CNAME/MX/NS/TXT records and performs reverse DNS on discovered IPs.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| domain | string | yes | Domain or subdomain to enrich |
| timeout | integer | no | Timeout in seconds (default 30) |

**Returns:** Discovered DNS records, reverse-DNS results, and any new assets (IPs, subdomains from CNAMEs).

## `enrich_ip`

Run reverse DNS on an IP address. Returns PTR records and any discovered hostnames as new assets.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| ip | string | yes | IPv4 or IPv6 address |
| timeout | integer | no | Timeout in seconds (default 30) |

**Returns:** PTR records and discovered hostnames.

## `enrich_service`

Probe an HTTP(S) service for status code, redirect chain, server headers, page title, TLS certificate info (CN, SAN, issuer, expiry), and technology fingerprinting.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | yes | Hostname or IP |
| port | integer | yes | Port number (1-65535) |
| scheme | string | no | `http` or `https` (auto-detected from port if omitted: https for 443/8443, http otherwise) |
| timeout | integer | no | Timeout in seconds (default 30) |

**Returns:** Status code, redirect chain, server headers, page title, TLS certificate details, and detected technologies.

## `ingest_scanner_output`

Parse output from common security scanners and import the results as assets and/or findings.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| scanner | string | yes | `nmap` (XML or greppable), `nuclei` (JSONL), `httpx` (JSONL), or `ffuf` (JSON) |
| output | string | yes | The scanner output to parse |

**Returns:** `{assets_created, findings_created, errors?}`.

## `ingest_masscan`

Parse masscan output (XML or JSON) and import discovered hosts and open ports as assets.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| format | string | yes | `xml` or `json` |
| data | string | yes | The masscan output to parse |

**Returns:** `{assets_created, findings_created: 0, errors?}`.
