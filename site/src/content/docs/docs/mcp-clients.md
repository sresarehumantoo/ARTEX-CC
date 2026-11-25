---
title: MCP clients
description: "Configure Claude Code, Cursor, Cline, Windsurf, Zed or any other MCP client to launch ARTEX."
---

ARTEX is a standard MCP server, so any MCP client can use it. Every client below launches the same command; only the config file location and wrapper keys differ.

Run `artex-cc init --pg "<dsn>" --goal "<goal>"` once first (see [Getting started](/docs/getting-started/)). Replace `postgres://localhost/ccartex` with your DSN in each example. Drop `"--proxy", ":8788"` if you do not need traffic capture; without it the traffic tools are not registered.

Client config formats change between releases. If a path or key below does not match your version, check the client's MCP documentation; the command and args stay the same.

## Generic stdio

For any MCP client that can launch a stdio server:

- Command: `artex-cc`
- Args: `["serve", "--stdio", "--pg", "<dsn>"]`

`artex-cc` must be on the `PATH` of the client process. Editors launched from a desktop menu often do not inherit your shell `PATH`; use an absolute path for `command` if the server fails to start.

For clients that support streamable HTTP, run `artex-cc serve --listen localhost:9400 --pg "<dsn>"` and point the client at `http://localhost:9400/mcp`. With `--web` (which `docker compose up` uses), `/mcp` also requires an `Authorization: Bearer <token>` header; the token is regenerated on every start and written to `mcp-config.json` in the data directory.

Without `--web`, a loopback listener (`localhost`, `127.0.0.0/8` or `[::1]`) serves `/mcp` without authentication. Any other address, including `:9400`, `0.0.0.0`, `[::]`, a LAN IP or a hostname, exposes every tool to the network, so `serve` refuses to start unless `ARTEX_MCP_TOKEN` is set. Clients then send that token as `Authorization: Bearer <token>`:

```sh
export ARTEX_MCP_TOKEN=$(openssl rand -hex 32)
artex-cc serve --listen 0.0.0.0:9400 --pg "<dsn>"
```

```json
{
  "mcpServers": {
    "artex": {
      "type": "http",
      "url": "http://artex-host:9400/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

If `ARTEX_MCP_TOKEN` is set on a loopback listener, it is required there too. The token travels in plain HTTP, so put a TLS proxy or an SSH tunnel in front of a listener that crosses an untrusted network.

## Separate claim holders

Every HTTP client that shares the bearer token is the same principal. To keep several agents from holding, renewing or cancelling each other's intent claims, give each one a signed identity and send it as an `X-Artex-Identity` header. A client with an identity holds its claims under that name, keeps them across reconnects and server restarts, and needs no `owner` argument.

Issue one with the data directory the server uses. It runs offline and needs no database:

```sh
artex-cc identity issue --data ./pentest --name recon-1
```

The token is printed on stdout and nothing else; the name, key ID and expiry go to stderr. `--format header` prints `X-Artex-Identity: <token>`, and `--format claude-json` prints a ready-made config:

```sh
artex-cc identity issue --data ./pentest --name recon-1 --format claude-json \
  --url http://artex-host:9400/mcp --bearer-env ARTEX_MCP_TOKEN > recon-1.mcp.json
claude --mcp-config recon-1.mcp.json
```

```json
{
  "mcpServers": {
    "artex": {
      "type": "http",
      "url": "http://artex-host:9400/mcp",
      "headers": {
        "Authorization": "Bearer ${ARTEX_MCP_TOKEN}",
        "X-Artex-Identity": "ai1.<payload>.<mac>"
      }
    }
  }
}
```

Claude Code expands `${ARTEX_MCP_TOKEN}` from its environment. Leave out `--bearer-env` for a loopback listener without a token, or replace the value with the token from `mcp-config.json` when the server runs with `--web`. Other clients take the same two headers in their own HTTP server settings.

- Names are lowercase letters, digits and `.`, `_`, `/`, `-`, up to 64 characters. `chat/` and `eval/` are reserved for the web UI and the eval runner.
- Use one name per concurrently running agent. Two clients with the same identity are one holder.
- Tokens last 7 days by default; `--ttl` takes a duration such as `12h` or `14d`, up to `30d`.
- The token is a credential for claims only. It grants no access without the bearer token, and the bearer token alone still reaches every tool.
- The signing key is `identity.key` in the data directory. Every server process serving that directory accepts the same tokens; a server with a different `--data` rejects them.
- `artex-cc identity show --data ./pentest -` reads a token from stdin and prints its name, key ID, issue and expiry times, and whether it verifies, without printing the token. With no token it prints the current key ID.
- `artex-cc identity rotate --data ./pentest` replaces the key, which revokes every token issued from that directory. Running servers pick up the new key without a restart. There is no per-token revocation; keep TTLs short instead.

A server that verifies identities rejects a request whose `X-Artex-Identity` is malformed, expired or signed by another key with HTTP 401, so a stale token fails loudly. Requests without the header behave as before. A server older than the identity release ignores the header, and claims fall back to the MCP session or the `owner` argument.

## Claude Code

File: `~/.claude.json` (user-wide) or `.mcp.json` in the project root.

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex-cc",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"]
    }
  }
}
```

Claude Code reads the `artex://engagement` resource automatically, so the orient/plan/execute workflow guide is delivered with no extra steps.

## Cursor

File: `.cursor/mcp.json` in the project root, or `~/.cursor/mcp.json` for all projects.

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex-cc",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"]
    }
  }
}
```

Notes:

- Restart Cursor after editing the file, or toggle the server in Settings, MCP.
- Cursor asks for approval before running each tool unless auto-run is enabled.

## Cline

Cline is a VS Code extension. Open the MCP Servers panel, choose Configure MCP Servers, and Cline opens `cline_mcp_settings.json`. It lives in VS Code's extension global storage, for example `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` on Linux.

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex-cc",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"],
      "disabled": false
    }
  }
}
```

Notes:

- Cline picks up changes when the file is saved; no restart is needed.
- Cline can use local models through Ollama or LM Studio, but ARTEX's multi-step workflow needs a capable model.

## Windsurf

File: `~/.codeium/windsurf/mcp_config.json`. You can also open it from the Cascade panel via the MCP, Configure button.

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex-cc",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"]
    }
  }
}
```

Notes:

- Press Refresh in the Cascade MCP panel, or restart Windsurf, after editing.
- Windsurf caps the total number of enabled tools across all MCP servers. If ARTEX's tools are cut off, disable tools from other servers.

## Zed

Zed calls MCP servers "context servers" and configures them in `settings.json` (`~/.config/zed/settings.json`, or open it with the `zed: open settings` action) under `context_servers`, not `language_models`.

```json
{
  "context_servers": {
    "artex": {
      "source": "custom",
      "command": "artex-cc",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"],
      "env": {}
    }
  }
}
```

Notes:

- Zed applies the change on save; check the server's status indicator in the Agent panel settings.
- MCP tools are only available in the Agent panel, and the model you select there must support tool calling.
- Zed does not read MCP resources, so the `artex://engagement` guide is not delivered. Start the conversation with "call `graph_overview`" so the model orients itself from the tool output.

## Troubleshooting

- `failed to open store`: PostgreSQL is not reachable with the DSN you gave. Start it or run `docker compose up -d postgres`.
- Server never appears: run the exact command and args from a terminal to see the error, and confirm `artex-cc` is on the client's `PATH`.
- Use `ARTEX_PG` in the client's `env` block instead of `--pg` if you prefer to keep the DSN out of the args.
