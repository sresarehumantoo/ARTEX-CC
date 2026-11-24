# Tool Reference

Complete parameter documentation for ARTEX's 59 MCP tools, organized by category.

## Table of contents

1. [Exploration graph](#exploration-graph) (12 tools)
2. [Assets and scope](#assets-and-scope) (8 tools)
3. [Findings](#findings) (5 tools)
4. [Credentials](#credentials) (2 tools)
5. [Evidence](#evidence) (4 tools)
6. [Enrichment and ingestion](#enrichment-and-ingestion) (5 tools)
7. [Engagement management](#engagement-management) (9 tools)
8. [Activity](#activity) (2 tools)
9. [Reporting and export](#reporting-and-export) (3 tools)
10. [Traffic](#traffic) (9 tools)

---

## Exploration graph

### `graph_overview`

The task board: goals and their states, open/running/done intents, recent facts, findings, credentials, coverage. Call this first at the start of every planning cycle.

_No parameters._

**Returns:** JSON object with goals, intents (by state), recent facts, findings, credentials, coverage stats, and strategic hints.

### `add_intent`

Generate exploration directions into the frontier. An intent is an open exploration direction described in one summary sentence.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| intents | array of objects | no | Preferred: array of exploration directions to add (batch) |
| summary | string | no | Single-mode: one sentence describing this exploration direction |
| asset_ids | array of integer | no | Target asset IDs this direction tests/attacks |
| parent_ids | array of integer | no | Upstream anchor IDs: confirmed fact/finding nodes this direction is synthesized from |
| priority | integer | no | Priority 0-10, default 5 |

Use either `intents` (batch, preferred) or the top-level `summary` (single). Each `intents` element accepts `summary`, `asset_ids`, `parent_ids`, and `priority`.

**Returns:** Single mode: `"intent created: <id>"`. Batch mode: `{ids: [...], errors: {...}}`.

### `record_fact`

Write an exploration fact or conclusion into the graph, linked to the intent that produced it.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| facts | array of objects | yes | The facts to record (usually one). Each element takes the fields below. |
| facts[].summary | string | yes | One-sentence summary of this conclusion |
| facts[].intent_id | integer | no | The intent that produced this fact |
| facts[].detail | string | no | Detailed description |
| facts[].evidence | string | no | One line of key evidence |
| facts[].confidence | string | no | `observed` (seen directly) or `inferred` |
| facts[].source | string | no | `target_response`, `tool_output`, `third_party_intel`, `operator_input`, or `inference` (default) |
| facts[].asset_ids | array of integer | no | Asset IDs explicitly involved |

**Returns:** Single fact: `"fact created: <id>"`. Multiple facts: `{ids: [...], errors: {...}}`.

### `list_facts`

Page through exploration facts/conclusions, newest first (compact: id + summary + state + origin + source).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| limit | integer | no | Number of entries to return, default 20, max 100 |

**Returns:** `{facts: [{id, summary, state, origin, source}, ...], total}`.

### `node_detail`

Fetch the full content of an exploration graph node by ID. For exploration node IDs from list_facts/list_findings/graph_overview only; use list_assets for assets.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | integer | yes | Exploration graph node ID |

**Returns:** Full node object with kind, state, payload, priority, origin, timestamps, and edges.

### `add_hint`

Attach a strategic hint to the exploration graph; the planner reads it the next time it generates intents.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| hints | array of objects | no | Preferred: array of hints, each with `text` and optional `asset_ids` |
| text | string | no | Single-mode: hint content |
| asset_ids | array of integer | no | Asset IDs to anchor the hint to |

**Returns:** Single mode: `"hint created: <id>"`. Batch mode: `{ids: [...], errors: {...}}`.

### `set_goals`

Add exploration goals to this task. A goal is a final deliverable/verifiable result, not an attack step.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| goals | array of objects | no | Preferred: array of goals, each with `text` and optional `vulnclass` |
| text | string | no | Single-mode: one independently verifiable final goal |
| vulnclass | string | no | Corresponding vulnerability class |

**Returns:** Single mode: `"goal created: <id>"`. Batch mode: `{ids: [...], errors: {...}}`.

### `set_constraints`

Add operation constraints to bound exploration: allow (operations permitted) or deny (operations forbidden).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| constraints | array of objects | no | Preferred: array of constraints, each with `text` and optional `type` |
| text | string | no | Single-mode: the content of one operation constraint |
| type | string | no | `allow` or `deny`; defaults to `deny` |

**Returns:** Single mode: `"constraint created: <id>"`. Batch mode: `{ids: [...], errors: {...}}`.

### `prove_goal`

Link evidence (a finding or fact) to a goal and mark the goal as met. A fact counts as evidence only when its confidence is `observed`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| goal_id | integer | yes | Goal node ID |
| evidence_id | integer | yes | ID of the finding or fact node that proves it |
| reason | string | no | Why this evidence satisfies the goal |

**Returns:** `"goal <id> marked met"` (or `"goal <id> marked met; all goals of this task are met"`).

### `start_intent`

Mark an open intent as running. Call this when you begin working on an intent.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| node_id | integer | yes | Intent node ID |

**Returns:** `"intent <id> is now running"`.

### `complete_intent`

Mark a running intent as done with a result summary. Call this when you have finished working on an intent and recorded all facts/findings.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| node_id | integer | yes | Intent node ID |
| result | string | no | Brief result summary |

**Returns:** `"intent <id> marked done"`.

### `cancel_intent`

Cancel an intent that is no longer needed. Use when an intent is redundant, no longer applicable, or was created in error.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| node_id | integer | yes | Intent node ID |
| reason | string | no | Why this intent is being cancelled |

**Returns:** `"intent <id> cancelled"`.

---

## Assets and scope

### `insert_assets`

Register newly discovered assets in bulk; multiple types can be mixed in one call.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| assets | array of objects | yes | Asset array; each element is one asset record (max 500) |
| assets[].type | string | yes | `root_domain`, `ip`, `subdomain`, `app`, `service`, or `endpoint` |
| assets[].domain | string | varies | Root domain or subdomain (required for root_domain/subdomain) |
| assets[].icp | string | no | ICP registration number |
| assets[].record_type | string | no | DNS record type: A/AAAA/CNAME/MX etc. (for subdomain) |
| assets[].record_value | array of string | no | DNS record values (for subdomain) |
| assets[].ip | string | varies | IPv4/IPv6 address (required for ip type, must not be a hostname) |
| assets[].bound_domains | array of string | no | Domains bound to this IP |
| assets[].open_ports | array of objects | no | Open ports, each with `port` (required) and `service` (optional) |
| assets[].app_name | string | varies | Application name (required for app type) |
| assets[].bundle_id | string | no | Bundle ID (for app) |
| assets[].category | string | no | Application category |
| assets[].description | string | no | Application description |
| assets[].app_icp | string | no | Application ICP registration |
| assets[].company_id | integer | no | Owning company ID |
| assets[].url | string | varies | Full URL including scheme and port (required for HTTP service and endpoint) |
| assets[].status_code | integer | no | HTTP response status code |
| assets[].content_length | integer | no | HTTP response body size in bytes |
| assets[].page_title | string | no | Page `<title>` content |
| assets[].favicon_mmh3 | string | no | Favicon MMH3 hash |
| assets[].technologies | array of string | no | Tech-stack list (append-merged, not overwritten) |
| assets[].auth | array of objects | no | Discovered credentials (append-merged, not overwritten) |
| assets[].service_name | string | varies | Service name, e.g. ssh/mysql/redis (required for non-HTTP service) |
| assets[].service_ip | string | no | Enrichment IP |
| assets[].port | integer | varies | Port number (required for non-HTTP service) |
| assets[].method | string | varies | HTTP method (required for endpoint) |
| assets[].params | array of objects | no | Request parameters (append-merged) |

**Returns:** `{results: [{index, id, type}, ...], errors: [{index, error}, ...]}`.

### `list_assets`

Query the asset library: search by DSL expression, or fetch directly by id/ids; supports pagination.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| dsl | string | varies | DSL query expression (required when id/ids are not passed) |
| type | string | no | Asset type filter: `root_domain`, `ip`, `subdomain`, `app`, `service`, `endpoint` |
| id | integer | no | Fetch by a single asset ID |
| ids | array of integer | no | Fetch by multiple asset IDs |
| limit | integer | no | Return cap, default 10 |
| offset | integer | no | Pagination offset, default 0 |

DSL syntax: `field=value` (fuzzy/ILIKE), `field==value` (exact), `field!=value` (exclusion), numeric fields support `>`, `>=`, `<`, `<=`, bare word is full-text fuzzy. Combine with AND/OR (AND binds tighter), parentheses for grouping. Available fields: `domain`, `root_domain`, `ip`, `url`, `page_title`, `icp`, `service_name`, `app_name`, `method`, `service_type`, `record_type`, `technology`, `port`, `status_code`, `company_id`.

**Returns:** `{count, assets: [...]}`.

### `list_companies`

List companies in the asset library with their asset scopes and attributed asset counts.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| search | string | no | Fuzzy filter by company name (case-insensitive) |

**Returns:** `{count, companies: [...]}`.

### `add_company_scope`

Add domains/IPs/CIDRs/ICP registrations/company keywords to a company's asset scope. Company names are unique: if the company doesn't exist it is created.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| company | string | yes | Company name (created if absent, reused if present) |
| scope | string | yes | Asset scope, one entry per line (auto-recognized: root domain / URL / single IP / CIDR range / ICP / company keyword) |
| reason | string | no | Attribution basis (evidence/source) |
| logo | string | no | Company logo URL |

**Returns:** `{company_id, added, skipped, invalid, errors?}`.

### `add_task_scope`

Add test scope to this task (coverage scope, not enforced): it is the denominator of asset test coverage.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| entries | array of objects | no | Batch: `[{kind, value}, ...]` |
| kind | string | no | `company`, `root_domain`, `subdomain`, `ip`, `cidr`, `icp`, or `keyword` |
| value | string | no | Company name or ID / domain / IP / CIDR / ICP / keyword |
| reason | string | no | Basis for adding (for audit) |

Use `entries` for batch or `kind`+`value` for a single entry.

**Returns:** `{added: [{kind, value}, ...], errors?}`.

### `list_untested_assets`

Query assets inside the scope of this task that haven't been covered by fact anchors yet.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| type | string | no | Asset type filter: `root_domain`, `subdomain`, `service`, `app`, `endpoint`, `ip` |
| page | integer | no | Page number, starting at 1 (default 1) |
| page_size | integer | no | Entries per page (default 10) |

**Returns:** `{assets: [...], total, page, page_size}`.

### `check_scope`

Check whether a target (domain, IP, or URL) is within the engagement's scope rules.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| target | string | yes | Domain, IP address, or URL to check |

**Returns:** `{in_scope: bool, reason, matching_rule}`.

### `record_coverage`

Record what you examined on a surface for one risk class, including negative results, so an absent finding is never read as a clean target.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| surface | string | yes | What was examined: a URL, endpoint, parameter, host, or feature |
| risk_area | string | yes | Risk class, e.g. `sqli`, `xss`, `idor` |
| outcome | string | yes | `reported`, `no_issue_found`, `ruled_out`, `not_applicable`, or `needs_follow_up` |
| evidence | string | no | Required for `no_issue_found`: a fact ID, intent ID, or traffic ID that shows the test |
| reason | string | no | Required for `ruled_out` and `not_applicable`: why |

**Returns:** `"recorded"`.

---

## Findings

### `report_finding`

Record a confirmed finding, with evidence providing verifiable proof such as command output and logs.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| vulnclass | string | yes | Vulnerability class |
| severity | string | yes | `critical`, `high`, `medium`, or `low` |
| summary | string | yes | Finding summary |
| name | string | no | Vulnerability name |
| intent_id | integer | no | The current task's intent ID |
| asset_ids | array of integer | no | Affected asset IDs |
| evidence | string | no | Evidence/PoC text |

**Returns:** `{finding_id, possible_duplicates?}`.

### `list_findings`

List the confirmed findings of this task (compact: id + vulnclass + severity + summary + state + status + traffic_count).

_No parameters._

**Returns:** Array of finding summaries.

### `bind_finding_traffic`

Link a traffic exchange (snapshot) to a finding as evidence. Use traffic_search to find the snapshot_id first.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | yes | Finding ID |
| snapshot_id | string | yes | Traffic snapshot ID from traffic_search/traffic_get |
| role | string | no | `baseline`, `proof`, `verification`, or `supporting` (default: `supporting`) |
| note | string | no | Note about this traffic evidence |

**Returns:** `"traffic <id> bound to finding <id> as <role>"`.

### `merge_findings`

Merge two or more findings into a single primary finding. Evidence and assets from secondary findings are folded into the primary.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_ids | array of integer | yes | All finding IDs to merge (must include primary_id) |
| primary_id | integer | yes | The finding to keep as the merged result |

**Returns:** Merged finding object.

### `dismiss_finding`

Mark a finding as dismissed (false positive). Dismissed findings are excluded from generate_report output.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | yes | The finding to dismiss |
| reason | string | yes | Why this finding is a false positive |

**Returns:** `"finding <id> dismissed: <reason>"`.

---

## Credentials

### `save_task_credential`

Record (or update) a test account for this task's target. Same (domain, username) is overwritten in place, not duplicated.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| domain | string | yes | Domain or host the account belongs to |
| username | string | yes | Login name or email used |
| password | string | yes | The exact password that was set |
| note | string | no | Optional context |

**Returns:** `{id, domain, username, source}`.

### `list_task_credentials`

List this task's test accounts: user-declared accounts plus accounts recorded by earlier runs.

_No parameters._

**Returns:** `{credentials: [...]}`.

---

## Evidence

### `save_evidence`

Save a piece of evidence (screenshot, PoC output, config extract, etc.) linked to a finding. Content is base64-encoded.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | yes | Finding to attach this evidence to |
| kind | string | yes | `screenshot`, `poc_output`, `request_response`, `config_file`, `log_extract`, or `other` |
| filename | string | yes | Original filename for the artifact |
| content_base64 | string | yes | Base64-encoded file content (50 MB limit) |

**Returns:** `{id, sha256, size, path}`.

### `list_evidence`

List evidence artifacts for the current engagement, optionally filtered by finding.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | no | Filter by finding ID |

**Returns:** `{count, evidence: [{id, finding_id, kind, filename, size, sha256, created_at}, ...]}`.

### `get_evidence`

Read evidence content by ID. Returns base64-encoded data.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | yes | Evidence ID |

**Returns:** `{id, filename, kind, content_base64, sha256}`.

### `capture_command_output`

Run a command and save its combined stdout+stderr as evidence (kind=poc_output). The command must only target in-scope hosts. 30-second timeout.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | yes | Finding to attach this evidence to |
| command | string | yes | Shell command to execute |
| description | string | no | What this command demonstrates |

**Returns:** `{evidence_id, exit_code, output_preview}`.

---

## Enrichment and ingestion

### `enrich_domain`

Run DNS enrichment on a domain: resolves A/AAAA/CNAME/MX/NS/TXT records and performs reverse DNS on discovered IPs.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| domain | string | yes | Domain or subdomain to enrich |
| timeout | integer | no | Timeout in seconds (default 30) |

**Returns:** Discovered DNS records, reverse-DNS results, and any new assets (IPs, subdomains from CNAMEs).

### `enrich_ip`

Run reverse DNS on an IP address. Returns PTR records and any discovered hostnames as new assets.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| ip | string | yes | IPv4 or IPv6 address |
| timeout | integer | no | Timeout in seconds (default 30) |

**Returns:** PTR records and discovered hostnames.

### `enrich_service`

Probe an HTTP(S) service for status code, redirect chain, server headers, page title, TLS certificate info (CN, SAN, issuer, expiry), and technology fingerprinting.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | yes | Hostname or IP |
| port | integer | yes | Port number (1-65535) |
| scheme | string | no | `http` or `https` (auto-detected from port if omitted: https for 443/8443, http otherwise) |
| timeout | integer | no | Timeout in seconds (default 30) |

**Returns:** Status code, redirect chain, server headers, page title, TLS certificate details, and detected technologies.

### `ingest_scanner_output`

Parse output from common security scanners and import the results as assets and/or findings.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| scanner | string | yes | `nmap` (XML or greppable), `nuclei` (JSONL), `httpx` (JSONL), or `ffuf` (JSON) |
| output | string | yes | The scanner output to parse |

**Returns:** `{assets_created, findings_created, errors?}`.

### `ingest_masscan`

Parse masscan output (XML or JSON) and import discovered hosts and open ports as assets.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| format | string | yes | `xml` or `json` |
| data | string | yes | The masscan output to parse |

**Returns:** `{assets_created, findings_created: 0, errors?}`.

---

## Engagement management

### `create_engagement`

Create a new pentest engagement (task + exploration graph).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| goal | string | yes | High-level goal for the engagement |
| description | string | no | Detailed description of the engagement scope (defaults to goal) |

**Returns:** `{task_id, exploration_id}`.

### `list_engagements`

List all pentest engagements with summary stats (goal, status, node/finding/asset counts).

_No parameters._

**Returns:** Array of engagement summaries, each with `id`, `goal`, `description`, `status`, `exploration_id`, `created_at`, `node_count`, `finding_count`, `asset_count`, `goals_met`, and `active` flag.

### `switch_engagement`

Switch to a different engagement without restarting the server. All tools will be re-registered to operate on the new engagement.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| task_id | integer | yes | Task ID of the engagement to switch to |

**Returns:** `"switched to engagement <id> (exploration <id>): <goal>"`.

### `update_engagement`

Update the current engagement's goal or description.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| goal | string | no | New goal for the engagement |
| description | string | no | New description for the engagement |

At least one of `goal` or `description` is required.

**Returns:** `"engagement <id> updated: <fields>"`.

### `delete_engagement`

Delete an engagement and all its data (nodes, edges, findings, credentials, coverage, assets, constraints). Irreversible.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| task_id | integer | yes | Task ID of the engagement to delete |
| confirm | boolean | yes | Must be `true` to confirm deletion |

**Returns:** `"engagement <id> deleted"`.

### `engagement_status`

Get the status of an engagement: goal, status, finding count, and coverage summary.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| task_id | integer | no | Task ID (defaults to current engagement) |

**Returns:** `{task_id, exploration_id, goal, status, findings_total, facts_total, frontier_open, done_intents, coverage?}`.

### `engagement_activity`

Show recent activity in the current engagement as a timeline of findings, assets, and graph nodes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| limit | integer | no | Maximum number of entries to return (default 20) |

**Returns:** Timeline of `"<timestamp> - <kind>: <summary>"` entries.

### `goal_met`

Mark the entire engagement as complete. Sets the task status to done.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| summary | string | no | Summary of what was accomplished |

**Returns:** `"engagement <id> marked complete"` with optional summary.

### `list_coverage_gaps`

Identify testing gaps: assets never targeted by an intent, intents with no recorded facts, goals not yet proven, and service ports discovered but not enriched.

_No parameters._

**Returns:** JSON object with gap categories and their entries.

---

## Activity

### `list_activity`

List recent activity log entries for this engagement: intents started/completed, facts recorded, findings reported, tool calls. Newest first.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| limit | integer | no | Number of entries to return, default 50, max 500 |
| node_id | integer | no | Filter to activity on a specific exploration node |
| kind | string | no | Filter by activity kind (e.g. `tool_use`, `tool_result`, `intent_start`, `intent_done`, `finding`) |

**Returns:** `{count, activity: [...]}`.

### `log_activity`

Record an activity log entry for this engagement. Use to log significant actions, decisions, or observations.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| action | string | yes | What happened (used as the activity kind) |
| node_id | integer | no | Related exploration node ID |
| detail | string | no | Detailed description of the activity |
| summary | string | no | One-line summary |

**Returns:** `"activity logged: <id>"`.

---

## Reporting and export

### `generate_report`

Generate a Markdown penetration test report from the current engagement's findings, assets, and coverage.

_No parameters._

**Returns:** Markdown report as text.

### `export_engagement`

Export the current engagement (or a specific one) as a portable archive. Includes assets, exploration graph, findings, credentials, coverage, and scope.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| task_id | integer | no | Task ID to export (defaults to current engagement) |
| format | string | no | `json` (default) or `csv` |

JSON returns the full export object. CSV returns per-entity CSV tables (assets, findings, nodes).

**Returns:** JSON: full engagement export object. CSV: `{format, exported_at, assets_csv, findings_csv, nodes_csv}`.

### `import_engagement`

Import a previously exported engagement from JSON. Creates a new engagement with all the imported data.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| data | string | yes | The exported engagement JSON string |

**Returns:** `{task_id, exploration_id, message}`.

---

## Traffic

These tools require `--proxy` on the `serve` command. Point your testing tools at the proxy to capture requests.

### `traffic_search`

Query captured traffic by host (required), optionally filtered by URL substring or body keyword. Returns an ultra-light index (id/method/url/status/resp_len) with no response content; use traffic_get to inspect individual results.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | yes | Target host: bare host, host:port, or full URL |
| query | string | no | URL/metadata substring filter |
| body_contains | string | no | Full-text body search (min 3 chars) |
| limit | integer | no | Results per page, default 3, max 10 |
| page | integer | no | 0-based page number for pagination |

When a port is specified in `host`, only that service's traffic is returned. `body_contains` supports any substring including CJK characters.

**Returns:** `{count, results: [{id, method, url, status, resp_len}, ...]}`.

### `traffic_get`

Fetch the raw request/response of one captured traffic record by ID (truncated when too large). WebSocket (status 101) or SSE (text/event-stream) records show only their handshake; use traffic_messages for their messages.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | yes | Exchange ID from traffic_search |

When proxy rewrite rules changed the exchange, the result starts with a `REWRITTEN` line naming the rule IDs, and shows the request as the client sent it and the response as the target sent it.

**Returns:** Text with `--- REQUEST ---` and `--- RESPONSE ---` sections.

### `traffic_blob`

Read a segment of an oversized request or response body by its blob hash (shown in traffic_get as `@blob sha256:<hash>`). Returns at most 8 KiB per call; use offset to page through.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| hash | string | yes | SHA-256 blob hash from traffic_get output |
| offset | integer | no | Byte offset to start reading (default 0) |
| limit | integer | no | Bytes to read, max 8192 (default 8192) |

**Returns:** `{total, offset, length, data}`.

### `traffic_messages`

List WebSocket or SSE messages of a captured stream exchange. Use after traffic_get shows status 101 (WebSocket) or text/event-stream (SSE).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | yes | Exchange ID |
| direction | string | no | Filter: `c2s` (client to server), `s2c` (server to client), or empty for both |
| limit | integer | no | Messages per page, default 50, max 200 |
| offset | integer | no | 0-based offset for pagination |

**Returns:** `{total, messages: [{seq, direction, type, size, payload, event?, blob?, truncated?}, ...]}`.

### `traffic_replay`

Replay a captured HTTP request with optional modifications. Fetches the original request, applies overrides, sends it through the recording proxy, and returns the new response.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | yes | Exchange ID to replay |
| modify_headers | object | no | Header overrides: `{name: value}` to set, `{name: ""}` to remove |
| modify_url | string | no | Override the request URL |
| modify_method | string | no | Override the HTTP method |
| modify_body | string | no | Override the request body |
| apply_rules | boolean | no | Apply capture rewrite rules to the replay (default false) |

**Returns:** `{status, duration_ms, recorded, truncated, raw}`.

### `traffic_hosts`

List all recorded target hosts with their exchange counts, most recent activity first.

_No parameters._

**Returns:** `{count, hosts: [...]}`.

### `traffic_har_export`

Export captured traffic as a HAR 1.2 JSON document. Optionally filter by host, method, or status code.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | no | Host substring filter (empty exports all) |
| method | string | no | HTTP method filter |
| status | string | no | Status code or class filter, e.g. `200` or `4xx` |

**Returns:** `{har, exported, totalMatched, truncated}`.

### `traffic_stats`

Aggregate statistics about all captured traffic: total exchanges, unique hosts, status code distribution, method distribution, top 10 hosts, time range, and average response size.

_No parameters._

**Returns:** Aggregate statistics object.

### `traffic_timeline`

Show traffic activity over time for a host, bucketed by minute, hour (default), or day. Useful for spotting rate limiting, session expiry, and WAF patterns.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | yes | Target host |
| bucket | string | no | `minute`, `hour` (default), or `day` |

**Returns:** `{host, buckets: [...]}`.
