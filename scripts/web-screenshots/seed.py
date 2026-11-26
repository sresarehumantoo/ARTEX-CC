#!/usr/bin/env python3
"""Seed a demo engagement into the artex-shots stack for README screenshots.

Talks to the real server: MCP tools over /mcp for the engagement data, the
recording proxy for traffic (sent at the fake target in target.py), and the
REST API to set the web password and look up recorded exchange ids. Only
reserved *.example names and documentation IP ranges appear on camera.
"""
import http.cookiejar
import json
import subprocess
import sys
import urllib.request
from urllib.parse import quote

BASE = "http://127.0.0.1:19400"
PROXY = "http://127.0.0.1:18788"
PASSWORD = "artex-demo-pass"
CONTAINER = "artex-shots-artex-1"
WORKSPACE = {
    "scope.txt": "acme.example\n*.acme.example\n203.0.113.0/24\n",
    "notes/recon.md": "# Recon\n\n- portal and api share an nginx/Express stack\n- vpn.acme.example is out of scope for exploitation\n",
}


class MCP:
    def __init__(self):
        cfg = json.loads(subprocess.check_output(["docker", "exec", CONTAINER, "cat", "/app/data/mcp-config.json"]))
        self.auth = cfg["mcpServers"]["artex"]["headers"]["Authorization"]
        self.session = None
        self.seq = 0
        self.rpc("initialize", {"protocolVersion": "2025-03-26", "capabilities": {}, "clientInfo": {"name": "seed", "version": "1"}})
        self.rpc("notifications/initialized", notify=True)

    def rpc(self, method, params=None, notify=False):
        self.seq += 1
        body = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            body["params"] = params
        if not notify:
            body["id"] = self.seq
        headers = {"Authorization": self.auth, "Content-Type": "application/json", "Accept": "application/json, text/event-stream"}
        if self.session:
            headers["Mcp-Session-Id"] = self.session
        resp = urllib.request.urlopen(urllib.request.Request(BASE + "/mcp", json.dumps(body).encode(), headers))
        self.session = resp.headers.get("Mcp-Session-Id") or self.session
        raw = resp.read().decode()
        if notify or not raw:
            return None
        if raw.startswith(("event:", "data:")):
            raw = [line[5:] for line in raw.splitlines() if line.startswith("data:")][-1]
        return json.loads(raw)

    def __call__(self, tool, **args):
        res = self.rpc("tools/call", {"name": tool, "arguments": args})
        if "error" in res:
            sys.exit(f"{tool}: {res['error']}")
        text = res["result"]["content"][0]["text"]
        if res["result"].get("isError"):
            sys.exit(f"{tool}: {text}")
        try:
            return json.loads(text)
        except ValueError:
            return text


def node_id(result):
    """add_intent answers 'intent created: N'; record_fact answers {"ids": [N]}."""
    if isinstance(result, dict):
        return result["ids"][0]
    return int(result.rsplit(":", 1)[1])


def send_traffic():
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({"http": PROXY}))
    opener.addheaders = [("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")]
    gets = [
        "http://portal.acme.example/", "http://portal.acme.example/robots.txt",
        "http://portal.acme.example/login", "http://portal.acme.example/admin",
        "http://portal.acme.example/.git/config",
        "http://api.acme.example/api/v2/users/1", "http://api.acme.example/api/v2/users/2",
        "http://api.acme.example/api/v2/users/3", "http://api.acme.example/api/v2/users/4",
        "http://api.acme.example/api/v2/users",
    ]
    posts = [
        ("http://api.acme.example/api/v2/auth/login", {"email": "admin@acme.example", "password": "admin"}),
        ("http://api.acme.example/api/v2/auth/login", {"email": "svc-backup@acme.example", "password": "Spring2026!"}),
        ("http://api.acme.example/api/v2/search", {"q": "invoice"}),
        ("http://api.acme.example/api/v2/search", {"q": "invoice'"}),
    ]
    tail = ["http://api.acme.example/api/v2/internal/health", "http://vpn.acme.example/"]

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *a, **k):
            return None

    opener.add_handler(NoRedirect())

    def fetch(req):
        try:
            opener.open(req).read()
        except urllib.error.HTTPError:
            pass

    for url in gets:
        fetch(url)
    for url, body in posts:
        fetch(urllib.request.Request(url, json.dumps(body).encode(), {"Content-Type": "application/json"}))
    for url in tail:
        fetch(url)


def rest_session():
    jar = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
    body = json.dumps({"password": PASSWORD}).encode()
    headers = {"Content-Type": "application/json"}
    try:
        opener.open(urllib.request.Request(BASE + "/api/auth/init", body, headers))
    except urllib.error.HTTPError as e:
        if e.code != 409:
            raise
        opener.open(urllib.request.Request(BASE + "/api/auth/login", body, headers))
    return opener


def seed_workspace(opener):
    for path, body in WORKSPACE.items():
        opener.open(urllib.request.Request(BASE + "/api/workspace/file?path=" + quote(path), body.encode(), method="PUT"))


def traffic_ids(opener):
    rows = json.load(opener.open(BASE + "/api/traffic?limit=100"))
    rows = rows.get("items") or rows.get("results") or rows.get("rows") or rows
    return {(r["method"], r["url"], r["status"]): r["id"] for r in rows}


def seed(c, snap):
    c("add_task_scope", entries=[{"kind": "root_domain", "value": "acme.example"}, {"kind": "cidr", "value": "203.0.113.0/24"}], reason="Rules of engagement, signed SOW section 2")
    c("set_constraints", constraints=[{"text": "No denial-of-service or destructive payloads", "type": "deny"}, {"text": "Testing window 09:00-18:00 UTC", "type": "allow"}])
    goals = c("set_goals", goals=[
        {"text": "Gain authenticated admin access to the customer portal", "vulnclass": "auth_bypass"},
        {"text": "Demonstrate access to customer PII", "vulnclass": "idor"},
        {"text": "Obtain a foothold on the internal network", "vulnclass": "rce"},
    ])
    goals = [g["id"] for g in goals["created"]]

    def ports(*pairs):
        return [{"port": p, "service": s} for p, s in pairs]

    assets = [
        {"type": "root_domain", "domain": "acme.example"},
        {"type": "subdomain", "domain": "portal.acme.example", "record_type": "A", "record_value": ["203.0.113.10"]},
        {"type": "subdomain", "domain": "api.acme.example", "record_type": "A", "record_value": ["203.0.113.11"]},
        {"type": "subdomain", "domain": "vpn.acme.example", "record_type": "A", "record_value": ["203.0.113.20"]},
        {"type": "subdomain", "domain": "git.acme.example", "record_type": "CNAME", "record_value": ["acme.gitlab-host.example"]},
        {"type": "subdomain", "domain": "mail.acme.example", "record_type": "MX", "record_value": ["203.0.113.25"]},
        {"type": "ip", "ip": "203.0.113.10", "open_ports": ports((80, "http"), (443, "https")), "bound_domains": ["portal.acme.example"]},
        {"type": "ip", "ip": "203.0.113.11", "open_ports": ports((443, "https"), (8443, "https-alt")), "bound_domains": ["api.acme.example"]},
        {"type": "ip", "ip": "203.0.113.20", "open_ports": ports((443, "https"), (1194, "openvpn")), "bound_domains": ["vpn.acme.example"]},
        {"type": "ip", "ip": "203.0.113.25", "open_ports": ports((25, "smtp"), (587, "submission"), (993, "imaps"))},
        {"type": "service", "url": "http://portal.acme.example", "status_code": 200, "page_title": "ACME Customer Portal", "technologies": ["nginx 1.25.3", "Express", "React"]},
        {"type": "service", "url": "http://api.acme.example", "status_code": 200, "technologies": ["nginx 1.25.3", "Express", "PostgreSQL"]},
        {"type": "service", "url": "https://vpn.acme.example", "status_code": 200, "page_title": "GlobalProtect Portal", "technologies": ["PAN-OS"]},
        {"type": "service", "port": 22, "service_name": "ssh", "service_ip": "203.0.113.20"},
        {"type": "service", "port": 25, "service_name": "smtp", "service_ip": "203.0.113.25"},
        {"type": "endpoint", "url": "http://api.acme.example/api/v2/users/{id}", "method": "GET", "params": [{"name": "id", "in": "path"}]},
        {"type": "endpoint", "url": "http://api.acme.example/api/v2/auth/login", "method": "POST", "params": [{"name": "email", "in": "body"}, {"name": "password", "in": "body"}]},
        {"type": "endpoint", "url": "http://api.acme.example/api/v2/search", "method": "POST", "params": [{"name": "q", "in": "body"}]},
        {"type": "endpoint", "url": "http://portal.acme.example/.git/config", "method": "GET"},
    ]
    res = c("insert_assets", assets=assets)["results"]
    a = {}
    for item, r in zip(assets, res):
        a[item.get("domain") or item.get("ip") or item.get("url") or f"{item.get('service_name')}:{item.get('service_ip')}"] = r["id"]

    c("add_hint", hints=[
        {"text": "Client says the API team reuses seasonal passwords for service accounts", "asset_ids": [a["api.acme.example"]]},
        {"text": "VPN appliance is out of scope for exploitation, fingerprint only", "asset_ids": [a["vpn.acme.example"]]},
    ])

    def intent(summary, assets, priority, parents=None):
        args = {"summary": summary, "asset_ids": assets, "priority": priority}
        if parents:
            args["parent_ids"] = parents
        return node_id(c("add_intent", **args))

    def fact(summary, intent_id, assets, evidence, source="target_response"):
        return node_id(c("record_fact", facts=[{"summary": summary, "intent_id": intent_id, "asset_ids": assets, "evidence": evidence, "source": source, "confidence": "observed"}]))

    def finding(name, severity, vulnclass, summary, assets, intent_id, evidence):
        return c("report_finding", name=name, severity=severity, vulnclass=vulnclass, summary=summary, asset_ids=assets, intent_id=intent_id, evidence=evidence)["finding_id"]

    def run(summary, assets, priority, parents=None):
        i = intent(summary, assets, priority, parents)
        c("start_intent", node_id=i)
        return i

    portal, api, vpn, mail = a["portal.acme.example"], a["api.acme.example"], a["vpn.acme.example"], a["mail.acme.example"]
    portal_svc, api_svc, vpn_svc = a["http://portal.acme.example"], a["http://api.acme.example"], a["https://vpn.acme.example"]
    users_ep, login_ep = a["http://api.acme.example/api/v2/users/{id}"], a["http://api.acme.example/api/v2/auth/login"]
    search_ep, git_ep = a["http://api.acme.example/api/v2/search"], a["http://portal.acme.example/.git/config"]

    i1 = run("Enumerate subdomains and map the external attack surface", [a["acme.example"]], 9)
    f1 = fact("Six subdomains resolve; portal and api share an nginx/Express stack", i1, [portal, api, vpn, a["git.acme.example"], mail], "subfinder + dnsx: portal, api, vpn, git, mail", "tool_output")
    c("complete_intent", node_id=i1, result="6 subdomains, 4 IPs, 3 HTTP services mapped")

    i2 = run("Probe the portal for exposed source control and debug files", [portal, portal_svc], 7, [f1])
    f2 = fact("/.git/config is served by the portal and names the internal GitLab remote", i2, [portal_svc, git_ep], "GET /.git/config -> 200, url = git@git.acme.example:web/portal.git")
    fd_git = finding("Exposed .git directory on customer portal", "medium", "info_disclosure", "The portal serves /.git/config, exposing repository metadata and the internal GitLab remote. Full source may be recoverable.", [portal_svc, git_ep], i2, "curl http://portal.acme.example/.git/config -> 200 OK with [remote origin] block")
    c("complete_intent", node_id=i2, result="Exposed .git confirmed")

    i3 = run("Test /api/v2/users/{id} for broken object level authorization", [api, users_ep], 9, [f1])
    f3 = fact("Unauthenticated GET /api/v2/users/{id} returns any user's email, role and MFA state", i3, [users_ep], "users/1 -> admin@acme.example role=admin mfa=false")
    fd_idor = finding("IDOR on /api/v2/users/{id} exposes all customer records", "high", "idor", "The users endpoint returns full profile data for any numeric id with no authentication or ownership check. Enumerating ids 1-3 returned every account including the administrator.", [api, users_ep], i3, "GET /api/v2/users/1 (no auth) -> 200 {email: admin@acme.example, role: admin}")
    c("complete_intent", node_id=i3, result="BOLA confirmed across all ids")

    i4 = run("Password spray service accounts discovered via the IDOR", [api, login_ep], 8, [f3])
    f4 = fact("svc-backup@acme.example authenticates with a seasonal password", i4, [login_ep], "POST /api/v2/auth/login svc-backup / Spring2026! -> 200 + JWT")
    fd_cred = finding("Weak password on svc-backup service account", "critical", "weak_credentials", "The svc-backup service account uses the seasonal password Spring2026! and has no MFA. Login returns a valid JWT with service role, granting API access.", [api, login_ep], i4, "POST /api/v2/auth/login {email: svc-backup@acme.example} -> 200, token issued")
    c("save_task_credential", domain="api.acme.example", username="svc-backup@acme.example", password="Spring2026!", note="service role JWT, no MFA")
    c("complete_intent", node_id=i4, result="1 valid credential")

    i5 = run("Fuzz /api/v2/search for injection", [api, search_ep], 8, [f1])
    f5 = fact("A single quote in q triggers a raw PostgreSQL syntax error", i5, [search_ep], "q=invoice' -> 500 pq: syntax error at or near \"'\"")
    fd_sqli = finding("SQL injection in /api/v2/search", "critical", "sqli", "The q parameter is concatenated into a PostgreSQL query. A single quote produces a database syntax error, and boolean payloads change result counts.", [api, search_ep], i5, "POST /api/v2/search {q: \"invoice'\"} -> 500 pq: syntax error")

    i6 = run("Check internal endpoints listed in robots.txt", [api, api_svc], 6, [f1])
    fact("/api/v2/internal/health leaks the internal database DSN", i6, [api_svc], "db: postgres://portal@db01.int.acme.example:5432/portal")
    fd_dsn = finding("Internal health endpoint discloses database DSN", "low", "info_disclosure", "The unauthenticated health endpoint returns the internal PostgreSQL host, port, user and database name.", [api_svc], i6, "GET /api/v2/internal/health -> 200 {db: postgres://portal@db01.int.acme.example:5432/portal}")
    c("complete_intent", node_id=i6, result="Internal DSN leaked")

    intent("Use svc-backup JWT to reach admin-only API routes", [api, api_svc], 9, [f4])
    intent("Exploit search SQLi to read the users table and password hashes", [api, search_ep], 10, [f5])
    intent("Recover portal source from the exposed .git directory", [portal, portal_svc], 6, [f2])
    intent("Fingerprint the VPN appliance version against known CVEs", [vpn, vpn_svc], 4)
    intent("Check mail.acme.example for open relay and user enumeration", [mail, a["smtp:203.0.113.25"]], 3)

    api_url = "http://api.acme.example/api/v2"
    for fid, key, role in [
        (fd_idor, ("GET", api_url + "/users/1", 200), "proof"),
        (fd_idor, ("GET", api_url + "/users/3", 200), "supporting"),
        (fd_cred, ("POST", api_url + "/auth/login", 200), "proof"),
        (fd_cred, ("POST", api_url + "/auth/login", 401), "baseline"),
        (fd_sqli, ("POST", api_url + "/search", 500), "proof"),
        (fd_sqli, ("POST", api_url + "/search", 200), "baseline"),
        (fd_git, ("GET", "http://portal.acme.example/.git/config", 200), "proof"),
        (fd_dsn, ("GET", api_url + "/internal/health", 200), "proof"),
    ]:
        c("bind_finding_traffic", finding_id=fid, snapshot_id=snap[key], role=role)

    for surface, risk, outcome, evidence in [
        (api_url + "/users/{id}", "idor", "reported", None),
        (api_url + "/search", "sqli", "reported", None),
        (api_url + "/auth/login", "weak_credentials", "reported", None),
        (api_url + "/auth/login", "sqli", "no_issue_found", f4),
        ("http://portal.acme.example/login", "xss", "no_issue_found", f2),
        ("http://portal.acme.example/admin", "auth_bypass", "needs_follow_up", None),
        (api_url + "/search", "xss", "no_issue_found", f5),
        ("https://vpn.acme.example", "rce", "ruled_out", None),
        ("mail.acme.example:25", "open_relay", "needs_follow_up", None),
    ]:
        args = {"surface": surface, "risk_area": risk, "outcome": outcome}
        if evidence:
            args["evidence"] = str(evidence)
        if outcome == "ruled_out":
            args["reason"] = "VPN exploitation excluded by rules of engagement"
        c("record_coverage", **args)

    c("prove_goal", goal_id=goals[1], evidence_id=f3, reason="IDOR returned every customer profile including PII without authentication")


def main():
    send_traffic()
    opener = rest_session()
    seed_workspace(opener)
    snap = traffic_ids(opener)
    seed(MCP(), snap)
    print(f"seeded; web password: {PASSWORD}")


if __name__ == "__main__":
    main()
