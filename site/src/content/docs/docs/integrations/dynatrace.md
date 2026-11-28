---
title: Dynatrace
description: Send the ARTEX audit stream and ARTEX metrics to Dynatrace over OTLP, through the Dynatrace OTel Collector, or with OneAgent.
sidebar:
  order: 13
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

## Route 1: OTLP straight from ARTEX

Dynatrace's OTLP endpoint takes protobuf over HTTP only (no gRPC, no JSON), which is ARTEX's default.

```sh
ARTEX_AUDIT_OTLP=on
ARTEX_OTEL_LOGS=on
ARTEX_OTEL_METRICS=on
OTEL_EXPORTER_OTLP_ENDPOINT=https://<environment-id>.live.dynatrace.com/api/v2/otlp
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_HEADERS="Authorization=Api-Token <token>"
OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta
```

Dynatrace rejects cumulative counters, so the temporality setting is required for metrics. A platform token works with `Authorization=Bearer <token>`. Scopes: `openpipeline:logs:ingest` for logs and `openpipeline:metrics:ingest` for metrics (classic tokens: `logs.ingest`, `metrics.ingest`).

## Route 2: Dynatrace OTel Collector

Tails the files, scrapes `/metrics` and converts it to delta:

```yaml
# Illustrative: written against the draft artex.audit/v1 schema.
receivers:
  file_log/artex:
    include:
      - /artex/audit/target.jsonl
      - /artex/audit/agent.jsonl
    start_at: beginning
    operators:
      - type: json_parser
        parse_to: attributes
        timestamp:
          parse_from: attributes.time
          layout_type: gotime
          layout: "2006-01-02T15:04:05.999999999Z07:00"
        severity:
          parse_from: attributes.severity
  prometheus/artex:
    config:
      scrape_configs:
        - job_name: artex
          static_configs:
            - targets: ["${env:ARTEX_METRICS_ADDR}"]

processors:
  batch: {}
  transform/body:
    log_statements:
      - context: log
        statements:
          - set(log.body, log.attributes["event.name"]) where log.attributes["event.name"] != nil
  # Dynatrace accepts delta metrics only; /metrics is cumulative.
  cumulativetodelta: {}

exporters:
  otlphttp/dynatrace:
    endpoint: "${env:DT_ENDPOINT}/api/v2/otlp"
    headers:
      Authorization: "Api-Token ${env:DT_API_TOKEN}"

service:
  pipelines:
    logs:
      receivers: [file_log/artex]
      processors: [transform/body, batch]
      exporters: [otlphttp/dynatrace]
    metrics:
      receivers: [prometheus/artex]
      processors: [cumulativetodelta, batch]
      exporters: [otlphttp/dynatrace]
```

The transform keeps the body to the event name, so attributes carry the fields and Dynatrace does not store the whole line again as `content`.

## Route 3: OneAgent or the Log Ingest API

On a host with OneAgent, add a custom log source for the absolute paths of the stream files and a log ingest rule (both are needed). The Log Ingest API v2 also takes JSON Lines (`Content-Type: application/jsonl`), so any shipper can post the files as they are. Records older than 72 hours are rejected.

## Storage and detections

- Built-in buckets have fixed retention (`default_logs` is 35 days). For a 7-day copy, create a custom bucket per stream and add an OpenPipeline bucket assignment on `artex.stream`. A permission rule can restrict the `anthropic` stream to a smaller group.
- Keep detection logic in ARTEX and alert on its records. A Davis anomaly detector with a records analyzer over this DQL fires per ARTEX alert:

```
fetch logs
| filter service.name == "artex-cc" and event.name == "artex.alert"
| fields timestamp, security_rule.name, artex.identity, server.address, artex.task_id
```

- Rates, for example out-of-scope requests per agent:

```
fetch logs
| filter service.name == "artex-cc" and artex.stream == "target" and event.name == "artex.egress.http"
| makeTimeseries out_of_scope = countIf(artex.in_scope == false), by:{artex.identity}
```

- Dynatrace's Claude Code monitoring uses Claude Code's own telemetry. ARTEX adds its `artex.*` resource attributes to the `claude` processes its web UI runner starts, so both can be filtered by engagement and identity in DQL.

## Validation

- Collector config: `dynatrace-otel-collector validate` 0.61.0 passed with placeholder endpoint and token.
- The environment variables, OneAgent setup, buckets and DQL were written from Dynatrace documentation only and were not run against a tenant.
