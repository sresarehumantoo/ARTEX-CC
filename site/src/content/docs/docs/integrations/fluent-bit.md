---
title: Fluent Bit
description: Tail ARTEX's audit JSONL files with Fluent Bit and send them over OTLP, syslog or to Microsoft Sentinel.
sidebar:
  order: 3
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

## Tail, parse, send

The `tail` input reads the active stream files, keeps its offsets in a database file and buffers to disk. The parser takes the record time as the event timestamp and keeps the original field.

```yaml
# Illustrative: written against the draft artex.audit/v1 schema.
service:
  flush: 5
  storage.path: /var/lib/fluent-bit/storage
  parsers_file: parsers.yaml

pipeline:
  inputs:
    - name: tail
      path: /artex/audit/target.jsonl,/artex/audit/agent.jsonl,/artex/audit/anthropic.jsonl
      path_key: artex_file
      tag: artex.audit
      parser: artex_jsonl
      db: /var/lib/fluent-bit/artex-tail.db
      storage.type: filesystem
      read_from_head: true

  outputs:
    # Any OTLP/HTTP intake, or an OTel Collector.
    - name: opentelemetry
      match: artex.audit
      host: ${OTLP_HOST}
      port: 4318
      logs_uri: /v1/logs
      tls: on
      header: Authorization ${OTLP_AUTH_HEADER}

    # Older SIEMs: RFC 5424 over TLS.
    - name: syslog
      match: artex.audit
      host: ${SIEM_HOST}
      port: 6514
      mode: tls
      syslog_format: rfc5424
      syslog_appname_preset: artex-cc
      syslog_message_key: event.name
```

`parsers.yaml`:

```yaml
parsers:
  - name: artex_jsonl
    format: json
    time_key: time
    time_format: "%Y-%m-%dT%H:%M:%S.%LZ"
    time_keep: on
```

The `opentelemetry` output sends to any OTLP/HTTP intake or to an [OpenTelemetry Collector](/docs/integrations/opentelemetry-collector/). The `syslog` output covers [older SIEMs](/docs/integrations/syslog-cef-leef/) over TLS; use `tcp` or TLS rather than UDP, which drops audit data silently.

## Microsoft Sentinel

```yaml
service:
  flush: 5
  parsers_file: parsers.yaml

pipeline:
  inputs:
    - name: tail
      path: /artex/audit/target.jsonl,/artex/audit/agent.jsonl
      tag: artex.audit
      parser: artex_jsonl
      db: /var/lib/fluent-bit/artex-tail.db
      read_from_head: true

  outputs:
    - name: azure_logs_ingestion
      match: artex.audit
      tenant_id: ${AZURE_TENANT_ID}
      client_id: ${AZURE_CLIENT_ID}
      client_secret: ${AZURE_CLIENT_SECRET}
      dce_url: ${AZURE_DCE_URL}
      dcr_id: ${AZURE_DCR_ID}
      table_name: ArtexAudit_CL
      time_generated: true
      compress: true
```

The custom table and the data collection rule must exist first; see [Microsoft Sentinel](/docs/integrations/microsoft-sentinel/).

## ARTEX's own telemetry

Fluent Bit's `prometheus_scrape` input can read ARTEX's `/metrics`, and the `opentelemetry` output forwards it. Server logs on stderr (`ARTEX_LOG_FORMAT=json`) can be read with the `systemd` input or from the container runtime's log files.

## Validation

- `fluent-bit -c <file> --dry-run` with Fluent Bit 5.2.1, for both files above with placeholder hosts and credentials: "configuration test is successful". A misspelled output property made the dry run fail, so it does check plugin properties.
- The `tail` input with `artex_jsonl` was run against sample records with a `stdout` output: record times were parsed into the event timestamp with sub-second precision.
- Not run against a real backend.
