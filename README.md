<div align="center">

# cc-artex

**Use your Claude subscription to run pentests. No API key needed.**

[![CI](https://github.com/SREsAreHumanToo/cc-artex/actions/workflows/ci.yml/badge.svg)](https://github.com/SREsAreHumanToo/cc-artex/actions/workflows/ci.yml)
[![Go 1.26+](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

</div>

cc-artex is an MCP tool server that turns [ARTEX](https://github.com/Autumn-27/artex)'s autonomous pentest model into tools Claude Code calls through your existing Claude subscription. ARTEX's asset graph, exploration tracking, finding management, and coverage recording, exposed as 20 MCP tools. No separate LLM API key, no token budget, no agent loop of its own.

You bring the brain. cc-artex brings the hands.

> **Authorized use only.** This is an offensive security tool, for penetration
> testing, red teaming, CTFs and lab work. Use it only against systems you own
> or have explicit permission to test.

## How it works

```
                        MCP (stdio or HTTP)
  +-------------+ <---------------------------> +-----------+
  | Claude Code |                               |  cc-artex |
  | (the brain) |                               |  (tools)  |
  +-------------+                               +-----+-----+
                                                      |
                                        +-------------+-------------+
                                        |             |             |
                                   +----+----+  +-----+-----+ +----+-----+
                                   |  PG DB  |  |   proxy   | |  local   |
                                   |  assets |  |  traffic  | | traffic  |
                                   |  graph  |  |  capture  | |  store   |
                                   +---------+  +-----------+ +----------+
```

Claude Code decides what to test, what tools to call, and in what order. cc-artex executes the calls: inserting assets, tracking the exploration graph, recording findings and credentials, and logging coverage. The exploration graph keeps Claude oriented across long engagements so it doesn't repeat work or lose context.

## Why this exists

ARTEX is a capable pentest framework with its own LLM agent loop. That agent loop needs an API key, a token budget, and careful management. If you already pay for Claude, that is a second brain doing the same job.

cc-artex removes the agent loop entirely and exposes the data layer as MCP tools. Claude Code becomes the agent. You get:

- **No API cost.** Your Claude subscription covers it. No token counting, no surprise bills.
- **Claude's full reasoning.** Not a constrained agent loop, but Claude's complete context window and tool-use capability driving the engagement.
- **Your judgment in the loop.** Claude proposes, you approve. The exploration graph means you can review its plan, steer it, and pick up where it left off.

## Quick start

**Prerequisites:** PostgreSQL, Go 1.26+

```sh
# Install
go install github.com/SREsAreHumanToo/cc-artex/cmd/cc-artex@latest

# Create the database and initialize an engagement
createdb ccartex
cc-artex init --pg "postgres://localhost/ccartex" --goal "Pentest example.com"
```

The init command creates the schema, sets up the engagement, and prints the MCP config you need. Add it to your Claude Code config (`~/.claude.json` or project `.mcp.json`):

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

Then start Claude Code. The tools appear automatically. Ask Claude to "pentest example.com" and watch it work.

## What a session looks like

```
$ claude
> Pentest example.com, starting with reconnaissance.

Claude calls: graph_overview → set_goals → insert_assets (domain: example.com)
         → add_intent (enumerate subdomains) → record_fact (found: api.example.com)
         → insert_assets (subdomain: api.example.com, service: https/443)
         → add_intent (test api.example.com for auth bypass)
         → record_coverage (api /admin, idor, no_issue_found)
         → report_finding (severity: high, broken access control on /api/v2/users)
         → prove_goal (evidence: finding #1)
```

Each tool call updates the exploration graph. Claude sees where it has been, what it found, and what it planned next. If you stop and resume hours later, `graph_overview` brings it back up to speed.

## Tool catalog

### Assets and scope

| Tool | What it does |
|------|-------------|
| `insert_assets` | Add domains, IPs, services, endpoints, or apps to the asset graph |
| `list_assets` | Query assets by type, DSL filter, or ID |
| `list_companies` | List tracked companies and their scope |
| `add_company_scope` | Add scope rules (domains, CIDRs) to a company |
| `add_task_scope` | Add in-scope markers for the current task |
| `list_untested_assets` | Find assets with no coverage records |

### Exploration graph

| Tool | What it does |
|------|-------------|
| `graph_overview` | Situational summary: asset counts, goals, hints, frontier, findings, coverage |
| `add_intent` | Plan a next step (queued for execution) |
| `record_fact` | Record a discovery or observation |
| `list_facts` | List recorded facts with optional filters |
| `node_detail` | Inspect a specific graph node |
| `add_hint` | Add a strategic hint to guide exploration |
| `set_goals` | Define or update engagement goals |
| `set_constraints` | Set rules the engagement must follow |
| `prove_goal` | Mark a goal as proved with evidence |

### Findings and credentials

| Tool | What it does |
|------|-------------|
| `report_finding` | Report a vulnerability with severity, evidence, and affected assets |
| `list_findings` | List findings for the current engagement |
| `save_task_credential` | Store a discovered credential |
| `list_task_credentials` | List stored credentials |

### Coverage

| Tool | What it does |
|------|-------------|
| `record_coverage` | Record what was tested, the outcome, and evidence |

## Architecture

```
cc-artex/
├── cmd/cc-artex/      CLI entrypoint: serve, init, version
├── store/             PostgreSQL data layer (assets, graph, findings, credentials, coverage)
├── mcpserver/         MCP protocol server (stdio and HTTP transports)
├── tools/             Tool implementations registered with the MCP server
├── config/            Configuration loading and engagement state (.cc-artex.json)
└── traffic/           HTTP traffic capture and storage (planned)
```

The server speaks [MCP](https://modelcontextprotocol.io/) over stdio (for Claude Code) or HTTP (for other clients). All state lives in PostgreSQL. The engagement config (`.cc-artex.json`) tracks the current task and exploration IDs so the server knows which engagement to serve.

## cc-artex vs ARTEX

| | ARTEX | cc-artex |
|---|-------|----------|
| LLM | Built-in agent loop (needs API key) | None. Claude Code is the LLM. |
| Interface | Autonomous agent | MCP tool server |
| Cost | API usage (per token) | Your Claude subscription |
| Asset graph | Yes | Same model, same schema |
| Exploration tracking | Yes | Same model |
| Findings | Yes | Same model |
| Human oversight | Watch the agent run | You steer Claude directly |

## cc-artex vs manual pentesting

A pentest has a lot of bookkeeping: what you have tested, what you found, what is left, which credentials go where. That bookkeeping is what cc-artex handles. Claude does the reasoning and tool selection; you do the judgment calls; cc-artex keeps the graph, the findings, and the coverage ledger so nothing falls through the cracks.

## Configuration

cc-artex reads its PostgreSQL connection string from `--pg` or the `CC_ARTEX_PG` environment variable. The engagement state lives in `.cc-artex.json` in the data directory (`--data`, defaults to `.`).

```sh
# Serve over stdio (Claude Code)
cc-artex serve --stdio --pg "postgres://localhost/ccartex"

# Serve over HTTP (other clients)
cc-artex serve --listen localhost:9400 --pg "postgres://localhost/ccartex"

# Start a new engagement
cc-artex init --pg "postgres://localhost/ccartex" --goal "Bug bounty: corp.example.com"
```

## Suggested GitHub topics

`mcp`, `claude`, `claude-code`, `pentest`, `penetration-testing`, `security`, `offensive-security`, `bug-bounty`, `ai-security`, `mcp-server`

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for build instructions, how to add a tool, and how to submit changes.

## License

[MIT](LICENSE)
