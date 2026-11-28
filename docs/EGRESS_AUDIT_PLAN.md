# Egress audit and prompt-injection visibility plan

Status: proposal for maintainer review. No code yet. Citations are against `main` at `4fe8063b`. Vendor and standards facts were checked against their documentation on 2026-11-28. Tracking issues will be linked here once they are filed.

## Summary

Give the operator a durable, attributed record of what agents did on the network and through ARTEX, and surface the patterns that suggest an agent is following injected instructions. ARTEX writes that record as an append-only, versioned audit stream (`artex.audit/v1`) in three streams: `target` (traffic to anything that is not an Anthropic host), `anthropic` (Claude Code's own API traffic) and `agent` (MCP tool calls and Claude Code hook events). Detections run inside ARTEX, where the engagement scope and the stored credentials live, and produce alert records in the same stream. Every record is a JSONL line on local disk first, and can also leave the box as OTLP logs or through a generic HTTP sink. Redaction is a rule file with default mask sets, and ARTEX's own logs and metrics share the same OpenTelemetry resource so all signals can be joined downstream.

ARTEX stays vendor agnostic: it writes standard formats, and the operator's shipper or collector (vendor agent, OpenTelemetry Collector, Vector, Fluent Bit) does the rest. Vendor dashboards, parsers and rules live in separate example repos, not here.

This is visibility, not containment. Claude still runs the tools, ARTEX keeps the record, and a person still has to read the alerts and check the agents' work.

## Current state (verified)

- The recording proxy (`serve --proxy`) captures full plaintext HTTP(S) for clients that choose to use it. The `exchanges` table has no task, identity, session or intent column (`traffic/traffic.go:69-81`), so a recorded exchange cannot be tied to the agent that made it.
- The only per-connection client state in the proxy is a `Proxy-Authorization` marker read in `sink.Requestheaders` and then stripped (`traffic/pace.go:32-44`). go-mitmproxy fires `Requestheaders` for the CONNECT request and shares one `ConnContext` across every flow inside the tunnel, so a proxy credential set on CONNECT reaches every inner HTTPS exchange. This is the attribution hook.
- When interception fails for a host, the proxy tunnels it unrecorded for a minute (`traffic/traffic.go:655-692`). Nothing records that it happened.
- MCP tool calls are logged as `slog.Info("tool called", "tool", t.Name)` with no arguments or caller (`mcpserver/handler.go:40`). Server logs are slog text on stderr (`cmd/artex-cc/main.go:159`), teed into the web UI log buffer. The `activity` table is whatever agents choose to write with `log_activity`, so it is not an audit trail.
- Nothing routes agent egress through the proxy. The web UI runner starts `claude` with `os.Environ()` plus model settings (`api/chat_runner.go:317`), `capture_command_output` runs commands with the server's environment, and the enrich probes dial directly. Claude Code honors `HTTPS_PROXY`, and so do most HTTP CLIs, but raw sockets, DNS, nmap, ssh and SMB never use an HTTP proxy.
- The web UI runner stores each conversation's tool inputs and results in its transcript, but ignores subagent attribution (`parent_tool_use_id`) and drops `rate_limit_event` lines.
- Signed identities (`X-Artex-Identity`, see `docs/CLIENT_IDENTITY_PLAN.md`) attribute MCP calls over HTTP. stdio callers have no identity, and none of it reaches the proxy.
- Scope matching exists (`Store.IsInScope`, `store/scope.go:215`) and returns the matching rule. It queries PostgreSQL per call, so the proxy would need a cached snapshot refreshed on scope change notifications.
- There is no OpenTelemetry, Prometheus or `/metrics` code and no such module in `go.mod`. Ported but unwired pieces that later phases can use: the SOCKS5 listener (`traffic/socks5.go`), the transparent TCP listener (`traffic/transparent.go`) and the `egress` package.

## 1. Goals and non-goals

Goals:

- G1. Every egress the proxy sees, every tunneled host, every MCP tool call and every hook event produces one audit record, written before or as the action completes.
- G2. Records say who acted, as precisely as the evidence allows, and say how much each attribution field can be trusted.
- G3. Cheap, rule-based detections for injection-shaped behavior, computed in ARTEX with access to scope and stored secrets.
- G4. Alerts reach the operator through channels they choose. Agents see only notices that help an honest agent fix a mistake.
- G5. Nothing secret leaves the box by default: ARTEX's own tokens never, stored credentials and auth material masked, prompts hashed.
- G6. One vendor-neutral record format that any SIEM or observability backend can ingest with its own tooling, plus published field mappings.
- G7. ARTEX's own logs and metrics are exportable over the same standards, so an operator can watch the server as well as the agents.

Non-goals:

- Not a guarantee that all egress is seen. Until egress is forced through the proxy (phase 6), traffic that ignores proxy settings is invisible to the target stream. The agent stream and hooks still record what the agent asked to run.
- Not prompt-injection prevention. Detections flag behavior after the fact; the default policy alerts rather than blocks.
- No vendor-specific clients, dashboards, parsers or rule packs in this repo.
- No change to how the local target evidence store keeps traffic for findings and reports.

## 2. Streams

| Stream | Contents | Default local retention | Exported by default |
|---|---|---|---|
| `target` | Proxy exchanges to non-Anthropic hosts, tunneled CONNECTs, alerts derived from them | 7 days | Yes, once an exporter is configured |
| `anthropic` | Claude Code's Anthropic API traffic through the proxy: model, token counts, status, latency, request id | 7 days | Only when listed explicitly |
| `agent` | MCP tool calls (tool, caller, intent, argument digest, outcome) and Claude Code hook events (tool name, command or URL, session, subagent) | 7 days | Yes, once an exporter is configured |

Settled:

- Anthropic API traffic goes through the proxy as its own stream. Hosts are `api.anthropic.com`, the host of `ANTHROPIC_BASE_URL` when set, and an operator-extendable list for gateways and cloud providers. The credential is masked at capture, before anything is written. The `anthropic` exchanges live in their own store, not reachable from the `traffic_*` tools.
- MCP tool calls and hook events form the third stream, `agent`. They are agent activity, not target traffic, and they need their own retention and redaction.
- Retention defaults to 7 days per stream and is configurable per stream. It covers ARTEX's local audit files only; whatever backend receives an export applies its own retention.

Alerts are records in the stream of the event that triggered them, with `event.name` `artex.alert`, so a backend that ingests a stream gets its alerts without a second source.

## 3. Record schema direction

Settled: OpenTelemetry semantic conventions are canonical. The schema is versioned as `artex.audit/v1`; fields are added within a major version and never change type or meaning.

- Each record is an OTel log record with a fixed event name, not a span. Audit needs every record, and spans are sampled. When an inbound W3C `traceparent` is known (HTTP MCP calls and Anthropic requests from Claude Code with tracing on), the record carries that trace and span id.
- The JSONL line and the OTLP record use the same names. The JSONL line is a flat object with dotted keys, one record per line.
- Standard HTTP fields use stable semconv names: `http.request.method`, `url.full`, `url.scheme`, `server.address`, `server.port`, `http.response.status_code`, `error.type`, `client.address`, `user_agent.original`. Development-status names are used where nothing stable exists (`url.template`, `http.request.body.size`, `http.response.body.size`).
- `gen_ai.*` is used where it applies: tool calls (`gen_ai.operation.name=execute_tool`, `gen_ai.tool.name`) and the Anthropic stream (`gen_ai.provider.name=anthropic`, `gen_ai.request.model`, `gen_ai.response.id`, `gen_ai.usage.*`). These conventions are Development status and pinned to a named version in one place.
- Alerts use `security_rule.name` (the detector code, such as `scope.out`), `security_rule.category`, `security_rule.version` and `security_rule.reference`, plus `artex.subject_event_id` pointing at the record that triggered them.
- Claude Code's own attribute names are reused verbatim where ARTEX records the same thing (`session.id`, `agent_id`, `parent_agent_id`), so a backend joins the two sources without a mapping. Per-run agent ids do not go in `gen_ai.agent.id`, which semconv reserves for a stable hosted-agent resource, and nothing goes in ECS `agent.*`, which names the shipper.
- ARTEX-only fields go under `artex.*`: `artex.stream`, `artex.task_id`, `artex.intent_id`, `artex.identity`, `artex.exchange_id`, `artex.in_scope`, `artex.scope_rule`, `artex.signals`, `artex.attribution.*` (section 4).
- Every record carries `schema`, `event.id` (time-ordered, for dedup under at-least-once shipping), a per-stream sequence number (so gaps are visible) and the event time in RFC 3339 UTC with nanoseconds.
- ARTEX computes the registrable domain (`artex.registered_domain`) itself. Detections need it, and OTLP data bypasses most backends' ingest pipelines.

Illustrative `target` record:

```json
{"schema":"artex.audit/v1","time":"2026-11-28T15:04:05.123456789Z","event.name":"artex.egress.http","event.id":"01JDZ6Q8K4V9W2B3N5M7P0R1ST","artex.stream":"target","artex.seq":4211,"severity":"INFO","service.name":"artex-cc","service.instance.id":"b2c4...","artex.task_id":"t-17","http.request.method":"POST","url.full":"https://app.example.test/api/login?token=REDACTED","server.address":"app.example.test","server.port":443,"http.response.status_code":200,"http.request.body.size":412,"http.response.body.size":1880,"artex.in_scope":true,"artex.scope_rule":"*.example.test","artex.identity":"chat/7f3a","artex.attribution.identity_trust":"verified","artex.exchange_id":"x-9b1e"}
```

Mappings, not aliases: v1 emits only the canonical names. The docs publish mapping tables from the canonical names to Elastic ECS, Splunk CIM, OCSF (HTTP Activity 4002, Network Activity 4001, Detection Finding 2004), CrowdStrike CPS and Google SecOps UDM. Shippers and collectors rename fields on the way in; reference configs for that live in the example repos.

Deliverables with the schema: a JSON Schema file per major version, golden fixture files with one record per event name and detector, and a test that validates emitted records against the schema.

## 4. Attribution and trust levels

Settled: per-agent tagging is implemented where feasible, and any label an agent process could supply or alter is marked as claimed.

Sources, from most to least trustworthy:

| Field | Source | Trust |
|---|---|---|
| `artex.task_id`, engagement | Server configuration | `server` |
| `artex.identity` on `agent` records | Verified `X-Artex-Identity` on the MCP request | `verified` |
| `session.id`, `agent_id`, `parent_agent_id` on hook records | Claude Code hook input | `reported` (written by Claude Code, not by the model) |
| `session.id` for stdio callers | `CLAUDE_CODE_SESSION_ID` in the environment `serve --stdio` was spawned with | `reported` |
| Conversation tag on `target` records | Per-conversation proxy credential that the web UI runner puts in `HTTPS_PROXY` | `claimed` |
| Per-subagent tag on `target` records | Hook-injected proxy credential on a Bash command | `claimed` |
| `client.address` | Proxy socket | `server` |

Each attribution field records its source and trust as `artex.attribution.<field>_source` and `artex.attribution.<field>_trust`. Rules:

- Subagents share one Claude Code process and environment, so the network layer can only tell sessions apart. Per-subagent attribution of traffic comes from hooks and is always `claimed`.
- `claimed` labels are for triage. They never drive enforcement on their own.
- A mismatch between sources (a proxy tag that disagrees with the hook's agent for the same window, untagged traffic from a tagged session) is itself a detection input.
- Records without an identity say so (`artex.identity` absent, trust `none`) rather than guessing.

## 5. Detections

All detectors are rule-based, run in-process at event time, emit an `artex.alert` record, and never put the matched secret value in that record.

| Detector | Fires on | Default severity |
|---|---|---|
| `scope.out` | A host that fails the scope check and is not on the infrastructure allow-list. A near miss (shares a registrable domain with scope, or is a redirect target) is reported separately at lower severity | warning (near miss: notice) |
| `domain.new` | First registrable domain seen in this task | info; folds into `scope.out` when out of scope |
| `dest.exfil` | Known paste, webhook, tunnel and request-capture services (bundled list, operator-extendable; the operator's own OOB server goes on the allow-list) | high |
| `body.large` | Large request body, or a request far larger than its response, to a non-target host | warning; raised when correlated |
| `secret.leak` | A stored credential, ARTEX's own token, or a configured secret in a request to a host other than the credential's own domain, in raw and common encodings | critical |
| `audit.tamper` | Tool calls that remove recorded traffic or engagement data | high |
| `proxy.bypass` | Hook-seen commands that disable proxy settings, or a host reached by a command but never seen at the proxy | high |
| `proxy.tunneled` | Interception failed and the host is passing unrecorded | notice |
| `tls.untrusted` | Clients failing the proxy's TLS handshake (CA not trusted) | notice |
| `cmd.net` | Hook-seen raw network tools, and DNS lookups with long or high-entropy labels | warning; high for DNS exfil shapes |
| `intent.none` | Egress or mutating calls from an identity that holds no claimed intent | info; warning if sustained |

Correlation can raise severity, for example `scope.out` plus `body.large` from the same identity within five minutes becomes `high`.

Settled: out-of-scope traffic is allowed with an alert by default. Blocking is opt-in. In block mode the proxy answers 403 and a hook denies the command, with a fixed neutral reason ("blocked by egress policy"), never the detector name for `high` and `critical`. An infrastructure allow-list (package registries, Anthropic hosts, the operator's OOB server) is configuration, not scope.

## 6. Alert routing

Severity caps what an agent may see, and specificity drops as severity rises.

| Severity | Operator | Agents |
|---|---|---|
| `info` | Audit stream only | Nothing |
| `notice` | Alert feed | A specific notice is allowed |
| `warning` | Alert feed and toast | An aggregate line in `graph_overview` may be enabled by the operator |
| `high` | Feed, toast, webhook | Nothing specific; the neutral refusal in block mode |
| `critical` | Feed, banner, webhook; eligible for opt-in quarantine | Nothing; a generic "paused by the operator" message if quarantined |

Settled: agents see only low-severity, mistake-shaped notices, and the operator sees everything. Every agent notice is also an audit record.

- Agent notices are for things an honest agent can fix: a near-miss host, a TLS trust failure with the fix, a host that is not being recorded, working without a claimed intent. They are delivered on the caller's next ARTEX tool result as a separate, clearly labeled server block, at most two per result, deduplicated per detector, identity and domain within a lease window.
- Notices come only from a server-written channel, never from engagement data that agents can also write.
- Operator channels: an alert feed in the web UI (reusing the Intercept page's table and SSE pattern), toasts by severity, and the webhook. Each alert offers acknowledge, add to scope, add to allow-list, send a correction to the conversation, and quarantine.
- Quarantine is per identity and manual by default: refuse that identity's claims and mutating calls, release its claims, stop its web conversation, and refuse it at the proxy once proxy credentials are per identity. Auto-quarantine is opt-in and limited to `secret.leak`. Quarantine cannot stop egress that bypasses the proxy, and the UI says so.
- A per-detector policy table sets `enabled`, severity override, agent visibility (`none`, `passive`, `inline`) and action (`none`, `block`, `quarantine`). Dedup and rate limits apply to the feed (one row per key with counters), the webhook (per key per five minutes, with suppressed counts) and agent notices.

## 7. Redaction

Settled: redaction is a rule file with default mask sets that users can extend, override by id or disable. ARTEX does not assume the shape of the target's data.

- Rules live in `<data>/redaction.toml`. A rule names locations (URL parts, query, headers, cookies, JSON, form and multipart bodies, WebSocket and SSE messages, tool arguments, hook input), a matcher (key name globs, RE2 regex with an optional group, literal values, or a known-secret source such as stored credentials, ARTEX's own tokens, an environment variable or a file), and an action (`mask`, keyed `hash`, `partial`, `length`, `drop`). Allow rules win over masking rules. Rules can be scoped by host, method, status, direction, content type and scope status.
- Rules group into named sets. Each stream picks which sets run at each stage: `capture` (before anything is written locally), `export` (anything leaving ARTEX, including the local audit files that shippers tail) and `display` (MCP tool output and the web UI). Each sink can name a stricter profile.
- Built-in sets: `builtin.artex` (ARTEX's own bearer token, identity tokens, proxy credentials, keys and session values), `builtin.auth` (auth headers and cookie values), `builtin.url` (userinfo and sensitive query keys, including the semconv default list), `builtin.body_keys`, `builtin.secrets` (common token formats), `builtin.credentials` (the engagement's stored credentials), `builtin.anthropic_prompts`, and `builtin.pii` (off by default, since in a pentest PII is often the finding).
- Settled: `builtin.artex` runs at every stage of every stream and cannot be disabled.
- Settled: the default profile is `evidence`. Local target evidence stays intact apart from `builtin.artex`, so findings and replay still work, and everything leaving the box gets the full export set. `strict` also masks at capture and exports no bodies. `off` leaves only `builtin.artex`.
- Settled: stored credentials are masked in tool output by default, with an opt-out. This applies to captured traffic and command output that tools return, not to the credential tools themselves.
- Settled: prompts are exported as hashes by default; scrubbed previews are opt-in. The `anthropic` stream exports no bodies unless configured.
- Hashes are keyed (HMAC-SHA256 with a per-install key), so equal values correlate within an install and weak secrets cannot be brute-forced offline without the key.
- Export redaction fails closed per field: a body too large or unparseable for a required rule is replaced with a marker, and the record notes it.
- Tooling: `artex-cc redact lint`, `redact test` (run a stored exchange, HAR or record through a stage and show the hits), `redact coverage` (per-rule hit counts and unmasked values that look secret) and golden test cases. Each exported record carries a redaction summary and a config hash.

## 8. Export outputs

Settled: ARTEX stays vendor agnostic. These outputs are generic; vendor configuration lives in separate example repos.

1. **JSONL files** (always on, authoritative). `<data>/audit/<stream>.jsonl`, created at startup even when empty. One complete UTF-8 line per record, written with a single append. Rotation renames to a name the active glob does not match (for example `target.jsonl.20261128T100000Z`) and never truncates in place, which is what file tailers (vendor agents, Filebeat, Splunk UF, Vector, Fluent Bit, the Collector's `filelog`) need. Individual fields are bounded so no line nears common 1 MB per-event limits. No MCP tool can delete or rewrite these files.
2. **OTLP logs** (opt-in with `ARTEX_AUDIT_OTLP`). Standard `OTEL_EXPORTER_OTLP_*` variables configure it once enabled; setting them alone does not turn export on, since they are often set host-wide. `http/protobuf` is the default protocol, because several backends accept nothing else. One instrumentation scope per stream plus the `artex.stream` attribute let a collector route streams; per-stream endpoint and header overrides cover direct-to-vendor setups. The OTLP copy can drop under backpressure and counts what it drops; the files cannot.
3. **HTTP sink** (opt-in). The alert webhook and a batched sink are one implementation: URL, static headers with values from environment variables or files, body framing `ndjson`, `json_array` or `single`, gzip, batching by count and bytes (1 MB and 1000 records by default), retry with backoff that honors `Retry-After`, and replay from the JSONL files after a restart, with a configurable maximum age. A filter selects `alerts` or `all` and streams. Its own traffic is excluded from the proxy and from detections.
4. **Metrics** (opt-in), over OTLP and a Prometheus `/metrics` endpoint on its own listener, loopback by default with optional bearer auth. Metric temporality follows the standard environment variable, since some backends accept only delta. See section 9.

Not planned for core: vendor clients, SigV4 or Google OAuth signing, Parquet output, native syslog/CEF/LEEF. Shippers already do those. An OCSF output mode and an OAuth2 client-credentials token source for the HTTP sink are candidates if users ask.

## 9. ARTEX's own logs and metrics

The audit stream answers "what did the agents do". Operators also need "is ARTEX healthy", and both should land in the same backend with the same resource attributes.

- **Logs.** Server logs go through an OTel slog bridge when their OTLP export is enabled, under their own instrumentation scope and never mixed into audit streams. Stderr gains an optional JSON format next to the current text format. The existing log redaction (identity tokens, session IDs) applies before either output, and `builtin.artex` known values are added to it.
- **Metrics.** Cardinality-safe instruments, with no host, identity or task as attributes:

| Area | Examples |
|---|---|
| Proxy | exchanges by stream and status class, bytes in and out, tunneled hosts, interception and TLS failures |
| Tool calls | calls, errors and duration by tool |
| Intents | claims, renewals, conflicts, takeovers and lapsed claims |
| Store | query duration and errors, connection pool use, notification lag |
| Runner | active conversations, exits by reason, usage-limit events |
| Audit and exports | records written per stream, write errors, exported, dropped and queued records per sink, alerts by detector and severity, heartbeat |

- **Resource.** Every signal (audit records, logs, metrics) shares one resource: `service.name=artex-cc` (overridable), `service.version`, `service.instance.id`, `artex.engagement_id`, plus anything in `OTEL_RESOURCE_ATTRIBUTES`.
- **Correlation with Claude Code.** When the operator enables Claude Code telemetry, the web UI runner adds the same `artex.*` resource attributes to the `claude` processes it spawns and removes `OTEL_SERVICE_NAME` from their environment, so the two services stay distinct but filterable together. ARTEX never turns Claude Code telemetry on by itself.
- **Heartbeat.** Each stream writes a heartbeat record every 60 seconds with counters, so a backend can tell a dead pipeline from idle agents.

## 10. Configuration surface (proposed)

Names are proposals and will be fixed in the implementing PRs. Precedence is flag, then environment, then file, then default, as for existing settings.

| Setting | Default | Purpose |
|---|---|---|
| `--audit-dir` / `ARTEX_AUDIT_DIR` | `<data>/audit` | JSONL location |
| `ARTEX_AUDIT_RETENTION`, `ARTEX_AUDIT_RETENTION_<STREAM>` | `7d` | Local retention, per stream |
| `ARTEX_EGRESS_POLICY` | `alert` | `alert` or `block` for out-of-scope egress |
| `ARTEX_EGRESS_ALLOW` | Anthropic hosts | Infrastructure allow-list (hosts or a file) |
| `--redaction` / `ARTEX_REDACTION`, `ARTEX_REDACTION_PROFILE` | `<data>/redaction.toml`, `evidence` | Rule file and profile |
| `ARTEX_ANTHROPIC_HOSTS` | none | Extra hosts for the `anthropic` stream |
| `ARTEX_AUDIT_OTLP` | `off` | Enables OTLP export of audit records; standard `OTEL_*` variables configure it but never enable it |
| `ARTEX_AUDIT_OTLP_STREAMS` | `target,agent` | Streams exported over OTLP |
| `ARTEX_HTTP_SINK_URL`, `_HEADERS`, `_FORMAT`, `_EVENTS` | unset | HTTP sink and alert webhook |
| `--metrics-listen` / `ARTEX_METRICS_LISTEN` | unset | Prometheus endpoint |
| `ARTEX_OTEL_LOGS`, `ARTEX_OTEL_METRICS` | `off` | OTLP export of ARTEX's own logs and metrics, configured by the same `OTEL_*` variables |
| `ARTEX_LOG_FORMAT` | `text` | `text` or `json` on stderr |

No MCP tool changes redaction, retention, policy or export settings.

## 11. Phases

| Phase | Scope | Depends on |
|---|---|---|
| 1. Audit core | Record type, schema file and fixtures, JSONL writer with rotation and retention, `agent` stream from the MCP handler, `target` stream from the proxy, tunnel records, heartbeat, `redact` package with `builtin.artex` | none |
| 2. Attribution and the Anthropic stream | Proxy credential parsing and new exchange columns, runner sets proxy and CA environment per conversation, Anthropic stream store and capture masking, stdio session id, trust fields | 1 |
| 3. Detections and operator alerts | Scope snapshot in the proxy, detectors, alert records, policy table, alert feed, HTTP sink and webhook, opt-in block mode, full redaction rule sets and `redact` CLI | 1, 2 |
| 4. OTLP and ARTEX telemetry | OTLP logs exporter, slog bridge and JSON stderr, metrics over OTLP and `/metrics`, shared resource, runner environment hygiene | 1 |
| 5. Hooks and agent notices | `PreToolUse`/`PostToolUse` HTTP hook endpoint, per-conversation hook settings in the runner, subagent attribution, trace context extraction, agent notices, quarantine | 2, 3 |
| 6. Agent process isolation and enforcement | Run agents apart from the server (separate user or container), route agent egress only through the proxy, wire the SOCKS5 and transparent listeners, route `capture_command_output`, plugins and enrich through the proxy | 2 |

Phase 1 is useful alone: a durable record of tool calls and proxied traffic with no new dependency. Mapping tables and the vendor example repos follow phase 1, and the integration docs on artex-cc.com stay marked as illustrative until the stream ships.

## 12. Open questions

1. OTel Go SDK or a hand-rolled OTLP/HTTP protobuf poster. The SDK with HTTP log and metric exporters adds about 9 MB to the stripped binary and pulls in gRPC packages; the log exporters and the slog bridge are still pre-1.0. A hand-rolled poster adds about 1.5 MB but means owning batching and retry.
2. Whether alerts should also go to a dedicated `alerts.jsonl` for receivers that only want alerts, or whether the HTTP sink filter is enough.
3. Hash correlation leaks equality (two accounts with one password hash the same). Keep per-install keyed hashes as the default, or salt per record unless correlation is asked for?
4. Whether `builtin.credentials` should mask usernames as well as passwords, and the minimum length below which known values are not masked.
5. Whether to ship a broad third-party secret rule set as `builtin.secrets` or as a separate, off-by-default set.
6. JSON path support in rules: is a small subset (`$.a.b`, `$..key`, `[*]`) enough for v1?
7. Defaults for dedup windows, notice caps, heartbeat interval and the `body.large` threshold, to be set from eval runs rather than guessed.
8. Trace-context joins depend on Claude Code's tracing, which is in beta. Confirm whether `traceparent` reaches HTTP MCP servers without it.
9. Whether an OCSF output mode is worth building for OCSF-native lakes, and which OCSF version to pin.

## Decisions

Recorded 2026-11-28 by the maintainer.

- Out-of-scope traffic is allowed with an alert by default; blocking is opt-in.
- Anthropic API traffic goes through the proxy as its own stream, with the credential masked at capture.
- A third `agent` stream holds MCP tool calls and hook events.
- Redaction uses a rule file with default mask sets users can customize; no assumptions about target data shapes. The default profile is `evidence`. Stored credentials are masked in tool output by default, with an opt-out.
- Prompts are exported as hashes by default; scrubbed previews are opt-in.
- `builtin.artex` cannot be disabled.
- Audit retention defaults to 7 days, configurable per stream.
- ARTEX stays vendor agnostic: no vendor dashboards or configs in this repo; examples live in separate example repos.
- Per-agent tagging is implemented where feasible; spoofable labels are marked as claimed.
- Agents see only low-severity, mistake-shaped notices; the operator sees everything.
- OpenTelemetry semantic conventions are canonical: `server.address`/`server.port` for the target host, stable HTTP semconv names, `gen_ai.*` where applicable, ARTEX-only fields under `artex.*`. ECS, Splunk CIM, OCSF, CrowdStrike CPS and Google SecOps UDM get published mapping tables, with no alias fields in v1.
- ARTEX's own logs and metrics are part of the observability story: an OTel log bridge with optional JSON output, operational metrics over OTLP and `/metrics`, and one shared resource across all signals.

Usage wrap-up, which shares the runner's `rate_limit_event` parsing with phase 4's runner metrics, is planned separately in `docs/USAGE_WRAPUP_PLAN.md`.
