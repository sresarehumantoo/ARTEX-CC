# Changelog

All notable changes to ARTEX are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed
- Captured traffic blobs, the traffic index directory and saved evidence are owner-only (files 0600, directories 0700) instead of world-readable, since they can hold credentials and session tokens. Existing traffic trees (including host directories captured by older versions, but not `_ca`) are tightened once at the first startup, and existing evidence trees at every startup.

### Fixed
- The web Graph page crashed with "Cannot read properties of null" on an engagement with no edges yet, because `GET /api/graph` returned `null` for empty `nodes` or `edges`; both are now always arrays.

### Fixed
- `POST /api/hints` (the web UI's Add hint) returned 500 because it stored hints in the `open` state, which the database rejects; it now stores them `active`, the same as `add_hint`.

### Added
- CI runs `govulncheck` against the module on every push and pull request.

### Fixed
- Bumped `github.com/gorilla/websocket` to v1.5.4-0.20240701034025-d67f41855da4 (GO-2026-6278, reached through the recording proxy) and `github.com/klauspost/compress` to v1.20.2 (GO-2026-6865).

### Added
- Target change journal. `record_target_change` records a change to a target (target, resource, kind, original fingerprint, backup, intent) before it is made; when another unrestored change on the same target and resource exists, it returns a warning naming who recorded it and the baseline, the first recorded original fingerprint, which is what to restore to. `record_target_restore` takes change ids and the fingerprint observed after restoring, and marks changes restored only on an exact match with the baseline; a mismatch is recorded as a failed attempt. `graph_overview` and `list_changes` list `unrestored_target_changes`, `generate_report` has a Target changes section, and engagement export and import carry the journal. The orchestration guide tells agents to record changes before making them and never to report a host restored while unrestored changes remain.

### Fixed
- Opening the store (`serve` or any CLI command) skips re-running `schema.sql` when the database already has the same schema, so starting a server or CLI against a database another server is using no longer risks `deadlock detected` errors in either one.

### Fixed
- `docker-compose.yml` built the default `ARTEX_PG` with a hardcoded `ccartex:ccartex` login, so overriding `POSTGRES_PASSWORD` (or `POSTGRES_USER`/`POSTGRES_DB`) left ARTEX unable to connect. The default DSN now uses the same variables and defaults as the postgres service.

### Fixed
- Asset URLs kept any `user:pass@` userinfo, so credentials surfaced in the graph, web UI and reports, and URLs differing only in scheme or host case or a default port (`:80` for http, `:443` for https) were stored as separate assets. URLs are now stored without userinfo (including a password containing a raw `/`, `?` or `#`), with a lowercase scheme and host and no default port; path, query and a Unicode host are kept as given, and `url==` and `url!=` filters match the same normalized form. Userinfo is recorded as a task credential for the host with source `asset_url` (percent-decoded; a username without a password is kept), and never replaces an existing credential for the same host and username. Importing an engagement export captures userinfo from its asset URLs the same way, after the exported credentials, so an exported credential for the same host and username is kept. Tool errors that echo an asset URL no longer include its userinfo.
- Credential domains were stored with their original case, so `Example.com` and `example.com` were separate credentials. Saved and imported credential domains are now lowercased; existing rows are not rewritten.

## [0.6.1] - 2026-11-25

### Added
- `serve --require-identity` (or `ARTEX_REQUIRE_IDENTITY=1`), off by default, makes `start_intent`, `complete_intent` and `cancel_intent` refuse callers without a verified `X-Artex-Identity`, whatever session or `owner` they have, with an error pointing at `artex-cc identity issue`. `list_changes` returns `claims_error` instead of `my_claims` for such callers; other tools are unaffected. `start_intent`'s description and the orchestration guide state the requirement when it is on. It applies to HTTP only: `serve --stdio` with it set exits at startup, and an `ARTEX_REQUIRE_IDENTITY` value that is not a boolean is a startup error. The eval runner clears an inherited value for the servers it starts in its anonymous modes (stdio and `--identity=false`).
- The web UI Dashboard's Active Intents card shows who holds each running intent's claim and how long it has left (`chat/<conversation id>` names shortened, full name on hover), and lists intents whose claim lapsed with a "claim lapsed" hint naming the previous holder.
- Eval runner: `--env KEY=VALUE` (repeatable) adds variables to every Claude instance's environment.

### Changed
- A request whose `X-Artex-Identity` fails to verify gets HTTP 400 instead of 401, with the same reason in the body. Claude Code reads a 401 as an OAuth challenge and hid the reason behind a Dynamic Client Registration error; a 403 would have cached the server as needing authentication. Bearer token failures still get 401.
- Each rejected identity logs one warning with the reason (`malformed`, `bad_signature`, `expired`, `unknown_key` or `header`), the claimed name and key ID from the unverified token, and the remote address. The token is never logged.

### Fixed
- Eval runner: scenario `scope` entries are now registered with kinds the store accepts (`cidr`, `ip`, or `subdomain` for an exact host), and a run stops if any entry fails to apply. Previously scope was silently not applied.

## [0.6.0] - 2026-11-25

### Added
- HTTP MCP clients can send an `X-Artex-Identity` token signed with the data directory's `identity.key`, on both `--web` and plain `--listen`. A verified identity holds claims as `id:<name>` (or `id:<name>/owner:<x>` with `owner`), independent of the MCP session, so it keeps its claims across reconnects, server restarts and protocol eras, and needs no `owner` on protocol 2026-07-28. A header that fails to verify (malformed, forged, expired, or from another data directory or a rotated key) gets HTTP 401 after bearer auth. Requests without the header behave as before.
- `list_sessions` returns an `identities` array (`name`, `self`, `last_active`) of identities that made requests to this server process within the claim lease, so clients on protocol 2026-07-28 show up.
- `artex-cc identity issue` prints a signed `X-Artex-Identity` token for an HTTP MCP client (`--format token|header|claude-json`, `--ttl` default 7d, max 30d), offline from the data directory's `identity.key`. Names under `chat/` and `eval/` are refused.
- `artex-cc identity rotate` replaces `identity.key` atomically, revoking every token issued from that data directory.
- `artex-cc identity show` prints the current key ID, or decodes a token (argument or `-` for stdin) and reports its name, key ID, issue and expiry times and whether it verifies, without printing the token.
- Shell completion for the `identity` subcommands.
- Eval runner: with `--transport http`, each instance gets its own `X-Artex-Identity` named `eval/<run>/i<n>`, minted per run with a TTL of the scenario timeout plus the lease and written only to that instance's 0600 MCP config in the temporary data directory. Prompts no longer ask HTTP instances to pick an `owner`. `--identity=false` keeps the anonymous owner path. The coordination scorecard parses identity holder labels, lists each instance's `holders`, attributes unhandled conflicts to the holding instance, and counts `holder_mismatches`.

### Changed
- Claims held by an identity are labeled with the identity name in `claimed_by`, `list_changes`, claim errors and exports (`<name>/h-...` when `owner` subdivides it). Anonymous holders keep their hashed labels.
- The MCP HTTP transport's logs redact anything shaped like an identity token.
- `serve` over HTTP (`--web` or `--listen`) loads or creates `identity.key` in the data directory at startup and exits if it cannot; stdio is unaffected.
- Web UI chat conversations hold claims as the identity `chat/<conversation id>`, minted from `identity.key` each time a conversation starts or resumes, so labels show that name and a resumed conversation keeps its claims across a server restart. This replaces the in-memory `X-Artex-Conversation` signer; a request still sending `X-Artex-Conversation` gets HTTP 400 telling the caller to start a new conversation.

## [0.5.1] - 2026-11-25

### Added
- Web UI chat conversations hold intent claims automatically through a server-signed per-conversation identity, so their first `start_intent` succeeds without an `owner` argument and separate conversations never share claims.
- Eval runner: `--instances`, `--transport stdio|http`, `--kill-after` and `--lease` run several Claude instances on one engagement, save each instance's transcript, and score coordination (claims, conflicts, duplicates, takeover) in the scorecard and `coordination.txt`.

### Changed
- `set_goals` sets goals once per engagement: concurrent calls serialize, later calls return `existing_goals` instead of adding more unless `append: true`, and goals matching an existing one are skipped. The response is now always JSON.
- `possible_duplicates` is scoped to the engagement and matches findings on a normalized vulnerability class (with aliases such as BOLA and IDOR), excluding dismissed and merged findings, and returns `{id, vulnclass, severity, summary}` entries.
- The orchestration guide tells parallel agents to adopt existing goals, check for duplicate findings before reporting, and poll `list_changes` instead of sleeping when every open intent is claimed.
- `serve --listen` without `--web` refuses to start on a non-loopback address (anything other than `localhost`, `127.0.0.0/8` or `::1`, including `:9400` and `0.0.0.0`) unless `ARTEX_MCP_TOKEN` is set, and then requires it as a bearer token on `/mcp`. Previously such a listener exposed every tool without authentication. Loopback listeners are unchanged, and honor `ARTEX_MCP_TOKEN` when it is set.
- `list_sessions` returns each session's hashed holder `label` and a `self` flag instead of its raw session ID.
- Claim holder labels are keyed with a random secret stored in `owner-label.key` in the data directory (created on first start, mode 0600), so processes sharing a data directory show the same labels. Labels shown before this change no longer match.

### Fixed
- Any MCP client could read other clients' session IDs from `list_sessions` and replay one as `Mcp-Session-Id` to complete or cancel intents claimed by that session.
- A low-entropy `owner` could be recovered from its claim label by guessing, since the label was an unkeyed hash.
- Session register and unregister debug logs, which reach `docker logs` and the web Logs page, carried raw session IDs; they now log the holder label.
- The MCP HTTP transport's own logs (rejected GETs, expired session sweeps, sampling responses, stream resume and event purge failures, session termination failures) carried raw session IDs; they now log the holder label. With `--web` these logs also reach the web Logs page.

## [0.5.0] - 2026-11-25

### Added
- Exec plugins can return `assets`, `facts` and `findings`, persisted through `insert_assets`, `record_fact` and `report_finding` with scope enforcement, and each invocation receives the active engagement, its scope, the data directory and the recording proxy address.
- Example plugin `examples/plugins/httpprobe.py` and a [Plugins](https://artex-cc.com/docs/plugins/) docs page.
- Intent claiming: `start_intent` takes a renewable 30-minute lease for the calling session or `owner`, other holders are refused until it expires, and `graph_overview` shows hashed holder labels. `cancel_intent` and `complete_intent` respect live claims.
- `list_changes` tool for polling new and updated findings, facts, intents, goals, hints and assets since a cursor, plus the caller's held claims; `graph_overview` returns a starting `changes_cursor`.
- Cross-process change notifications over PostgreSQL LISTEN/NOTIFY, so `serve --web` shows changes made by other artex-cc processes on the same database.
- Orchestration guide and hands-free docs cover running several agents on one engagement.
- `ARTEX_INTENT_LEASE` sets the claim lease in whole minutes (default 30m), for tests and evals that need claims to lapse quickly.

### Changed
- Plugin limits: 8 MiB stdout, 500 structured items per result, 1 minute to persist results, 4 KiB of error text.
- Web UI refreshes the views each mutating tool affects (including intent and plugin tools) and coalesces bursts of changes into one refresh per event type. The "Data refreshed" toast is gone.
- Migrations add `exploration_nodes.claim_expires_at` and `findings.updated_at`. Intents already running before the upgrade have no lease and are immediately claimable.

### Fixed
- Data races on the MCP server's active engagement during `switch_engagement`.
- Plugin results after `switch_engagement` were written into the startup engagement.
- The example plugin probed `file://` and `ftp://` URLs on in-scope hosts.
- Web UI: the Dashboard's Assets card showed the fact count, and active intents showed `Intent #N` instead of their summary.

### Removed
- Retired Go Report Card badge from the README.

## [0.4.1] - 2026-11-24

### Added
- `artex-cc tools [--json]` prints the MCP tool catalog (name, category, description, input schema, read-only and proxy-only flags) without a database.
- Project site at https://artex-cc.com: docs, evals, changelog, a blog, a tool reference generated from `artex-cc tools --json`, and a static live demo of the web UI at `/demo/`.
- Homebrew cask: releases publish `artex-cc` to `SREsAreHumanToo/homebrew-tap`. Install with `brew install --cask sresarehumantoo/tap/artex-cc` on macOS or Linux (Homebrew 7.0.11 or later).
- Docker image ships a pentest tooling layer (nmap, impacket, evil-winrm, nuclei, ffuf and more); opt out with `--build-arg PENTEST_TOOLS=0`.
- Eval runner: `--target`, `--model`, `--mcp-config` and `--skip-health` flags, and TCP health checks for non-HTTP targets.

### Changed
- `artex-cc` is the command name throughout the CLI help, init output and docs. Shell completions now register for `artex-cc` (they targeted `artex`, so they never attached to the installed binary).
- README trimmed to an overview; full documentation moved to https://artex-cc.com.

### Fixed
- `insert_assets` failed with a NOT NULL violation when re-inserting a known HTTP service with no technologies.
- Web UI: the Report page crashed on activity entries without a summary, and Activity search crashed on entries without a tool or worker.
- Web UI: doubled page titles on several pages, and the command palette needing Escape twice to close.

## [0.4.0] - 2026-11-24

### Breaking
- Project renamed from cc-artex to ARTEX-CC. The binary is now `artex-cc` (was `cc-artex`) and the Go module is `github.com/SREsAreHumanToo/ARTEX-CC`. MCP tool names (`mcp__artex__*`) and `ARTEX_*` environment variables are unchanged.
- Relicensed from MIT to AGPL-3.0-only. Parts of ARTEX-CC are derived from AGPL-3.0 [ARTEX](https://github.com/BaaRai/ARTEX); see NOTICE. Releases v0.1.0 through v0.3.0 carried an MIT license file in error and are AGPL-3.0 as well.

### Added
- Web UI (`serve --web`, enabled in the Docker image): chat with a Claude subprocess runner, agents and presets, engagement tasks, live logs, commands log, intercept approvals, MCP server config, tool catalog, skills, token usage, workspace browser, LLM settings, and password auth.
- Exec-based plugin system for registering external tools.
- Multi-client MCP session tracking and the `list_sessions` tool.
- `traffic_patterns` tool for temporal traffic analysis and `traffic_clear` to delete captured traffic by host.
- Request size statistics in `traffic_stats`.
- `list_activity` `--since` filter and CSV export.
- `artex-cc init --dry-run` and `--format json`.
- `ARTEX_LISTEN` environment variable for the listen address.
- SQLite traffic DB health check.
- Docker image builds the web frontend and installs the Claude CLI.
- ARTEX-CC logo, favicon and brand mark; web UI links to its source code (AGPL section 13).

### Fixed
- `report_finding` and scanner ingest now create a graph node for each finding, so findings show up in `graph_overview`, the dashboard and the graph, and can be passed to `prove_goal`. `report_finding` returns the `node_id` to use.
- Dismissed findings and merged duplicates no longer count in the graph overview.
- `traffic_search` and `traffic_har_export` only ever returned empty-body responses.
- `traffic_stats` failed whenever average sizes were not whole numbers.
- `bind_finding_traffic` accepted snapshot ids that were never captured; it now validates them and is registered only with `--proxy`.
- Docker docs: HTTP MCP endpoint is `/mcp` with a bearer token (not `/sse`), the binary path is `/app/artex-cc`, and the stdio example no longer collides with the main proxy port.
- SSE connections flush on connect so the web UI's `EventSource.onopen` fires.
- Goreleaser v2 deprecation warnings.
- Windows builds failed because the web chat runner used Unix-only process-group calls.

## [0.3.0] - 2026-11-22

### Added
- `artex health --format json` for structured health check output (#105)
- `artex health --timeout` for configurable health check timeout (#108)
- Shell completions for `--format` and `--timeout` health flags in bash/zsh/fish (#106)
- Web UI: search page, coverage filter, traffic explorer with URL params
- Web UI: finding detail, asset chart, interactive legend, activity indicator
- Web UI: credentials export, activity export, report executive summary
- Web UI: 404 page, loading skeletons, settings stats and progress
- Web UI: timeline chart, severity bar, traffic export, empty states
- Web UI: connection banner, recent findings, syntax highlighting
- Web UI: graph tooltips, palette view-all, dashboard banner
- API handler tests (0% to 74% coverage)
- Tools test coverage pushed from 57.6% to 82.5% with evidence, ingest, traffic, export, and report tests
- Go Report Card, Go Reference, and License badges in README
- Traffic rewrite rule validation tests (`rule_test.go`)
- Good-first-issue templates (#105-#109)
- GitHub repository topics and metadata

### Changed
- Migrated `log.Printf` to structured `slog` in traffic/ and egress/
- 910+ test runs across all packages (up from 480+)

### Fixed
- `gofmt -s` struct alignment in health.go
- Goreleaser web build hook (`bash -c` wrapper)
- Missing `Link` import in Settings.tsx
- Homebrew tap upload permission (skipped until PAT configured)
- Ingest test data corrections

## [0.2.0] - 2026-11-21

### Added
- `artex health` subcommand for PostgreSQL connectivity checks
- `artex completion bash/zsh/fish` for shell completions
- `artex version --json` for structured version output
- `artex serve --timeout` for configurable graceful shutdown deadline
- Codecov coverage reporting in CI
- `gofmt -s` check in CI pipeline
- Tool reference documentation (docs/TOOLS.md)
- MCP client configuration guide (docs/MCP_CLIENTS.md)
- CHANGELOG.md
- AGENTS.md contributor guide
- CODE_OF_CONDUCT.md

### Improved
- Error message when ARTEX_PG is not set now shows both options with examples
- CONTRIBUTING.md with dev setup, testing, code style sections
- Roadmap refreshed with current stats
- Test coverage: report 96.1%, egress 98.9%

## [0.1.1] - 2026-11-21

### Added
- .deb package support via goreleaser nfpms
- Demo GIF in README
- Social preview image for GitHub SEO
- Homebrew tap shared with dfinstall at SREsAreHumanToo/homebrew-tap

## [0.1.0] - 2026-11-21

### Added
- 59 MCP tools across 10 categories
- PostgreSQL-backed exploration graph (goals, intents, facts, findings, hints, constraints)
- Asset management with bulk insert, DSL-based queries, and 6 asset types
- Finding triage: merge duplicates, dismiss false positives, duplicate detection
- Credential storage with AES-256-GCM encryption at rest
- Evidence capture (save, list, get, capture_command_output)
- Coverage ledger with gap analysis
- Report generation (markdown and CSV export)
- Passive reconnaissance (enrich_domain, enrich_ip, enrich_service)
- Scanner ingestion (nmap, nuclei, httpx, ffuf, masscan)
- Scope enforcement with override mechanism
- Multi-engagement switching
- Activity logging and audit trail
- Full MITM traffic recording proxy with SQLite index, HAR export, request replay
- Traffic rewrite engine with custom hook VM
- Egress governor with rate limiting profiles
- SOCKS5 proxy with UDP relay
- Raw TCP/UDP capture
- Transparent proxy (Linux)
- Intercept rules with allow/deny/ask actions
- Web UI with dashboard, graph visualization, findings, assets, and traffic views
- Self-hosted evaluation harness (Docker-based with DVWA, Juice Shop)
- Engagement export/import to portable JSON archives
- Cross-platform binaries via goreleaser
- Dockerfile for container deployments
- GitHub Actions CI with PostgreSQL service container

[Unreleased]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.6.1...HEAD
[0.6.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/releases/tag/v0.1.0
