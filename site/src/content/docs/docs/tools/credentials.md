---
title: Credentials
description: "Store and list credentials discovered during an engagement."
sidebar:
  order: 4
---

## `save_task_credential`

Record (or update) a test account for this task's target. Same (domain, username) is overwritten in place, not duplicated.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| domain | string | yes | Domain or host the account belongs to |
| username | string | yes | Login name or email used |
| password | string | yes | The exact password that was set |
| note | string | no | Optional context |

**Returns:** `{id, domain, username, source}`.

## `list_task_credentials`

List this task's test accounts: user-declared accounts plus accounts recorded by earlier runs.

_No parameters._

**Returns:** `{credentials: [...]}`.
