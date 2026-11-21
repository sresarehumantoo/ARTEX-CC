# cc-artex Roadmap

## 1. Current state summary

cc-artex is a working MCP tool server for penetration testing. It exposes ARTEX's
data model as tools Claude Code calls directly, with no separate LLM API key.

**Lines of code:** ~19,600 Go (63 source files, 4 test files with ~1,000 LOC).

**Tool count:** 26 registered tools (plus 7 traffic tools when the proxy is enabled), totaling 33.

**What works today:**

- **CLI:** `serve` (stdio and HTTP transports), `init` (creates engagement, schema, CA cert), `version`.
- **Store (PostgreSQL):** Full schema with auto-migration. Assets (6 types), companies, company scope, task scope, exploration graph (nodes, edges, anchors, constraints), findings, credentials, coverage ledger with history, intercept rules, activity log.
- **Exploration graph:** Goals, intents, facts, findings, hints, constraints. Edges connect them (derived_from, yields, spawns, proves). Frontier query. Graph overview for situational awareness. Prove-goal workflow with confidence gating.
- **Asset management:** Bulk insert with upsert, DSL-based queries (fuzzy, exact, exclusion, numeric comparisons, boolean combinators, parentheses), untested-asset queries, company scope attribution.
- **Findings and credentials:** Report findings with severity/evidence/assets, list findings. Save and list credentials with upsert-on-domain+username.
- **Coverage:** Record coverage entries (surface x risk_area x outcome), coverage history.
- **Report generation:** Markdown report from findings, assets, and coverage. Finding quality assessment (PoE level, verification status, counterevidence). CSV and markdown export formats.
- **Traffic recording proxy:** Full MITM proxy with SQLite index, blob storage, body spill for large payloads, WebSocket/SSE stream capture, URL path templating, host listing, paginated search (including body full-text search), HAR 1.2 export, request replay with modifications.
- **Traffic rewrite engine:** Rule-based request/response rewriting with a custom hook VM (eval.go, stdlib, validation). Rewrite flow with match/transform pipeline.
- **Egress governor:** Rate limiting with profiles (quiet/balanced/aggressive/off), per-host and global concurrency caps, jitter, 429/503 backoff, GET cache/coalescing, configurable user agent.
- **External traffic ingest:** Browser-extension capture endpoint.
- **SOCKS5 proxy:** SOCKS5 with UDP relay support.
- **Raw TCP/UDP capture.**
- **Transparent proxy (Linux):** iptables-based transparent interception.
- **Intercept rules:** Tool-call interception with allow/deny/ask actions and timeout policy.
- **MCP resource:** `artex://engagement` resource serves the orchestration guide with live engagement context.
- **CI:** GitHub Actions with `go build`, `go vet`, `go test`, and store tests against a real PostgreSQL instance.
- **Community files:** MIT license, CONTRIBUTING.md.


## 2. P0: Ship blockers

These must be addressed before recommending cc-artex to users outside the author's circle.

### 2.1 SECURITY.md

**What:** No responsible disclosure policy exists. An offensive security tool needs one more than most.
**Effort:** S
**Dependencies:** None.
**Rationale:** Users running this against production systems need to know how to report vulnerabilities in cc-artex itself, and downstream consumers need to trust the project takes security seriously.

### 2.2 Credential encryption at rest

**What:** `task_credentials.password` is stored as plaintext in PostgreSQL. Anyone with database read access sees every credential the agent discovers.
**Effort:** M
**Dependencies:** None.
**Rationale:** This is an offensive security tool; discovered credentials are the highest-value data it stores. Even in a local-only deployment, plaintext passwords in a database are a poor default. Encrypt with a key derived from a user-supplied passphrase or a file-based key, with the passphrase prompted once at `serve` startup.

### 2.3 SQLite WAL mode and concurrent access safety

**What:** The traffic SQLite database is opened without explicit WAL mode or busy-timeout configuration. Under concurrent proxy writes and tool reads, this risks `SQLITE_BUSY` errors.
**Effort:** S
**Dependencies:** None.
**Rationale:** A recording proxy under load generates many writes per second. Without WAL mode and a busy timeout, read queries from MCP tools will intermittently fail.

### 2.4 Graceful shutdown of traffic proxy

**What:** The traffic proxy is started in a goroutine (`go t.Start()`) with no drain or connection-close logic. `t.Close()` is deferred but the proxy may have in-flight connections.
**Effort:** S
**Dependencies:** None.
**Rationale:** An unclean shutdown can corrupt the SQLite index or lose the last few captured exchanges.

### 2.5 Input validation hardening on tool schemas

**What:** Several tools silently swallow malformed JSON (`_ = json.Unmarshal(in, &a)`). If the LLM sends garbage, the tool runs with zero-value fields and either does nothing or inserts empty records.
**Effort:** M
**Dependencies:** None.
**Rationale:** Silent failures waste Claude's context window. Every tool should validate its required fields and return a clear error message.


## 3. P1: High-value features

Features that make cc-artex significantly more useful for real engagements.

### 3.1 Enrich package (DNS, whois, certificate transparency, tech fingerprinting)

**What:** An `enrich/` package that, given a domain or IP, runs passive reconnaissance: DNS resolution (all record types), reverse DNS, WHOIS, certificate transparency log search (crt.sh), and technology fingerprinting (Wappalyzer-style). Results feed directly into `insert_assets`. Expose as one or two MCP tools (`enrich_domain`, `enrich_ip`).
**Effort:** L
**Dependencies:** Egress governor (already built).
**Rationale:** Today Claude has to call external tools or write ad-hoc HTTP requests for reconnaissance. A built-in enrich pipeline with structured output eliminates the most tedious part of the workflow and produces cleaner asset data.

### 3.2 Evidence package (screenshot, proof-of-concept capture)

**What:** An `evidence/` package for capturing screenshots (headless Chrome via CDP) and PoC artifacts (request/response pairs snapshotted at the moment a finding is reported, with tamper-evident hashes). Expose as `capture_screenshot` and `snapshot_evidence` tools. Screenshots stored in the data directory with metadata in PG.
**Effort:** L
**Dependencies:** Traffic proxy (for request snapshots), Chrome/Chromium available on host.
**Rationale:** A pentest report without screenshots and reproducible PoC steps is incomplete. Currently evidence is free-text in the finding record. Structured evidence with screenshots makes reports credible.

### 3.3 Engagement export and import

**What:** `cc-artex export <task-id>` dumps the full engagement (assets, graph, findings, credentials, coverage, traffic metadata) to a portable JSON archive. `cc-artex import <archive>` loads it into a fresh database. Traffic blobs optionally included as a tar companion.
**Effort:** M
**Dependencies:** None.
**Rationale:** Engagements are currently locked to one PostgreSQL instance. Export/import enables sharing results, archiving completed engagements, and migrating between machines.

### 3.4 Multi-engagement switching at runtime

**What:** Today the server binds to one engagement (task_id + exploration_id from `.cc-artex.json`) at startup. Add a `switch_engagement` tool or extend `create_engagement` to update the server's active context without a restart.
**Effort:** M
**Dependencies:** None.
**Rationale:** Bug bounty workflows often involve multiple targets in one session. Restarting the server to switch is friction.

### 3.5 Intent lifecycle management

**What:** Tools to transition intent state: `start_intent` (open -> running), `complete_intent` (running -> done), `block_intent`, `pause_intent`. Today `add_intent` creates them and `graph_overview` shows them, but there is no tool to mark an intent as running or done; the graph has no way to know an intent was picked up.
**Effort:** S
**Dependencies:** None.
**Rationale:** Without state transitions, the graph cannot distinguish "open and nobody has started" from "someone is working on it" from "done." This is essential for subagent coordination and stale-intent detection.

### 3.6 Activity log tools

**What:** The `activity` table exists in the schema but no MCP tools expose it. Add `list_activity` (paginated, filterable by worker/node/tool) and wire tool calls to automatically log activity entries.
**Effort:** S
**Dependencies:** None.
**Rationale:** The activity log is the audit trail of what was done and when. Without tools to read it, the data is inaccessible from Claude's side.


## 4. P2: Quality and polish

### 4.1 Test coverage

**What:** Current test coverage is thin: 4 test files, ~1,000 LOC against ~18,500 LOC of source. Packages with no tests: `config/`, `egress/`, `report/`, `traffic/rewrite/`, `traffic/hookvm/`, `cmd/cc-artex/`.
**Effort:** L (ongoing)
**Dependencies:** None.
**Rationale:** The store and mcpserver tests cover the core path, but the egress governor, report generation, rewrite engine, and hook VM are untested. The rewrite engine and hook VM are particularly risky since they evaluate user-supplied rules.

Suggested priority within this item:
1. `egress/` (rate limiter correctness)
2. `traffic/rewrite/` and `traffic/hookvm/` (security-sensitive)
3. `report/` (output correctness)
4. `config/` (edge cases in loading)

### 4.2 Error message consistency

**What:** Some tools return `errResult("some error")` with inconsistent formatting. Some prepend a prefix ("save_task_credential failed:"), others don't. Some swallow unmarshal errors.
**Effort:** S
**Dependencies:** None.
**Rationale:** Consistent error messages help Claude recover from failures. A standard format (tool name + colon + message) would make errors parseable.

### 4.3 Structured logging

**What:** The server uses `log.Logger` with plain text. The traffic proxy logs to stderr. The egress governor logs to stderr. No correlation between log entries.
**Effort:** M
**Dependencies:** None.
**Rationale:** When debugging a failed engagement, structured logs (JSON, with request IDs and timestamps) make the difference between finding the issue in minutes vs. hours. `slog` from the standard library is sufficient.

### 4.4 README accuracy

**What:** The README says `traffic/` is "planned" in the architecture section, but it is fully implemented (2,430 LOC in `traffic.go` alone, plus rewrite engine, hook VM, SOCKS5, etc.). The tool catalog lists 20 tools; the actual count is 33 (26 core + 7 traffic). The architecture diagram doesn't show the egress governor or rewrite engine.
**Effort:** S
**Dependencies:** None.
**Rationale:** An inaccurate README undersells the project and confuses contributors.

### 4.5 MCP config snippet completeness

**What:** The init command prints a minimal MCP config snippet that doesn't include `--data` or `--proxy`. Users who want traffic recording have to figure out the flags on their own.
**Effort:** S
**Dependencies:** None.
**Rationale:** The setup flow should produce a working config for the full feature set, not just the minimum.


## 5. P3: Distribution and community

### 5.1 goreleaser configuration

**What:** No release automation. Users must `go install` from source or build manually.
**Effort:** S
**Dependencies:** None.
**Rationale:** Pre-built binaries for Linux, macOS, and Windows lower the barrier to entry. goreleaser integrates with GitHub Actions and produces checksummed archives.

### 5.2 Dockerfile

**What:** No container image.
**Effort:** S
**Dependencies:** None.
**Rationale:** A multi-stage Dockerfile (build + scratch/distroless) makes cc-artex deployable in any container environment. Useful for CI/CD pipelines and isolated pentest environments.

### 5.3 Homebrew formula

**What:** No Homebrew tap.
**Effort:** S
**Dependencies:** goreleaser (5.1).
**Rationale:** macOS users expect `brew install`. goreleaser can auto-generate the formula.

### 5.4 Example engagement walkthrough

**What:** A `docs/examples/` directory with one or two annotated engagement transcripts showing the full workflow: init, reconnaissance, vulnerability discovery, finding reporting, goal proving, report generation.
**Effort:** M
**Dependencies:** None.
**Rationale:** The README shows the tool catalog, but a real walkthrough demonstrates the exploration graph workflow and subagent dispatch pattern that makes cc-artex different from a bag of tools.

### 5.5 Good first issues

**What:** Tag 5-10 GitHub issues as "good first issue" covering small, self-contained improvements: add a missing validation, improve an error message, add a test for a specific function, support a new asset type.
**Effort:** S
**Dependencies:** None.
**Rationale:** Contributors need an on-ramp. Well-scoped issues with clear acceptance criteria attract first-time contributors.

### 5.6 pkg.go.dev documentation

**What:** Ensure all exported types and functions have godoc comments. Currently many exported types (e.g. in `tools/deps.go`) lack them.
**Effort:** S
**Dependencies:** None.
**Rationale:** Go developers expect browsable API documentation on pkg.go.dev.


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
**Dependencies:** Multi-engagement switching (3.4), intent lifecycle (3.5).
**Rationale:** Large engagements benefit from multiple agents working in parallel. PostgreSQL's advisory locks and LISTEN/NOTIFY provide the primitives; the challenge is the MCP-level coordination protocol.

### 6.4 Scope enforcement (not just recording)

**What:** Today task scope is a recording mechanism for coverage tracking. Add optional enforcement: before an MCP tool makes a request to a target, check whether the target is in scope and block out-of-scope actions.
**Effort:** M
**Dependencies:** Egress governor integration.
**Rationale:** Accidental out-of-scope testing is the top risk in penetration testing. Enforcement (with an override mechanism) adds a safety net.

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
