---
title: Plugins
description: Add your own MCP tools with exec plugins that receive the engagement context and write assets, facts and findings back to the graph.
---

A plugin is any executable that speaks a small JSON protocol over stdin and stdout. ARTEX runs it once at startup to learn which tools it provides, registers each one as an MCP tool, and runs it again every time Claude calls one of those tools. Plugins can be written in any language. They receive the active engagement, its scope and the recording proxy address, and they can return assets, facts and findings that ARTEX writes into the graph for them.

## Discovery

On `serve`, ARTEX scans the plugin directory:

| Source | Value |
|--------|-------|
| `--plugin-dir DIR` | Highest priority |
| `ARTEX_PLUGIN_DIR` | Used when the flag is not set |
| Default | `~/.artex/plugins` |

`--no-plugins` turns loading off. A missing directory is not an error.

Every regular file in the directory (not subdirectories) with an execute bit is called with a single argument, `--manifest`, and has 10 seconds to print its manifest. Files that fail, time out, print invalid JSON or return a manifest without a `name` are skipped with a warning in the server log. Discovery runs only at startup, so restart the server after adding or changing a plugin.

## Manifest

```json
{
  "name": "httpprobe",
  "description": "HTTP probe for in-scope URLs",
  "version": "1.0.0",
  "tools": [
    {
      "name": "probe",
      "description": "Request each URL once and record status, title and server as service assets.",
      "schema": {
        "type": "object",
        "properties": {
          "urls": { "type": "array", "items": { "type": "string" } }
        },
        "required": ["urls"]
      }
    }
  ]
}
```

Each tool is registered as `<plugin name>_<tool name>`, so the tool above appears to Claude as `httpprobe_probe`. `description` is what Claude reads when deciding whether to call the tool, so write it the way you would for any MCP tool. `schema` is a JSON Schema object for the tool's input. A tool without a schema gets `{"type": "object"}`; a tool whose schema is not a JSON object is skipped and the plugin's other tools still load.

## Invocation

When Claude calls a plugin tool, ARTEX starts the executable with no arguments and writes one invocation to its stdin:

```json
{
  "tool": "probe",
  "input": { "urls": ["https://app.example.com/", "https://other.test/"] },
  "config": {
    "engagement": { "id": 3, "exploration_id": 7, "name": "Pentest example.com" },
    "in_scope": [
      { "kind": "root_domain", "value": "example.com" },
      { "kind": "ip", "value": "10.0.0.5/32", "reason": "staging host" }
    ],
    "data_dir": "/home/user/pentest",
    "proxy": {
      "url": "http://127.0.0.1:8788",
      "ca_cert": "/home/user/pentest/traffic/_ca/mitmproxy-ca-cert.pem"
    }
  }
}
```

| Field | Meaning |
|-------|---------|
| `tool` | The tool name from the manifest, without the plugin prefix. One executable can serve several tools by switching on it. |
| `input` | The arguments Claude passed, unchanged. |
| `config.engagement` | The active engagement. `name` is the engagement goal. The config is built per call, so it follows `switch_engagement`. |
| `config.in_scope` | The engagement's scope rules followed by the scope rules of each company it covers through a `company` entry. Always an array, possibly empty. `kind` is one of `root_domain`, `subdomain`, `domain`, `ip`, `cidr`, `company`, `icp` or `keyword` (`domain` comes from company scope rows); IP rules are expressed as CIDRs. `reason` is omitted when empty. |
| `config.data_dir` | Absolute path of the `--data` directory, a good place for raw scanner output. |
| `config.proxy` | Present only when `serve` runs with `--proxy`. Send HTTP(S) through `url` and trust `ca_cert` (an absolute PEM path) so the traffic is recorded and shows up in the `traffic_*` tools. |

The process inherits the server's environment and working directory. Anything it writes to stderr is included in the error message if it exits non-zero.

## Result

The plugin writes a single JSON object to stdout and exits 0:

```json
{
  "content": [
    { "type": "text", "text": "https://app.example.com/ [200] [5120] [Sign in] [nginx/1.25.3]" }
  ],
  "assets": [
    {
      "type": "service",
      "url": "https://app.example.com/",
      "status_code": 200,
      "content_length": 5120,
      "page_title": "Sign in",
      "technologies": ["nginx/1.25.3"]
    }
  ],
  "facts": [
    {
      "summary": "1 of 2 probed URLs answered over HTTP",
      "detail": "https://app.example.com/",
      "confidence": "observed",
      "source": "tool_output"
    }
  ],
  "findings": [
    {
      "vulnclass": "information_disclosure",
      "severity": "low",
      "summary": "Server header discloses the exact nginx version",
      "evidence": "Server: nginx/1.25.3"
    }
  ]
}
```

| Field | Meaning |
|-------|---------|
| `content` | Text shown to Claude. Multiple items are joined with newlines. |
| `isError` | Set to `true` when the tool failed in a way Claude should see (bad input, target unreachable). Defaults to `false`. |
| `assets` | Optional. Each item has the shape of one element of `insert_assets`' `assets` array: `type` plus the fields that type requires, such as `domain` for `subdomain`, `ip` for `ip`, `url` for an HTTP `service`. |
| `facts` | Optional. Each item has the shape of one element of `record_fact`'s `facts` array: `summary` is required; `detail`, `evidence`, `confidence`, `source`, `intent_id` and `asset_ids` are optional. |
| `findings` | Optional. Each item is a full `report_finding` input: `vulnclass`, `severity` (`critical`, `high`, `medium` or `low`) and `summary` are required. |

See the [tool reference](/docs/tools/) for every field these three tools accept.

A non-zero exit, a timeout, more than 8 MiB on stdout or output that is not valid JSON fails the tool call, and the error Claude sees includes stderr or the raw output.

### How structured items are persisted

After a successful run (`isError` not set), ARTEX writes each asset, fact and finding through the same `insert_assets`, `record_fact` and `report_finding` tools Claude would call. They go to the engagement that was active when the call started, which is the same one the plugin received in `config.engagement` and whose scope it received in `config.in_scope`. If `switch_engagement` runs while the plugin is still working, its results still land in the engagement it was told about; the next call follows the switch. Scope checks and validation apply exactly as they would to Claude. Items go in one at a time, so one bad item never blocks the others, and a bad item never fails the call.

A summary line and one line per problem (up to 20, then a count of the rest) are appended to the plugin's own text:

```
https://app.example.com/ [200] [5120] [Sign in] [nginx/1.25.3]
inserted 1 asset, 1 fact, 0 findings; skipped 1 out of scope
skipped asset 1: out of scope (https://other.test/): target does not match any scope rule
failed finding 0: severity "urgent" is not valid (use critical/high/medium/low)
```

Assets outside the engagement scope are counted as skipped rather than failed. Note that this check guards the graph, not the network: by the time an out-of-scope asset is rejected, the plugin has already sent its traffic. A plugin that touches targets should check `config.in_scope` itself before sending anything, as the example below does.

When `isError` is `true`, ARTEX shows `content` and persists nothing.

## Limits

| Limit | Value | What happens when it is hit |
|-------|-------|-----------------------------|
| Run time | 5 minutes | The process is killed and the call fails. |
| Stdout | 8 MiB | The process is killed and the call fails with an error naming the limit. |
| Structured items | 500 per result (`assets`, `facts` and `findings` combined) | Nothing is persisted. `content` is still shown, followed by `persisted nothing: result has N structured items, max 500`. The result is rejected whole rather than truncated, so the graph never holds an arbitrary part of it. |
| Persistence time | 1 minute, separate from the run time | Items not yet written are dropped and the summary line ends with `stopped (context deadline exceeded) with N items not attempted`. Items already written stay. |
| Error text | 4 KiB | When a plugin fails, at most 4 KiB of its stderr (or of unparseable stdout) is quoted in the error, followed by `... (N more bytes)`. The rest of stderr is read and discarded, so a noisy plugin never blocks. |
| Problem lines | 20 | Further skipped or failed items are summarized as `... and N more problems`. The counts on the summary line stay exact. |
| `--manifest` | 10 seconds | The plugin is skipped with a warning. |

Plugins that wrap long scans should cap their own work (for example with a scanner's rate or time limit flags) and return partial results before the run time runs out. A plugin that can produce more than 500 items should return the most useful ones, or split the work across several calls.

## Example: an HTTP probe

The repository ships a reference plugin at `examples/plugins/httpprobe.py`: a single Python 3 file with no dependencies that does a small subset of what `httpx` does. It takes a list of URLs, requests each one once without following redirects, and reports the status code, body size, page title and `Server` header. A Go test runs it through discovery and persistence on every CI run, so it always matches the contract described here.

Install it by copying it into the plugin directory:

```sh
mkdir -p ~/.artex/plugins
cp examples/plugins/httpprobe.py ~/.artex/plugins/httpprobe
chmod +x ~/.artex/plugins/httpprobe
~/.artex/plugins/httpprobe --manifest    # prints the manifest
```

After restarting `artex-cc serve`, Claude has an `httpprobe_probe` tool. You can also drive it by hand, which is the quickest way to debug a plugin:

```sh
echo '{"tool":"probe","input":{"urls":["http://127.0.0.1:8000/","http://other.test/"]},
  "config":{"in_scope":[{"kind":"ip","value":"127.0.0.1/32"}]}}' | ~/.artex/plugins/httpprobe
```

The parts worth reading:

- **`--manifest`.** `main()` prints the `MANIFEST` dict and exits when called with `--manifest`. Otherwise it reads one invocation from stdin, dispatches on `tool`, and prints one result.
- **Scope before traffic.** `in_scope()` applies the same rules as the server: `root_domain`, `subdomain` and `domain` rules match the host or any subdomain of it, `ip` and `cidr` rules match addresses. Other kinds (`company`, `icp`, `keyword`) cannot match a host and are ignored. Unlike the server, which treats an empty scope as "everything allowed", the example treats an empty scope as "nothing allowed", so it never probes a target nobody has put in scope. Skipped URLs show up in the output and cost no traffic.
- **Schemes.** Only `http` and `https` URLs are probed. Anything else, such as `file://` or `ftp://`, is skipped with a reason in the output even when its host is in scope, because urllib would otherwise read local files or connect directly, bypassing the recording proxy.
- **The proxy.** `build_opener()` routes both `http` and `https` through `config.proxy.url` when it is present and loads `config.proxy.ca_cert` into the TLS context, so HTTPS requests are intercepted and recorded instead of failing certificate checks. Without a proxy it disables any proxy from the environment and connects directly.
- **Structured output.** Each URL that answers becomes a `service` asset with `url`, `status_code`, `content_length`, `page_title` and `technologies`. If any URL answered, one fact summarizes the run with `source: tool_output`, following `record_fact`'s guidance to consolidate one exploration into one fact. The plugin does not report findings; deciding that something is a vulnerability is left to Claude.
- **Errors.** Bad input (`urls` missing or not an array of strings, `timeout` not a number between 0 and 120) returns `isError: true` instead of crashing the plugin. A connection failure for one URL is a line in the output, not an error for the whole call.

To write your own, keep the same shape: answer `--manifest`, read one invocation, check scope before touching a target, route traffic through the proxy, and return text for Claude plus structured items for the graph.
