# Agents

Instructions for AI agents contributing to the ARTEX codebase.

## Build and test

```sh
go build ./...
go test ./...
go vet ./...
```

Store tests need PostgreSQL and are skipped without the env var:

```sh
ARTEX_TEST_PG="postgres://localhost/ccartex_test" go test ./store/...
```

`go test ./...` is always safe to run without a database.

## Project structure

```
cmd/artex-cc/      CLI entrypoint: serve, init, health, tools, identity, completion, version
store/             PostgreSQL data layer (assets, graph, findings, credentials, coverage)
store/migrations/  Numbered SQL migrations, embedded at build time
mcpserver/         MCP protocol server (stdio and HTTP transports)
tools/             Tool implementations registered with the MCP server
config/            Configuration loading and engagement state (.artex.json)
identity/          Signed client identity tokens (X-Artex-Identity)
traffic/           Recording MITM proxy, SOCKS5, replay, HAR export, rewrite engine
traffic/rewrite/   Rule-based request/response rewriting
traffic/hookvm/    Sandboxed expression evaluator for rewrite hooks
evidence/          Evidence capture and storage
report/            Markdown report generation with quality analysis
egress/            Rate-limiting governor for outbound requests
enrich/            DNS and HTTP enrichment logic
ingest/            Scanner output parsers (nmap, nuclei, httpx, ffuf, masscan)
eval/              Evaluation harness
```

## Module

```
github.com/SREsAreHumanToo/ARTEX-CC
```

## Database

PostgreSQL is the only backend. Connection string comes from `--pg` or `ARTEX_PG`. The schema (`store/schema.sql`) is applied on first connect. Migrations in `store/migrations/` are embedded via `//go:embed`, checksummed, and applied in order. Add new migrations as `store/migrations/NNNN_slug.sql` with the next sequence number.

## Key interfaces

`tools.Store` in `tools/deps.go` is the central interface. It defines every data-layer method the tools call. `store.Store` implements it (`var _ tools.Store = (*Store)(nil)` in `store/store.go`). The mock in `tools/tools_test.go` (`mockStore`) must also satisfy it. When you add a Store method, update all three: the interface, the real implementation, and the mock.

## Tool registration

Tools are defined in `tools/` grouped by category: `asset_tools.go`, `exploration_tools.go`, `finding_tools.go`, `credential_tools.go`, `scope_tools.go`, `engagement_tools.go`, `report_tools.go`, `traffic_tools.go`, `evidence_tools.go`, `enrich_tools.go`, `ingest_tools.go`, `activity_tools.go`, `intent_tools.go`, `changes_tools.go`, `target_change_tools.go`, `export_tools.go`.

Each category has a `Register*Tools` function called from `tools/register.go` (`RegisterAll`). Traffic tools are the exception: they depend on `*traffic.Traffic` and are registered separately in `cmd/artex-cc/main.go` when `--proxy` is set. Evidence tools also register separately (they need the evidence store).

A `Tool` has `Name`, `Description`, `Schema`, and `Run`. Schemas use the helpers in `tools/helpers.go` (`obj`, `str`, `enm`, etc.). Set `ReadOnly: true` on tools that don't modify state.

## Code style

- Use `log/slog` for logging. No `log.Printf`, no `fmt.Printf` to stderr.
- Match the existing style in whatever file you are editing.
- No comments that restate what the code does. Comment only non-obvious constraints.
- Errors are returned, not logged and swallowed.

## Branch workflow

Feature branches off `main`, PRs reviewed before merge. Run `make ci` (gofmt check, `go vet`, golangci-lint, `go test`) before pushing. CI runs the same checks.
