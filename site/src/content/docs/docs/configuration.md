---
title: Configuration
description: Commands, flags, environment variables, credential encryption and proxy CA trust.
---

ARTEX reads its PostgreSQL connection string from `--pg` or the `ARTEX_PG` environment variable. The engagement state lives in `.artex.json` in the data directory (`--data`, defaults to `.`).

## Commands

| Command | What it does |
|---------|--------------|
| `artex-cc serve` | Start the MCP tool server (stdio or HTTP) |
| `artex-cc init` | Create the schema and a new engagement, generate the proxy CA |
| `artex-cc health` | Check PostgreSQL connectivity |
| `artex-cc tools` | List the MCP tool catalog, marking the proxy-only tools (`--json` for machine output) |
| `artex-cc identity` | Issue, rotate or inspect `X-Artex-Identity` tokens for HTTP clients (see [MCP clients](/docs/mcp-clients/#separate-claim-holders)) |
| `artex-cc completion` | Print a shell completion script (`bash`, `zsh`, `fish`) |
| `artex-cc version` | Print the version (`--json` for build details) |

```sh
# Serve over stdio (Claude Code)
artex-cc serve --stdio --pg "postgres://localhost/ccartex"

# Serve with the recording proxy
artex-cc serve --stdio --pg "postgres://localhost/ccartex" --proxy :8788

# Serve over HTTP (other clients), with the web UI
artex-cc serve --listen localhost:9400 --pg "postgres://localhost/ccartex" --web

# Start a new engagement
artex-cc init --pg "postgres://localhost/ccartex" --goal "Bug bounty: corp.example.com"
```

## `serve` flags

| Flag | Purpose | Default |
|------|---------|---------|
| `--pg DSN` | PostgreSQL connection string (or `ARTEX_PG`) | none, required |
| `--stdio` | Use the stdio transport instead of HTTP | off |
| `--listen ADDR` | HTTP listen address (or `ARTEX_LISTEN`). Without `--web`, a non-loopback address requires `ARTEX_MCP_TOKEN` | `localhost:9400` |
| `--web` | Enable the [web UI](/docs/web-ui/) and REST API | off |
| `--auth` | Require password login for the web UI on a loopback listener (or `ARTEX_AUTH`) | off |
| `--proxy ADDR` | Start the recording MITM proxy on `ADDR`, for example `:8788` | off |
| `--data DIR` | Directory for traffic, evidence and the engagement config | `.` |
| `--claude-bin PATH` | `claude` CLI used by the web UI chat (or `ARTEX_CLAUDE_BIN`) | `claude` |
| `--plugin-dir DIR` | Plugin directory (or `ARTEX_PLUGIN_DIR`) | `~/.artex/plugins` |
| `--no-plugins` | Disable plugin loading | off |
| `--require-identity` | Refuse intent claims from callers without a verified `X-Artex-Identity` (or `ARTEX_REQUIRE_IDENTITY`). HTTP only; `serve --stdio` refuses to start with it. See [Requiring an identity](/docs/mcp-clients/#requiring-an-identity) | off |
| `--timeout N` | Graceful shutdown timeout in seconds | `30` |
| `--debug` | Debug-level logging | off |

Without `--proxy`, the eleven `traffic_*` tools and `bind_finding_traffic` are not registered.

See [Plugins](/docs/plugins/) for how the plugin directory is scanned and what a plugin receives and returns.

## `init` flags

| Flag | Purpose | Default |
|------|---------|---------|
| `--pg DSN` | PostgreSQL connection string (or `ARTEX_PG`) | none, required |
| `--goal TEXT` | Engagement goal | |
| `--description TEXT` | Target description | |
| `--data DIR` | Where to write `.artex.json` and the proxy CA. Created 0711 if missing, like `serve` does | `.` |
| `--dry-run` | Show what would be created without creating it | off |
| `--format FMT` | Output format, `text` or `json` | `text` |

## Environment variables

Each variable is a fallback for the matching flag; a flag on the command line wins.

| Variable | Flag | Purpose | Default |
|----------|------|---------|---------|
| `ARTEX_PG` | `--pg` | PostgreSQL connection string | none, required |
| `ARTEX_LISTEN` | `--listen` | HTTP listen address for `serve` | `localhost:9400` |
| `ARTEX_MCP_TOKEN` | | Bearer token `/mcp` requires when serving HTTP without `--web`. Required for a non-loopback `--listen`; optional on loopback. Ignored with `--web`, which generates its own token. See [MCP clients](/docs/mcp-clients/#generic-stdio) | none |
| `ARTEX_AUTH` | `--auth` | Require password login for the web UI on a loopback listener (`1`/`true`). Non-loopback listeners always require it. | off |
| `ARTEX_CREDENTIAL_KEY` | | Hex-encoded 32-byte AES-256-GCM key for encrypting stored credentials | none (plaintext) |
| `ARTEX_INTENT_LEASE` | | How long a `start_intent` claim stays exclusive, in whole minutes (Go duration such as `2m`). Shorten it only for tests and evals | `30m` |
| `ARTEX_REQUIRE_IDENTITY` | `--require-identity` | Refuse intent claims from callers without a verified identity (`1`/`true`). Any value that is not a boolean is a startup error. HTTP only | off |
| `ARTEX_CLAUDE_BIN` | `--claude-bin` | Path to the `claude` CLI used by the web UI chat | `claude` |
| `ARTEX_PLUGIN_DIR` | `--plugin-dir` | Plugin directory | `~/.artex/plugins` |

The Docker image also reads these on startup:

| Variable | Purpose | Default |
|----------|---------|---------|
| `ARTEX_GOAL` | Goal for the engagement created on first run | `Pentest engagement` |
| `ARTEX_PROXY` | Recording proxy listen address | `:8788` |

## Data directory

The data directory holds `.artex.json`, the identity and claim label keys (`identity.key`, `owner-label.key`), captured traffic and the proxy CA under `traffic/`, saved evidence under `evidence/`, and with `--web` the web UI's state (`auth.json`, `mcp-config.json`, conversations, settings, and the [workspace](/docs/web-ui/) under `workspace/`).

ARTEX writes these owner-only: files 0600, directories 0700. When `init` or `serve` has to create the data directory it creates it 0711, so other users can reach the public CA certificate in `traffic/_ca` without listing anything else; an existing directory keeps its mode. On the first start after upgrading from a version that wrote them world-readable, and only in a directory that already holds a file only ARTEX writes (such as `.artex.json` or `owner-label.key`), ARTEX removes group and other access from the files and directories it owns there and leaves everything else alone. In a fresh or unrelated directory, such as a project folder with its own `workspace/`, it changes nothing.

Because the files are owner-only, run `init` and `serve` as the same user. A `serve` running as another user cannot read `.artex.json` and exits with `failed to load engagement config`.

## Credential encryption

Set `ARTEX_CREDENTIAL_KEY` to a hex-encoded 32-byte key to encrypt stored credential passwords with AES-256-GCM:

```sh
export ARTEX_CREDENTIAL_KEY=$(openssl rand -hex 32)
```

Without it, credentials are stored in plaintext and the server logs a warning at startup. Keep the key somewhere durable: credentials encrypted with a lost key cannot be recovered.

If the server exits with `invalid credential key`, the variable is set but is not valid hex or not 32 bytes. Unset it to skip encryption, or generate a new one with the command above.

## Recording proxy

When `--proxy` is set, ARTEX starts a recording MITM proxy on the given address. Configure your HTTP client, browser or scanner to use it, and the proxy captures every request and response for the [traffic tools](/docs/tools/traffic/): search, inspection, replay, HAR export and binding to findings.

## Proxy CA trust

`artex-cc init` generates a CA certificate at `traffic/_ca/mitmproxy-ca-cert.pem` (under the data directory) and prints OS-specific trust instructions. Until the CA is trusted, HTTPS through the proxy fails with certificate errors.

For a single tool, passing the CA explicitly is often simpler than trusting it system-wide:

```sh
curl --proxy http://localhost:8788 --cacert traffic/_ca/mitmproxy-ca-cert.pem https://app.example.com/
```

## Shell completion

```sh
eval "$(artex-cc completion zsh)"    # or bash, fish
```
