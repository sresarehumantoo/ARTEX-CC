---
title: Splunk
description: Get the ARTEX audit stream into Splunk Enterprise or Splunk Cloud Platform with a Universal Forwarder, the Splunk OTel Collector, Vector, or HEC, and map it to CIM.
sidebar:
  order: 10
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

The audit stream belongs in the Splunk platform (Enterprise or Cloud Platform), where Enterprise Security also runs. Splunk Observability Cloud documents OTLP ingest for traces and metrics, not logs, and its Log Observer Connect reads logs stored in the Splunk platform. So ARTEX's metrics can go to Observability Cloud if you want them there, but audit records go to an index.

The Splunk platform does not accept OTLP logs directly. Use one of these routes.

## Route 1: Universal Forwarder tailing the files

`inputs.conf` on the forwarder:

```ini
[monitor:///opt/artex/data/audit/target.jsonl]
sourcetype = artex:target
index = artex
ignoreOlderThan = 7d

[monitor:///opt/artex/data/audit/agent.jsonl]
sourcetype = artex:agent
index = artex
ignoreOlderThan = 7d
```

The forwarder recognizes files by a CRC of their first 256 bytes. ARTEX rotates by rename and starts every line with a unique `time` and `event.id`, so rotated files are neither skipped nor indexed twice. Give the `anthropic` stream its own index if you ingest it, so retention and role access can differ.

`props.conf` for the sourcetypes, in an add-on installed on search heads and indexers:

```ini
[artex:target]
SHOULD_LINEMERGE = false
LINE_BREAKER = ([\r\n]+)
TIME_PREFIX = "time":"
MAX_TIMESTAMP_LOOKAHEAD = 40
KV_MODE = json
FIELDALIAS-artex_cim = server.address AS dest server.port AS dest_port http.request.method AS http_method url.full AS url http.response.status_code AS status http.request.body.size AS bytes_out http.response.body.size AS bytes_in artex.identity AS user
EVAL-app = "artex"

[artex:agent]
SHOULD_LINEMERGE = false
LINE_BREAKER = ([\r\n]+)
TIME_PREFIX = "time":"
MAX_TIMESTAMP_LOOKAHEAD = 40
KV_MODE = json
FIELDALIAS-artex_cim = artex.identity AS user
EVAL-app = "artex"
```

Use search-time `KV_MODE = json` or forwarder-side `INDEXED_EXTRACTIONS = json`, not both, or fields are extracted twice. Note the byte direction: CIM `bytes_out` is client to server, the request body.

`eventtypes.conf` and `tags.conf`, so CIM Web and Alerts searches find the data:

```ini
# eventtypes.conf
[artex_web]
search = sourcetype="artex:target" "event.name"="artex.egress.http"

[artex_alert]
search = sourcetype="artex:target" "event.name"="artex.alert"
```

```ini
# tags.conf
[eventtype=artex_web]
web = enabled
proxy = enabled

[eventtype=artex_alert]
alert = enabled
```

## Route 2: Splunk OTel Collector

The collector tails the files and sends to HEC. It sets the HEC sourcetype and index from `artex.stream` with a transform, so ARTEX never emits Splunk-specific `com.splunk.*` attributes. It also scrapes ARTEX's `/metrics` into a metrics index.

```yaml
# Illustrative: written against the draft artex.audit/v1 schema.
receivers:
  file_log/artex:
    include:
      - /artex/audit/target.jsonl
      - /artex/audit/agent.jsonl
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
  prometheus/artex:
    config:
      scrape_configs:
        - job_name: artex
          static_configs:
            - targets: ["${env:ARTEX_METRICS_ADDR}"]

extensions:
  file_storage:
    directory: /var/lib/otelcol/storage
    create_directory: true

processors:
  batch: {}
  # Pick the HEC sourcetype and index from the ARTEX stream, here in the collector.
  transform/splunk_meta:
    log_statements:
      - context: log
        statements:
          - set(log.attributes["com.splunk.sourcetype"], Concat(["artex", log.attributes["artex.stream"]], ":"))
          - set(log.attributes["com.splunk.index"], "artex")

exporters:
  splunk_hec/logs:
    token: "${env:SPLUNK_HEC_TOKEN}"
    endpoint: "${env:SPLUNK_HEC_URL}/services/collector"
    source: artex-cc
  splunk_hec/metrics:
    token: "${env:SPLUNK_HEC_TOKEN}"
    endpoint: "${env:SPLUNK_HEC_URL}/services/collector"
    index: artex_metrics

service:
  extensions: [file_storage]
  pipelines:
    logs:
      receivers: [file_log/artex]
      processors: [transform/splunk_meta, batch]
      exporters: [splunk_hec/logs]
    metrics:
      receivers: [prometheus/artex]
      processors: [batch]
      exporters: [splunk_hec/metrics]
```

The sourcetypes it produces are `artex:target` and `artex:agent`, matching the add-on above.

## Route 3: Vector

```yaml
sinks:
  splunk_target:
    type: splunk_hec_logs
    inputs: [artex_by_stream.target, artex_by_stream.agent]
    endpoint: "${SPLUNK_HEC_URL}"
    default_token: "${SPLUNK_HEC_TOKEN}"
    index: artex
    sourcetype: "artex:audit"
    encoding:
      codec: json
```

Use with the [Vector base config](/docs/integrations/vector/).

## Route 4: HEC from ARTEX's HTTP sink

For small setups with no forwarder, ARTEX's planned HTTP sink can post NDJSON to HEC's raw endpoint, where each line becomes one event and metadata goes in the query string:

```sh
ARTEX_HTTP_SINK_URL="https://http-inputs-<stack>.splunkcloud.com/services/collector/raw?sourcetype=artex:target&index=artex"
ARTEX_HTTP_SINK_HEADERS="Authorization=Splunk ${SPLUNK_HEC_TOKEN},X-Splunk-Request-Channel=<GUID>"
ARTEX_HTTP_SINK_FORMAT=ndjson
ARTEX_HTTP_SINK_EVENTS=all
```

Splunk Enterprise listens on port 8088 by default; paid Splunk Cloud Platform stacks use 443 on `http-inputs-<stack>.splunkcloud.com`. Indexer acknowledgment is not supported for generic clients on Splunk Cloud Platform, so the sink treats a 2xx as delivered and retries otherwise. The sink batches at 1 MB by default, below any HEC request limit reported for Splunk Cloud.

## Searches

```
# ARTEX-raised alerts, high and critical
index=artex sourcetype="artex:target" "event.name"="artex.alert" "artex.alert.severity" IN (high, critical)
| table _time security_rule.name user dest artex.task_id artex.subject_event_id

# Out-of-scope egress per agent, 5-minute buckets
index=artex sourcetype="artex:target" "artex.in_scope"=false
| timechart span=5m count by user

# The audit stream went quiet (a dead pipeline looks like idle agents otherwise)
| tstats latest(_time) as last where index=artex by host
| where last < relative_time(now(), "-15m")
```

In Enterprise Security 8, one event-based detection over `tag=alert app=artex` can map `artex.alert.severity` to a risk score with the agent identity (`user`) as the risk object, and a finding-based detection can raise a finding when one agent piles up several weaker signals.

## ARTEX's own telemetry

Route 2 already scrapes `/metrics` into a metrics index. If you use Observability Cloud, ARTEX's OTLP metrics can go there directly with `OTEL_EXPORTER_OTLP_HEADERS=X-SF-Token=<token>` and `ARTEX_OTEL_METRICS=on`.

## Validation

- Splunk OTel Collector config: `otelcol validate` from `quay.io/signalfx/splunk-otel-collector` 0.165.1 passed with placeholder endpoints.
- Vector sink: `vector validate` 0.62.0 passed, see [Vector](/docs/integrations/vector/).
- `inputs.conf`, `props.conf`, `eventtypes.conf`, `tags.conf`, the HEC settings and the SPL were written from Splunk documentation only and have not been loaded into a Splunk instance.
