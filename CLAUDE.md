cc-artex is an MCP tool server that exposes pentest capabilities (asset graph, exploration tracking, finding management, coverage recording, traffic capture, report generation) for Claude Code. It makes zero LLM API calls; Claude Code is the LLM.

## Build

    go build ./cmd/cc-artex

## Test

    go test ./...

Store tests require a PostgreSQL instance:

    CC_ARTEX_TEST_PG="postgres://localhost/ccartex_test" go test ./store/...

## Lint

    go vet ./...

## Module

    github.com/SREsAreHumanToo/cc-artex

## Architecture

- `store/` -- PostgreSQL data layer (assets, exploration graph, findings, credentials, coverage)
- `mcpserver/` -- MCP protocol server (stdio and HTTP transports)
- `tools/` -- Tool implementations registered with the MCP server; traffic tools registered separately when --proxy is set
- `config/` -- Configuration and engagement state (.cc-artex.json)
- `traffic/` -- Recording MITM proxy, SOCKS5, replay, HAR export, rewrite engine
- `traffic/rewrite/` -- Rule-based request/response rewriting
- `traffic/hookvm/` -- Sandboxed expression evaluator for rewrite hooks
- `report/` -- Markdown report generation with quality analysis
- `egress/` -- Rate-limiting governor for outbound requests
- `cmd/cc-artex/` -- CLI entrypoint (serve, init, version)
