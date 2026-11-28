---
title: CrowdStrike
description: Send the ARTEX audit stream to CrowdStrike Falcon Next-Gen SIEM or Falcon LogScale, and what Falcon Complete has to do with it.
sidebar:
  order: 12
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

## Which CrowdStrike product takes the data

| Product | What it is | Takes ARTEX records? |
|---|---|---|
| Falcon Next-Gen SIEM | CrowdStrike's SIEM, built on the LogScale engine. Third-party data arrives through data connectors and is parsed to the CrowdStrike Parsing Standard (CPS) | Yes, this is the target |
| Falcon LogScale | The same engine sold on its own, cloud or self-hosted | Yes, for standalone LogScale customers |
| Falcon LogScale Collector | CrowdStrike's shipper: tails files, sends to NG-SIEM or LogScale | The shipper for the JSONL files |
| Falcon Complete | CrowdStrike's managed detection and response service, run by CrowdStrike analysts | No ingest of its own. It works on top of your Falcon data, including NG-SIEM |
| Falcon Data Replicator | Exports Falcon telemetry out to your own storage | No, it points the other way |

Falcon Complete is a service, not a log store, so "send ARTEX to Falcon Complete" means "send it to Next-Gen SIEM". Whether Falcon Complete analysts triage detections from your own correlation rules over a custom source is not stated in public documentation; treat it as a question for your CrowdStrike account team, and assume your own SOC owns the rule until they confirm otherwise.

## Route 1: LogScale Collector to an NG-SIEM HEC connector (recommended)

1. In the Falcon console, open Next-Gen SIEM, Data onboarding (Data connectors), and add an "HEC / HTTP Event Connector". Pick or create a parser (below). The connector's detail page shows its API key and URL after creation. Create one connector per stream, so each gets its own parser, key and retention.
2. Configure the LogScale Collector to tail the files:

```yaml
dataDirectory: /var/lib/logscale-collector
sources:
  artex_target:
    type: file
    include:
      - /opt/artex/data/audit/target.jsonl
    sink: ngsiem_target
  artex_agent:
    type: file
    include:
      - /opt/artex/data/audit/agent.jsonl
    sink: ngsiem_agent
sinks:
  ngsiem_target:
    type: hec
    proxy: none
    token: <target connector API key>
    url: <target connector URL from the console>
    maxEventSize: 910000
    maxBatchSize: 12000000
    workers: 4
  ngsiem_agent:
    type: hec
    proxy: none
    token: <agent connector API key>
    url: <agent connector URL from the console>
```

The collector advances its checkpoint only after the destination confirms receipt, so delivery is at least once; deduplicate on `event.id`. `proxy: none` keeps the collector's own traffic out of the ARTEX recording proxy. Use the URL exactly as the console shows it; third-party guides disagree on whether a `/services/collector` path belongs on it.

## Parser (CPS)

CPS is based on ECS. An ARTEX parser needs to parse the JSON, set the required tags, and move ARTEX-specific fields under `Vendor.`:

```
parseJson()
| #Vendor := "artex"
| #event.module := "artex"
| #event.dataset := format("artex.%s", field=["artex.stream"])
| #event.kind := if(event.name == "artex.alert", then="alert", else="event")
| #ecs.version := "9.0.0"
| #Cps.version := "1.2.0"
| Parser.version := "0.1.0"
| destination.domain := rename("server.address")
| destination.port := rename("server.port")
| Vendor.artex.task_id := rename("artex.task_id")
| Vendor.artex.identity := rename("artex.identity")
```

Fill in `event.category`, `event.type` and `#observer.type` per the CPS guidelines, and use the parser editor's CPS validation with sample records. See [Field mappings](/docs/integrations/field-mappings/) for the rest of the fields.

## Detections

Correlation rules are scheduled CQL queries, so latency equals the schedule. ARTEX already computes its detections, so one rule can promote them:

```
#event.module="artex" #event.kind="alert"
| groupBy([security_rule.name, Vendor.artex.identity])
```

Map rule ids to ATT&CK where useful, for example `dest.exfil` to T1567 (Exfiltration Over Web Service). The value of sending ARTEX here rather than to a plain log store is joining it with Falcon sensor data from the host that runs the agents; keep the host name consistent between the two.

CrowdStrike also offers its own controls for AI coding agents that hook Claude Code directly. They see prompts and tool activity but not engagement scope, intents or the proxy's view of what left the box, so the two are complementary. ARTEX's web UI runner uses its own settings file and does not replace hooks you configure.

## Route 2: OTLP to standalone LogScale

LogScale accepts OTLP at `/api/v1/ingest/otlp`, over HTTP with binary protobuf only and a bearer ingest token. CrowdStrike does not document an OTLP connector for NG-SIEM, so use Route 1 there.

```sh
ARTEX_AUDIT_OTLP=on
OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=https://<logscale host>/api/v1/ingest/otlp/v1/logs
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer <ingest token>"
```

## ARTEX's own telemetry

NG-SIEM is not a metrics backend. Send ARTEX's server logs the same way as audit records (JSON stderr tailed by the collector, or OTLP to standalone LogScale), and keep metrics in an observability tool.

## Validation

None of the CrowdStrike configuration here was run: the LogScale Collector config, the CQL parser and the correlation rule were written from CrowdStrike and LogScale documentation only, and no CrowdStrike service was contacted.
