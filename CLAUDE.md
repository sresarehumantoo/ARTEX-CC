ARTEX is an MCP tool server for autonomous pentesting with Claude Code. 59 tools, PostgreSQL backend, recording proxy, evidence capture. You bring the brain, ARTEX brings the hands.

## Quick setup

```sh
# 1. Start PostgreSQL (pick one)
docker compose up -d postgres          # Docker
createdb ccartex                       # or use an existing PostgreSQL

# 2. Initialize an engagement
artex init --pg "postgres://localhost/ccartex" --goal "Pentest example.com"

# 3. Add MCP config (the init command prints this too)
```

Add to `~/.claude.json` (or project `.mcp.json`):

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex",
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

**Docker (all-in-one):** `docker compose up` starts PostgreSQL and ARTEX together. Use `http://localhost:9400/sse` as the MCP endpoint or the Docker stdio config from README.md.

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
| `--listen :9400` | Serve over HTTP instead of stdio |

## Credential encryption

Set `ARTEX_CREDENTIAL_KEY` (hex-encoded 32-byte AES-256-GCM key) to encrypt stored credentials. Without it, credentials are stored in plaintext and the server logs a warning.

## Proxy CA trust

The `init` command generates a CA certificate at `traffic/_ca/mitmproxy-ca-cert.pem` and prints OS-specific trust instructions. Without trusting the CA, HTTPS interception through the recording proxy will fail with TLS errors.

## Common issues

**PostgreSQL not running:** `artex serve` exits with "failed to open store". Start PostgreSQL or run `docker compose up -d postgres`.

**No engagement config:** "failed to load engagement config" means `.artex.json` is missing from the data directory. Run `artex init` first, or point `--data` at the directory where you ran init.

**Credential key invalid:** "invalid credential key" means `ARTEX_CREDENTIAL_KEY` is set but not valid hex or not 32 bytes. Unset it to skip encryption, or generate a valid key: `openssl rand -hex 32`.

**Proxy CA not trusted:** HTTPS traffic through the proxy fails with certificate errors. Trust the CA cert (see `artex init` output for the command).

**Traffic tools missing:** The nine `traffic_*` tools only appear when `--proxy` is set on `serve`.

## Shell completion

```sh
eval "$(artex completion zsh)"    # or bash/fish
```
