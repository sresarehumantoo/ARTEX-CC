#!/usr/bin/env python3
"""Capture the static web demo fixtures (web/src/demo/fixtures/) from the
seeded artex-shots stack.

Run through demo-fixtures.sh, which starts the stack and runs seed.py first.
On top of that seed this adds an activity timeline and two scripted chat
conversations, then records every REST response the dashboard reads, scrubs
it and stretches the few seconds the seed took into a plausible afternoon.
"""
import importlib.util
import ipaddress
import json
import pathlib
import re
import subprocess
import sys
import uuid
from datetime import datetime, timedelta, timezone
from urllib.parse import quote

here = pathlib.Path(__file__).resolve().parent
out_dir = here.parent.parent / "web" / "src" / "demo" / "fixtures"

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("seed", here / "seed.py")
seed = importlib.util.module_from_spec(spec)
spec.loader.exec_module(seed)

BASE = seed.BASE
CONTAINER = seed.CONTAINER
PG = "artex-shots-postgres-1"
SPAN = timedelta(hours=3)


def psql(sql):
    return subprocess.check_output(["docker", "exec", "-i", PG, "psql", "-U", "ccartex", "-d", "ccartex", "-At", "-v", "ON_ERROR_STOP=1"], input=sql.encode()).decode()


def node_times():
    rows = psql("SELECT id, created_at, updated_at FROM exploration_nodes ORDER BY id").splitlines()
    out = {}
    for row in rows:
        nid, created, updated = row.split("|")
        out[int(nid)] = (datetime.fromisoformat(created), datetime.fromisoformat(updated))
    return out


def seed_activity():
    psql("""
DELETE FROM activity;

INSERT INTO activity (exploration_id, node_id, worker, kind, tool, summary, detail, created_at)
SELECT n.exploration_id, n.id,
       CASE WHEN n.kind = 'intent' THEN 'pentest-lead'
            WHEN n.payload::text ILIKE '%subdomain%' THEN 'recon-specialist'
            ELSE 'web-tester' END,
       CASE n.kind WHEN 'fact' THEN 'node' ELSE n.kind END,
       CASE n.kind WHEN 'intent' THEN 'add_intent' WHEN 'fact' THEN 'record_fact' ELSE 'report_finding' END,
       CASE n.kind WHEN 'intent' THEN 'Planned: ' || (n.payload->>'summary')
                   WHEN 'fact' THEN n.payload->>'summary'
                   ELSE 'Reported ' || (n.payload->>'severity') || ': ' || (n.payload->>'name') END,
       COALESCE(n.payload->>'evidence', ''),
       n.created_at
FROM exploration_nodes n WHERE n.kind IN ('intent', 'fact', 'finding') AND n.state <> 'origin' ORDER BY n.id;

INSERT INTO activity (exploration_id, node_id, worker, kind, tool, summary, detail, created_at)
SELECT exploration_id, id, 'pentest-lead', 'intent', 'complete_intent',
       'Completed: ' || (payload->>'summary'), payload->>'result', updated_at
FROM exploration_nodes WHERE kind = 'intent' AND state = 'done';

INSERT INTO activity (exploration_id, worker, kind, tool, summary, detail, created_at)
SELECT 1, 'recon-specialist', 'asset', 'insert_assets', 'Inserted 19 assets from subdomain and port enumeration',
       '6 subdomains, 4 IPs, 5 services, 4 endpoints', min(created_at) - interval '1 millisecond'
FROM exploration_nodes WHERE kind = 'intent';

INSERT INTO activity (exploration_id, worker, kind, tool, is_error, summary, detail, created_at)
SELECT 1, 'recon-specialist', 'node', 'Bash', true, 'nmap -sV timed out against 203.0.113.25',
       'Ports 993 and 587 filtered; retried with -T3', created_at + interval '5 milliseconds'
FROM exploration_nodes WHERE id = 8;

INSERT INTO activity (exploration_id, worker, kind, tool, summary, detail, created_at)
SELECT 1, 'web-tester', 'coverage', 'record_coverage', 'Coverage: ' || risk_area || ' on ' || surface || ' (' || outcome || ')',
       COALESCE(reason, ''), updated_at
FROM coverage_ledger;
""")


def tool(name, tid, inp, result, error=False):
    return [
        {"kind": "tool_use", "tool_name": name, "tool_id": tid, "input": inp},
        {"kind": "tool_result", "tool_id": tid, "text": result, "is_error": error},
    ]


def conversations(times):
    recon = [
        {"kind": "user", "text": "Start recon on acme.example. Scope is the root domain and 203.0.113.0/24."},
        {"kind": "thinking", "text": "Orient first with graph_overview, then enumerate subdomains and record what resolves."},
        *tool("mcp__artex__graph_overview", "t1", {}, '{"goal": "Obtain domain admin on the ACME Corp external perimeter", "assets": 0, "open_intents": 0}'),
        {"kind": "text", "text": "The engagement is fresh. I'll enumerate subdomains and resolve them before inserting assets."},
        *tool("Bash", "t2", {"command": "subfinder -silent -d acme.example | dnsx -silent -a -resp", "description": "Enumerate and resolve subdomains"},
              "portal.acme.example [203.0.113.10]\napi.acme.example [203.0.113.11]\nvpn.acme.example [203.0.113.20]\nmail.acme.example [203.0.113.25]\ngit.acme.example [acme.gitlab-host.example]\n"),
        *tool("Bash", "t3", {"command": "nmap -sT -p 22,25,80,443,587,993,1194,8443 203.0.113.10-25", "description": "Port scan in-scope hosts"},
              "203.0.113.10: 80/tcp open http, 443/tcp open https\n203.0.113.11: 443/tcp open https, 8443/tcp open https-alt\n203.0.113.20: 443/tcp open https, 1194/tcp open openvpn\n203.0.113.25: 25/tcp open smtp\n"),
        *tool("mcp__artex__insert_assets", "t4", {"assets": [{"type": "subdomain", "domain": "portal.acme.example"}, {"type": "subdomain", "domain": "api.acme.example"}]},
              '{"results": [{"index": 0, "id": 2, "type": "subdomain"}, {"index": 1, "id": 3, "type": "subdomain"}], "errors": []}'),
        {"kind": "text", "text": "## Recon summary\n\n| Host | IP | Services |\n|------|----|----------|\n| portal.acme.example | 203.0.113.10 | http, https |\n| api.acme.example | 203.0.113.11 | https, https-alt |\n| vpn.acme.example | 203.0.113.20 | https, openvpn |\n| mail.acme.example | 203.0.113.25 | smtp |\n\nThe portal and API share an nginx/Express stack. Next up: the API's object-level authorization."},
        {"kind": "result", "usage": {"input_tokens": 18420, "output_tokens": 2310, "cache_read_tokens": 41200, "cache_creation_tokens": 6300, "cost_usd": 0.4172, "duration_ms": 94000, "num_turns": 6}},
    ]
    api = [
        {"kind": "user", "text": "Test the users API for IDOR, then check whether any service account has a weak password."},
        *tool("Bash", "t1", {"command": "curl -s http://api.acme.example/api/v2/users/1", "description": "Fetch user 1 without auth"},
              '{\n  "id": 1,\n  "email": "admin@acme.example",\n  "role": "admin",\n  "mfa": false\n}'),
        *tool("Bash", "t2", {"command": "for i in 2 3; do curl -s http://api.acme.example/api/v2/users/$i; done", "description": "Enumerate more ids"},
              '{"id": 2, "email": "j.doe@acme.example", "role": "user", "mfa": true}\n{"id": 3, "email": "svc-backup@acme.example", "role": "service", "mfa": false}'),
        {"kind": "text", "text": "Every id returns a full profile with no session at all. That is a textbook BOLA, and it leaks a service account."},
        *tool("mcp__artex__report_finding", "t3", {"name": "IDOR on /api/v2/users/{id} exposes all customer records", "severity": "high", "vulnclass": "idor"}, '{"finding_id": 2}'),
        *tool("Bash", "t4", {"command": "sqlmap -u http://api.acme.example/api/v2/search --data '{\"q\":\"x\"}' --batch"}, "sh: sqlmap: command not found", True),
        *tool("Bash", "t5", {"command": "curl -s -X POST http://api.acme.example/api/v2/auth/login -H 'Content-Type: application/json' -d '{\"email\":\"svc-backup@acme.example\",\"password\":\"Spring2026!\"}'", "description": "Try a seasonal password"},
              '{"token": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIzIiwicm9sZSI6InNlcnZpY2UifQ.c2lnbmF0dXJl", "user": {"id": 3, "role": "service"}}'),
        *tool("mcp__artex__save_task_credential", "t6", {"domain": "api.acme.example", "username": "svc-backup@acme.example", "note": "service role JWT, no MFA"}, "credential saved: 1"),
        {"kind": "text", "text": "`svc-backup` accepts **Spring2026!** and returns a service-role JWT. I saved the credential and reported it as critical. Suggested next intent: use that token against the admin-only routes."},
        {"kind": "result", "usage": {"input_tokens": 26310, "output_tokens": 3870, "cache_read_tokens": 58800, "cache_creation_tokens": 7100, "cost_usd": 0.2216, "duration_ms": 131000, "num_turns": 9}},
    ]
    start = times[7][0] - timedelta(milliseconds=40)
    mid = times[12][0] - timedelta(milliseconds=20)
    return [
        ("Recon acme.example", "opus", "pentest-lead", start, recon),
        ("API authorization testing", "sonnet", "web-tester", mid, api),
    ]


def write_conversations(times):
    for title, model, agent, start, events in conversations(times):
        cid = str(uuid.uuid5(uuid.NAMESPACE_URL, "artex-demo/" + title))
        step = timedelta(milliseconds=8)
        lines = []
        for i, ev in enumerate(events, 1):
            ev = {"seq": i, **ev, "time": (start + step * i).isoformat().replace("+00:00", "Z")}
            if not ev.get("is_error"):
                ev.pop("is_error", None)
            lines.append(json.dumps(ev))
        end = start + step * len(events)
        meta = {"id": cid, "title": title, "model": model, "agent": agent, "session_id": str(uuid.uuid5(uuid.NAMESPACE_URL, cid)),
                "status": "idle", "created_at": start.isoformat().replace("+00:00", "Z"), "updated_at": end.isoformat().replace("+00:00", "Z")}
        d = f"/app/data/conversations/{cid}"
        script = f"mkdir -p {d} && cat > {d}/meta.json && printf '%s\\n' \"$@\" > {d}/events.jsonl"
        subprocess.run(["docker", "exec", "-i", CONTAINER, "sh", "-c", script, "sh", *lines], input=json.dumps(meta).encode(), check=True)


class Client:
    def __init__(self):
        self.opener = seed.rest_session()

    def get(self, path, raw=False):
        body = self.opener.open(BASE + path).read().decode()
        return body if raw else json.loads(body)


def capture():
    c = Client()
    routes = {}

    def keep(path, raw=False):
        routes[path] = c.get(path, raw)
        return routes[path]

    for p in ["/api/engagement", "/api/graph", "/api/findings", "/api/coverage", "/api/coverage/gaps", "/api/credentials",
              "/api/constraints", "/api/tasks", "/api/system", "/api/tools", "/api/mcp/servers", "/api/skills", "/api/agents",
              "/api/llm/settings", "/api/llm/status", "/api/usage", "/api/usage/totals", "/api/commands/stats",
              "/api/claude/status", "/api/intercept/rules", "/api/approvals", "/api/conversations"]:
        keep(p)
    for t in routes["/api/tasks"]:
        keep(f"/api/tasks/{t['id']}/goals")
    for f in routes["/api/findings"]:
        keep(f"/api/findings/{f['id']}")
    for a in routes["/api/agents"]:
        keep(f"/api/agents/{a['id']}")
    for conv in routes["/api/conversations"]:
        keep(f"/api/conversations/{conv['id']}")
        keep(f"/api/conversations/{conv['id']}/messages")

    assets = c.get("/api/assets?limit=1000")["assets"]
    for a in assets:
        keep(f"/api/assets/{a['id']}")
        keep(f"/api/assets/{a['id']}/findings")
    traffic = c.get("/api/traffic?size=500")["rows"]
    for t in traffic:
        keep(f"/api/traffic/{t['id']}")

    workspace = {"dirs": {}, "files": {}}

    def walk(path):
        entries = c.get("/api/workspace?path=" + quote(path)) or []
        workspace["dirs"][path] = entries
        for e in entries:
            child = f"{path}/{e['name']}" if path else e["name"]
            if e["type"] == "dir":
                walk(child)
            elif e["size"] < 64_000:
                workspace["files"][child] = c.get("/api/workspace/file?path=" + quote(child), raw=True)

    walk("")

    return {
        "routes": routes,
        "assets": assets,
        "traffic": traffic,
        "activity": c.get("/api/activity?limit=500") or [],
        "logs": c.get("/api/logs") or [],
        "commands": c.get("/api/commands?limit=1000") or [],
        "workspace": workspace,
    }


TS = re.compile(r"(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}:\d{2}(?:\.\d+)?)(Z|[+-]\d{2}(?::?\d{2})?)")
IP = re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b")
DOC_NETS = [ipaddress.ip_network(n) for n in ("192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "127.0.0.0/8", "0.0.0.0/32")]
HOST = re.compile(r"\b(?:[a-z0-9-]+\.)+(?:com|net|org|io|dev|app|local|lan|internal|corp|home|cloud|ai|co|me)\b", re.I)
ALLOWED_HOSTS = {"example.com", "example.net", "example.org", "artex-cc.com", "github.com"}


def parse_ts(m):
    frac = m.group(2)
    if "." in frac:
        whole, digits = frac.split(".")
        frac = f"{whole}.{digits[:6].ljust(6, '0')}"
    zone = m.group(3)
    if zone == "Z":
        zone = "+00:00"
    elif len(zone) == 3:
        zone += ":00"
    return datetime.fromisoformat(f"{m.group(1)}T{frac}{zone}")


def walk_strings(value, fn):
    if isinstance(value, dict):
        return {k: walk_strings(v, fn) if not (k == "ts" and isinstance(v, (int, float))) else fn(v, ts=True) for k, v in value.items()}
    if isinstance(value, list):
        return [walk_strings(v, fn) for v in value]
    if isinstance(value, str):
        return fn(value)
    return value


def collect_times(data):
    found = []

    def fn(v, ts=False):
        if ts:
            found.append(datetime.fromtimestamp(v, timezone.utc))
        else:
            found.extend(parse_ts(m) for m in TS.finditer(v))
        return v

    walk_strings(data, fn)
    return found


def stretch(data, first, last):
    """Stretch the seed window [first, last] to SPAN, ending at last.

    Earlier times (server start-up) are clamped to just before the seed so a
    slow docker build does not push them into last week. Later times (the
    capture itself) keep their real spacing after the window.
    """
    k = SPAN / (last - first)

    def remap(t):
        t = max(t, first - timedelta(milliseconds=200))
        return last - (last - t) * k if t <= last else t

    def fn(v, ts=False):
        if ts:
            return int(remap(datetime.fromtimestamp(v, timezone.utc)).timestamp())
        return TS.sub(lambda m: remap(parse_ts(m)).isoformat(timespec="milliseconds").replace("+00:00", "Z"), v)

    return walk_strings(data, fn), max(remap(t) for t in collect_times(data))


def scrub(data):
    seen_ips, hosts = {}, set()

    def fn(v, ts=False):
        if ts:
            return v

        def ip(m):
            try:
                addr = ipaddress.ip_address(m.group(0))
            except ValueError:
                return m.group(0)
            if any(addr in n for n in DOC_NETS):
                return m.group(0)
            return seen_ips.setdefault(m.group(0), f"192.0.2.{100 + len(seen_ips)}")

        v = IP.sub(ip, v)
        for m in HOST.finditer(v):
            if m.group(0).lower() not in ALLOWED_HOSTS:
                hosts.add(m.group(0))
        return v

    data = walk_strings(data, fn)
    for orig, repl in seen_ips.items():
        print(f"scrubbed IP {orig} -> {repl}", file=sys.stderr)
    for h in sorted(hosts):
        print(f"review hostname: {h}", file=sys.stderr)
    return data


def main():
    times = node_times()
    seed_activity()
    write_conversations(times)
    data = capture()
    data["routes"]["/api/auth/status"] = {"auth_required": False, "initialized": True}
    data = scrub(data)
    seed_times = [t for nid, pair in times.items() if nid != 1 for t in pair]
    data, captured = stretch(data, min(seed_times), max(seed_times))

    out_dir.mkdir(parents=True, exist_ok=True)
    for name, value in data.items():
        (out_dir / f"{name}.json").write_text(json.dumps(value, indent=1, ensure_ascii=False) + "\n")
    (out_dir / "meta.json").write_text(json.dumps({"captured_at": captured.isoformat(timespec="milliseconds").replace("+00:00", "Z")}) + "\n")
    print(f"wrote {out_dir}")


if __name__ == "__main__":
    main()
