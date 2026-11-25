---
title: Engagement management
description: "Create, switch, update and close engagements, and inspect their status."
sidebar:
  order: 7
---

## `create_engagement`

Create a new pentest engagement (task + exploration graph).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| goal | string | yes | High-level goal for the engagement |
| description | string | no | Detailed description of the engagement scope (defaults to goal) |

**Returns:** `{task_id, exploration_id}`.

## `list_engagements`

List all pentest engagements with summary stats (goal, status, node/finding/asset counts).

_No parameters._

**Returns:** Array of engagement summaries, each with `id`, `goal`, `description`, `status`, `exploration_id`, `created_at`, `node_count`, `finding_count`, `asset_count`, `goals_met`, and `active` flag.

## `switch_engagement`

Switch to a different engagement without restarting the server. All tools will be re-registered to operate on the new engagement.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| task_id | integer | yes | Task ID of the engagement to switch to |

**Returns:** `"switched to engagement <id> (exploration <id>): <goal>"`.

## `update_engagement`

Update the current engagement's goal or description.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| goal | string | no | New goal for the engagement |
| description | string | no | New description for the engagement |

At least one of `goal` or `description` is required.

**Returns:** `"engagement <id> updated: <fields>"`.

## `delete_engagement`

Delete an engagement and all its data (nodes, edges, findings, credentials, coverage, assets, constraints). Irreversible.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| task_id | integer | yes | Task ID of the engagement to delete |
| confirm | boolean | yes | Must be `true` to confirm deletion |

**Returns:** `"engagement <id> deleted"`.

## `engagement_status`

Get the status of an engagement: goal, status, finding count, and coverage summary.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| task_id | integer | no | Task ID (defaults to current engagement) |

**Returns:** `{task_id, exploration_id, goal, status, findings_total, facts_total, frontier_open, done_intents, coverage?}`.

## `engagement_activity`

Show recent activity in the current engagement as a timeline of findings, assets, and graph nodes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| limit | integer | no | Maximum number of entries to return (default 20) |

**Returns:** Timeline of `"<timestamp> - <kind>: <summary>"` entries.

## `goal_met`

Mark the entire engagement as complete. Sets the task status to done.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| summary | string | no | Summary of what was accomplished |

**Returns:** `"engagement <id> marked complete"` with optional summary.

## `list_coverage_gaps`

Identify testing gaps: assets never targeted by an intent, intents with no recorded facts, goals not yet proven, and service ports discovered but not enriched.

_No parameters._

**Returns:** JSON object with gap categories and their entries.

## `list_sessions`

List all active MCP client sessions connected to this server.

_No parameters._

**Returns:** `{count, sessions: [{id, created_at, last_active, context}, ...]}`.
