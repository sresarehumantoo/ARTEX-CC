---
title: Traffic
description: "Search, inspect, replay and export traffic captured by the recording proxy."
sidebar:
  order: 10
---

These tools require `--proxy` on the `serve` command. Point your testing tools at the proxy to capture requests.

## `traffic_search`

Query captured traffic by host (required), optionally filtered by URL substring or body keyword. Returns an ultra-light index (id/method/url/status/resp_len) with no response content; use traffic_get to inspect individual results.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | yes | Target host: bare host, host:port, or full URL |
| query | string | no | URL/metadata substring filter |
| body_contains | string | no | Full-text body search (min 3 chars) |
| limit | integer | no | Results per page, default 3, max 10 |
| page | integer | no | 0-based page number for pagination |

When a port is specified in `host`, only that service's traffic is returned. `body_contains` supports any substring including CJK characters.

**Returns:** `{count, results: [{id, method, url, status, resp_len}, ...]}`.

## `traffic_get`

Fetch the raw request/response of one captured traffic record by ID (truncated when too large). WebSocket (status 101) or SSE (text/event-stream) records show only their handshake; use traffic_messages for their messages.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | yes | Exchange ID from traffic_search |

When proxy rewrite rules changed the exchange, the result starts with a `REWRITTEN` line naming the rule IDs, and shows the request as the client sent it and the response as the target sent it.

**Returns:** Text with `--- REQUEST ---` and `--- RESPONSE ---` sections.

## `traffic_blob`

Read a segment of an oversized request or response body by its blob hash (shown in traffic_get as `@blob sha256:<hash>`). Returns at most 8 KiB per call; use offset to page through.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| hash | string | yes | SHA-256 blob hash from traffic_get output |
| offset | integer | no | Byte offset to start reading (default 0) |
| limit | integer | no | Bytes to read, max 8192 (default 8192) |

**Returns:** `{total, offset, length, data}`.

## `traffic_messages`

List WebSocket or SSE messages of a captured stream exchange. Use after traffic_get shows status 101 (WebSocket) or text/event-stream (SSE).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | yes | Exchange ID |
| direction | string | no | Filter: `c2s` (client to server), `s2c` (server to client), or empty for both |
| limit | integer | no | Messages per page, default 50, max 200 |
| offset | integer | no | 0-based offset for pagination |

**Returns:** `{total, messages: [{seq, direction, type, size, payload, event?, blob?, truncated?}, ...]}`.

## `traffic_replay`

Replay a captured HTTP request with optional modifications. Fetches the original request, applies overrides, sends it through the recording proxy, and returns the new response.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | yes | Exchange ID to replay |
| modify_headers | object | no | Header overrides: `{name: value}` to set, `{name: ""}` to remove |
| modify_url | string | no | Override the request URL |
| modify_method | string | no | Override the HTTP method |
| modify_body | string | no | Override the request body |
| apply_rules | boolean | no | Apply capture rewrite rules to the replay (default false) |

**Returns:** `{status, duration_ms, recorded, truncated, raw}`.

## `traffic_hosts`

List all recorded target hosts with their exchange counts, most recent activity first.

_No parameters._

**Returns:** `{count, hosts: [...]}`.

## `traffic_har_export`

Export captured traffic as a HAR 1.2 JSON document. Optionally filter by host, method, or status code.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | no | Host substring filter (empty exports all) |
| method | string | no | HTTP method filter |
| status | string | no | Status code or class filter, e.g. `200` or `4xx` |

**Returns:** `{har, exported, totalMatched, truncated}`.

## `traffic_stats`

Aggregate statistics about all captured traffic: total exchanges, unique hosts, status code distribution, method distribution, top 10 hosts, time range, and average response size.

_No parameters._

**Returns:** Aggregate statistics object.

## `traffic_timeline`

Show traffic activity over time for a host, bucketed by minute, hour (default), or day. Useful for spotting rate limiting, session expiry, and WAF patterns.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | yes | Target host |
| bucket | string | no | `minute`, `hour` (default), or `day` |

**Returns:** `{host, buckets: [...]}`.

## `traffic_patterns`

Identify traffic patterns: most-requested paths (by URL template), hosts with the most HTTP errors (4xx/5xx with a status breakdown), and endpoints that appeared recently.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | no | Host substring filter (empty analyzes all traffic) |

**Returns:** `{top_paths: [{path, count}], error_hosts: [{host, total, codes_4xx, codes_5xx, status_samples}], new_endpoints: [{host, path, first_ts, count}]}`.

## `traffic_clear`

Delete all captured traffic for hosts matching a substring. This cannot be undone.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| host | string | yes | Host substring to match for deletion |

**Returns:** `{deleted, host}`.
