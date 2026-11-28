---
title: Microsoft Sentinel
description: Send the ARTEX audit stream to Microsoft Sentinel through the Logs Ingestion API with Vector or Fluent Bit, or with the Azure Monitor Agent.
sidebar:
  order: 15
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

Sentinel stores data in a Log Analytics workspace. Custom data arrives through the Logs Ingestion API, which needs a data collection rule (DCR) and a target table, and Entra ID authentication. Because Azure wants an OAuth token rather than a static key, ARTEX does not post to it directly; a shipper does.

## Prerequisites

1. A custom table, for example `ArtexAudit_CL`. Column names allow letters, digits and underscores only (at most 45 characters), so ARTEX's dotted keys must be flattened, as the shippers below do (`server.address` becomes `server_address`).
2. A DCR with a stream declaration matching those columns and an optional KQL transform, plus a data collection endpoint if you need one.
3. An Entra app registration with the Monitoring Metrics Publisher role on the DCR.

## Route 1: Vector

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

Use with the [Vector base config](/docs/integrations/vector/).

## Route 2: Fluent Bit

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

Fluent Bit's `azure_logs_ingestion` output posts the record keys as given, so add a flattening step (a Lua filter or the DCR's KQL transform) to turn dotted keys into valid column names.

## Route 3: Azure Monitor Agent

AMA can collect custom JSON log files: one JSON object per line, appended only. Do not let a rotated file keep a name that also matches the collection pattern, or AMA ingests it twice; ARTEX's rotated names (`target.jsonl.<timestamp>`) do not match `*.jsonl`. Non-Azure hosts need Azure Arc.

AMA can also forward syslog and CEF into `Syslog` or `CommonSecurityLog` through a Linux forwarder; see [Syslog, CEF and LEEF](/docs/integrations/syslog-cef-leef/) for producing CEF.

## Limits

1 MB per call and 64 KB per field value (longer values are truncated). The shippers batch below that, and ARTEX bounds field sizes.

## Detections

An analytics rule over ARTEX's alerts (KQL):

```
ArtexAudit_CL
| where event_name == "artex.alert"
| summarize count() by security_rule_name, artex_identity, bin(TimeGenerated, 5m)
```

## ARTEX's own telemetry

Send metrics to Azure Monitor with an OTel Collector (`azure_auth` extension) or to a Prometheus-compatible store of your choice. Server logs can use the same route as audit records with `ARTEX_LOG_FORMAT=json`.

## Validation

- Vector: `vector validate` 0.62.0 passed; the flattening transform was run on sample records and produced underscore column names.
- Fluent Bit: `fluent-bit --dry-run` 5.2.1 passed.
- The table, DCR, AMA setup and KQL were written from Microsoft documentation only. Nothing was sent to Azure.
