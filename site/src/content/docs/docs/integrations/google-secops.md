---
title: Google SecOps
description: Send the ARTEX audit stream to Google Security Operations (Chronicle) with Vector or a webhook feed.
sidebar:
  order: 16
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

Google SecOps does not document an OTLP intake. Either route below needs a custom log type and a parser (or a UDM mapping) on the SecOps side; see [Field mappings](/docs/integrations/field-mappings/) for the UDM fields.

## Route 1: Vector

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

Use with the [Vector base config](/docs/integrations/vector/). The service account key file grants ingestion access; keep it out of ARTEX's data directory.

## Route 2: Webhook feed from ARTEX's HTTP sink

A SecOps HTTPS webhook feed gives you an endpoint URL and a secret, and accepts newline-separated JSON. ARTEX's planned HTTP sink can post to it with two static headers:

```sh
ARTEX_HTTP_SINK_URL="<feed endpoint URL>"
ARTEX_HTTP_SINK_HEADERS="X-goog-api-key=<API key>,X-Webhook-Access-Key=<feed secret>"
ARTEX_HTTP_SINK_FORMAT=ndjson
ARTEX_HTTP_SINK_EVENTS=all
```

SecOps limits batches to 4 MB and deduplicates on content within a log type; `event.id` makes each record distinct.

## Google Cloud Logging

Cloud Logging is a different product. Use the Ops Agent (`files` receiver with `parse_json`) or an OTel Collector with Google authentication; ARTEX does not post to it directly because it needs Google OAuth.

## Validation

- Vector sink: `vector validate` 0.62.0 passed with placeholder customer ID and log type.
- The webhook feed settings were written from Google documentation only. Nothing was sent to Google.
