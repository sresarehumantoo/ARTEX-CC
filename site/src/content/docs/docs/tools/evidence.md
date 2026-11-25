---
title: Evidence
description: "Save, list and retrieve proof-of-concept evidence, or capture command output as evidence."
sidebar:
  order: 5
---

## `save_evidence`

Save a piece of evidence (screenshot, PoC output, config extract, etc.) linked to a finding. Content is base64-encoded.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | yes | Finding to attach this evidence to |
| kind | string | yes | `screenshot`, `poc_output`, `request_response`, `config_file`, `log_extract`, or `other` |
| filename | string | yes | Original filename for the artifact |
| content_base64 | string | yes | Base64-encoded file content (50 MB limit) |

**Returns:** `{id, sha256, size, path}`.

## `list_evidence`

List evidence artifacts for the current engagement, optionally filtered by finding.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | no | Filter by finding ID |

**Returns:** `{count, evidence: [{id, finding_id, kind, filename, size, sha256, created_at}, ...]}`.

## `get_evidence`

Read evidence content by ID. Returns base64-encoded data.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | yes | Evidence ID |

**Returns:** `{id, filename, kind, content_base64, sha256}`.

## `capture_command_output`

Run a command and save its combined stdout+stderr as evidence (kind=poc_output). The command must only target in-scope hosts. 30-second timeout.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| finding_id | integer | yes | Finding to attach this evidence to |
| command | string | yes | Shell command to execute |
| description | string | no | What this command demonstrates |

**Returns:** `{evidence_id, exit_code, output_preview}`.
