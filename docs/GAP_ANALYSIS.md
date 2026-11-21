# Gap Analysis: orig-artex / ARTEX / cc-artex

## 1. orig-artex overview

The historical pin (`/home/owen/projects/orig-artex/ARTEX/`) is a monolithic Go
application with a built-in web UI, LLM agent loop, PostgreSQL backend, and an
HTTP/REST API server. Key packages:

| Package | Purpose |
|---|---|
| `agent/` | LLM agent loop: planner/worker architecture, prompt catalog, tool catalog, finding recorder, cold-graph compression, session management. 50+ Go files. |
| `server/` | HTTP API + WebSocket broadcast: task management, engine lifecycle, conversations, finding workflow, intercept review, skill upload, authentication, goal/constraint APIs. 60+ Go files. |
| `db/` | PostgreSQL schema (1386 lines) + Go store: assets, explorations, findings, tasks, task archives, intercept rules, notifications, LLM usage, settings, side questions, tool/skill usage, company scope. 70+ Go files. |
| `traffic/` | MITM recording proxy, evidence snapshots, archive (10 Go files). |
| `enrich/` | Async DNS + HTTP probe engine: resolves domains via dnsx, probes web assets through the proxy. |
| `evidence/` | Evidence store for traffic snapshots. |
| `intercept/` | Tool-call interception: trace, prompt review, review context. |
| `guard/` | Safety guard for tool calls. |
| `report/` | Markdown report generator (findings by severity). |
| `notify/` | Push notification channels: email, Telegram, DingTalk, Feishu, WeCom, generic webhook; with redaction, filtering, rate limiting, SSRF protection. |
| `sidequestion/` | Side-question LLM service (one-shot Q&A without the full agent harness). |
| `llmpool/` | LLM provider pool with health tracking and failover. |
| `llmrec/` | LLM call recording/replay for testing and debugging. |
| `mcphttp/` | MCP-over-HTTP client (SSE transport). |
| `selfupdate/` | GitHub-based binary self-update. |
| `config/` | Configuration loading. |
| `skills/` | 3 skill packs: `api-recon` (API reconnaissance with scripts), `playwright-cli` (browser automation), `scopesentry` (scope management). |
| `web/` | Next.js web frontend. |

### Agent tools in orig-artex

Planner tools: `graph_overview`, `list_findings`, `list_facts`, `node_detail`,
`expand_digest`, `get_worker_output`, `get_worker_trace`,
`search_all_worker_traces`, `list_goals`, `add_intent`, `prove_goal`,
`goal_met`, `kill_work`, `steer_work`, `report_finding`, `list_companies`,
`list_assets`, `add_company_scope`, `add_task_scope`, `list_untested_assets`.

Worker tools: `record_fact`, `report_finding`, `insert_assets`, `add_hint`,
`set_goals`, `set_constraints`, `graph_overview`, `list_findings`, `list_facts`,
`node_detail`, `expand_digest`, `get_worker_output`, `get_worker_trace`,
`search_all_worker_traces`, `list_worker_traces`, `list_assets`,
`list_companies`.

Main agent tools: all planner tools + `set_goals`, `set_constraints`,
`add_hint`, `record_fact`, `insert_assets`.

### Dependencies (orig-artex)

- `github.com/Autumn-27/norma`: LLM harness framework (tool definitions, permission model, provider abstraction).
- `pgx/v5`: PostgreSQL.
- `go-mitmproxy`: MITM proxy base.
- `dnsx`: DNS resolution.
- `modernc.org/sqlite`: SQLite (evidence store).

---

## 2. Current ARTEX overview

The current ARTEX (`/home/owen/projects/ARTEX/`) has evolved significantly. Major
additions since orig-artex:

### New packages

| Package | What it adds |
|---|---|
| `archivestore/` | S3-compatible cold storage for task archive packages (AWS SDK v2). |
| `egress/` | Outbound request governor: noise profiles (quiet/balanced/aggressive), per-host rate limiting, jitter, concurrency caps, 429/503 backoff, GET coalescing/caching. |
| `extension/` | Chrome extension (manifest v3) for browser-side integration. |

### New agent features (in `agent/`)

| File(s) | Feature |
|---|---|
| `bbot_ingest.go`, `bbot_ingest_tool.go` | Deterministic bbot output ingestion: typed-field-only parsing, scope_distance gating, untrusted-text quarantine. |
| `scan_ingest.go`, `scan_ingest_tool.go` | Generic scanner output ingestion: nmap XML, nuclei JSONL, httpx JSON, ffuf JSON. |
| `webfetch.go` | WebFetch tool with egress governor integration, target-gate redirect checking, pinned dialer. |
| `filetools.go`, `filetools_unix.go` | File operation tools (platform-specific). |
| `tools_creds.go` | `save_task_credential` and `list_task_credentials` tools. |
| `coverage.go`, `coverage_ledger.go` | Coverage tracking with ledger-based recording (surface/risk_area/outcome). |
| `budget.go`, `noa_budget.go` | LLM token budget management and NOA (number-of-actions) budgets. |
| `noise_profile.go` | Runtime noise profile switching. |
| `toolrag.go` | Tool RAG: lexical projection of tool catalog based on task context (reduces tool list for large catalogs). |
| `spill.go` | Session spill to disk when context exceeds limits. |
| `data_envelope.go` | Data envelope for structured tool output. |
| `fact_input.go`, `fact_assets.go` | Richer fact input validation and asset anchoring. |
| `untrusted_boundary.go`, `untrusted_prompt.go` | Untrusted-content boundary enforcement. |
| `session.go` | Session management. |
| `providerstop.go` | Provider stop/quota management. |

### New traffic features

| File(s) | Feature |
|---|---|
| `external.go` | External traffic import. |
| `har.go` | HAR export. |
| `replay.go` | Traffic replay with rewrite support. |
| `raw_tcp.go`, `raw_udp.go` | Raw TCP/UDP stream capture. |
| `socks5.go`, `socks5_udp.go` | SOCKS5 proxy with UDP relay. |
| `transparent.go`, `transparent_linux.go` | Transparent proxy (Linux iptables integration). |
| `stream.go` | Bidirectional stream management. |
| `pace.go` | Traffic pacing/throttling. |
| `rewrite/`, `rewrite_flow.go`, `rewrite_hooks.go` | Rule-based request/response rewrite engine. |
| `hookvm/` | Sandboxed expression evaluator for rewrite hooks. |

### New skills

| Skill | Description |
|---|---|
| `api-security/` | API security testing methodology. |
| `bbot-recon/` | BBOT reconnaissance integration. |
| `codeaudit/` | Source code audit methodology. |
| `edr-bypass-re/` | EDR bypass reverse engineering. |
| `patch-diff/` | Patch diffing methodology. |
| `php2shell/` | PHP exploitation chain. |
| `rea/` | Real estate of attack surface analysis. |

### Schema differences

The current ARTEX schema (1615 lines vs orig-artex's 1386) adds:
- `task_credentials` table.
- `coverage_ledger` and `coverage_ledger_history` tables.
- `finding_quality` columns/logic.
- `evidence_rewrite` flags.
- `activity_outbox` table.
- `browser_mcp` table.
- `dashboard_usage` indexes.
- Asset DSL enhancements (OR scope, limit).
- Coverage graph support for large datasets.

### New dependencies

- `aws-sdk-go-v2`: S3 archive storage.
- `gorilla/websocket`: Direct WebSocket usage.
- `mvdan.cc/sh/v3`: Shell parser for untrusted-command analysis.

---

## 3. cc-artex overview

cc-artex (`/home/owen/projects/cc-artex/`) strips the LLM agent loop and web
UI, exposing the data layer as MCP tools for Claude Code to drive directly.

### Packages

| Package | Purpose |
|---|---|
| `store/` | PostgreSQL store (schema: 353 lines). Assets, explorations, findings, credentials, coverage ledger, tasks, intercept rules, finding traffic bindings. |
| `mcpserver/` | MCP protocol server: stdio + HTTP transports, session management, tool registration via mcp-go. |
| `tools/` | MCP tool implementations: asset, exploration, finding, credential, scope, report, engagement, traffic tools. |
| `config/` | Configuration and engagement state. |
| `traffic/` | Full traffic stack ported from ARTEX: MITM proxy, SOCKS5, raw TCP/UDP, replay, HAR, rewrite engine, hookvm, transparent proxy, pacing, evidence. |
| `report/` | Enhanced report generator with quality analysis, coverage gaps, finding export (Markdown, CSV, per-finding reports). |
| `egress/` | Rate-limiting governor (ported from ARTEX). |
| `cmd/cc-artex/` | CLI entrypoint. |

### MCP tools exposed

**Asset tools**: `insert_assets`, `list_assets`, `list_companies`,
`add_company_scope`, `add_task_scope`, `list_untested_assets`.

**Exploration tools**: `graph_overview`, `add_intent`, `record_fact`,
`list_facts`, `node_detail`, `add_hint`, `set_goals`, `set_constraints`,
`prove_goal`.

**Finding tools**: `report_finding`, `list_findings`.

**Credential tools**: `save_task_credential`, `list_task_credentials`.

**Scope tools**: `record_coverage`.

**Report tools**: `generate_report`.

**Engagement tools**: `create_engagement`, `list_engagements`,
`engagement_status`.

**Traffic tools**: `traffic_search`, `traffic_get`, `traffic_blob`,
`traffic_messages`, `traffic_replay`, `traffic_hosts`, `traffic_har_export`.

---

## 4. Missing from cc-artex (prioritized)

### Priority 1: High value, moderate effort

#### 4.1 Enrichment engine (`enrich/`)
**What**: Async DNS resolution (dnsx) and HTTP probing of discovered assets.
When a domain is inserted, it auto-resolves A/AAAA/CNAME records and upserts
IP nodes. When a site is inserted, it auto-probes for status code, title,
content length.

**Why it matters**: Without enrichment, Claude Code must manually resolve every
domain and probe every URL. The enrichment engine handles this automatically in
the background after `insert_assets`, making the asset graph self-completing.

**Effort**: Medium. Requires porting `enrich/enrich.go` (293 lines) and wiring
it into the store's asset insertion path. Needs `dnsx` dependency. The
recording-proxy routing is already available in cc-artex's traffic package.

**Blocked by**: Nothing. The store and traffic packages already provide the
necessary interfaces.

#### 4.2 `goal_met` tool
**What**: Explicit "task complete" signal. Currently cc-artex has `prove_goal`
(mark individual goals met) but not `goal_met` (declare the entire engagement
finished).

**Why it matters**: Claude Code needs a way to signal engagement completion
for the engagement lifecycle.

**Effort**: Low. A thin wrapper that marks all goals met and updates task
status.

#### 4.3 Worker trace tools: `get_worker_output`, `get_worker_trace`, `list_worker_traces`, `search_all_worker_traces`
**What**: The activity-based trace/output tools that let the planner inspect
what workers did. In ARTEX these are used by the planner to review worker
execution.

**Why it matters**: In cc-artex, Claude Code is both planner and worker, so the
trace tools are less critical. However, `get_worker_output` would let Claude Code
review past intent execution results when resuming a session. The activity table
exists in cc-artex's schema but the tools to query it are missing.

**Effort**: Low-medium. The `activity` table exists. Need to implement query
functions in the store and register the tools.

#### 4.4 Finding traffic binding
**What**: Linking traffic exchanges to findings as evidence (`traffic_refs` in
`report_finding`, `bind_finding_traffic` tool). ARTEX's `report_finding` accepts
`traffic_refs` with roles (baseline/proof/verification/supporting) and validates
them. cc-artex's `report_finding` has no traffic binding.

**Why it matters**: Traffic evidence is the proof-of-exploit. Without binding,
findings have text evidence only. The `finding_traffic_bindings` table exists
in cc-artex's schema but is unused by the tool layer.

**Effort**: Medium. The schema table exists. Need to add traffic ref validation
and binding logic to `report_finding`, plus a standalone `bind_finding_traffic`
tool.

### Priority 2: Useful, low-medium effort

#### 4.5 `expand_digest` tool (cold-graph digests)
**What**: Restores folded cold-region exploration nodes. ARTEX's cold-graph
compression system collapses old, low-value exploration nodes into digests.
`expand_digest` lets the planner drill into them.

**Why it matters**: For long engagements, the exploration graph grows large.
Cold digests keep the overview manageable. Without them, `graph_overview`
either omits old data or becomes too large.

**Effort**: Medium. Requires the cold-graph compression system (`coldgraph.go`,
`compaction.go`) plus the digest storage and retrieval. This is a significant
subsystem.

#### 4.6 `kill_work` / `steer_work` tools
**What**: Planner tools to terminate or redirect a running worker.

**Why it matters**: Less relevant for cc-artex since Claude Code is the
single-threaded executor. If cc-artex ever supports concurrent intent execution,
these become important. Currently not needed.

**Effort**: Low, but architecturally irrelevant until concurrent execution is
supported.

#### 4.7 Asset intercept (scope gating)
**What**: Asset-level scope enforcement (`asset_intercept.go`,
`asset_intercept_match.go`). When inserting assets or creating intents, checks
them against intercept rules (domain/IP/URL patterns) and blocks out-of-scope
ones.

**Why it matters**: Safety net. Prevents the agent from accidentally testing
assets outside the engagement scope. The `intercept_rules` table exists in
cc-artex's schema but the enforcement logic is not wired.

**Effort**: Medium. Need to port the matching logic and wire it into
`insert_assets` and `add_intent`.

#### 4.8 Scanner ingestion tools
**What**: `ingest_bbot_output` and `ingest_scan_output` tools that parse
bbot/nmap/nuclei/httpx/ffuf output files and extract structured assets and
facts.

**Why it matters**: Claude Code often runs these scanners. Deterministic
ingestion is safer and more reliable than having the LLM parse scanner output.
Prevents prompt injection via target-controlled data in scanner output.

**Effort**: Medium-high. The parsers (`bbot_ingest.go` at ~300 lines,
`scan_ingest.go` at ~500 lines) are substantial. The tool wrappers
(`bbot_ingest_tool.go`, `scan_ingest_tool.go`) add ~200 lines each. The parsers
depend on `db.AssetStore` types which would need adaptation for cc-artex's
`tools.Store` interface.

### Priority 3: Lower priority or not applicable

#### 4.9 Notification system (`notify/`)
**What**: Push notifications via email, Telegram, DingTalk, Feishu, WeCom,
generic webhook. 26 Go files.

**Why it matters**: Useful for team alerting on findings, but cc-artex targets a
single operator (Claude Code) who already sees findings in-context. Low priority.

**Effort**: High. Large, standalone package with many channel adapters.

#### 4.10 Web UI + REST API (`server/`, `web/`)
**What**: Full web frontend and HTTP API server (60+ Go files in server/).

**Why it matters**: Not applicable. cc-artex is designed to be driven by Claude
Code, not a web browser. The MCP server replaces the REST API.

**Effort**: N/A.

#### 4.11 LLM provider infrastructure (`llmpool/`, `llmrec/`, `sidequestion/`)
**What**: Multi-provider LLM pool with health tracking, call recording/replay,
one-shot Q&A service.

**Why it matters**: Not applicable. cc-artex makes zero LLM calls; Claude Code
is the LLM.

**Effort**: N/A.

#### 4.12 Self-update (`selfupdate/`)
**What**: GitHub-based binary self-update.

**Why it matters**: Not applicable for a development tool built from source.

**Effort**: N/A.

#### 4.13 S3 archive storage (`archivestore/`)
**What**: S3-compatible cold storage for task archive packages.

**Why it matters**: Low priority. Task archival is a team/enterprise feature.

**Effort**: Medium (AWS SDK dependency already not in cc-artex).

#### 4.14 Tool RAG
**What**: Lexical projection of the tool catalog based on task context.

**Why it matters**: Low priority. cc-artex exposes ~30 tools, well within
Claude Code's context window. ARTEX needs it because its combined planner +
worker + custom tool catalog can exceed 50 tools.

**Effort**: Medium, but unnecessary.

#### 4.15 Session spill, budget management, NOA budgets
**What**: Context overflow management, token budget tracking, action budgets.

**Why it matters**: Not applicable. Claude Code manages its own context and
budget.

**Effort**: N/A.

#### 4.16 Chrome extension
**What**: Browser extension for integration.

**Why it matters**: Not applicable for CLI-driven usage.

**Effort**: N/A.

#### 4.17 Skills (7 new skill packs)
**What**: api-security, bbot-recon, codeaudit, edr-bypass-re, patch-diff,
php2shell, rea.

**Why it matters**: These are prompt/reference packs for the ARTEX agent. Claude
Code has its own skill system. The reference material (methodology docs) could
be adapted but the ARTEX skill format is not directly usable.

**Effort**: Low to port as Claude Code skills; medium to maintain both.

---

## 5. cc-artex advantages (what we have that they don't)

### 5.1 MCP protocol server
cc-artex exposes tools via the MCP protocol (stdio + HTTP transports), making it
usable by any MCP-compatible client. ARTEX has only an HTTP REST API + its own
agent loop.

### 5.2 Engagement lifecycle tools
`create_engagement`, `list_engagements`, `engagement_status`: cc-artex has
first-class engagement management tools. ARTEX manages tasks via the web UI API
(creating tasks is a server endpoint, not an agent tool).

### 5.3 English-language tool descriptions
cc-artex's tool descriptions are in English, matching Claude Code's language.
ARTEX's tool descriptions are in Chinese, optimized for its own bilingual agent.

### 5.4 `generate_report` tool
cc-artex has an MCP tool that generates a Markdown report on demand. ARTEX's
report generation is a server-side API endpoint, not an agent tool.

### 5.5 Enhanced report package
cc-artex's `report/` package includes quality analysis (PoE level, verification
status, confidence rationale, counterevidence), coverage gap reporting, finding
export (Markdown summary, per-finding Markdown, CSV), and stale-report
detection. ARTEX's report is a simple Markdown renderer.

### 5.6 `record_coverage` tool
cc-artex has an explicit coverage recording tool (surface/risk_area/outcome
ledger). ARTEX has the coverage ledger in its DB but the recording is wired
through the agent's internal fact-writing path, not as a standalone tool.

### 5.7 Traffic tools as MCP
Traffic tools (`traffic_search`, `traffic_get`, `traffic_blob`,
`traffic_messages`, `traffic_replay`, `traffic_hosts`, `traffic_har_export`) are
first-class MCP tools. In ARTEX, traffic search/get are server API endpoints and
the agent accesses them through the platform tool system.

### 5.8 Full traffic stack from day one
cc-artex ships the complete modern traffic stack (SOCKS5, raw TCP/UDP,
transparent proxy, rewrite engine, hookvm) that ARTEX only gained after the
orig-artex pin.

### 5.9 Simpler deployment
cc-artex is a single binary + PostgreSQL. ARTEX requires PostgreSQL + the Go
server + optionally the Node.js web frontend.

### 5.10 Credential tools ported and working
cc-artex has `save_task_credential` and `list_task_credentials` as MCP tools.
These were added to ARTEX after orig-artex and cc-artex independently ported
them.

---

## 6. Recommendations

### Port next (by priority)

1. **Enrichment engine** (4.1). This is the single highest-value missing feature.
   Every `insert_assets` call that adds a domain should auto-resolve it, and
   every new site should be auto-probed. Without it Claude Code spends tool calls
   doing what the engine does for free. Port `enrich/enrich.go` and wire it to
   fire after asset insertion in the store. Add `dnsx` to `go.mod`. Estimated
   effort: 1-2 sessions.

2. **`goal_met` tool** (4.2). Trivial addition, closes the engagement lifecycle.
   Estimated effort: 15 minutes.

3. **Finding traffic binding** (4.4). The schema is ready. Add `traffic_refs`
   parameter to `report_finding` and implement the binding logic. Estimated
   effort: 1 session.

4. **Activity/trace query tools** (4.3). The `activity` table exists. Add
   `get_worker_output` and optionally `get_worker_trace` to let Claude Code
   review past exploration results when resuming. Estimated effort: 1 session.

5. **Asset intercept enforcement** (4.7). Wire the existing `intercept_rules`
   table into `insert_assets` and `add_intent`. Keeps the agent from wandering
   out of scope. Estimated effort: 1 session.

6. **Scanner ingestion** (4.8). Port the bbot/nmap/nuclei/httpx/ffuf parsers.
   This is a larger effort but high value since Claude Code frequently runs
   these tools. The deterministic parsing is safer than LLM interpretation of
   scanner output. Estimated effort: 2-3 sessions.

### Do not port

- Web UI, REST API server, LLM provider pool, LLM recording, side-question
  service, self-update, Chrome extension, session spill, budget management,
  tool RAG. These are either not applicable to the cc-artex architecture or
  are handled by Claude Code itself.

### Consider later

- Cold-graph compression (4.5): only matters for very long engagements.
- Notification channels (4.9): only matters for team workflows.
- S3 archive storage (4.13): only matters for data retention policies.
- Skill packs (4.17): could be ported as Claude Code skills if the methodologies
  prove useful.
