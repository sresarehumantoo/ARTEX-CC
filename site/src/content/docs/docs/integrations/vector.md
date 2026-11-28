---
title: Vector
description: Tail ARTEX's audit JSONL files with Vector, route the streams and send them to Splunk, Datadog, Sentinel, Google SecOps, CloudWatch or a syslog SIEM.
sidebar:
  order: 2
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

Vector has sinks for most backends on these pages, so one base config plus one sink file covers them. Vector loads several config files and merges them.

## Base: source, parse, route

```yaml
# Illustrative: written against the draft artex.audit/v1 schema.
data_dir: /var/lib/vector

sources:
  artex_audit:
    type: file
    include:
      - /artex/audit/target.jsonl
      - /artex/audit/agent.jsonl
      - /artex/audit/anthropic.jsonl
    read_from: beginning

transforms:
  artex_parsed:
    type: remap
    inputs: [artex_audit]
    source: |
      . = parse_json!(string!(.message))
      .timestamp = parse_timestamp!(string!(."time"), format: "%+")

  artex_by_stream:
    type: route
    inputs: [artex_parsed]
    route:
      target: '."artex.stream" == "target"'
      agent: '."artex.stream" == "agent"'
      anthropic: '."artex.stream" == "anthropic"'

  artex_alerts:
    type: filter
    inputs: [artex_parsed]
    condition: '."event.name" == "artex.alert"'
```

`artex_by_stream.target`, `.agent` and `.anthropic` are the routed outputs; `artex_alerts` carries only ARTEX's alert records. Leave `anthropic` unconnected unless you mean to export it.

## Sinks

Each vendor page shows its sink. They are collected here for reference.

Splunk HEC:

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

Datadog:

```yaml
sinks:
  datadog:
    type: datadog_logs
    inputs: [artex_by_stream.target, artex_by_stream.agent]
    default_api_key: "${DD_API_KEY}"
    site: "${DD_SITE}"
    compression: gzip
```

Microsoft Sentinel (Logs Ingestion API). Azure column names cannot contain dots, so the transform flattens them:

```yaml
transforms:
  artex_flat_columns:
    type: remap
    inputs: [artex_by_stream.target, artex_by_stream.agent]
    source: |
      # Azure column names allow letters, digits and underscores only.
      . = map_keys(.) -> |key| { replace(key, ".", "_") }
      .TimeGenerated = del(.timestamp)

sinks:
  sentinel:
    type: azure_logs_ingestion
    inputs: [artex_flat_columns]
    endpoint: "${AZURE_DCE_URL}"
    dcr_immutable_id: "${AZURE_DCR_ID}"
    stream_name: "Custom-ArtexAudit_CL"
    auth:
      azure_credential_kind: client_secret_credential
      azure_tenant_id: "${AZURE_TENANT_ID}"
      azure_client_id: "${AZURE_CLIENT_ID}"
      azure_client_secret: "${AZURE_CLIENT_SECRET}"
```

Google SecOps:

```yaml
sinks:
  secops:
    type: gcp_chronicle_unstructured
    inputs: [artex_by_stream.target, artex_by_stream.agent]
    customer_id: "${SECOPS_CUSTOMER_ID}"
    credentials_path: /etc/vector/secops-sa.json
    log_type: "${SECOPS_LOG_TYPE}"
    region: us
    encoding:
      codec: json
```

AWS CloudWatch Logs, one log stream per ARTEX stream:

```yaml
sinks:
  cloudwatch:
    type: aws_cloudwatch_logs
    inputs: [artex_by_stream.target, artex_by_stream.agent]
    region: "${AWS_REGION}"
    group_name: /artex/audit
    stream_name: '{{ "artex.stream" }}'
    encoding:
      codec: json
```

Syslog with a CEF body, alerts only:

```yaml
transforms:
  artex_cef:
    type: remap
    inputs: [artex_alerts]
    source: |
      sev = if ."severity" == "WARN" { "6" } else if ."severity" == "ERROR" { "9" } else { "3" }
      cef = "CEF:0|ARTEX-CC|artex-cc|v1|" + (string(."security_rule.name") ?? "artex.alert") + "|ARTEX egress alert|" + sev +
        "|rt=" + (string(."time") ?? "") +
        " dhost=" + (string(."server.address") ?? "") +
        " suser=" + (string(."artex.identity") ?? "") +
        " cs1Label=task cs1=" + (string(."artex.task_id") ?? "")
      # RFC 5424 header, facility local0 (16), severity informational.
      . = {"message": "<134>1 " + (string(."time") ?? "-") + " artex-host artex-cc - artex-alert - " + cef}

sinks:
  legacy_siem:
    type: socket
    inputs: [artex_cef]
    mode: tcp
    address: "${SIEM_SYSLOG_ADDR}"
    framing:
      method: newline_delimited
    encoding:
      codec: text
```

Run with `vector --config base.yaml --config sink-splunk.yaml`.

## Environment variables in Vector 0.62

Vector 0.62.0 interpolates `${VAR}` in config files only when started with `--dangerously-allow-env-var-interpolation` (or `VECTOR_DANGEROUSLY_ALLOW_ENV_VAR_INTERPOLATION`). Without it, the `${...}` placeholders above are read literally and validation fails with "invalid uri character". Either pass the flag or move the credentials to one of Vector's secret backends.

## ARTEX's own telemetry

Vector can scrape ARTEX's `/metrics` with a `prometheus_scrape` source and forward it with the matching metrics sink of your backend. For server logs, collect stderr (`ARTEX_LOG_FORMAT=json`) with the `docker_logs` or `journald` source.

## Validation

- `vector validate --no-environment --dangerously-allow-env-var-interpolation base.yaml <sink>.yaml` with Vector 0.62.0, for each of the six sink files, using placeholder endpoints and credentials: all passed.
- The Sentinel flattening transform and the CEF transform were run against sample records with a console sink, and produced flat keys and an RFC 5424 line with a CEF body as shown.
- `--no-environment` skips health checks, so no endpoint was contacted. Not run against a real backend.
