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
- **Stale work is visible.** `start_intent` claims an intent for 30 minutes by default (`ARTEX_INTENT_LEASE` changes it). A claim that is not renewed lapses, and the intent shows up as open again for any session to pick up.

## Steering

You do not have to stop the loop to change direction:

- `add_hint` attaches a strategic hint the planner sees on the next `graph_overview`. Ask Claude to add one, or say it directly in the conversation.
- `set_constraints` adds allow or deny rules for operations.
- `add_company_scope` and `add_task_scope` widen or narrow what is in scope; `check_scope` is enforced by the asset, enrich and traffic tools.
- In the [web UI](/docs/web-ui/), intercept rules can require your approval for specific tool calls.

## Parallel work

Claude Code can dispatch subagents. A common pattern is one subagent per open intent, each recording its own facts, coverage and findings into the shared graph. The [walkthrough](/docs/walkthrough/#dispatch-subagents-for-parallel-work) shows this in detail.

You can also point several Claude Code instances (or other MCP clients) at the same engagement. Each one claims an intent with `start_intent` before working it, and a claim held by another session means "pick a different intent". Instead of re-reading the whole graph every iteration, each instance polls `list_changes` for what the others recorded since its last cursor, and sees the claims it still holds with their expiry. A loop prompt for that setup:

```
/loop 5m Continue the pentest alongside other agents: poll list_changes from your last cursor, claim open intents with start_intent and skip any another session holds, renew claims you still need, complete or cancel intents when done, prove goals when evidence exists.
```

Subagents inside one Claude Code process share its MCP session, so claims separate instances, not the subagents of one instance.

Claims belong to the MCP session: each stdio process and each HTTP client on the older session-based protocol gets its own without doing anything. HTTP clients on MCP protocol 2026-07-28 have no session, so `start_intent` asks for an `owner` argument instead. The agent picks a random hex string once and passes it as `owner` on every `start_intent`, `complete_intent`, `cancel_intent` and `list_changes` call. An agent that has a session can pass `owner` too, which makes it a separate holder within that session (useful for giving subagents their own claims), as long as it keeps passing the same value.

Holders are shown as hashed labels (`claimed_by`, the `session` field of `list_changes`, and claim errors), never as raw session IDs or owner strings, so one client cannot copy another's identity to renew or cancel its claims. `complete_intent` and `cancel_intent` are both refused on an intent another holder has a live claim on.

Intents that were already running before claims were introduced have no lease, so any session can start, complete or cancel them right away.

## Stopping

Interrupt the loop in Claude Code. Nothing is lost: facts, findings and credentials are persisted the moment they are recorded. When you are done, `goal_met` marks the engagement complete and `generate_report` writes the report.
