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
- **Stale work is visible.** `start_intent` claims an intent for 30 minutes by default (`ARTEX_INTENT_LEASE` changes it). The holder renews it by recording results against the intent (`intent_id` on `record_fact`, `report_finding` or `record_coverage`) or by calling `start_intent` again. A claim that is not renewed lapses, and the intent shows up as open again for any session to pick up.

## Steering

You do not have to stop the loop to change direction:

- `add_hint` attaches a strategic hint the planner sees on the next `graph_overview`. Ask Claude to add one, or say it directly in the conversation.
- `set_constraints` adds allow or deny rules for operations.
- `add_task_scope` sets what is in scope. Targets are matched against its domain, IP and CIDR entries and, for each `company` entry, the domain, IP and CIDR lines `add_company_scope` recorded for that company. An engagement with none of those allows every target, so the first one restricts it to what is listed and each later one widens it. A `company` entry whose company has no domain, IP or CIDR lines allows nothing on its own rather than everything. `insert_assets`, the enrich tools, `traffic_replay` and `capture_command_output` refuse targets outside that scope, and `check_scope` tests a target against it. The recording proxy records traffic to any host. `add_company_scope` on its own does not change what is in scope.
- In the [web UI](/docs/web-ui/), intercept rules can require your approval for specific tool calls.

## Parallel work

Claude Code can dispatch subagents. A common pattern is one subagent per open intent, each recording its own facts, coverage and findings into the shared graph. The [walkthrough](/docs/walkthrough/#dispatch-subagents-for-parallel-work) shows this in detail.

You can also point several Claude Code instances (or other MCP clients) at the same engagement. Each one claims an intent with `start_intent` before working it, and a claim held by another session means "pick a different intent". Instead of re-reading the whole graph every iteration, each instance polls `list_changes` for what the others recorded since its last cursor, and sees the claims it still holds with their expiry. A loop prompt for that setup:

```
/loop 5m Continue the pentest alongside other agents: poll list_changes from your last cursor, claim open intents with start_intent and skip any another session holds, renew claims you still need, complete or cancel intents when done, check list_findings before reporting and merge duplicates, prove goals when evidence exists. If every open intent is claimed, finish your own work instead of sleeping.
```

Subagents inside one Claude Code process share its MCP session, so claims separate instances, not the subagents of one instance.

A few rules keep several instances from stepping on each other:

- **Goals are set once.** When instances start together, each may see an empty board and call `set_goals`. The server serializes those calls per engagement: the first one creates its goals, and later ones add nothing and get `existing_goals` back with a note to adopt them. A goal whose text matches an existing goal, ignoring case and whitespace, is always skipped. To add goals to an engagement that already has some, pass `append: true`.
- **Intents are not planned twice.** Instances that start together also tend to plan the same intents. The server serializes `add_intent` per engagement and skips an intent whose summary (ignoring case, punctuation and spacing) and `asset_ids` match an open or running intent, returning the existing id under `skipped` instead. Reworded duplicates still get through, so `add_intent` also lists up to ten `open_intents` that nobody holds; an instance that finds its new intent already covered there works the existing one and cancels its own.
- **Check before reporting.** Before `report_finding`, an instance checks `list_findings` (or the findings in `list_changes`) for the same vulnclass on the same asset and adds to that finding instead of filing a new one. `report_finding` returns `possible_duplicates` for open findings with the same vulnclass, or the same severity on an overlapping asset. Vulnclass matching ignores case and punctuation and treats common long forms like "Insecure Direct Object Reference" as `idor`. The instance merges any that are the same issue with `merge_findings`. Reporting is never blocked; distinct issues stay separate.
- **Don't wait on claims.** When every open intent is claimed by someone else, an instance polls `list_changes` once more and takes any new or reopened intent. If there is none, it adds intents for untested directions or finishes its own work (complete its intents, prove goals that have evidence) and stops, rather than sleeping until a claim frees up.
- **Journal changes to the target.** Before modifying, creating or deleting anything on a target host, an instance calls `record_target_change` with the resource's original fingerprint (for a file, `sha256:<hex>`; `absent` for something it creates). If another instance already has an unrestored change on the same target and resource (the target ignoring case, the resource ignoring case only for Windows paths and registry keys, so `/srv/App.conf` and `/srv/app.conf` stay separate), the result says so and returns the baseline: the original fingerprint recorded first, which is the state to restore, rather than whatever the second instance backed up. After undoing its change, the instance calls `record_target_restore` with the fingerprint it observes. The server marks the change restored only when that matches the baseline exactly, which also restores every other live change on that resource; a mismatch is recorded as a failed attempt. Unrestored changes appear in `graph_overview` and `list_changes` as `unrestored_target_changes`, and in the Target changes section of `generate_report`, so no instance should report a host as restored while any are listed.

An HTTP client that sends an `X-Artex-Identity` header, a token signed with the data directory's `identity.key`, holds claims as that identity on either protocol version and keeps them across reconnects and server restarts, so it needs no `owner`. Give each concurrently running agent its own identity name: two clients with the same identity are one holder. A header that does not verify is rejected with HTTP 400 rather than ignored.

Web UI chat conversations need nothing: each time a conversation's `claude` process starts or resumes, the server mints it an identity named `chat/<conversation id>` from the same `identity.key`, so its claims show that name and survive a server restart when the conversation is resumed.

To make identities mandatory for claims on an HTTP server, run `serve --require-identity` (or set `ARTEX_REQUIRE_IDENTITY=1`). Claim calls from a caller without a verified identity are then refused with an error pointing at `artex-cc identity issue`, whatever session or `owner` it has. See [Requiring an identity](/docs/mcp-clients/#requiring-an-identity).

Without an identity, claims belong to the MCP session: each stdio process and each HTTP client on the older session-based protocol gets its own without doing anything. Other HTTP clients on MCP protocol 2026-07-28 have no session, so `start_intent` asks for an `owner` argument instead. The agent picks a random hex string once and passes it as `owner` on every `start_intent`, `complete_intent`, `cancel_intent` and `list_changes` call. An agent that has a session can pass `owner` too, which makes it a separate holder within that session (useful for giving subagents their own claims), as long as it keeps passing the same value. Recording results with `owner` renews only that holder's claims. Recording results without it renews the claims of the session or identity and of any `owner` within it, so a client with no session or identity has to pass its `owner` to `record_fact`, `report_finding` and `record_coverage` for them to renew its claims.

Holders are shown as labels (`claimed_by`, the `session` field of `list_changes`, and claim errors), never as raw session IDs, owner strings or identity tokens, so one client cannot copy another's holder to renew or cancel its claims. An identity's label is its name, with a hashed suffix when `owner` subdivides it; every other holder gets a hashed label. `list_sessions` lists the identities active on the server alongside the sessions. `complete_intent` and `cancel_intent` are both refused on an intent another holder has a live claim on. On an open intent that nobody has started, they take the claim and close the intent in one step, and the intent records the closer's label as `completed_by` or `cancelled_by`. Completing an intent that was never started also reminds the agent to call `start_intent` first next time, since work done without a claim is invisible to the other instances.

Intents that were already running before claims were introduced have no lease, so any session can start, complete or cancel them right away.

## Stopping

Interrupt the loop in Claude Code. Nothing is lost: facts, findings and credentials are persisted the moment they are recorded. When you are done, `goal_met` marks the engagement complete and `generate_report` writes the report.
