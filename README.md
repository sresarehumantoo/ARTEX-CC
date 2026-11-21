# cc-artex

MCP tool server that exposes pentest capabilities from [ARTEX](https://github.com/Autumn-27/artex) for Claude Code.

## What is this?

cc-artex turns ARTEX's autonomous pentest capabilities into an MCP tool server. Instead of running its own LLM agent loop (which requires a separate API key), it exposes the asset graph, exploration tracking, finding management, and reporting as tools that Claude Code calls through your existing Claude subscription.

You bring the brain. cc-artex brings the hands.

## How it works

```
┌─────────────┐         MCP (stdio/HTTP)         ┌──────────┐
│  Claude Code │◄────────────────────────────────►│ cc-artex │
│  (the brain) │                                  │  (tools) │
└─────────────┘                                   └────┬─────┘
                                                       │
                                          ┌────────────┼────────────┐
                                          │            │            │
                                     ┌────▼───┐  ┌────▼───┐  ┌────▼────┐
                                     │  PG DB │  │ proxy  │  │ SQLite  │
                                     │ assets │  │ traffic│  │ traffic │
                                     │ graph  │  │ capture│  │  store  │
                                     └────────┘  └────────┘  └─────────┘
```

Claude Code decides what to test, what tools to call, and in what order. cc-artex executes the calls: inserting assets, recording findings, managing the exploration graph, and capturing traffic. The exploration graph keeps Claude Code oriented across long engagements.

## Quick start

Prerequisites: PostgreSQL, Go 1.22+

```bash
go install github.com/SREsAreHumanToo/cc-artex/cmd/cc-artex@latest

createdb ccartex
cc-artex init --pg "postgres://localhost/ccartex" --goal "Pentest example.com"
```

Add to your Claude Code MCP config (`~/.claude.json` or project `.mcp.json`):

```json
{
  "mcpServers": {
    "cc-artex": {
      "command": "cc-artex",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex"]
    }
  }
}
```

## Tool catalog

**Asset Management**

| Tool | Description |
|------|-------------|
| `insert_assets` | Add domains, IPs, services, endpoints, or apps to the asset graph |
| `list_assets` | Query assets by type, DSL filter, or ID |
| `list_companies` | List tracked companies and their scope |
| `add_company_scope` | Add scope rules (domains, CIDRs) to a company |
| `add_task_scope` | Add in-scope markers for the current task |
| `list_untested_assets` | Find assets with no coverage records |

**Exploration Graph**

| Tool | Description |
|------|-------------|
| `graph_overview` | Summary of the current exploration state |
| `add_intent` | Plan a next step (queued for execution) |
| `record_fact` | Record a discovery or observation |
| `list_facts` | List recorded facts |
| `node_detail` | Inspect a specific graph node |
| `add_hint` | Add a hint to guide exploration |
| `set_goals` | Define or update engagement goals |
| `set_constraints` | Set rules the engagement must follow |
| `prove_goal` | Mark a goal as proved with evidence |

**Findings**

| Tool | Description |
|------|-------------|
| `report_finding` | Report a vulnerability with severity, evidence, and affected assets |
| `list_findings` | List findings for the current exploration |

**Credentials**

| Tool | Description |
|------|-------------|
| `save_task_credential` | Store a discovered credential |
| `list_task_credentials` | List stored credentials |

**Coverage**

| Tool | Description |
|------|-------------|
| `record_coverage` | Record what was tested, the outcome, and evidence |

## Architecture

| Package | Purpose |
|---------|---------|
| `cmd/cc-artex` | CLI entrypoint (serve, init, version) |
| `store` | PostgreSQL data layer (assets, graph, findings, credentials) |
| `mcpserver` | MCP protocol server (stdio and HTTP transports) |
| `tools` | Tool implementations registered with the MCP server |
| `config` | Configuration loading and engagement state |
| `traffic` | HTTP traffic capture and storage (future) |

## vs ARTEX

| | ARTEX | cc-artex |
|---|-------|----------|
| LLM | Built-in agent loop (needs API key) | None. Claude Code is the LLM. |
| Interface | Autonomous agent | MCP tool server |
| Asset graph | Yes | Same model |
| Exploration tracking | Yes | Same model |
| Findings | Yes | Same model |
| Cost | API usage | Your Claude subscription |

## License

MIT
