---
title: Troubleshooting
description: Fixes for the errors people hit most often when setting up ARTEX.
---

## `failed to open store`

`artex-cc serve` cannot reach PostgreSQL with the DSN you gave. Start PostgreSQL, or run `docker compose up -d postgres`, and check the DSN with:

```sh
artex-cc health --pg "postgres://localhost/ccartex"
```

## `failed to load engagement config`

`.artex.json` is missing from the data directory. Run `artex-cc init` first, or point `--data` at the directory where you ran it.

## `invalid credential key`

`ARTEX_CREDENTIAL_KEY` is set but is not valid hex or not 32 bytes. Unset it to skip encryption, or generate a valid key with `openssl rand -hex 32`. See [Credential encryption](/docs/configuration/#credential-encryption).

## HTTPS through the proxy fails with certificate errors

The proxy CA is not trusted. Trust `traffic/_ca/mitmproxy-ca-cert.pem` using the instructions `artex-cc init` printed, or pass it to the tool directly (for example `curl --cacert`). See [Proxy CA trust](/docs/configuration/#proxy-ca-trust).

## Traffic tools are missing

The eleven `traffic_*` tools and `bind_finding_traffic` are only registered when `serve` runs with `--proxy`. Add `"--proxy", ":8788"` to the MCP server args.

If another process already holds the proxy port (for example the Docker container's main process), give the second process a throwaway port such as `127.0.0.1:0`. It still registers the traffic tools and reads the same captured traffic.

## The server never appears in my MCP client

Run the exact command and args from the client config in a terminal to see the error. Editors launched from a desktop menu often do not inherit your shell `PATH`, so use an absolute path for `command`. See [MCP clients](/docs/mcp-clients/#troubleshooting).

## HTTP clients get `401` from `/mcp`

With `--web` (which the Docker image uses), `/mcp` requires `Authorization: Bearer <token>`. The token changes on every start; read the current one from `mcp-config.json` in the data directory:

```sh
docker compose exec artex cat /app/data/mcp-config.json
```

A rejected `X-Artex-Identity` header gets `400` with the reason in the body, not `401`. See [When a token is rejected](/docs/mcp-clients/#when-a-token-is-rejected).

## Claude repeats work or forgets what it did

Make sure it calls `graph_overview` at the start of each planning cycle. Clients that do not read MCP resources (Zed, for example) never receive the `artex://engagement` workflow guide, so start the conversation with "call `graph_overview`".

## Still stuck

Run `serve` with `--debug` for verbose logs, and open an issue on [GitHub](https://github.com/SREsAreHumanToo/ARTEX-CC/issues) with the output. For questions, use [Discussions](https://github.com/SREsAreHumanToo/ARTEX-CC/discussions).
