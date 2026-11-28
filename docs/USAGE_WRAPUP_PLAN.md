# Usage wrap-up plan

Status: proposal for maintainer review. No code yet. Citations are against `main` at `4fe8063b`. Claude Code and Agent SDK documentation was checked on 2026-11-28. Tracking issue: #356.

## Summary

When the Claude account behind an engagement is close to its usage limit, tell every agent working the engagement to stop starting work, save what it has, hand back its intents with a note, and stop. Today an agent that hits the limit simply dies mid-task: its claims block other agents until the lease lapses, then reopen with no record of what was done.

The plan adds a per-engagement wrap-up state in PostgreSQL, fed by usage signals (the web UI runner's stream, an optional bridge for interactive sessions, a manual button or tool, and a cost budget for API-key setups), delivered on every ARTEX tool result, and a `release_intent` tool so an agent can hand an intent back instead of lying about completing it or throwing it away. It is on by default with an opt-out, and does nothing without a signal source.

## Current state (verified)

- No wrap-up exists that is tied to usage. The orchestration guide tells agents to complete their intents and stop when nothing is left (`mcpserver/server.go`). The web UI runner's `Stop` sends SIGINT and kills after a grace period (`api/chat_runner.go`); neither path asks the model to save state first.
- Claims: `start_intent` claims for `ARTEX_INTENT_LEASE` (30m default). Writes that pass `intent_id` renew a claim with less than half a lease left. Claims end only through `complete_intent` (`done`) or `cancel_intent` (`stopped`). No tool returns an intent to `open`, although `Store.UpdateNodeState` can.
- When an agent dies, nothing releases its claims. The intent stays `running` until the lease lapses, then `graph_overview` lists it as open with `claim_expired: true` and the next `start_intent` takes it over. Lapses do not show up in `list_changes`.
- The runner starts `claude -p --output-format stream-json` and parses `system`, `assistant`, `user` and `result` lines (`api/chat_runner.go`). It drops `rate_limit_event` lines and does not parse the fields that mark a usage-limit stop (`terminal_reason`, `api_error`, `api_error_status`).
- ARTEX's own eval transcripts show what the stream carries for a subscription session: `rate_limit_event` lines with `status` (`allowed`, `allowed_warning`, `rejected`) and live utilization of the five-hour and seven-day windows, followed at the limit by a `result` with `terminal_reason: "api_error"` and `api_error: "usage_limit_reached"`. Claude Code itself warns at 0.9 five-hour utilization.
- Every MCP tool call goes through one wrapper (`mcpserver/handler.go`), and subagents in one Claude Code process share its MCP session, so a notice appended there reaches the coordinator and every subagent on their next ARTEX call with no client configuration.

## 1. Goals and non-goals

Goals:

- G1. Agents stop starting work before the limit, while they can still afford a few turns.
- G2. Held intents go back to `open` with a resume note, instead of waiting out a lease.
- G3. A resuming agent can see why the last run stopped and where to pick up.
- G4. Works across processes (stdio, several sessions, the web UI) because the state is in PostgreSQL.

Non-goals:

- Preempting an agent in the middle of a long model turn or command. It sees the notice at its next ARTEX call.
- Inventing a plan-limit signal where the provider gives none (section 2).
- Tracking usage per engagement. Limits belong to the Claude account; two engagements on one account only see each other's signal if a source writes to both.

## 2. Signals

| Signal | Where | Coverage | Early warning? |
|---|---|---|---|
| `rate_limit_event` in stream-json | Web UI runner, eval runner, Agent SDK | Subscription (Pro, Max) | Yes: status and utilization per window, with reset time. Only `status`, `resetsAt`, `utilization` are documented; window details are observed but undocumented, so parse defensively |
| Statusline `rate_limits` | Interactive sessions | Subscription, and gateway spend limits | Yes, refreshed on assistant messages or on an interval |
| `result` error fields | stream-json | All | No, after the fact |
| Hooks | Claude Code settings | All | No. No hook input carries rate-limit data; `StopFailure` fires after the turn has died |
| `total_cost_usd` on results, OTel cost metrics | stream-json, Claude Code telemetry | All | Only against a budget ARTEX is given |

API-key, Bedrock, Vertex and Foundry sessions have per-minute limits and a monthly spend cap with no warning signal, so for them the only proactive option is an ARTEX-side cost budget.

## 3. What "wrap up" means

The notice gives agents this checklist and asks them to spend as few tokens on it as possible:

1. Stop starting work: no new `start_intent`, no new subagents, no long scans.
2. Record results: `record_fact`, `report_finding` with evidence in hand, `record_coverage`, always with `intent_id`.
3. Target hygiene: restore and `record_target_restore` recorded changes if there is budget; otherwise leave them listed and say so in the note. Never claim a cleanup that did not happen.
4. For each held intent: `complete_intent` if done, `cancel_intent` if pointless, otherwise `release_intent` with a resume note (what was done, what is next, relevant fact and finding ids, any listener or background job left running).
5. `log_activity` with action `wrapup` and a one-line summary.
6. Stop. Do not sleep, poll or retry.

### New tool: `release_intent`

- Input: `node_id`, `note`, optional `owner`. Refused unless the caller holds the claim.
- Effect: state back to `open`, owner and claim expiry cleared, `resume_note`, `released_by` and `released_at` merged into the payload (with a short history if released more than once), `updated_at` bumped so the release shows in `list_changes`.
- Useful outside usage limits too: context exhaustion, a user stop, server shutdown.

## 4. Design

### State

One row per engagement: level (`none`, `soft`, `hard`), reason, source, window (`five_hour`, `seven_day`, `spend`, `budget`, `manual`), utilization, reset time, set and cleared times. Cached in the handler for a few seconds, refreshed on change notifications.

### Sources

1. Web UI runner (automatic): parse `rate_limit_event` and the usage-limit `result` fields, emit a usage event for a meter in the UI, set the engagement state when thresholds are crossed, and mark a conversation that hit the limit as `limited` with its reset time instead of plain idle.
2. Interactive bridge (opt-in): `artex-cc wrapup signal --utilization 0.93 --window five_hour --resets-at <epoch>`, plus a sample statusline snippet that calls it when the numbers change.
3. Manual: a "Wrap up now" button in the web UI and `request_wrapup` / `clear_wrapup` tools.
4. Budget fallback: a configured USD budget compared with the runner's summed `total_cost_usd`.

### Delivery

- While the level is `soft` or `hard`, every ARTEX tool result gets a short second block (under about 300 characters) with the window, utilization, reset time and the checklist pointer. It repeats on every call, since subagents share the session.
- At `hard`, `start_intent` and `add_intent` are refused with the same message, so no fresh claim becomes a stranded claim.
- `graph_overview` gains a `wrapup` block, and a `last_wrapup` after the state clears, so a resuming agent reads resume notes first. `list_changes` reports state changes.
- The web UI runner also queues one user message with the checklist per running conversation at `soft`, and stops conversations that stay silent for a grace period after `hard`. Server shutdown reuses the same path.

### Thresholds (defaults, to be measured in evals)

| Level | Trigger | Effect |
|---|---|---|
| `soft` | five-hour utilization at or above 0.85, seven-day at or above 0.95, or 85% of budget | Notice on every result, runner message |
| `hard` | utilization at or above 0.95, status `rejected`, or budget reached | Also refuse new claims and intents; runner stops silent conversations after the grace period |
| clear | A later event below `soft`, or the reset time has passed | Level cleared, `last_wrapup` kept |

If the reset is only minutes away, pause (refuse new claims) rather than wrap up, since the window resets before a lease would lapse.

### Configuration (proposed)

| Setting | Default | Purpose |
|---|---|---|
| `ARTEX_WRAPUP` | `on` | `off` disables automatic triggers; manual `request_wrapup` still works |
| `ARTEX_WRAPUP_SOFT`, `ARTEX_WRAPUP_HARD` | `0.85`, `0.95` | Five-hour thresholds |
| `ARTEX_WRAPUP_WEEKLY_SOFT` | `0.95` | Seven-day threshold |
| `ARTEX_WRAPUP_GRACE` | `3m` | Runner: time after `hard` before stopping silent conversations |
| `ARTEX_WRAPUP_BUDGET_USD` | unset | Budget fallback |
| `ARTEX_WRAPUP_RESUME` | `off` | Runner: resume `limited` conversations at the reset time |

## 5. Phases

| PR | Scope |
|----|-------|
| 1 | `release_intent` and its `list_changes` visibility (#405) |
| 2 | Runner parsing of `rate_limit_event` and usage-limit results, usage event, UI meter (#406) |
| 3 | Wrap-up state, handler notice, `graph_overview.wrapup`, `request_wrapup` / `clear_wrapup`, `hard` claim refusal (#407) |
| 4 | Runner thresholds, queued message, grace stop, optional auto-resume (#408) |
| 5 | `artex-cc wrapup signal`, statusline sample, docs (#409) |
| 6 | Budget fallback (#410) |

## 6. Limits

- No plan-limit signal for API-key or cloud-provider sessions; a budget is the only option.
- The undocumented window fields may change; the documented `utilization` is optional.
- The wrap-up costs usage itself. If several subagents cross 0.95 together they may all hit the limit before finishing. Lease expiry stays the safety net; `release_intent` makes the good path faster and carries the notes.

## Decisions

Recorded 2026-11-28 by the maintainer.

- Wrap-up is on by default with an opt-out (`ARTEX_WRAPUP=off`), and does nothing without a signal source.
