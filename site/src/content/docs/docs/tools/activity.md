---
title: Activity
description: "Read and write the engagement activity log."
sidebar:
  order: 8
---

## `list_activity`

List recent activity log entries for this engagement: intents started/completed, facts recorded, findings reported, tool calls. Newest first.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| limit | integer | no | Number of entries to return, default 50, max 500 |
| node_id | integer | no | Filter to activity on a specific exploration node |
| kind | string | no | Filter by activity kind (e.g. `tool_use`, `tool_result`, `intent_start`, `intent_done`, `finding`) |

**Returns:** `{count, activity: [...]}`.

## `log_activity`

Record an activity log entry for this engagement. Use to log significant actions, decisions, or observations.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| action | string | yes | What happened (used as the activity kind) |
| node_id | integer | no | Related exploration node ID |
| detail | string | no | Detailed description of the activity |
| summary | string | no | One-line summary |

**Returns:** `"activity logged: <id>"`.
