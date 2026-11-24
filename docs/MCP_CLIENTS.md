# MCP Client Configuration

cc-artex is a standard MCP server, so any MCP client can use it. Every client below launches the same command; only the config file location and wrapper keys differ.

Run `cc-artex init --pg "<dsn>" --goal "<goal>"` once first (see the [README](../README.md#quick-start)). Replace `postgres://localhost/ccartex` with your DSN in each example. Drop `"--proxy", ":8788"` if you do not need traffic capture; without it the traffic tools are not registered.

Client config formats change between releases. If a path or key below does not match your version, check the client's MCP documentation; the command and args stay the same.

## Generic stdio

For any MCP client that can launch a stdio server:

- Command: `cc-artex`
- Args: `["serve", "--stdio", "--pg", "<dsn>"]`

`cc-artex` must be on the `PATH` of the client process. Editors launched from a desktop menu often do not inherit your shell `PATH`; use an absolute path for `command` if the server fails to start.

For clients that support HTTP/SSE, run `cc-artex serve --listen localhost:9400 --pg "<dsn>"` (or `docker compose up`) and point the client at `http://localhost:9400/sse`.

## Claude Code

File: `~/.claude.json` (user-wide) or `.mcp.json` in the project root.

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

Claude Code reads the `artex://engagement` resource automatically, so the orient/plan/execute workflow guide is delivered with no extra steps.

## Cursor

File: `.cursor/mcp.json` in the project root, or `~/.cursor/mcp.json` for all projects.

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

Notes:

- Restart Cursor after editing the file, or toggle the server in Settings, MCP.
- Cursor asks for approval before running each tool unless auto-run is enabled.

## Cline

Cline is a VS Code extension. Open the MCP Servers panel, choose Configure MCP Servers, and Cline opens `cline_mcp_settings.json`. It lives in VS Code's extension global storage, for example `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` on Linux.

```json
{
  "mcpServers": {
    "cc-artex": {
      "command": "cc-artex",
      "args": ["serve", "--stdio", "--pg", "postgres://localhost/ccartex", "--proxy", ":8788"],
      "disabled": false
    }
  }
}
```

Notes:

- Cline picks up changes when the file is saved; no restart is needed.
- Cline can use local models through Ollama or LM Studio, but cc-artex's multi-step workflow needs a capable model. See [MULTI_CLIENT_RESEARCH.md](internal/MULTI_CLIENT_RESEARCH.md).

## Windsurf

File: `~/.codeium/windsurf/mcp_config.json`. You can also open it from the Cascade panel via the MCP, Configure button.

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

Notes:

- Press Refresh in the Cascade MCP panel, or restart Windsurf, after editing.
- Windsurf caps the total number of enabled tools across all MCP servers. If cc-artex's tools are cut off, disable tools from other servers.

## Zed

Zed calls MCP servers "context servers" and configures them in `settings.json` (`~/.config/zed/settings.json`, or open it with the `zed: open settings` action) under `context_servers`, not `language_models`.

```json
{
  "context_servers": {
    "cc-artex": {
      "source": "custom",
      "command": "cc-artex",
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
- Server never appears: run the exact command and args from a terminal to see the error, and confirm `cc-artex` is on the client's `PATH`.
- Use `CC_ARTEX_PG` in the client's `env` block instead of `--pg` if you prefer to keep the DSN out of the args.
