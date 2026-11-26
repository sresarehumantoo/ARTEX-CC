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

`.artex.json` in the data directory exists but could not be read or parsed. It is written owner-only (0600), so the usual cause is running `serve` as a different user than `init`; otherwise check that the file is valid JSON. See [Data directory](/docs/configuration/#data-directory).

## Tools report `requires a task context`, or the board is empty

`serve` found no `.artex.json` in its data directory, so it started without an engagement. With no `--data`, the data directory is the server's working directory, which for an MCP client is wherever the client launched it. Run `artex-cc init` first, and pass `--data` pointing at the directory where you ran it.

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

Without `--web`, `/mcp` requires the bearer token only when `ARTEX_MCP_TOKEN` is set on the server; send that value. See [MCP clients](/docs/mcp-clients/#generic-stdio).

A rejected `X-Artex-Identity` header gets `400` with the reason in the body, not `401`. See [When a token is rejected](/docs/mcp-clients/#when-a-token-is-rejected).

## `serve` refuses to start: `is not a loopback address, so /mcp needs a bearer token`

`--listen` (or `ARTEX_LISTEN`) is an address other than `localhost`, `127.0.0.0/8` or `[::1]`, such as `:9400` or `0.0.0.0:9400`, and the server runs without `--web`. Set `ARTEX_MCP_TOKEN` (for example `export ARTEX_MCP_TOKEN=$(openssl rand -hex 32)`) and send it as `Authorization: Bearer <token>`, or listen on `localhost`.

## HTTP requests get `421` with `host not allowed`

A request that arrives on a loopback address must carry a `Host` of `localhost` or a loopback IP, so a DNS-rebinding web page cannot reach the server. A reverse proxy on the same host that forwards the original `Host` header, or a hostname that resolves to `127.0.0.1`, gets 421. Point the client at `localhost` or `127.0.0.1`, or have the proxy send `Host: localhost`. Connections on other local addresses are not affected.

## Claude repeats work or forgets what it did

Make sure it calls `graph_overview` at the start of each planning cycle. Clients that do not read MCP resources (Zed, for example) never receive the `artex://engagement` workflow guide, so start the conversation with "call `graph_overview`".

## Still stuck

Run `serve` with `--debug` for verbose logs, and open an issue on [GitHub](https://github.com/SREsAreHumanToo/ARTEX-CC/issues) with the output. For questions, use [Discussions](https://github.com/SREsAreHumanToo/ARTEX-CC/discussions).
