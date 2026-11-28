---
title: AWS
description: Send the ARTEX audit stream to Amazon CloudWatch Logs, and what AWS Security Lake would need.
sidebar:
  order: 17
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

## CloudWatch Logs

### Route 1: Vector

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

Use with the [Vector base config](/docs/integrations/vector/). Vector signs requests with SigV4 from the usual AWS credential chain (instance role, environment, profile), which AWS recommends for production.

### Route 2: CloudWatch agent

The agent's `logs.logs_collected.files.collect_list` takes a `file_path`, `log_group_name` and `log_stream_name` per file. Point one entry at each stream file.

### Route 3: OTLP

CloudWatch Logs has an OTLP endpoint (`/v1/logs`) that accepts SigV4 or a bearer API key with `logs:CallWithBearerToken`, with the log group and stream in `x-aws-log-group` and `x-aws-log-stream` headers. A static bearer key works from ARTEX directly:

```sh
ARTEX_AUDIT_OTLP=on
OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=https://logs.<region>.amazonaws.com/v1/logs
OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer <token>,x-aws-log-group=/artex/audit,x-aws-log-stream=target"
```

AWS recommends SigV4 for production, which a Collector handles better than a static header.

CloudWatch rejects events older than 14 days or more than 2 hours in the future.

## Security Lake

Security Lake custom sources take OCSF only (version 1.3 or earlier), as Parquet on S3, partitioned by region, account and day, with one OCSF class per source. ARTEX's records would map to HTTP Activity (4002), Network Activity (4001) for tunnels, and Detection Finding (2004) for alerts, so three custom sources. No generic output feeds this today: it needs an OCSF mapping in a pipeline (Vector's `aws_s3` sink with Parquet encoding and a remap, or a Cribl pack). An OCSF output mode in ARTEX is an open question in the plan.

## ARTEX's own telemetry

Scrape `/metrics` with the CloudWatch agent's Prometheus support or Amazon Managed Service for Prometheus, or send OTLP metrics through a Collector.

## Validation

- Vector sink: `vector validate` 0.62.0 passed with a placeholder region.
- The CloudWatch agent and OTLP settings and the Security Lake description were written from AWS documentation only. Nothing was sent to AWS.
