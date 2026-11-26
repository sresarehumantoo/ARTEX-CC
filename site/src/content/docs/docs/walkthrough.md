---
title: Example walkthrough
description: "A complete bug bounty engagement against a fictional target, from first contact to final report."
---

This document walks through a complete bug bounty engagement against a
hypothetical target, `app.example.com`, using ARTEX and Claude Code. It
shows the tool calls Claude makes, what comes back, and how the exploration
graph steers the session from first contact to final report.

The target is fictional. The findings are realistic examples of what a bug
bounty program surfaces.

---

## Setup

Create the database, initialize the engagement, and point Claude Code at the
MCP server.

```
createdb ccartex
artex-cc init --pg "postgres://localhost/ccartex" --goal "Bug bounty: app.example.com"
```

The init command prints MCP config and CA trust instructions. Add the config to
`.mcp.json` in your project directory:

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex-cc",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"]
    }
  }
}
```

Trust the CA cert so the recording proxy can intercept HTTPS (the init command
prints the exact command for your OS). Then start Claude Code:

```
claude
```

All 65 MCP tools appear automatically (the `traffic_*` tools and
`bind_finding_traffic` only because the config passes `--proxy`). Claude also reads the
`artex://engagement` resource, which contains the orchestration guide and your
engagement goal. You are ready to go.

```
> Run a bug bounty engagement against app.example.com. The program scope
> includes *.example.com. Focus on authentication, authorization, and
> data exposure.
```

---

## Phase 1: Reconnaissance

### Orient

Claude's first move is always `graph_overview`. The graph is empty: the goal
passed to `init` is the engagement's description in `task.goal`, not a goal node
yet.

```
Claude calls graph_overview()
```

```json
{
  "task": {"goal": "Bug bounty: app.example.com", "description": ""},
  "done_intents_total": 0,
  "frontier_open": 0,
  "findings_total": 0,
  "findings_dismissed": 0,
  "findings_merged": 0,
  "facts": 0,
  "coverage": {"entries": 0, "gaps": 0},
  "changes_cursor": "c1795765200000000..0"
}
```

Empty board. Claude now breaks the high-level goal into testable objectives.

### Define objectives

```
Claude calls set_goals({
  goals: [
    {text: "Identify authentication bypass or broken access control", vulnclass: "idor"},
    {text: "Find sensitive data exposure in API responses", vulnclass: "information-disclosure"},
    {text: "Test for injection vulnerabilities in user-facing inputs", vulnclass: "sqli"}
  ]
})
```

```json
{"created": [
  {"id": 2, "text": "Identify authentication bypass or broken access control", "state": "open"},
  {"id": 3, "text": "Find sensitive data exposure in API responses", "state": "open"},
  {"id": 4, "text": "Test for injection vulnerabilities in user-facing inputs", "state": "open"}
]}
```

Three goals, each independently provable. Claude also sets constraints to stay
within the bug bounty rules:

```
Claude calls set_constraints({
  constraints: [
    {type: "deny", text: "Do not test against production user accounts"},
    {type: "deny", text: "Do not perform denial-of-service testing"},
    {type: "allow", text: "Automated scanning of *.example.com is in scope"}
  ]
})
```

### Seed the root domain

```
Claude calls insert_assets({
  assets: [
    {type: "root_domain", domain: "example.com"}
  ]
})
```

```json
{"results": [{"index": 0, "id": 1, "type": "root_domain"}], "errors": []}
```

### DNS enumeration

```
Claude calls enrich_domain({domain: "example.com"})
```

```json
{
  "domain": "example.com",
  "records": {
    "A": ["93.184.216.34"],
    "MX": ["mail.example.com"],
    "NS": ["ns1.example.com", "ns2.example.com"],
    "TXT": ["v=spf1 include:_spf.google.com ~all"]
  },
  "reverse_dns": {
    "93.184.216.34": ["example.com"]
  },
  "new_assets": ["93.184.216.34"]
}
```

Claude records what it learned and seeds the discovered assets:

```
Claude calls record_fact({
  facts: [{
    summary: "example.com resolves to 93.184.216.34; MX is mail.example.com; SPF includes Google Workspace",
    evidence: "DNS A/MX/TXT records",
    confidence: "observed",
    source: "tool_output"
  }]
})
```

```
fact created: 5
```

Now Claude enriches `app.example.com` specifically, since that is the primary
target:

```
Claude calls enrich_domain({domain: "app.example.com"})
```

```json
{
  "domain": "app.example.com",
  "records": {
    "A": ["93.184.216.40"],
    "CNAME": []
  },
  "reverse_dns": {
    "93.184.216.40": ["app.example.com"]
  },
  "new_assets": ["93.184.216.40"]
}
```

### Register discovered assets

Claude batches everything it has found so far into one insert:

```
Claude calls insert_assets({
  assets: [
    {type: "subdomain", domain: "app.example.com", record_type: "A", record_value: ["93.184.216.40"]},
    {type: "subdomain", domain: "mail.example.com"},
    {type: "ip", ip: "93.184.216.34", bound_domains: ["example.com"]},
    {type: "ip", ip: "93.184.216.40", bound_domains: ["app.example.com"]}
  ]
})
```

```json
{
  "results": [
    {"index": 0, "id": 2, "type": "subdomain"},
    {"index": 1, "id": 3, "type": "subdomain"},
    {"index": 2, "id": 4, "type": "ip"},
    {"index": 3, "id": 5, "type": "ip"}
  ],
  "errors": []
}
```

### Plan the next steps

Claude queues exploration intents for each direction worth pursuing:

```
Claude calls add_intent({
  intents: [
    {summary: "Probe app.example.com:443 for tech stack, headers, and TLS info", asset_ids: [2], priority: 7},
    {summary: "Enumerate additional subdomains via certificate transparency logs", priority: 5},
    {summary: "Scan 93.184.216.40 for additional open services", asset_ids: [5], priority: 6}
  ]
})
```

```json
{"ids": [6, 7, 8]}
```

---

## Phase 2: Service enumeration

### Probe the primary target

Claude picks up the highest-priority open intent and claims it, so no other
session or subagent works it at the same time:

```
Claude calls start_intent({node_id: 6})
```

```
intent 6 is now running; claimed by h-3f9a1c2b7d4e until 2026-11-21T10:30:00Z
```

The claim lasts 30 minutes and renews each time Claude records a fact, finding
or coverage entry against the intent.

```
Claude calls enrich_service({host: "app.example.com", port: 443})
```

```json
{
  "url": "https://app.example.com",
  "status_code": 200,
  "redirect_chain": [],
  "headers": {
    "Server": "nginx/1.24.0",
    "X-Powered-By": "Express",
    "Content-Type": "text/html; charset=utf-8",
    "X-Frame-Options": "SAMEORIGIN",
    "Strict-Transport-Security": "max-age=31536000"
  },
  "tls": {
    "cn": "*.example.com",
    "san": ["*.example.com", "example.com"],
    "issuer": "Let's Encrypt Authority X3",
    "not_after": "2027-03-15T00:00:00Z"
  },
  "technologies": ["nginx", "Express", "Node.js"],
  "page_title": "Example App - Dashboard"
}
```

Claude records the tech stack and registers the service:

```
Claude calls record_fact({
  facts: [{
    summary: "app.example.com runs nginx/1.24.0 reverse-proxying Express/Node.js; wildcard TLS cert from Let's Encrypt; page title 'Example App - Dashboard'",
    intent_id: 6,
    evidence: "Server: nginx/1.24.0, X-Powered-By: Express",
    confidence: "observed",
    source: "tool_output",
    asset_ids: [2]
  }]
})
```

```
Claude calls insert_assets({
  assets: [{
    type: "service",
    url: "https://app.example.com",
    status_code: 200,
    page_title: "Example App - Dashboard",
    technologies: ["nginx", "Express", "Node.js"]
  }]
})
```

```json
{"results": [{"index": 0, "id": 6, "type": "service"}], "errors": []}
```

With the intent's work recorded, Claude closes it, which releases the claim:

```
Claude calls complete_intent({node_id: 6, result: "Tech stack and TLS details recorded; service asset registered"})
```

```
intent 6 marked done
```

### Dispatch subagents for parallel work

With multiple intents open, Claude dispatches subagents to work them in
parallel. Each subagent receives one intent and has access to the same MCP
tools. Subagents share the coordinator's MCP session, so each one passes its
own `owner` to `start_intent` and `complete_intent` to hold a separate claim.

```
Claude launches 2 subagents in parallel:

  Agent 1: "Enumerate subdomains via CT logs and DNS brute-force for *.example.com (intent 7)"
  Agent 2: "Scan 93.184.216.40 for open ports and services beyond 443 (intent 8)"
```

While the subagents run, Claude continues probing the primary target. When the
subagents finish, their results appear in the graph. Here is what
`graph_overview` shows after they complete:

```
Claude calls graph_overview()
```

The overview now shows new facts from the subagents:

```json
{
  "goals": [
    {"id": 2, "text": "Identify authentication bypass or broken access control", "state": "open"},
    {"id": 3, "text": "Find sensitive data exposure in API responses", "state": "open"},
    {"id": 4, "text": "Test for injection vulnerabilities in user-facing inputs", "state": "open"}
  ],
  "open_intents": [],
  "recent_done_intents": [
    {"id": 6, "summary": "Probe app.example.com:443 for tech stack, headers, and TLS info", "state": "done"},
    {"id": 7, "summary": "Enumerate additional subdomains via certificate transparency logs", "state": "done"},
    {"id": 8, "summary": "Scan 93.184.216.40 for additional open services", "state": "done"}
  ],
  "done_intents_total": 3,
  "frontier_open": 0,
  "findings_total": 0,
  "facts": 5,
  "recent_facts": [
    {"id": 12, "summary": "CT logs reveal api.example.com and admin.example.com as additional subdomains"},
    {"id": 13, "summary": "Port scan: 93.184.216.40 has 22/ssh, 443/https, 8080/http open"},
    {"id": 11, "summary": "admin.example.com:8080 returns HTTP 200 with title 'Admin Panel - Login'"},
    {"id": 9, "summary": "app.example.com runs nginx/1.24.0 reverse-proxying Express/Node.js..."}
  ],
  "coverage": {"entries": 0, "gaps": 0}
}
```

The subagents found two new subdomains (`api.example.com`,
`admin.example.com`) and an exposed admin panel on port 8080. Claude plans the
next round:

```
Claude calls add_intent({
  intents: [
    {summary: "Test api.example.com for IDOR on user-facing endpoints", asset_ids: [7], parent_ids: [12], priority: 8},
    {summary: "Probe admin.example.com:8080 for default credentials and authentication bypass", parent_ids: [11], priority: 9},
    {summary: "Register a test account on app.example.com and map authenticated API endpoints", asset_ids: [6], priority: 7},
    {summary: "Test app.example.com input fields for SQL injection and XSS", asset_ids: [6], parent_ids: [9], priority: 6}
  ]
})
```

```json
{"ids": [14, 15, 16, 17]}
```

### Track coverage

As Claude tests each surface and finds nothing, it records negative results so
the graph distinguishes "tested clean" from "not tested":

```
Claude calls record_coverage({
  surface: "app.example.com /login",
  risk_area: "sqli",
  outcome: "no_issue_found",
  intent_id: 17,
  evidence: "tested username and password fields with standard SQL injection payloads; all returned 401 with consistent error message"
})
```

```
recorded
```

---

## Phase 3: Vulnerability testing

### Finding 1: IDOR on the user API

Claude (or a subagent working intent 14) discovers that the API returns user
records without authorization checks. It has been proxying requests through the
recording proxy on `:8788`, so the traffic is captured.

```
Claude calls traffic_search({host: "api.example.com", query: "/api/v2/users"})
```

```json
{
  "count": 3,
  "results": [
    {"id": "a1b2c3d4", "method": "GET", "url": "https://api.example.com/api/v2/users/me", "status": 200, "resp_len": 1842},
    {"id": "e5f6a7b8", "method": "GET", "url": "https://api.example.com/api/v2/users/1", "status": 200, "resp_len": 1837},
    {"id": "c9d0e1f2", "method": "GET", "url": "https://api.example.com/api/v2/users/2", "status": 200, "resp_len": 1920}
  ]
}
```

Claude inspects the unauthorized request:

```
Claude calls traffic_get({id: "e5f6a7b8"})
```

```
--- REQUEST ---
GET /api/v2/users/1 HTTP/1.1
Host: api.example.com
Accept: application/json

--- RESPONSE ---
HTTP/1.1 200 OK
Content-Type: application/json

{"id":1,"email":"alice@example.com","name":"Alice Chen","role":"admin",
 "created_at":"2025-01-15T08:30:00Z","last_login":"2026-11-20T14:22:00Z",
 "phone":"+1-555-0142","address":"742 Evergreen Terrace"}
```

No authentication required. The response includes PII. Claude files the
finding:

```
Claude calls report_finding({
  vulnclass: "idor",
  name: "Unauthenticated IDOR on /api/v2/users/{id}",
  severity: "high",
  summary: "The /api/v2/users/{id} endpoint returns full user profiles including email, phone, and address without requiring authentication. Any user ID can be enumerated sequentially.",
  intent_id: 14,
  asset_ids: [7],
  evidence: "GET /api/v2/users/1 with no auth header returns 200 with full PII (email, phone, address). Verified by requesting /api/v2/users/2, which returned a different user's data."
})
```

```json
{"finding_id": 1, "node_id": 18}
```

Claude binds the captured traffic as proof:

```
Claude calls bind_finding_traffic({
  finding_id: 1,
  snapshot_id: "e5f6a7b8",
  role: "proof",
  note: "Unauthenticated request to /api/v2/users/1 returning full PII"
})
```

```
traffic e5f6a7b8 bound to finding 1 as proof
```

And records coverage:

```
Claude calls record_coverage({
  surface: "api.example.com /api/v2/users/{id}",
  risk_area: "idor",
  outcome: "reported",
  evidence: "finding 1"
})
```

### Finding 2: Exposed admin panel with default credentials

The subagent working intent 15 discovers that `admin.example.com:8080` accepts
the default credentials `admin:admin`.

```
Claude calls save_task_credential({
  domain: "admin.example.com:8080",
  username: "admin",
  password: "admin",
  note: "Default credentials on exposed admin panel"
})
```

```json
{"id": 1, "domain": "admin.example.com:8080", "username": "admin", "source": "agent"}
```

```
Claude calls report_finding({
  vulnclass: "default-credentials",
  name: "Admin panel accessible with default credentials",
  severity: "critical",
  summary: "admin.example.com:8080 exposes an administrative panel that accepts default credentials admin:admin. The panel provides access to user management, system configuration, and application logs.",
  intent_id: 15,
  evidence: "POST /login with body username=admin&password=admin returns 302 to /dashboard. The dashboard shows 'System Administration' with links to User Management, Configuration, and Logs."
})
```

```json
{"finding_id": 2, "node_id": 19}
```

### Finding 3: Missing rate limiting on login

While testing, Claude notices the login endpoint has no rate limiting:

```
Claude calls report_finding({
  vulnclass: "missing-rate-limit",
  name: "No rate limiting on /auth/login endpoint",
  severity: "medium",
  summary: "The /auth/login endpoint on app.example.com accepts unlimited authentication attempts with no throttling, lockout, or CAPTCHA. An attacker can brute-force credentials.",
  intent_id: 16,
  asset_ids: [6],
  evidence: "Sent 50 sequential POST /auth/login requests with invalid credentials in under 10 seconds; all returned 401 with identical response times (~45ms). No 429, no CAPTCHA challenge, no account lockout."
})
```

```json
{"finding_id": 3, "node_id": 20}
```

---

## Phase 4: Wrap-up

### Prove goals

Claude has evidence for two of three goals. It links findings to goals:

```
Claude calls prove_goal({
  goal_id: 2,
  evidence_id: 18,
  reason: "Finding 1 (IDOR on /api/v2/users) and Finding 2 (default admin credentials) demonstrate broken access control"
})
```

(`evidence_id` takes the graph node, so Claude passes the `node_id` 18 that
`report_finding` returned for finding 1.)

```
goal 2 marked met
```

```
Claude calls prove_goal({
  goal_id: 3,
  evidence_id: 18,
  reason: "Finding 1 demonstrates sensitive data exposure: full PII returned without authentication"
})
```

```
goal 3 marked met
```

For the SQL injection goal, Claude tested all identified input surfaces and
found nothing:

```
Claude calls record_coverage({
  surface: "app.example.com /api/v2/*",
  risk_area: "sqli",
  outcome: "no_issue_found",
  intent_id: 17,
  evidence: "tested all identified parameters with error-based, blind, and time-based payloads; all returned expected error responses with consistent timing"
})
```

The third goal remains open (no injection found), but all surfaces have been
tested. Claude moves to report generation.

### Generate the report

```
Claude calls generate_report()
```

The tool produces a full Markdown penetration test report:

```markdown
# Penetration Test Report — Bug bounty: app.example.com

- **Task goal**: Bug bounty: app.example.com
- **Generated at**: 2026-11-21 10:42:17

## Summary

- Confirmed findings: **3**
- Assets: ip 2, root_domain 1, service 1, subdomain 4

## Findings

### 1. [CRITICAL] Admin panel accessible with default credentials
...

### 2. [HIGH] Unauthenticated IDOR on /api/v2/users/{id}
...

### 3. [MEDIUM] No rate limiting on /auth/login endpoint
...
```

### Close the engagement

```
Claude calls goal_met({
  summary: "3 findings reported (1 critical, 1 high, 1 medium). Auth bypass and data exposure goals proved. SQL injection testing completed with no issues found across all identified surfaces."
})
```

```
engagement 1 marked complete: 3 findings reported (1 critical, 1 high, 1 medium). Auth bypass and data exposure goals proved. SQL injection testing c…
```

---

## Key concepts

### The exploration graph is a task board

Every piece of state lives in the graph, not in Claude's context window. Goals,
intents, facts, findings, credentials, and coverage records are all graph nodes.
Claude reads the board (`graph_overview`), plans work (`add_intent`), executes
it, and writes results back (`record_fact`, `report_finding`,
`record_coverage`).

This means:
- Nothing is lost when Claude's context fills up.
- The human can review the plan at any time.
- Multiple sessions can contribute to the same engagement.

### `graph_overview` keeps Claude oriented

Claude calls `graph_overview` at the start of every planning cycle. After
subagents finish, it calls it again. The overview shows what has been done, what
is in progress, and what needs doing next. It is the single source of truth.

If you start a fresh Claude Code session against the same database, Claude calls
`graph_overview` and picks up exactly where the previous session left off. All
facts, findings, and intents are still there.

### Subagent dispatch for parallel work

When the graph has multiple independent intents, Claude dispatches subagents to
work them in parallel. Each subagent gets one intent, has access to the same
MCP tools, claims its intent with `start_intent`, and writes its results
directly into the graph. The coordinator (Claude) reads `graph_overview` after
the subagents finish and plans the next round.

The pattern:

1. `graph_overview` to see open intents
2. Launch one Agent per intent (parallel, in one message); each claims its
   intent, records results, and finishes with `complete_intent`
3. When agents finish, `graph_overview` again
4. Plan next intents based on new facts
5. Repeat

### Context recovery

The graph survives context exhaustion. If Claude runs out of context or you
start a new session:

1. Claude calls `graph_overview`.
2. It sees all goals, completed and open intents, facts, findings.
3. Running intents show who holds them (`claimed_by`) and when the claim
   expires. A claim lapses after 30 minutes without renewal, and the intent is
   listed as open again (with `claim_expired`) for any session to
   `start_intent`.
4. Work continues from where it left off.

You never lose work. Facts, findings, and credentials are persisted the moment
they are recorded.

### Hands-free operation with `/loop`

For continuous autonomous operation:

```
/loop 5m Continue the pentest: call graph_overview, assess progress, dispatch subagents for open intents, prove goals when evidence exists.
```

Claude runs the prompt every 5 minutes: checking the graph, dispatching work,
recording results. You can watch it run, steer it with `add_hint`, or walk
away and review the findings later.
