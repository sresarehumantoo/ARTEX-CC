cc-artex is an MCP tool server that exposes pentest capabilities (asset graph, exploration tracking, finding management, coverage recording) for Claude Code. It makes zero LLM API calls; Claude Code is the LLM.

## Build

    go build ./cmd/cc-artex

## Test

    go test ./...

## Lint

    go vet ./...

## Module

    github.com/SREsAreHumanToo/cc-artex

## Architecture

- `store/` -- PostgreSQL data layer (assets, exploration graph, findings, credentials, coverage)
- `mcpserver/` -- MCP protocol server (stdio and HTTP transports)
- `tools/` -- Tool implementations registered with the MCP server
- `config/` -- Configuration and engagement state (.cc-artex.json)
- `cmd/cc-artex/` -- CLI entrypoint
