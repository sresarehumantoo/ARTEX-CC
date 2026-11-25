---
title: Hands-free mode
description: Let Claude Code run the engagement on a loop with /loop, and steer it with hints.
---

Claude Code's `/loop` command re-runs a prompt on an interval. Pointed at ARTEX, it turns into an autonomous engagement driver:

```
/loop 5m Continue the pentest: call graph_overview, assess progress, dispatch subagents for open intents, prove goals when evidence exists.
```

Every five minutes Claude checks the graph, dispatches work for open intents, records results, and proves goals when the evidence is there. You can watch it run, steer it, or walk away and review the findings later.

## Why it works

Each iteration starts from `graph_overview`, not from conversation memory. The exploration graph holds the goals, the open, running and done intents, the facts, findings, credentials and coverage. That means:

- **No repeated work.** Done intents and coverage records tell Claude what has already been tested.
- **Context loss is harmless.** If the conversation is compacted or restarted, the next iteration rebuilds its picture from the graph.
- **Stale work is visible.** Intents left in `running` with no recent progress belong to an earlier session; Claude replaces them.

## Steering

You do not have to stop the loop to change direction:

- `add_hint` attaches a strategic hint the planner sees on the next `graph_overview`. Ask Claude to add one, or say it directly in the conversation.
- `set_constraints` adds allow or deny rules for operations.
- `add_company_scope` and `add_task_scope` widen or narrow what is in scope; `check_scope` is enforced by the asset, enrich and traffic tools.
- In the [web UI](/docs/web-ui/), intercept rules can require your approval for specific tool calls.

## Parallel work

Claude Code can dispatch subagents. A common pattern is one subagent per open intent, each recording its own facts, coverage and findings into the shared graph. The [walkthrough](/docs/walkthrough/#dispatch-subagents-for-parallel-work) shows this in detail.

## Stopping

Interrupt the loop in Claude Code. Nothing is lost: facts, findings and credentials are persisted the moment they are recorded. When you are done, `goal_met` marks the engagement complete and `generate_report` writes the report.
