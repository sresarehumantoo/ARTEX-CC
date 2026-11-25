---
title: Reporting and export
description: "Generate the Markdown report and move engagements between servers."
sidebar:
  order: 9
---

## `generate_report`

Generate a Markdown penetration test report from the current engagement's findings, assets, and coverage.

_No parameters._

**Returns:** Markdown report as text.

## `export_engagement`

Export the current engagement (or a specific one) as a portable archive. Includes assets, exploration graph, findings, credentials, coverage, and scope.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| task_id | integer | no | Task ID to export (defaults to current engagement) |
| format | string | no | `json` (default) or `csv` |

JSON returns the full export object. CSV returns per-entity CSV tables (assets, findings, nodes).

**Returns:** JSON: full engagement export object. CSV: `{format, exported_at, assets_csv, findings_csv, nodes_csv}`.

## `import_engagement`

Import a previously exported engagement from JSON. Creates a new engagement with all the imported data.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| data | string | yes | The exported engagement JSON string |

**Returns:** `{task_id, exploration_id, message}`.
