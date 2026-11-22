# cc-artex Roadmap

## 1. Current state summary

cc-artex is a working MCP tool server for penetration testing. It exposes ARTEX's
data model as tools Claude Code calls directly, with no separate LLM API key.

**Lines of code:** ~31,000 Go.

**Tool count:** 59 registered tools (core + traffic + evidence + enrich + ingest + scope + intent + activity + triage + coverage).

**Test count:** 890+ test runs across all packages.

**What works today:**

- **CLI:** `serve` (stdio and HTTP transports), `init` (creates engagement, schema, CA cert), `version` (with `--json`), `health` (PostgreSQL connectivity check), `completion` (bash/zsh/fish). `--version` flag on all subcommands. `--timeout` for graceful shutdown. `CC_ARTEX_PG` env var override.
- **Store (PostgreSQL):** Full schema with auto-migration. Assets (6 types), companies, company scope, task scope, exploration graph (nodes, edges, anchors, constraints), findings, credentials, coverage ledger with history, intercept rules, activity log. Engagement export/import to portable JSON archives.
- **Exploration graph:** Goals, intents, facts, findings, hints, constraints. Edges connect them (derived_from, yields, spawns, proves). Frontier query. Graph overview for situational awareness. Prove-goal workflow with confidence gating. Intent lifecycle (start, complete, cancel).
- **Asset management:** Bulk insert with upsert, DSL-based queries (fuzzy, exact, exclusion, numeric comparisons, boolean combinators, parentheses), untested-asset queries, company scope attribution.
- **Findings and credentials:** Report findings with severity/evidence/assets, list findings, merge duplicates, dismiss false positives, duplicate detection, bind findings to captured traffic. Save and list credentials with upsert-on-domain+username. AES-256-GCM encryption at rest for credential passwords.
- **Coverage:** Record coverage entries (surface x risk_area x outcome), coverage history, coverage gaps analysis (`list_coverage_gaps`).
- **Evidence capture:** `save_evidence`, `list_evidence`, `get_evidence`, and `capture_command_output` tools for structured proof-of-concept storage.
- **Report generation:** Markdown report from findings, assets, and coverage. Finding quality assessment (PoE level, verification status, counterevidence). CSV and markdown export formats for findings, assets, and nodes.
- **Enrich:** Passive reconnaissance tools (`enrich_domain`, `enrich_ip`, `enrich_service`) with DNS resolution, reverse DNS, WHOIS, certificate transparency, technology fingerprinting, and URL validation.
- **Scanner ingestion:** Parsers for nmap, nuclei, httpx, ffuf, and masscan (XML and JSON) output. Results feed directly into the asset graph.
- **Scope enforcement:** `check_scope` tool and automatic enforcement in asset, enrich, and traffic tools. Blocks out-of-scope actions with an override mechanism.
- **Multi-engagement switching:** `switch_engagement` tool for changing active context without a server restart.
- **Engagement management:** Update and delete engagements, activity timeline view.
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
- **CI:** GitHub Actions with fmt-check, `go build`, `go vet`, `go test`, Codecov coverage reporting, and store tests against a real PostgreSQL instance.
- **Distribution:** v0.1.1 released with cross-platform binaries (goreleaser), .deb packaging, Dockerfile, Homebrew formula auto-generated.
- **Documentation:** Tool reference (docs/TOOLS.md), MCP client configs (docs/MCP_CLIENTS.md), AGENTS.md contributor guide, demo GIF.
- **Community files:** MIT license, CONTRIBUTING.md, SECURITY.md, CODE_OF_CONDUCT.md, example engagement walkthrough, godoc comments on all exported types. Zero open issues, 76+ PRs merged.


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

### ✅ 3.2 Evidence package

Done. `save_evidence`, `list_evidence`, `get_evidence`, and `capture_command_output` tools in `evidence/` and `tools/evidence_tools.go`. Structured PoC storage with metadata in PostgreSQL.

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

### ✅ 3.7 Finding triage

Done. `merge_findings`, `dismiss_finding`, and duplicate detection tools for managing finding lifecycle.

### ✅ 3.8 Coverage gaps analysis

Done. `list_coverage_gaps` tool identifies untested surfaces and risk areas.

### ✅ 3.9 Engagement management

Done. `update_engagement`, `delete_engagement`, and activity timeline tools for full engagement lifecycle.

### ✅ 3.10 Masscan ingestion

Done. XML and JSON parser support in `ingest/` for masscan output, feeding results into the asset graph.

### ✅ 3.11 CSV export

Done. CSV export format for findings, assets, and nodes alongside existing markdown export.


## 4. P2: Quality and polish

### ✅ 4.1 Test coverage

890+ test runs across all packages. Integration-level multi-tool workflow tests added. Coverage reporting via Codecov. Nine packages above 85% coverage.

### ✅ 4.2 Error message consistency

Done. Standardized error format across all tools.

### ✅ 4.3 Structured logging

Done. `slog`-based JSON logging across all packages. Only `log.Fatal` calls remain in `eval/` entry points.

### ✅ 4.4 README accuracy

Done. Tool count corrected, architecture section updated, README restructured.

### ✅ 4.5 MCP config snippet

Done. `init` now shows both minimal and full configuration examples including `--data` and `--proxy` flags.


## 5. P3: Distribution and community

### ✅ 5.1 goreleaser

Done. Cross-platform binary builds for Linux, macOS, and Windows via GitHub Actions.

### ✅ 5.2 Dockerfile

Done. Multi-stage Dockerfile for container deployments.

### ✅ 5.3 Homebrew formula

Done. goreleaser auto-generates the formula to `SREsAreHumanToo/homebrew-tap`. Install with `brew install SREsAreHumanToo/tap/cc-artex`.

### ✅ 5.4 Example engagement walkthrough

Done. Annotated walkthrough in `docs/examples/` showing init through report generation.

### ✅ 5.5 Good first issues

Done. Good-first-issues created and resolved. Zero open issues.

### ✅ 5.6 pkg.go.dev documentation

Done. 64 godoc comments added to all exported types and functions.

### ✅ 5.7 CLAUDE.md and AGENTS.md

Done. Project-level CLAUDE.md and AGENTS.md for contributor and agent onboarding.

### ✅ 5.8 Release v0.1.0

Done. First tagged release with cross-platform binaries via goreleaser and GitHub Actions.


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

### ✅ 6.5 Finding deduplication and triage

Done. `merge_findings`, `dismiss_finding`, and `bind_finding_traffic` tools in `tools/finding_tools.go`.

### 6.6 Temporal analysis of traffic

**What:** Tools to analyze traffic patterns over time: request frequency by host, response time trends, status code distribution, new endpoints appearing. Useful for identifying rate limiting, WAF behavior, and session expiry.
**Effort:** M
**Dependencies:** Traffic proxy (already built).
**Rationale:** Traffic analysis is currently limited to search and individual exchange inspection. Temporal analysis reveals patterns that individual exchanges don't.

### 6.7 Demo GIF / screencast

**What:** An animated demo (GIF or asciinema recording) showing cc-artex in action: initializing an engagement, running reconnaissance, discovering a finding, and generating a report. Embed in the README.
**Effort:** S
**Dependencies:** None.
**Status:** Script exists but recording not yet done.
**Rationale:** A 60-second visual walkthrough communicates the tool's value faster than any amount of documentation.

### 6.8 Multi-client MCP support

**What:** Support multiple simultaneous MCP client connections over the HTTP transport, with per-client session state and shared engagement data.
**Effort:** M
**Dependencies:** Collaborative multi-agent support (6.3) would benefit from this.
**Rationale:** The HTTP transport currently serves one client. Supporting multiple clients enables team-based workflows where different operators or agents connect to the same running server.
