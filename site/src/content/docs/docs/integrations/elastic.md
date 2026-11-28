---
title: Elastic
description: Ship the ARTEX audit stream to Elasticsearch with Filebeat or Elastic Agent, or over OTLP, and promote ARTEX alerts in Elastic Security.
sidebar:
  order: 11
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

## Route 1: Filebeat or Elastic Agent, filestream with ndjson (recommended)

Filebeat writes each stream to its own data stream under `logs-*-*`, which picks up the built-in `ecs@mappings` template. The `ndjson` parser puts keys at the event root and `expand_keys` turns dotted keys into objects. The optional `rename` processor moves the OTel names that differ from ECS (see [Field mappings](/docs/integrations/field-mappings/)), so Elastic Security content that expects `destination.*` works.

```yaml
# Illustrative: written against the draft artex.audit/v1 schema.
filebeat.inputs:
  - type: filestream
    id: artex-target
    paths:
      - /artex/audit/target.jsonl
    index: "logs-artex.target-default"
    parsers:
      - ndjson:
          target: ""
          expand_keys: true
          add_error_key: true
          overwrite_keys: true
    processors:
      - timestamp:
          field: time
          layouts:
            - "2006-01-02T15:04:05.999999999Z07:00"
          test:
            - "2026-11-28T15:04:05.123456789Z"
      # Optional: ECS names for Elastic Security content. See the field mapping page.
      - rename:
          fields:
            - from: "server.address"
              to: "destination.domain"
            - from: "server.port"
              to: "destination.port"
            - from: "http.request.body.size"
              to: "http.request.body.bytes"
            - from: "http.response.body.size"
              to: "http.response.body.bytes"
          ignore_missing: true
          fail_on_error: false
      # Lets Elastic Security's prebuilt External Alerts rule promote ARTEX alerts.
      - add_fields:
          when:
            equals:
              event.name: "artex.alert"
          target: ""
          fields:
            event.kind: alert
            event.module: artex
            event.category: [intrusion_detection]
  - type: filestream
    id: artex-agent
    paths:
      - /artex/audit/agent.jsonl
    index: "logs-artex.agent-default"
    parsers:
      - ndjson:
          target: ""
          expand_keys: true
          add_error_key: true
          overwrite_keys: true

output.elasticsearch:
  hosts: ["${ES_HOSTS}"]
  api_key: "${ES_API_KEY}"
```

The Fleet equivalent is the "Custom Logs (Filestream)" integration with the same parser settings.

Give the shipper a narrow API key: cluster `monitor`, and `auto_configure` plus `create_doc` on `logs-artex.*-*`. Without `delete` or `index`, the shipper can only append, so the Elastic copy cannot be rewritten from the ARTEX host.

Retention: set `data_retention` on the data stream lifecycle, for example `7d` for `logs-artex.target-*` and less for an `anthropic` stream. ARTEX's own 7-day setting covers its local files only.

## Route 2: OTLP

- Elastic Cloud Hosted and Serverless offer a managed OTLP endpoint. Point ARTEX at it with `OTEL_EXPORTER_OTLP_ENDPOINT=<motlp endpoint>` and `OTEL_EXPORTER_OTLP_HEADERS="Authorization=ApiKey <key>"`, plus `ARTEX_AUDIT_OTLP=on`.
- Self-managed Elasticsearch has an `/_otlp` endpoint that takes OTLP/HTTP protobuf only. Its logs support is in technical preview in 9.5.
- OTel-native storage keeps attributes under `attributes.*` and bypasses ingest pipelines. Elastic's passthrough makes them queryable at the top level, but whether prebuilt Security rules behave the same on `logs-*.otel-*` streams is not documented; test before relying on it. Route 1 is the safer choice for Security.
- To route the streams to separate data streams over OTLP, put an [OpenTelemetry Collector](/docs/integrations/opentelemetry-collector/) (or EDOT) in front and set `data_stream.dataset` from `artex.stream` there.

## Detections

ARTEX's alerts can be promoted without writing rules: the prebuilt "External Alerts" rule picks up `event.kind: alert` records outside a few excluded modules. The `add_fields` processor in the Filebeat config above sets `event.kind`, `event.module` and `event.category` on ARTEX's alert records for this.

Independent rules, as defense in depth (KQL unless noted):

| ARTEX signal | Elastic rule |
|---|---|
| `scope.out` | `data_stream.dataset:artex.target and artex.in_scope:false` |
| `domain.new` | New terms on `artex.registered_domain`, history window `now-14d` |
| `dest.exfil` | Indicator match against a list of paste and webhook services you maintain |
| `body.large` | `http.request.body.bytes > 65536 and artex.in_scope:false` |
| `audit.tamper` | `event.name:artex.alert and security_rule.name:audit.tamper` |

`secret.leak` stays in ARTEX: only ARTEX holds the stored credentials, and the exported records do not contain them.

## ARTEX's own telemetry

Scrape `/metrics` with the Elastic Agent Prometheus integration or EDOT's Prometheus receiver. Send server logs with `ARTEX_OTEL_LOGS=on` over OTLP, or collect JSON stderr (`ARTEX_LOG_FORMAT=json`) with the container input.

## Validation

- `filebeat test config` with Filebeat 9.5.6, placeholder host and key: "Config OK", including the `rename` and `add_fields` processors.
- API key privileges, data stream lifecycle and the rule table were written from Elastic documentation only.
- Not run against a real cluster.
