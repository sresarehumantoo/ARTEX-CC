# Changelog

All notable changes to cc-artex are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Go Report Card, Go Reference, and License badges in README
- Traffic rewrite rule validation tests (`rule_test.go`)

### Improved
- Test coverage: config 96%, mcpserver 93.5%, hookvm 89.1%, rewrite 97.6%, evidence 87.9%
- Fixed goreleaser web build hook (`bash -c` wrapper)

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

[Unreleased]: https://github.com/SREsAreHumanToo/cc-artex/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/SREsAreHumanToo/cc-artex/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/SREsAreHumanToo/cc-artex/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/SREsAreHumanToo/cc-artex/releases/tag/v0.1.0
