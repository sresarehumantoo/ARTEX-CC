# Changelog

All notable changes to ARTEX are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed
- Homebrew install: releases now publish an `artex-cc` cask to `SREsAreHumanToo/homebrew-tap`. Install with `brew install --cask sresarehumantoo/tap/artex-cc` on macOS or Linux.

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

[Unreleased]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/releases/tag/v0.1.0
