---
title: Field mappings
description: Draft mapping from the artex.audit/v1 record names (OpenTelemetry semantic conventions) to ECS, Splunk CIM, OCSF, CrowdStrike CPS and Google SecOps UDM.
sidebar:
  order: 30
---

:::caution[Draft]
These tables are a draft against the unreleased `artex.audit/v1` schema. They have not been checked field by field against each standard's current version, and will be published as a reference with the audit stream.
:::

ARTEX writes one set of names: OpenTelemetry semantic conventions for standard fields, `gen_ai.*` where it applies (Development status upstream), and `artex.*` for everything else. It does not emit alias fields. Rename in your shipper or parser when a backend expects another schema; the vendor pages show where.

## HTTP egress (`artex.egress.http`)

| ARTEX (OTel) | ECS | Splunk CIM (Web) | OCSF HTTP Activity (4002) | CrowdStrike CPS | Google SecOps UDM |
|---|---|---|---|---|---|
| `time` | `@timestamp` | `_time` | `time` | `@timestamp` | `metadata.event_timestamp` |
| `event.id` | `event.id` | `id` | `metadata.uid` | `event.id` | `metadata.product_log_id` |
| `http.request.method` | `http.request.method` | `http_method` | `http_request.http_method` | `http.request.method` | `network.http.method` |
| `url.full` | `url.full` | `url` | `http_request.url.url_string` | `url.full` | `target.url` |
| `server.address` | `destination.domain` | `dest` | `dst_endpoint.hostname` | `destination.domain` | `target.hostname` |
| `server.port` | `destination.port` | `dest_port` | `dst_endpoint.port` | `destination.port` | `target.port` |
| `client.address` | `source.ip` | `src` | `src_endpoint.ip` | `source.ip` | `principal.ip` |
| `http.response.status_code` | `http.response.status_code` | `status` | `http_response.code` | `http.response.status_code` | `network.http.response_code` |
| `http.request.body.size` | `http.request.body.bytes` | `bytes_out` | `traffic.bytes_out` | `http.request.body.bytes` | `network.sent_bytes` |
| `http.response.body.size` | `http.response.body.bytes` | `bytes_in` | `traffic.bytes_in` | `http.response.body.bytes` | `network.received_bytes` |
| `user_agent.original` | `user_agent.original` | `http_user_agent` | `http_request.user_agent` | `user_agent.original` | `network.http.user_agent` |
| `artex.identity` | `user.name` | `user` | `actor.user.name` | `user.name` | `principal.user.userid` |
| `session.id` | `session.id` (custom) | `session_id` | `actor.session.uid` | `Vendor.session.id` | `network.session_id` |
| `artex.*` | `artex.*` | `artex.*` | `unmapped.artex.*` | `Vendor.artex.*` | `additional.fields` |

## Alerts (`artex.alert`)

| ARTEX (OTel) | ECS | Splunk CIM (Alerts) | OCSF Detection Finding (2004) | CrowdStrike CPS | Google SecOps UDM |
|---|---|---|---|---|---|
| `security_rule.name` | `rule.id` | `signature_id` | `finding_info.analytic.uid` | `rule.id` | `security_result.rule_id` |
| `security_rule.category` | `rule.category` | `category` | `finding_info.types` | `rule.category` | `security_result.category_details` |
| `artex.alert.severity` | `event.severity` (numeric) | `severity` | `severity_id` | `event.severity` | `security_result.severity` |
| `artex.subject_event_id` | `event.reference` (custom) | `artex.subject_event_id` | `evidences` | `Vendor.artex.subject_event_id` | `security_result.about` |
| constant | `event.kind: alert` | tag `alert` | `class_uid: 2004` | `#event.kind: alert` | `metadata.event_type: GENERIC_EVENT` |

ARTEX severities are `info`, `notice`, `warning`, `high`, `critical`. CIM expects `informational`, `low`, `medium`, `high`, `critical`; map `notice` to `low` and `warning` to `medium`.

## Tool calls (`artex.mcp.tool_call`)

| ARTEX (OTel) | ECS | Splunk CIM | OCSF | Google SecOps UDM |
|---|---|---|---|---|
| `gen_ai.tool.name` | `gen_ai.tool.name` | `command` | `api.operation` (API Activity, 6003) | `target.application` |
| `gen_ai.operation.name` | `gen_ai.operation.name` | `action` | `activity_name` | `metadata.product_event_type` |
| `agent_id` | `artex.agent_id` (custom) | `artex.agent_id` | `unmapped.agent_id` | `additional.fields` |

Agent ids are per-run Claude Code ids. Do not map them to OTel `gen_ai.agent.id`, which names a stable hosted agent, or to ECS `agent.*`, which names the shipper.
