# cc-artex Roadmap

## 1. Current state summary

cc-artex is a working MCP tool server for penetration testing. It exposes ARTEX's
data model as tools Claude Code calls directly, with no separate LLM API key.

**Lines of code:** ~24,200 Go (91 files: 78 source, 13 test files with ~2,600 LOC).

**Tool count:** 47 registered tools (core + traffic + enrich + ingest + scope + intent + activity).

**Test count:** 76 test functions across 13 test files.

**What works today:**

- **CLI:** `serve` (stdio and HTTP transports), `init` (creates engagement, schema, CA cert), `version`.
- **Store (PostgreSQL):** Full schema with auto-migration. Assets (6 types), companies, company scope, task scope, exploration graph (nodes, edges, anchors, constraints), findings, credentials, coverage ledger with history, intercept rules, activity log. Engagement export/import to portable JSON archives.
- **Exploration graph:** Goals, intents, facts, findings, hints, constraints. Edges connect them (derived_from, yields, spawns, proves). Frontier query. Graph overview for situational awareness. Prove-goal workflow with confidence gating. Intent lifecycle (start, complete, cancel).
- **Asset management:** Bulk insert with upsert, DSL-based queries (fuzzy, exact, exclusion, numeric comparisons, boolean combinators, parentheses), untested-asset queries, company scope attribution.
- **Findings and credentials:** Report findings with severity/evidence/assets, list findings. Save and list credentials with upsert-on-domain+username. AES-256-GCM encryption at rest for credential passwords.
- **Coverage:** Record coverage entries (surface x risk_area x outcome), coverage history.
- **Report generation:** Markdown report from findings, assets, and coverage. Finding quality assessment (PoE level, verification status, counterevidence). CSV and markdown export formats.
- **Enrich:** Passive reconnaissance tools (`enrich_domain`, `enrich_ip`, `enrich_service`) with DNS resolution, reverse DNS, WHOIS, certificate transparency, and technology fingerprinting.
- **Scanner ingestion:** Parsers for nmap, nuclei, httpx, and ffuf output. Results feed directly into the asset graph.
- **Scope enforcement:** `check_scope` tool and automatic enforcement in asset, enrich, and traffic tools. Blocks out-of-scope actions with an override mechanism.
- **Multi-engagement switching:** `switch_engagement` tool for changing active context without a server restart.
- **Activity log:** `list_activity` and `log_activity` tools for audit trail access.
- **Traffic recording proxy:** Full MITM proxy with SQLite index (WAL mode, busy-timeout), blob storage, body spill for large payloads, WebSocket/SSE stream capture, URL path templating, host listing, paginated search (including body full-text search), HAR 1.2 export, request replay with modifications. Graceful shutdown with connection draining.
- **Traffic rewrite engine:** Rule-based request/response rewriting with a custom hook VM (eval.go, stdlib, validation). Rewrite flow with match/transform pipeline.
- **Egress governor:** Rate limiting with profiles (quiet/balanced/aggressive/off), per-host and global concurrency caps, jitter, 429/503 backoff, GET cache/coalescing, configurable user agent.
- **External traffic ingest:** Browser-extension capture endpoint.
- **SOCKS5 proxy:** SOCKS5 with UDP relay support.
- **Raw TCP/UDP capture.**
- **Transparent proxy (Linux):** iptables-based transparent interception.
- **Intercept rules:** Tool-call interception with allow/deny/ask actions and timeout policy.
- **MCP resource:** `artex://engagement` resource serves the orchestration guide with live engagement context.
- **Structured logging:** slog-based JSON logging across all packages.
- **CI:** GitHub Actions with `go build`, `go vet`, `go test`, and store tests against a real PostgreSQL instance.
- **Distribution:** goreleaser configuration for cross-platform binaries, Dockerfile, Homebrew tap section.
- **Community files:** MIT license, CONTRIBUTING.md, SECURITY.md, 11 good-first-issue labels, example engagement walkthrough, godoc comments on all exported types.


## 2. P0: Ship blockers (all complete)

All P0 items have been addressed:

- ✅ **2.1 SECURITY.md** -- Responsible disclosure policy published.
- ✅ **2.2 Credential encryption at rest** -- AES-256-GCM encryption for stored passwords.
- ✅ **2.3 SQLite WAL mode** -- WAL mode and busy-timeout configured for concurrent access.
- ✅ **2.4 Graceful shutdown** -- Connection draining on traffic proxy shutdown.
- ✅ **2.5 Input validation** -- All tools validate required fields and return clear errors.


## 3. P1: High-value features

### ✅ 3.1 Enrich package

Done. `enrich_domain`, `enrich_ip`, `enrich_service` tools with DNS, reverse DNS, WHOIS, certificate transparency, and technology fingerprinting. Results feed into `insert_assets`.

### 3.2 Evidence package (screenshot, proof-of-concept capture)

**What:** An `evidence/` package for capturing screenshots (headless Chrome via CDP) and PoC artifacts (request/response pairs snapshotted at the moment a finding is reported, with tamper-evident hashes). Expose as `capture_screenshot` and `snapshot_evidence` tools. Screenshots stored in the data directory with metadata in PG.
**Effort:** L
**Dependencies:** Traffic proxy (for request snapshots), Chrome/Chromium available on host.
**Rationale:** A pentest report without screenshots and reproducible PoC steps is incomplete. Currently evidence is free-text in the finding record. Structured evidence with screenshots makes reports credible.

### ✅ 3.3 Engagement export and import

Done. `export_engagement` and `import_engagement` tools in `store/export.go`. Portable JSON archive format with optional traffic blob companion.

### ✅ 3.4 Multi-engagement switching

Done. `switch_engagement` tool in `mcpserver/switch.go` updates the server's active context without a restart.

### ✅ 3.5 Intent lifecycle

Done. `start_intent`, `complete_intent`, `cancel_intent` tools for transitioning intent state.

### ✅ 3.6 Activity log tools

Done. `list_activity` (paginated, filterable) and `log_activity` tools.

### ✅ Scanner ingestion

Done. `ingest/` package with parsers for nmap, nuclei, httpx, and ffuf. Results feed into the asset graph.

### ✅ Scope enforcement

Done. `check_scope` tool and automatic enforcement in asset, enrich, and traffic tools. Originally P4, promoted and implemented in P1.


## 4. P2: Quality and polish

### ✅ 4.1 Test coverage

Significantly expanded: 13 test files, 76 test functions, ~2,600 test LOC (up from 4 files and ~1,000 LOC). Coverage now includes enrich, report, egress, config, crypto, ingest, intent, and scope packages.

Remaining gaps worth filling:
1. `traffic/rewrite/` and `traffic/hookvm/` (security-sensitive evaluation of user-supplied rules)
2. Higher coverage in `traffic/` core
3. Integration-level tests for multi-tool workflows

### ✅ 4.2 Error message consistency

Done. Standardized error format across all tools.

### ✅ 4.3 Structured logging

Done. `slog`-based JSON logging across all packages, replacing `log.Logger` and raw stderr writes.

### ✅ 4.4 README accuracy

Done. Tool count corrected, architecture section updated, README restructured.

### ✅ 4.5 MCP config snippet

Done. `init` now shows both minimal and full configuration examples including `--data` and `--proxy` flags.


## 5. P3: Distribution and community

### ✅ 5.1 goreleaser

Done. Cross-platform binary builds for Linux, macOS, and Windows via GitHub Actions.

### ✅ 5.2 Dockerfile

Done. Multi-stage Dockerfile for container deployments.

### 5.3 Homebrew formula

Partially done. goreleaser has a tap section that auto-generates the formula. A standalone tap repository has not been set up.

### ✅ 5.4 Example engagement walkthrough

Done. Annotated walkthrough in `docs/examples/` showing init through report generation.

### ✅ 5.5 Good first issues

Done. 11 issues created (#25-#35) covering small, self-contained improvements.

### ✅ 5.6 pkg.go.dev documentation

Done. 64 godoc comments added to all exported types and functions.


## 6. P4: Future vision

### 6.1 Plugin system for external tools

**What:** A plugin interface that lets users register custom MCP tools (e.g., nuclei scanner, sqlmap, ffuf) that write results directly into the asset graph and exploration state.
**Effort:** L
**Dependencies:** None directly, but benefits from stable tool/store interfaces.
**Rationale:** cc-artex's value is the data model and orchestration, not any one scanner. A plugin system lets the community contribute tool integrations without modifying core.

### 6.2 Web dashboard

**What:** A read-only web UI that visualizes the exploration graph, asset inventory, findings timeline, and coverage heatmap. Served alongside the HTTP transport.
**Effort:** L
**Dependencies:** None.
**Rationale:** The exploration graph is the core abstraction, but it is only accessible through JSON tool responses today. A visual graph makes it useful for humans reviewing an engagement, not just Claude.

### 6.3 Collaborative multi-agent support

**What:** Multiple Claude Code instances (or other MCP clients) working the same engagement simultaneously, with locking on intents to prevent duplicate work and a notification mechanism when new findings are reported.
**Effort:** L
**Dependencies:** Multi-engagement switching (3.4, done), intent lifecycle (3.5, done).
**Rationale:** Large engagements benefit from multiple agents working in parallel. PostgreSQL's advisory locks and LISTEN/NOTIFY provide the primitives; the challenge is the MCP-level coordination protocol.

### ✅ 6.4 Scope enforcement

Done. Promoted to P1 and implemented. `check_scope` tool with enforcement in asset, enrich, and traffic tools.

### 6.5 Finding deduplication and triage

**What:** Automatic deduplication of findings by vulnerability class + affected asset, with a triage workflow (confirm/dismiss/merge) exposed as MCP tools.
**Effort:** M
**Dependencies:** None.
**Rationale:** In long engagements, the same vulnerability is often discovered multiple times from different angles. Deduplication reduces noise in the report and helps the agent focus on new findings.

### 6.6 Temporal analysis of traffic

**What:** Tools to analyze traffic patterns over time: request frequency by host, response time trends, status code distribution, new endpoints appearing. Useful for identifying rate limiting, WAF behavior, and session expiry.
**Effort:** M
**Dependencies:** Traffic proxy (already built).
**Rationale:** Traffic analysis is currently limited to search and individual exchange inspection. Temporal analysis reveals patterns that individual exchanges don't.

### 6.7 Demo GIF / screencast

**What:** An animated demo (GIF or asciinema recording) showing cc-artex in action: initializing an engagement, running reconnaissance, discovering a finding, and generating a report. Embed in the README.
**Effort:** S
**Dependencies:** None.
**Rationale:** A 60-second visual walkthrough communicates the tool's value faster than any amount of documentation.

### 6.8 Multi-client MCP support

**What:** Support multiple simultaneous MCP client connections over the HTTP transport, with per-client session state and shared engagement data.
**Effort:** M
**Dependencies:** Collaborative multi-agent support (6.3) would benefit from this.
**Rationale:** The HTTP transport currently serves one client. Supporting multiple clients enables team-based workflows where different operators or agents connect to the same running server.
