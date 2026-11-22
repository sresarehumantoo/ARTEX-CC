# Changelog

All notable changes to cc-artex are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.3.0] - 2026-11-22

### Added
- `cc-artex health --format json` for structured health check output (#105)
- `cc-artex health --timeout` for configurable health check timeout (#108)
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
- `cc-artex health` subcommand for PostgreSQL connectivity checks
- `cc-artex completion bash/zsh/fish` for shell completions
- `cc-artex version --json` for structured version output
- `cc-artex serve --timeout` for configurable graceful shutdown deadline
- Codecov coverage reporting in CI
- `gofmt -s` check in CI pipeline
- Tool reference documentation (docs/TOOLS.md)
- MCP client configuration guide (docs/MCP_CLIENTS.md)
- CHANGELOG.md
- AGENTS.md contributor guide
- CODE_OF_CONDUCT.md

### Improved
- Error message when CC_ARTEX_PG is not set now shows both options with examples
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

[Unreleased]: https://github.com/SREsAreHumanToo/cc-artex/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/SREsAreHumanToo/cc-artex/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/SREsAreHumanToo/cc-artex/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/SREsAreHumanToo/cc-artex/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/SREsAreHumanToo/cc-artex/releases/tag/v0.1.0
