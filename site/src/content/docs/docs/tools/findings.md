---
title: Findings
description: "Report, list, merge, dismiss and bind findings to captured traffic."
sidebar:
  order: 3
---

## `report_finding`

Record a confirmed finding, with evidence providing verifiable proof such as command output and logs.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| vulnclass | string | yes | Vulnerability class |
| severity | string | yes | `critical`, `high`, `medium`, or `low` |
| summary | string | yes | Finding summary |
| name | string | no | Vulnerability name |
| intent_id | integer | no | The current task's intent ID |
| asset_ids | array of integer | no | Affected asset IDs |
| evidence | string | no | Evidence/PoC text |

**Returns:** `{finding_id, node_id, possible_duplicates?}`. Pass `node_id` as `evidence_id` to `prove_goal`.

## `list_findings`

List the confirmed findings of this task (compact: id + vulnclass + severity + summary + state + status + traffic_count).

_No parameters._

**Returns:** Array of finding summaries.

## `bind_finding_traffic`

Link a traffic exchange (snapshot) to a finding as evidence. Use traffic_search to find the snapshot_id first. Rejects snapshot ids that are not in captured traffic. Only registered when `--proxy` is set.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | yes | Finding ID |
| snapshot_id | string | yes | Traffic snapshot ID from traffic_search/traffic_get |
| role | string | no | `baseline`, `proof`, `verification`, or `supporting` (default: `supporting`) |
| note | string | no | Note about this traffic evidence |

**Returns:** `"traffic <id> bound to finding <id> as <role>"`.

## `merge_findings`

Merge two or more findings into a single primary finding. Evidence and assets from secondary findings are folded into the primary.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_ids | array of integer | yes | All finding IDs to merge (must include primary_id) |
| primary_id | integer | yes | The finding to keep as the merged result |

**Returns:** Merged finding object.

## `dismiss_finding`

Mark a finding as dismissed (false positive). Dismissed findings are excluded from generate_report output.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | yes | The finding to dismiss |
| reason | string | yes | Why this finding is a false positive |

**Returns:** `"finding <id> dismissed: <reason>"`.
