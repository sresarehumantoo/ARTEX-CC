<div align="center">
  <img src="docs/logo.svg" alt="cc-artex" width="200">
</div>

<div align="center">

# cc-artex

**Use your Claude subscription to run pentests. No API key needed.**

[![CI](https://github.com/SREsAreHumanToo/cc-artex/actions/workflows/ci.yml/badge.svg)](https://github.com/SREsAreHumanToo/cc-artex/actions/workflows/ci.yml)
[![Go 1.26+](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

</div>

cc-artex is an MCP tool server that turns [ARTEX](https://github.com/Autumn-27/artex)'s autonomous pentest model into tools Claude Code calls through your existing Claude subscription. ARTEX's asset graph, exploration tracking, finding management, coverage recording, traffic capture, and report generation, exposed as 40+ MCP tools. No separate LLM API key, no token budget, no agent loop of its own.

You bring the brain. cc-artex brings the hands.

> **Authorized use only.** This is an offensive security tool, for penetration
> testing, red teaming, CTFs and lab work. Use it only against systems you own
> or have explicit permission to test.

## How it works

<div align="center">
  <img src="docs/architecture.svg" alt="cc-artex architecture" width="800">
</div>

Claude Code decides what to test, what tools to call, and in what order. cc-artex executes the calls: inserting assets, tracking the exploration graph, recording findings and credentials, logging coverage, and capturing HTTP traffic through its built-in recording proxy. The exploration graph keeps Claude oriented across long engagements so it doesn't repeat work or lose context.

## Why this exists

ARTEX is a capable pentest framework with its own LLM agent loop. That agent loop needs an API key, a token budget, and careful management. If you already pay for Claude, that is a second brain doing the same job.

cc-artex removes the agent loop entirely and exposes the data layer as MCP tools. Claude Code becomes the agent. You get:

- **No API cost.** Your Claude subscription covers it. No token counting, no surprise bills.
- **Claude's full reasoning.** Not a constrained agent loop, but Claude's complete context window and tool-use capability driving the engagement.
- **Your judgment in the loop.** Claude proposes, you approve. The exploration graph means you can review its plan, steer it, and pick up where it left off.

## Install

**Homebrew** (macOS and Linux):

```sh
brew install SREsAreHumanToo/tap/cc-artex
```

**Pre-built binaries:** download from the [Releases](https://github.com/SREsAreHumanToo/cc-artex/releases) page. Each release includes checksums.

**From source:**

```sh
go install github.com/SREsAreHumanToo/cc-artex/cmd/cc-artex@latest
```

**Docker:**

```sh
docker compose up
```

## Quick start

**Prerequisites:** PostgreSQL, Go 1.26+

```sh
# Create the database and initialize an engagement
createdb ccartex
cc-artex init --pg "postgres://localhost/ccartex" --goal "Pentest example.com"
```

The init command creates the schema, sets up the engagement, generates a CA certificate for HTTPS interception, and prints trust instructions for your OS. Add the printed MCP config to your Claude Code config (`~/.claude.json` or project `.mcp.json`):

```json
{
  "mcpServers": {
    "cc-artex": {
      "command": "cc-artex",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"]
    }
  }
}
```

The `--proxy` flag starts a recording MITM proxy alongside the MCP server. Point your tools at the proxy address (e.g., `http://localhost:8788`) to capture and analyze traffic. Without `--proxy`, the traffic tools are not registered and cc-artex runs as a pure data server.

Then start Claude Code. The tools appear automatically. Ask Claude to "pentest example.com" and watch it work.

## Docker quick start

**Prerequisites:** Docker and Docker Compose

```sh
docker compose up
```

This starts PostgreSQL and cc-artex together. On first run it automatically creates the database schema and a default engagement. The MCP server listens on `localhost:9400` and the recording proxy on `localhost:8788`.

To set a specific engagement goal:

```sh
CC_ARTEX_GOAL="Pentest example.com" docker compose up
```

Add this to your Claude Code MCP config (`~/.claude.json` or project `.mcp.json`) to connect over HTTP:

```json
{
  "mcpServers": {
    "cc-artex": {
      "type": "sse",
      "url": "http://localhost:9400/sse"
    }
  }
}
```

Or use stdio transport through Docker:

```json
{
  "mcpServers": {
    "cc-artex": {
      "command": "docker",
      "args": [
        "compose", "exec", "-T", "cc-artex",
        "/app/cc-artex", "serve", "--stdio",
        "--pg", "postgres://ccartex:ccartex@postgres:5432/ccartex?sslmode=disable",
        "--data", "/app/data", "--proxy", ":8788"
      ]
    }
  }
}
```

Point your testing tools at `http://localhost:8788` to capture traffic through the recording proxy.

## What a session looks like

```
$ claude
> Pentest example.com, starting with reconnaissance.

Claude calls: graph_overview → set_goals → insert_assets (domain: example.com)
         → add_intent (enumerate subdomains) → record_fact (found: api.example.com)
         → insert_assets (subdomain: api.example.com, service: https/443)
         → add_intent (test api.example.com for auth bypass)
         → traffic_hosts → traffic_search (host: api.example.com)
         → traffic_get (inspect the auth request)
         → traffic_replay (replay with modified auth header)
         → record_coverage (api /admin, idor, no_issue_found)
         → report_finding (severity: high, broken access control on /api/v2/users)
         → prove_goal (evidence: finding #1) → generate_report
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

### Engagement management

| Tool | What it does |
|------|-------------|
| `create_engagement` | Create a new pentest engagement (task + exploration graph) |
| `list_engagements` | List all engagements with id, goal, and status |
| `engagement_status` | Get finding count, coverage summary, and frontier for an engagement |

### Report generation

| Tool | What it does |
|------|-------------|
| `generate_report` | Generate a Markdown penetration test report from the current engagement |

### Traffic capture

Requires `--proxy` on `serve`. Point your testing tools at the proxy to capture requests.

| Tool | What it does |
|------|-------------|
| `traffic_search` | Search captured exchanges by host, URL, or body content |
| `traffic_get` | Read the full request and response of a captured exchange |
| `traffic_blob` | Page through oversized request or response bodies by blob hash |
| `traffic_messages` | List WebSocket or SSE messages of a captured stream |
| `traffic_replay` | Replay a captured request with optional modifications |
| `traffic_hosts` | List all recorded target hosts with exchange counts |
| `traffic_har_export` | Export captured traffic as a HAR 1.2 JSON document |

## Architecture

```
cc-artex/
├── cmd/cc-artex/      CLI entrypoint: serve, init, version
├── store/             PostgreSQL data layer (assets, graph, findings, credentials, coverage)
├── mcpserver/         MCP protocol server (stdio and HTTP transports)
├── tools/             Tool implementations registered with the MCP server
├── config/            Configuration loading and engagement state (.cc-artex.json)
├── traffic/           Recording MITM proxy, SOCKS5, replay, HAR export, rewrite engine
│   ├── rewrite/       Rule-based request/response rewriting
│   └── hookvm/        Sandboxed expression evaluator for rewrite hooks
├── report/            Markdown report generation with quality analysis
└── egress/            Rate-limiting governor for outbound requests
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

# Serve with the recording proxy
cc-artex serve --stdio --pg "postgres://localhost/ccartex" --proxy :8788

# Serve over HTTP (other clients)
cc-artex serve --listen localhost:9400 --pg "postgres://localhost/ccartex"

# Start a new engagement
cc-artex init --pg "postgres://localhost/ccartex" --goal "Bug bounty: corp.example.com"
```

When `--proxy` is set, cc-artex starts a recording MITM proxy on the given address. Configure your HTTP client or browser to use it, and the proxy captures every request and response for analysis through the traffic tools. The `init` command generates a CA certificate (at `traffic/_ca/mitmproxy-ca-cert.pem`) and prints OS-specific trust instructions so the proxy can intercept HTTPS.

## Examples

See [docs/EXAMPLE_WALKTHROUGH.md](docs/EXAMPLE_WALKTHROUGH.md) for a complete
engagement walkthrough showing every phase of a pentest session: reconnaissance,
service enumeration, vulnerability testing, and report generation.

## Suggested GitHub topics

`mcp`, `claude`, `claude-code`, `pentest`, `penetration-testing`, `security`, `offensive-security`, `bug-bounty`, `ai-security`, `mcp-server`

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for build instructions, how to add a tool, and how to submit changes.

## License

[MIT](LICENSE)
