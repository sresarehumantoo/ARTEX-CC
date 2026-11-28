---
title: Datadog
description: Send the ARTEX audit stream and metrics to Datadog with the Agent, Vector, or agentless OTLP.
sidebar:
  order: 14
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

## Route 1: Datadog Agent tailing the files

Enable log collection (`logs_enabled: true` in `datadog.yaml`), then add `conf.d/artex.d/conf.yaml`:

```yaml
logs:
  - type: file
    path: /opt/artex/data/audit/target.jsonl
    service: artex-cc
    source: artex
    tags: ["artex_stream:target"]
  - type: file
    path: /opt/artex/data/audit/agent.jsonl
    service: artex-cc
    source: artex
    tags: ["artex_stream:agent"]
```

Each line must end with a newline, or the Agent waits for it indefinitely; ARTEX writes whole lines. JSON lines are parsed into attributes automatically. Add a pipeline date remapper on `time` so the record time is used.

For ARTEX's metrics, the Agent's OpenMetrics check can scrape `/metrics`.

## Route 2: Vector

```yaml
sinks:
  datadog:
    type: datadog_logs
    inputs: [artex_by_stream.target, artex_by_stream.agent]
    default_api_key: "${DD_API_KEY}"
    site: "${DD_SITE}"
    compression: gzip
```

Use with the [Vector base config](/docs/integrations/vector/).

## Route 3: Agentless OTLP

Datadog accepts OTLP logs over HTTP with protobuf and a `dd-api-key` header:

```sh
ARTEX_AUDIT_OTLP=on
ARTEX_OTEL_METRICS=on
OTEL_EXPORTER_OTLP_ENDPOINT=<Datadog OTLP intake for your site>
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_HEADERS="dd-api-key=<api key>"
OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta
```

Datadog recommends an Agent or Collector in production. Logs request bodies over 5.1 MiB are rejected, and log timestamps more than 18 hours old are dropped, so a long replay after an outage loses records on the Datadog side; the local files keep them.

## Detections

Create a log monitor or a Cloud SIEM rule on `service:artex-cc @event.name:artex.alert`, grouped by `@security_rule.name` and `@artex.identity`. Datadog's standard attributes use different names (`http.url_details.host`, `network.bytes_written`); remap in a pipeline if you want them, see [Field mappings](/docs/integrations/field-mappings/).

## Validation

- Vector sink: `vector validate` 0.62.0 passed, see [Vector](/docs/integrations/vector/).
- The Agent config, OTLP settings and monitor query were written from Datadog documentation only. Nothing was sent to Datadog.
