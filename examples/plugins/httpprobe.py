#!/usr/bin/env python3
"""Example ARTEX exec plugin: an httpx-style HTTP probe using only the stdlib.

Install by copying this file into the plugin directory (default
~/.artex/plugins) and making it executable. ARTEX registers its tool as
httpprobe_probe. See https://artex-cc.com/docs/plugins/ for the contract.
"""

import ipaddress
import json
import re
import ssl
import sys
import urllib.error
import urllib.parse
import urllib.request

MANIFEST = {
    "name": "httpprobe",
    "description": "HTTP probe for in-scope URLs",
    "version": "1.0.0",
    "tools": [
        {
            "name": "probe",
            "description": "Request each URL once and record status, title and server as service assets. "
            "URLs outside the engagement scope are skipped without sending any traffic.",
            "schema": {
                "type": "object",
                "properties": {
                    "urls": {"type": "array", "items": {"type": "string"}, "description": "full URLs to probe"},
                    "timeout": {"type": "integer", "description": "per-request timeout in seconds (default 10)"},
                },
                "required": ["urls"],
            },
        }
    ],
}

TITLE_RE = re.compile(rb"<title[^>]*>(.*?)</title>", re.IGNORECASE | re.DOTALL)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def in_scope(host, rules):
    # Mirrors the server's own check: domain rules match the host or any
    # subdomain, ip and cidr rules match addresses. Unlike the server, an empty
    # scope means nothing is in scope, so the demo never probes blindly.
    try:
        addr = ipaddress.ip_address(host)
    except ValueError:
        addr = None
    for rule in rules:
        kind, value = rule.get("kind"), rule.get("value", "").lower()
        if addr is not None and kind in ("ip", "cidr"):
            if addr in ipaddress.ip_network(value, strict=False):
                return True
        elif addr is None and kind in ("root_domain", "subdomain", "domain"):
            if host == value or host.endswith("." + value):
                return True
    return False


def build_opener(proxy):
    handlers = [NoRedirect()]
    if proxy and proxy.get("url"):
        handlers.append(urllib.request.ProxyHandler({"http": proxy["url"], "https": proxy["url"]}))
        if proxy.get("ca_cert"):
            ctx = ssl.create_default_context(cafile=proxy["ca_cert"])
            handlers.append(urllib.request.HTTPSHandler(context=ctx))
    else:
        handlers.append(urllib.request.ProxyHandler({}))
    return urllib.request.build_opener(*handlers)


def fetch(opener, url, timeout):
    try:
        resp = opener.open(urllib.request.Request(url, headers={"User-Agent": "artex-httpprobe/1.0"}), timeout=timeout)
    except urllib.error.HTTPError as e:
        resp = e
    with resp:
        body = resp.read(1 << 20)
        m = TITLE_RE.search(body)
        return {
            "status": resp.status,
            "length": len(body),
            "title": m.group(1).decode("utf-8", "replace").strip() if m else "",
            "server": resp.headers.get("Server", ""),
        }


def probe(inv):
    args = inv.get("input") or {}
    urls = args.get("urls") or []
    if not urls:
        return {"content": [{"type": "text", "text": "urls must be a non-empty array"}], "isError": True}
    timeout = args.get("timeout") or 10
    config = inv.get("config") or {}
    rules = config.get("in_scope") or []
    opener = build_opener(config.get("proxy"))

    lines, assets, live = [], [], []
    for url in urls:
        host = (urllib.parse.urlsplit(url).hostname or "").lower()
        if not in_scope(host, rules):
            lines.append(f"{url} skipped: out of scope")
            continue
        try:
            r = fetch(opener, url, timeout)
        except (urllib.error.URLError, OSError) as e:
            lines.append(f"{url} failed: {getattr(e, 'reason', e)}")
            continue
        lines.append(f"{url} [{r['status']}] [{r['length']}] [{r['title']}] [{r['server']}]")
        asset = {"type": "service", "url": url, "status_code": r["status"], "content_length": r["length"]}
        if r["title"]:
            asset["page_title"] = r["title"]
        if r["server"]:
            asset["technologies"] = [r["server"]]
        assets.append(asset)
        live.append(url)

    result = {"content": [{"type": "text", "text": "\n".join(lines)}], "assets": assets}
    if live:
        result["facts"] = [{
            "summary": f"{len(live)} of {len(urls)} probed URLs answered over HTTP",
            "detail": "\n".join(live),
            "confidence": "observed",
            "source": "tool_output",
        }]
    return result


def main():
    if sys.argv[1:] == ["--manifest"]:
        json.dump(MANIFEST, sys.stdout)
        return
    inv = json.load(sys.stdin)
    if inv.get("tool") != "probe":
        result = {"content": [{"type": "text", "text": f"unknown tool {inv.get('tool')!r}"}], "isError": True}
    else:
        result = probe(inv)
    json.dump(result, sys.stdout)


if __name__ == "__main__":
    main()
