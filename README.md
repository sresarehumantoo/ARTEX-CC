<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/logo-dark.svg">
    <img src="docs/brand/logo-light.svg" alt="ARTEX-CC" width="420">
  </picture>
</h1>

<div align="center">

**Use your Claude subscription to run pentests. No API key needed.**

[![CI](https://github.com/SREsAreHumanToo/ARTEX-CC/actions/workflows/ci.yml/badge.svg)](https://github.com/SREsAreHumanToo/ARTEX-CC/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/SREsAreHumanToo/ARTEX-CC?logo=github)](https://github.com/SREsAreHumanToo/ARTEX-CC/releases)
[![codecov](https://codecov.io/gh/SREsAreHumanToo/ARTEX-CC/graph/badge.svg)](https://codecov.io/gh/SREsAreHumanToo/ARTEX-CC)
[![Go Report Card](https://goreportcard.com/badge/github.com/SREsAreHumanToo/ARTEX-CC)](https://goreportcard.com/report/github.com/SREsAreHumanToo/ARTEX-CC)
[![Go Reference](https://pkg.go.dev/badge/github.com/SREsAreHumanToo/ARTEX-CC.svg)](https://pkg.go.dev/github.com/SREsAreHumanToo/ARTEX-CC)
[![Go 1.26+](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/SREsAreHumanToo/ARTEX-CC?style=flat&logo=github)](https://github.com/SREsAreHumanToo/ARTEX-CC/stargazers)

</div>

ARTEX is an autonomous penetration testing platform that exposes [upstream ARTEX](https://github.com/BaaRai/ARTEX)'s pentest model as MCP tools Claude Code calls through your existing Claude subscription. Asset graph, exploration tracking, finding management, coverage recording, traffic capture, and report generation, exposed as 62 MCP tools. No separate LLM API key, no token budget, no agent loop of its own.

You bring the brain. ARTEX brings the hands.

<p align="center">
  <img src="docs/web-demo.gif" alt="ARTEX-CC web UI walkthrough: dashboard, exploration graph, findings, traffic, coverage, report" width="860">
</p>

> **Authorized use only.** This is an offensive security tool, for penetration
> testing, red teaming, CTFs and lab work. Use it only against systems you own
> or have explicit permission to test.

## How it works

<div align="center">
  <img src="docs/architecture.svg" alt="ARTEX architecture" width="800">
</div>

Claude Code decides what to test, what tools to call, and in what order. ARTEX executes the calls: inserting assets, tracking the exploration graph, recording findings and credentials, logging coverage, and capturing HTTP traffic through its built-in recording proxy. The exploration graph keeps Claude oriented across long engagements so it doesn't repeat work or lose context.

### What a session looks like

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

## Web UI

`artex-cc serve --web` adds a dashboard on the same port as the MCP endpoint. Everything Claude records through the tools shows up here live: the exploration graph, findings with their bound traffic, coverage, credentials, and a printable report.

| Exploration graph | Findings |
| :---: | :---: |
| ![Exploration graph](docs/screenshots/graph.png) | ![Findings with traffic evidence](docs/screenshots/finding-detail.png) |

| Traffic (recording proxy) | Coverage matrix |
| :---: | :---: |
| ![Traffic](docs/screenshots/traffic.png) | ![Coverage](docs/screenshots/coverage.png) |

| Assets | Engagement report |
| :---: | :---: |
| ![Assets](docs/screenshots/assets.png) | ![Report](docs/screenshots/report.png) |

<sub>Screenshots use a seeded demo engagement against reserved `*.example` names and documentation IP ranges.</sub>

### From the terminal

<p align="center">
  <img src="docs/demo.gif" alt="ARTEX-CC MCP tool calls in a terminal" width="720">
</p>

## Why this exists

The upstream ARTEX framework includes its own LLM agent loop. That agent loop needs an API key, a token budget, and careful management. If you already pay for Claude, that is a second brain doing the same job.

ARTEX removes the agent loop entirely and exposes the data layer as MCP tools. Claude Code becomes the agent. You get:

- **No API cost.** Your Claude subscription covers it. No token counting, no surprise bills.
- **Claude's full reasoning.** Not a constrained agent loop, but Claude's complete context window and tool-use capability driving the engagement.
- **Your judgment in the loop.** Claude proposes, you approve. The exploration graph means you can review its plan, steer it, and pick up where it left off.

## Install

**Homebrew** (macOS and Linux):

```sh
brew install SREsAreHumanToo/tap/artex
```

**Pre-built binaries:** download from the [Releases](https://github.com/SREsAreHumanToo/ARTEX-CC/releases) page. Each release includes checksums.

**From source:**

```sh
go install github.com/SREsAreHumanToo/ARTEX-CC/cmd/artex-cc@latest
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
artex init --pg "postgres://localhost/ccartex" --goal "Pentest example.com"
```

The init command creates the schema, sets up the engagement, generates a CA certificate for HTTPS interception, and prints trust instructions for your OS. Add the printed MCP config to your Claude Code config (`~/.claude.json` or project `.mcp.json`):

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"]
    }
  }
}
```

The `--proxy` flag starts a recording MITM proxy alongside the MCP server. Point your tools at the proxy address (e.g., `http://localhost:8788`) to capture and analyze traffic. Without `--proxy`, the traffic tools are not registered and ARTEX runs as a pure data server.

Then start Claude Code. The tools appear automatically. Ask Claude to "pentest example.com" and watch it work.

For Cursor, Cline, Windsurf, Zed, and other MCP clients, see [MCP Client Configuration](docs/MCP_CLIENTS.md).

## Docker quick start

**Prerequisites:** Docker and Docker Compose

```sh
docker compose up
```

This starts PostgreSQL and ARTEX together. On first run it automatically creates the database schema and a default engagement. The MCP server listens on `localhost:9400` and the recording proxy on `localhost:8788`.

To set a specific engagement goal:

```sh
ARTEX_GOAL="Pentest example.com" docker compose up
```

Add this to your Claude Code MCP config (`~/.claude.json` or project `.mcp.json`) to connect over HTTP:

```json
{
  "mcpServers": {
    "artex": {
      "type": "http",
      "url": "http://localhost:9400/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

The container generates a new bearer token every time it starts and writes a ready-made config containing it to `/app/data/mcp-config.json`. Read it with `docker compose exec artex cat /app/data/mcp-config.json` and update the header after each restart.

Or use stdio transport through Docker:

```json
{
  "mcpServers": {
    "artex": {
      "command": "docker",
      "args": [
        "compose", "exec", "-T", "artex",
        "/app/artex-cc", "serve", "--stdio",
        "--pg", "postgres://ccartex:ccartex@postgres:5432/ccartex?sslmode=disable",
        "--data", "/app/data", "--proxy", "127.0.0.1:0"
      ]
    }
  }
}
```

The container's main process already runs the recording proxy on `:8788`, so this second process takes a throwaway port (`127.0.0.1:0`) only to enable the traffic tools. They read the same captured traffic. Point your testing tools at `http://localhost:8788` to capture traffic through the recording proxy.

## Tool catalog

62 MCP tools across ten categories. Claude calls these automatically based on what the engagement needs.
For detailed parameter documentation, see [Tool Reference](docs/TOOLS.md).

<details open>
<summary><strong>Exploration graph</strong></summary>

| Tool | What it does |
|------|-------------|
| `graph_overview` | Situational summary: asset counts, goals, hints, frontier, findings, coverage |
| `add_intent` | Plan a next step (queued for execution) |
| `start_intent` | Move a queued intent to in-progress |
| `complete_intent` | Mark a running intent as done with a result summary |
| `cancel_intent` | Cancel an intent that is no longer needed |
| `record_fact` | Record a discovery or observation linked to an intent |
| `list_facts` | Page through exploration facts, newest first |
| `node_detail` | Inspect a specific graph node by ID |
| `add_hint` | Attach a strategic hint for the planner |
| `set_goals` | Define or update engagement goals |
| `set_constraints` | Add allow/deny operation constraints |
| `prove_goal` | Link evidence to a goal and mark it proved |

</details>

<details>
<summary><strong>Assets and scope</strong></summary>

| Tool | What it does |
|------|-------------|
| `insert_assets` | Add domains, IPs, services, endpoints, or apps in bulk |
| `list_assets` | Query assets by type, DSL filter, or ID |
| `list_companies` | List tracked companies and their asset scopes |
| `add_company_scope` | Add scope rules (domains, CIDRs) to a company |
| `add_task_scope` | Add in-scope markers for the current task |
| `list_untested_assets` | Find assets with no coverage records |
| `check_scope` | Check whether a target is within scope rules |
| `record_coverage` | Record what was tested, the outcome, and evidence |

</details>

<details>
<summary><strong>Findings</strong></summary>

| Tool | What it does |
|------|-------------|
| `report_finding` | Report a vulnerability with severity, evidence, and affected assets |
| `list_findings` | List findings for the current engagement |
| `bind_finding_traffic` | Link a finding to captured traffic exchanges (requires `--proxy`) |
| `merge_findings` | Merge duplicate findings into one |
| `dismiss_finding` | Mark a finding as dismissed (false positive) |

</details>

<details>
<summary><strong>Credentials</strong></summary>

| Tool | What it does |
|------|-------------|
| `save_task_credential` | Store a discovered credential for the current task |
| `list_task_credentials` | List stored credentials |

</details>

<details>
<summary><strong>Evidence</strong></summary>

| Tool | What it does |
|------|-------------|
| `save_evidence` | Store a proof-of-concept artifact (file, screenshot, or text) |
| `list_evidence` | List evidence items, optionally filtered by finding |
| `get_evidence` | Retrieve stored evidence content by ID |
| `capture_command_output` | Run a command and save its output as evidence |

</details>

<details>
<summary><strong>Enrichment and ingestion</strong></summary>

| Tool | What it does |
|------|-------------|
| `enrich_domain` | DNS enrichment: A/AAAA/CNAME/MX/NS/TXT records and reverse DNS |
| `enrich_ip` | Reverse DNS on an IP address |
| `enrich_service` | Probe an HTTP(S) service for headers, redirects, and fingerprints |
| `ingest_scanner_output` | Parse and import output from nmap, nuclei, httpx, or ffuf |
| `ingest_masscan` | Parse masscan XML or JSON and import hosts and open ports |

</details>

<details>
<summary><strong>Engagement management</strong></summary>

| Tool | What it does |
|------|-------------|
| `create_engagement` | Create a new pentest engagement (task + exploration graph) |
| `list_engagements` | List all engagements with summary stats |
| `switch_engagement` | Change the active engagement without restarting |
| `update_engagement` | Update the current engagement's goal or description |
| `delete_engagement` | Permanently delete an engagement and all its data |
| `engagement_status` | Get goal, status, finding count, and coverage summary |
| `engagement_activity` | Recent activity timeline: findings, assets, graph nodes |
| `goal_met` | Mark the entire engagement as complete |
| `list_coverage_gaps` | Identify untested assets, unproven goals, and missing enrichment |
| `list_sessions` | List the MCP client sessions connected to this server |

</details>

<details>
<summary><strong>Activity</strong></summary>

| Tool | What it does |
|------|-------------|
| `list_activity` | List recent activity log entries for the engagement |
| `log_activity` | Record a manual activity entry |

</details>

<details>
<summary><strong>Reporting and export</strong></summary>

| Tool | What it does |
|------|-------------|
| `generate_report` | Generate a Markdown penetration test report from the current engagement |
| `export_engagement` | Export an engagement to a portable JSON archive |
| `import_engagement` | Import an engagement from a JSON archive |

</details>

<details>
<summary><strong>Traffic capture</strong> (requires <code>--proxy</code> on <code>serve</code>)</summary>

Point your testing tools at the proxy to capture requests.

| Tool | What it does |
|------|-------------|
| `traffic_search` | Search captured exchanges by host, URL, or body content |
| `traffic_get` | Read the full request and response of a captured exchange |
| `traffic_blob` | Page through oversized bodies by blob hash |
| `traffic_messages` | List WebSocket or SSE messages of a captured stream |
| `traffic_replay` | Replay a captured request with optional modifications |
| `traffic_hosts` | List all recorded target hosts with exchange counts |
| `traffic_har_export` | Export captured traffic as a HAR 1.2 JSON document |
| `traffic_stats` | Aggregate statistics: hosts, status codes, methods, response sizes |
| `traffic_timeline` | Traffic activity over time, bucketed by minute, hour, or day |
| `traffic_patterns` | Most-requested paths, error-heavy hosts, and newly seen endpoints |
| `traffic_clear` | Delete captured traffic for matching hosts |

</details>

## Architecture

```
ARTEX/
├── cmd/artex-cc/      CLI entrypoint: serve, init, version
├── store/             PostgreSQL data layer (assets, graph, findings, credentials, coverage)
├── mcpserver/         MCP protocol server (stdio and HTTP transports)
├── tools/             Tool implementations registered with the MCP server
├── config/            Configuration loading and engagement state (.artex.json)
├── traffic/           Recording MITM proxy, SOCKS5, replay, HAR export, rewrite engine
│   ├── rewrite/       Rule-based request/response rewriting
│   └── hookvm/        Sandboxed expression evaluator for rewrite hooks
├── evidence/          Evidence capture and storage
├── report/            Markdown report generation with quality analysis
└── egress/            Rate-limiting governor for outbound requests
```

The server speaks [MCP](https://modelcontextprotocol.io/) over stdio (for Claude Code) or HTTP (for other clients). All state lives in PostgreSQL. The engagement config (`.artex.json`) tracks the current task and exploration IDs so the server knows which engagement to serve.

## Why ARTEX?

- **Your Claude subscription IS the AI engine.** No API key management, no token budgets, no surprise bills. If you have Claude, you have everything you need.
- **The exploration graph means Claude never loses context.** Every recon result, every tested endpoint, every credential, every planned next step is tracked in a persistent graph. Resume hours later and `graph_overview` brings Claude back up to speed.
- **62 tools covering the full pentest lifecycle.** Asset discovery, scope management, exploration planning, traffic capture, evidence capture, credential storage, finding management, coverage tracking, and report generation.
- **Works with any MCP client.** Claude Code, Cline, Cursor, or anything else that speaks the [Model Context Protocol](https://modelcontextprotocol.io/).
- **One command to get started.** `docker compose up` or install the binary and run `artex init`.

## ARTEX vs alternatives

| | ARTEX | PentestGPT | HexStrike AI | Pentest-AI |
|---|----------|-----------|-------------|-----------|
| **State management** | Exploration graph (persistent, resumable) | Stateless / limited context | None | None |
| **MCP native** | Yes (62 tools) | No (agent framework) | Yes (150+ tools) | Yes (205 tools) |
| **Evidence chains** | Yes (findings linked to assets, coverage, traffic) | No | No | No |
| **Traffic capture** | Built-in recording proxy with replay | No | No | No |
| **Multi-client support** | Any MCP client (Claude Code, Cline, Cursor) | Own interface | Claude Code | Claude Code |
| **License** | AGPL-3.0 | Research | Unknown | Unknown |

ARTEX's differentiator is not tool count. It is the exploration graph and evidence chain model: every finding traces back to the asset, the coverage record, and the captured traffic that produced it.

## ARTEX vs upstream ARTEX

| | Upstream ARTEX | This project |
|---|-------|----------|
| LLM | Built-in agent loop (needs API key) | None. Claude Code is the LLM. |
| Interface | Autonomous agent | MCP tool server |
| Cost | API usage (per token) | Your Claude subscription |
| Asset graph | Yes | Same model, same schema |
| Exploration tracking | Yes | Same model |
| Findings | Yes | Same model |
| Human oversight | Watch the agent run | You steer Claude directly |
| License | AGPL-3.0 | AGPL-3.0, with ARTEX-derived code credited in NOTICE |

## ARTEX vs manual pentesting

A pentest has a lot of bookkeeping: what you have tested, what you found, what is left, which credentials go where. That bookkeeping is what ARTEX handles. Claude does the reasoning and tool selection; you do the judgment calls; ARTEX keeps the graph, the findings, and the coverage ledger so nothing falls through the cracks.

## Configuration

ARTEX reads its PostgreSQL connection string from `--pg` or the `ARTEX_PG` environment variable. The engagement state lives in `.artex.json` in the data directory (`--data`, defaults to `.`). Config files can be JSON or TOML (the file extension determines the parser).

```sh
# Serve over stdio (Claude Code)
artex serve --stdio --pg "postgres://localhost/ccartex"

# Serve with the recording proxy
artex serve --stdio --pg "postgres://localhost/ccartex" --proxy :8788

# Serve over HTTP (other clients)
artex serve --listen localhost:9400 --pg "postgres://localhost/ccartex"

# Start a new engagement
artex init --pg "postgres://localhost/ccartex" --goal "Bug bounty: corp.example.com"
```

When `--proxy` is set, ARTEX starts a recording MITM proxy on the given address. Configure your HTTP client or browser to use it, and the proxy captures every request and response for analysis through the traffic tools. The `init` command generates a CA certificate (at `traffic/_ca/mitmproxy-ca-cert.pem`) and prints OS-specific trust instructions so the proxy can intercept HTTPS.

### Environment variables

Each variable is a fallback for the matching flag; a flag on the command line wins.

| Variable | Flag | Purpose | Default |
|----------|------|---------|---------|
| `ARTEX_PG` | `--pg` | PostgreSQL connection string | _(none, required)_ |
| `ARTEX_LISTEN` | `--listen` | HTTP listen address for `serve` | `localhost:9400` |
| `ARTEX_AUTH` | `--auth` | Require password login for the web UI on a loopback listener (`1`/`true`). Non-loopback listeners always require it. | off |
| `ARTEX_CREDENTIAL_KEY` | | Hex-encoded 32-byte AES-256-GCM key for encrypting stored credentials (`openssl rand -hex 32`) | _(none; credentials stored in plaintext)_ |
| `ARTEX_CLAUDE_BIN` | `--claude-bin` | Path to the `claude` CLI used by the web UI chat | `claude` |
| `ARTEX_PLUGIN_DIR` | `--plugin-dir` | Plugin directory | `~/.artex/plugins` |
| `ARTEX_NOISE_PROFILE` | | Egress pacing profile: `off`, `quiet`, `balanced` or `aggressive` | `off` |
| `ARTEX_TARGET_USER_AGENT` | | User-Agent for outbound requests when the noise profile is not `off` | _(original header kept)_ |

The Docker image also reads these on startup:

| Variable | Purpose | Default |
|----------|---------|---------|
| `ARTEX_GOAL` | Goal for the engagement created on first run | `Pentest engagement` |
| `ARTEX_PROXY` | Recording proxy listen address | `:8788` |

## Examples

See [docs/EXAMPLE_WALKTHROUGH.md](docs/EXAMPLE_WALKTHROUGH.md) for a complete
engagement walkthrough showing every phase of a pentest session: reconnaissance,
service enumeration, vulnerability testing, and report generation.

## Get help

- [GitHub Issues](https://github.com/SREsAreHumanToo/ARTEX-CC/issues) for bugs and feature requests
- [GitHub Discussions](https://github.com/SREsAreHumanToo/ARTEX-CC/discussions) for questions and ideas
- [CONTRIBUTING.md](CONTRIBUTING.md) for build instructions and how to add a tool

## License

ARTEX-CC is licensed under the [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0-only).

Parts of it, including the traffic proxy, egress governor, report generation and database schema, are derived from [ARTEX](https://github.com/BaaRai/ARTEX) by Autumn-27, BaaRai and the ARTEX contributors, which is also AGPL-3.0. [NOTICE](NOTICE) lists the derived files and third-party notices.

If you run a modified version as a network service, AGPL section 13 requires you to offer its users the corresponding source. The web UI links to the source for this reason.

Releases v0.1.0 through v0.3.0 were published with an MIT license file in error. They contain the same ARTEX-derived code and are distributed under AGPL-3.0 as well.
