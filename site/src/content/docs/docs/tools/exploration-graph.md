---
title: Exploration graph
description: "Plan, track and prove work in the exploration graph: intents, facts, hints, goals and constraints."
sidebar:
  order: 1
---

## `graph_overview`

The task board: goals and their states, open/running/done intents, recent facts, findings, credentials, coverage. Call this first at the start of every planning cycle.

_No parameters._

**Returns:** JSON object with goals, intents (by state), recent facts, findings, credentials, coverage stats, and strategic hints.

## `add_intent`

Generate exploration directions into the frontier. An intent is an open exploration direction described in one summary sentence.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| intents | array of objects | no | Preferred: array of exploration directions to add (batch) |
| summary | string | no | Single-mode: one sentence describing this exploration direction |
| asset_ids | array of integer | no | Target asset IDs this direction tests/attacks |
| parent_ids | array of integer | no | Upstream anchor IDs: confirmed fact/finding nodes this direction is synthesized from |
| priority | integer | no | Priority 0-10, default 5 |

Use either `intents` (batch, preferred) or the top-level `summary` (single). Each `intents` element accepts `summary`, `asset_ids`, `parent_ids`, and `priority`.

**Returns:** Single mode: `"intent created: <id>"`. Batch mode: `{ids: [...], errors: {...}}`.

## `record_fact`

Write an exploration fact or conclusion into the graph, linked to the intent that produced it.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| facts | array of objects | yes | The facts to record (usually one). Each element takes the fields below. |
| facts[].summary | string | yes | One-sentence summary of this conclusion |
| facts[].intent_id | integer | no | The intent that produced this fact |
| facts[].detail | string | no | Detailed description |
| facts[].evidence | string | no | One line of key evidence |
| facts[].confidence | string | no | `observed` (seen directly) or `inferred` |
| facts[].source | string | no | `target_response`, `tool_output`, `third_party_intel`, `operator_input`, or `inference` (default) |
| facts[].asset_ids | array of integer | no | Asset IDs explicitly involved |

**Returns:** Single fact: `"fact created: <id>"`. Multiple facts: `{ids: [...], errors: {...}}`.

## `list_facts`

Page through exploration facts/conclusions, newest first (compact: id + summary + state + origin + source).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| limit | integer | no | Number of entries to return, default 20, max 100 |

**Returns:** `{facts: [{id, summary, state, origin, source}, ...], total}`.

## `node_detail`

Fetch the full content of an exploration graph node by ID. For exploration node IDs from list_facts/list_findings/graph_overview only; use list_assets for assets.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | integer | yes | Exploration graph node ID |

**Returns:** Full node object with kind, state, payload, priority, origin, timestamps, and edges.

## `add_hint`

Attach a strategic hint to the exploration graph; the planner reads it the next time it generates intents.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| hints | array of objects | no | Preferred: array of hints, each with `text` and optional `asset_ids` |
| text | string | no | Single-mode: hint content |
| asset_ids | array of integer | no | Asset IDs to anchor the hint to |

**Returns:** Single mode: `"hint created: <id>"`. Batch mode: `{ids: [...], errors: {...}}`.

## `set_goals`

Add exploration goals to this task. A goal is a final deliverable/verifiable result, not an attack step.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| goals | array of objects | no | Preferred: array of goals, each with `text` and optional `vulnclass` |
| text | string | no | Single-mode: one independently verifiable final goal |
| vulnclass | string | no | Corresponding vulnerability class |

**Returns:** Single mode: `"goal created: <id>"`. Batch mode: `{ids: [...], errors: {...}}`.

## `set_constraints`

Add operation constraints to bound exploration: allow (operations permitted) or deny (operations forbidden).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| constraints | array of objects | no | Preferred: array of constraints, each with `text` and optional `type` |
| text | string | no | Single-mode: the content of one operation constraint |
| type | string | no | `allow` or `deny`; defaults to `deny` |

**Returns:** Single mode: `"constraint created: <id>"`. Batch mode: `{ids: [...], errors: {...}}`.

## `prove_goal`

Link evidence (a finding or fact) to a goal and mark the goal as met. A fact counts as evidence only when its confidence is `observed`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| goal_id | integer | yes | Goal node ID |
| evidence_id | integer | yes | ID of the finding or fact node that proves it |
| reason | string | no | Why this evidence satisfies the goal |

**Returns:** `"goal <id> marked met"` (or `"goal <id> marked met; all goals of this task are met"`).

## `start_intent`

Mark an open intent as running. Call this when you begin working on an intent.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| node_id | integer | yes | Intent node ID |

**Returns:** `"intent <id> is now running"`.

## `complete_intent`

Mark a running intent as done with a result summary. Call this when you have finished working on an intent and recorded all facts/findings.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| node_id | integer | yes | Intent node ID |
| result | string | no | Brief result summary |

**Returns:** `"intent <id> marked done"`.

## `cancel_intent`

Cancel an intent that is no longer needed. Use when an intent is redundant, no longer applicable, or was created in error.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| node_id | integer | yes | Intent node ID |
| reason | string | no | Why this intent is being cancelled |

**Returns:** `"intent <id> cancelled"`.
