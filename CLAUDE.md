ARTEX is an MCP tool server for autonomous pentesting with Claude Code. 65 tools, PostgreSQL backend, recording proxy, evidence capture. You bring the brain, ARTEX brings the hands.

## Quick setup

```sh
# 1. Start PostgreSQL (pick one)
docker compose up -d postgres          # Docker
createdb ccartex                       # or use an existing PostgreSQL

# 2. Initialize an engagement
artex-cc init --pg "postgres://localhost/ccartex" --goal "Pentest example.com"

# 3. Add MCP config (the init command prints this too)
```

Add to `~/.claude.json` (or project `.mcp.json`):

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex-cc",
      "args": [
        "serve", "--stdio",
        "--pg", "postgres://localhost/ccartex",
        "--proxy", ":8788"
      ]
    }
  }
}
```

Without `--proxy`, traffic capture tools are not registered.

**Docker (all-in-one):** `docker compose up` starts PostgreSQL and ARTEX together. Use `http://localhost:9400/mcp` (type `http`) as the MCP endpoint with the bearer token from `/app/data/mcp-config.json` in the container (regenerated on every start), or the Docker stdio config from the [getting started](https://artex-cc.com/docs/getting-started/#docker) page.

## Start a pentest

After setup, open Claude Code and ask:

> Pentest example.com, starting with reconnaissance.

Claude calls `graph_overview` to orient, then drives the engagement: inserting assets, planning intents, testing surfaces, recording findings, and tracking coverage. If you stop and resume later, `graph_overview` brings it back up to speed.

## Hands-free mode

```
/loop 5m Continue the pentest: call graph_overview, assess progress, dispatch subagents for open intents, prove goals when evidence exists.
```

Claude runs autonomously on a 5-minute loop. You can steer it with `add_hint` or walk away and review findings later.

## Key flags

| Flag | Purpose |
|------|---------|
| `--proxy :8788` | Start the recording MITM proxy for traffic capture |
| `--data ./pentest` | Set the directory for traffic, evidence, and config |
| `--debug` | Enable debug-level logging (verbose) |
| `--pg DSN` | PostgreSQL connection string (or set `ARTEX_PG` env var) |
| `--stdio` | Use the stdio transport instead of HTTP |
| `--listen localhost:9400` | HTTP listen address, the default transport (or `ARTEX_LISTEN`; a non-loopback address needs `ARTEX_MCP_TOKEN` unless `--web` is set) |
| `--web` | Serve the web UI and REST API alongside `/mcp`, which then uses a generated bearer token written to `mcp-config.json` in the data directory |
| `--auth` | Require password login for the web UI (or `ARTEX_AUTH`); always on when `--web` listens on a non-loopback address |
| `--require-identity` | Refuse intent claims from callers without a verified `X-Artex-Identity` header (or `ARTEX_REQUIRE_IDENTITY`; HTTP only, rejected with `--stdio`) |
| `--claude-bin PATH` | Path to the `claude` CLI the web UI runner launches (or `ARTEX_CLAUDE_BIN`, default `claude`) |
| `--plugin-dir DIR` | Plugin directory (or `ARTEX_PLUGIN_DIR`, default `~/.artex/plugins`) |
| `--no-plugins` | Disable plugin loading |
| `--timeout 30` | Graceful shutdown timeout in seconds |
| `ARTEX_INTENT_LEASE` | Env var: how long a `start_intent` claim stays exclusive before another session may take it over, in whole minutes (for example `45m`; default `30m`) |

## Credential encryption

Set `ARTEX_CREDENTIAL_KEY` (hex-encoded 32-byte AES-256-GCM key) to encrypt stored credentials. Without it, credentials are stored in plaintext and the server logs a warning.

## Proxy CA trust

The `init` command generates a CA certificate at `traffic/_ca/mitmproxy-ca-cert.pem` and prints OS-specific trust instructions. Without trusting the CA, HTTPS interception through the recording proxy will fail with TLS errors.

## Common issues

**PostgreSQL not running:** `artex-cc serve` exits with "failed to open store". Start PostgreSQL or run `docker compose up -d postgres`.

**No engagement config:** "failed to load engagement config" means `.artex.json` is missing from the data directory. Run `artex-cc init` first, or point `--data` at the directory where you ran init.

**Credential key invalid:** "invalid credential key" means `ARTEX_CREDENTIAL_KEY` is set but not valid hex or not 32 bytes. Unset it to skip encryption, or generate a valid key: `openssl rand -hex 32`.

**Proxy CA not trusted:** HTTPS traffic through the proxy fails with certificate errors. Trust the CA cert (see `artex-cc init` output for the command).

**Traffic tools missing:** The eleven `traffic_*` tools and `bind_finding_traffic` only appear when `--proxy` is set on `serve`.

## Shell completion

```sh
eval "$(artex-cc completion zsh)"    # or bash/fish
```
