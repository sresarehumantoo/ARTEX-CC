---
title: OpenTelemetry Collector
description: One Collector that takes ARTEX's audit stream over OTLP or from the JSONL files, routes the three streams, adds a redaction gate and fans out to several backends.
sidebar:
  order: 1
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

The Collector is the most general route: every backend on these pages accepts OTLP directly or has a Collector exporter, and vendor-specific work (renaming fields, extra redaction, routing, fan-out) belongs in the Collector rather than in ARTEX. Use the contrib distribution, or a vendor distribution that includes the components below.

## What this config does

- Receives ARTEX's OTLP export on port 4318 (`ARTEX_AUDIT_OTLP=on`, plus ARTEX's own logs and metrics if enabled), and also tails the JSONL files with `file_log`. Use one or the other for audit records; both are shown.
- Parses each JSONL line into attributes, takes the record time as the log timestamp and the severity from `severity`, and moves `service.name` onto the resource. It then replaces the body with the event name, because the parsed line would otherwise still carry every field in the body, including any the redaction step later removes from the attributes.
- Scrubs common secret-bearing query parameters again as defense in depth. ARTEX already redacts before export; this is for a site policy that is stricter.
- Routes by `artex.stream` to three pipelines. The `anthropic` pipeline runs an allow-list `redaction` processor, so only metadata leaves.
- Scrapes ARTEX's `/metrics` and forwards metrics with the OTLP ones.
- Fans the `target` and `agent` streams out to two exporters. Each exporter gets its own copy.

```yaml
# Illustrative: written against the draft artex.audit/v1 schema.
receivers:
  # ARTEX's OTLP export (ARTEX_AUDIT_OTLP=on) and its own logs and metrics.
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

  # Or tail the JSONL files instead of (or as well as) OTLP.
  file_log/artex:
    include:
      - ${env:ARTEX_AUDIT_DIR}/target.jsonl
      - ${env:ARTEX_AUDIT_DIR}/agent.jsonl
      - ${env:ARTEX_AUDIT_DIR}/anthropic.jsonl
    start_at: beginning
    storage: file_storage
    operators:
      - type: json_parser
        parse_to: attributes
        timestamp:
          parse_from: attributes.time
          layout_type: gotime
          layout: "2006-01-02T15:04:05.999999999Z07:00"
        severity:
          parse_from: attributes.severity
      - type: move
        from: attributes["service.name"]
        to: resource["service.name"]

  # ARTEX's Prometheus endpoint (--metrics-listen).
  prometheus/artex:
    config:
      scrape_configs:
        - job_name: artex
          scrape_interval: 30s
          static_configs:
            - targets: ["${env:ARTEX_METRICS_ADDR}"]

extensions:
  file_storage:
    directory: /var/lib/otelcol/storage
    create_directory: true

processors:
  batch: {}
  # Defense in depth: ARTEX already redacts before export.
  transform/scrub:
    error_mode: ignore
    log_statements:
      - context: log
        statements:
          # The parsed JSONL line stays in the body; keep only the event name there.
          - set(log.body, log.attributes["event.name"]) where log.attributes["event.name"] != nil
          - replace_pattern(log.attributes["url.full"], "([?&](token|key|code|session)=)[^&]*", "$${1}REDACTED")
          - delete_key(log.attributes, "user_agent.original")
  # Allow-list gate for the anthropic stream.
  redaction/anthropic:
    allow_all_keys: false
    allowed_keys:
      - schema
      - time
      - event.name
      - event.id
      - severity
      - artex.stream
      - artex.seq
      - session.id
      - artex.identity
      - gen_ai.provider.name
      - gen_ai.request.model
      - gen_ai.response.id
      - gen_ai.usage.input_tokens
      - gen_ai.usage.output_tokens
      - server.address
      - http.response.status_code
      - artex.prompt.sha256
    blocked_values:
      - "sk-ant-[A-Za-z0-9_-]+"
    summary: info

connectors:
  routing/streams:
    default_pipelines: [logs/target]
    table:
      - condition: log.attributes["artex.stream"] == "anthropic"
        pipelines: [logs/anthropic]
      - condition: log.attributes["artex.stream"] == "agent"
        pipelines: [logs/agent]

exporters:
  # Replace with your backend: otlphttp to any OTLP intake, or a vendor exporter.
  otlphttp/backend:
    endpoint: ${env:BACKEND_OTLP_ENDPOINT}
    headers:
      Authorization: ${env:BACKEND_AUTH_HEADER}
  otlphttp/archive:
    endpoint: ${env:ARCHIVE_OTLP_ENDPOINT}
  debug:
    verbosity: basic

service:
  extensions: [file_storage]
  pipelines:
    logs/in:
      receivers: [otlp, file_log/artex]
      processors: [transform/scrub]
      exporters: [routing/streams]
    logs/target:
      receivers: [routing/streams]
      processors: [batch]
      exporters: [otlphttp/backend, otlphttp/archive]
    logs/agent:
      receivers: [routing/streams]
      processors: [batch]
      exporters: [otlphttp/backend, otlphttp/archive]
    logs/anthropic:
      receivers: [routing/streams]
      processors: [redaction/anthropic, batch]
      exporters: [otlphttp/backend]
    metrics:
      receivers: [otlp, prometheus/artex]
      processors: [batch]
      exporters: [otlphttp/backend]
```

Environment:

| Variable | Example |
|---|---|
| `ARTEX_AUDIT_DIR` | `/artex/audit` (the ARTEX data volume mounted read-only) |
| `ARTEX_METRICS_ADDR` | `artex:9464` |
| `BACKEND_OTLP_ENDPOINT` | Your backend's OTLP/HTTP base URL |
| `BACKEND_AUTH_HEADER` | The value of the `Authorization` header your backend expects |
| `ARCHIVE_OTLP_ENDPOINT` | A second OTLP destination, for example a long-retention store |

On the ARTEX side, point the OTLP exporter at the Collector:

```sh
ARTEX_AUDIT_OTLP=on
ARTEX_AUDIT_OTLP_STREAMS=target,agent,anthropic
ARTEX_OTEL_LOGS=on
ARTEX_OTEL_METRICS=on
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

## Notes

- Replace `otlphttp/backend` with a vendor exporter where one is better suited (`splunk_hec`, `elasticsearch`, `datadog`), or keep OTLP for backends that take it natively. The vendor pages show which.
- In `replace_pattern`, a capture group in the replacement must be written `${1}`, and `$` is escaped as `$$` in Collector config, giving `$${1}`. A bare `$$1REDACTED` silently drops the match, because Go reads `$1REDACTED` as a group named `1REDACTED`. The dry run caught this.
- `file_log` is the current receiver name; `filelog` still works in 0.166.0 but logs a deprecation warning.
- `file_storage` keeps read offsets across restarts, so the Collector resumes where it stopped instead of re-reading from the start.
- If Claude Code's own telemetry (`CLAUDE_CODE_ENABLE_TELEMETRY=1`) goes to the same Collector, route on `resource.attributes["service.name"]` to separate `claude-code` from `artex-cc`. The web UI runner adds ARTEX's `artex.*` resource attributes to the `claude` processes it starts, so both services can be filtered together downstream.

## Validation

- `otelcol-contrib validate --config=collector.yaml` with otelcol-contrib 0.166.0: passed. A deliberately misspelled OTTL function failed validation, so the check does parse OTTL statements.
- A run with the exporters replaced by `debug` and three sample records per stream: records were parsed with the right timestamps and severities, routed to the matching pipeline, the `url.full` query value stayed `REDACTED`, and the `anthropic` record kept only allow-listed attributes.
- Not run against a real backend.
