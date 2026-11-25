# Multi-Agent Coordination Eval Plan

## 1. What is being tested

Since v0.5.0, several Claude Code instances (or other MCP clients) can work one engagement at the same time:

- `start_intent` claims an intent for 30 minutes by default (`tools.IntentLease`, overridden by `ARTEX_INTENT_LEASE`). A claim held by another holder means "pick a different intent"; calling `start_intent` again renews it.
- `complete_intent` and `cancel_intent` are refused on an intent another holder has a live claim on.
- Claims belong to the MCP session. HTTP clients can carry a signed `X-Artex-Identity` instead (see `docs/CLIENT_IDENTITY_PLAN.md`); the identity then holds the claim regardless of session or protocol era, and its label is the identity name. Anonymous HTTP clients on MCP protocol 2026-07-28 have no session and pass an `owner` argument; their holders appear as hashed labels.
- `list_changes` returns what other instances recorded since a cursor, plus the caller's live claims.
- Changes from other `serve` processes on the same database arrive over `pg_notify` and reach the web UI.

The mechanics already have unit and integration coverage: claim conflict and renewal, expiry takeover, refused completion by another holder, the owner argument, cursor paging, cross-process notification and reconnects (`tools/intent_tools_test.go`, `tools/changes_tools_test.go`, `store/changes_test.go`, `store/notify_test.go`, `mcpserver/claim_http_test.go`).

What has no coverage is the part that decides whether the feature works in practice: **do real agents, running side by side, use claims and the change feed well?** That needs LLM-in-the-loop runs. This plan covers those runs, first on the local VulnApp target and then on the Winforge lab VM.

## 2. Questions the eval must answer

1. **No duplicate work.** Do two instances ever work the same intent at the same time, re-run the same recon, or report the same finding twice?
2. **Conflict handling.** When `start_intent` says another holder has the intent, does the agent move to a different one instead of retrying, waiting, or working it anyway?
3. **Shared awareness.** Does each instance poll `list_changes` and build on facts another instance recorded, instead of rediscovering them?
4. **Recovery.** If an instance dies mid-intent, does another take the intent over after the lease lapses, without losing the facts already recorded?
5. **Worth it.** Compared with one instance, does parallel work finish faster or cover more, and at what extra token cost?
6. **Transports.** Do claims behave the same with stdio (one session per process), with HTTP identities, and with anonymous HTTP on protocol 2026-07-28 (owner argument)?

## 3. Prerequisites

Three small changes make the runs measurable. All three have landed on main.

### 3.1 Configurable lease

A takeover test that waits 30 minutes per attempt is impractical, so `serve` reads an override from `ARTEX_INTENT_LEASE` (for example `ARTEX_INTENT_LEASE=2m`, parsed with `time.ParseDuration`, whole minutes of at least 1m, defaulting to 30m) and `start_intent`'s description states the effective lease. Tests cover the default and an invalid value.

### 3.2 Eval runner: several instances on one engagement

`eval/cmd/eval-runner` used to run one `claude --print` process. It now has:

- `--instances N` (default 1): start N `claude` processes concurrently against the same engagement, each with the same goal and a short instance-specific suffix ("you are instance 2 of 3; other agents are working this engagement").
- `--transport stdio|http`: stdio gives each instance its own `artex-cc serve --stdio` process (its own MCP session, shared PostgreSQL). HTTP points every instance at one `artex-cc serve --listen`.
- `--identity` (default true, HTTP only): each HTTP instance gets its own `X-Artex-Identity` named `eval/<run>/i<n>`, minted by the runner with the run's data directory key and a TTL of the scenario timeout plus the lease, in its own 0600 MCP config that is deleted with the data directory. The prompt then says nothing about `owner`, and holder labels in transcripts and reports name the instance. `--identity=false` keeps the anonymous path: one shared config, and each agent is told to pick an owner label once and pass it on claim calls.
- `--kill-after INSTANCE:DURATION` for the recovery test: kill one instance after a delay, with its claims still live.
- `--lease DURATION`: passed to every `serve` process as `ARTEX_INTENT_LEASE`.
- `--output-format stream-json` for every instance, saved as `transcripts-<scenario>-<timestamp>/instance-N.jsonl` under `--output`, so every tool call, argument and result is on record. Claim history is not persisted in the database (an intent only stores its current holder and `claim_expires_at`), so transcripts are the source of truth for coordination metrics.

### 3.3 Coordination scorecard

The runner scores the transcripts and the final engagement after every run, adding a coordination section to the scorecard and writing `coordination.txt` next to the transcripts. The metrics:

| Metric | Source | Pass |
|---|---|---|
| Concurrent double claims: the same intent successfully started by two holders while the first claim was live | transcripts (`start_intent` results with timestamps) | 0 |
| Conflict responses handled: share of "claimed by another holder" results followed by `start_intent` on a different intent within the next 5 tool calls | transcripts | 90% or more |
| Refused completions: `complete_intent` or `cancel_intent` refused because of another holder's claim | transcripts | report; should be rare |
| Duplicate findings: same vulnclass on the same asset reported by more than one instance | final engagement | 0 after merges; report raw count |
| Redundant recon: identical commands (normalized) run by more than one instance, e.g. the same nmap invocation | transcripts (Bash tool calls) | report; lower than baseline per instance |
| Shared awareness: `list_changes` calls per instance per loop, and facts or findings whose parents were recorded by another instance | transcripts and graph edges | each instance polls; cross-instance parents present |
| Takeover: intent held by the killed instance started by a survivor after its lease lapsed, with the original facts still attached | transcripts and graph | taken over once, nothing lost |
| Outcome: flags or expected findings, wall clock, total tokens and cost per run | scorecard, `claude` usage | parity with baseline or better |

## 4. Phase 1: local VulnApp (cheap, repeatable)

VulnApp (`eval/vulnapp`) has five independent problem areas: SQL injection (`/api/search`, `/login`), directory traversal (`/files`), SSRF (`/fetch`), IDOR (`/api/users/:id`), and an unauthenticated admin panel (`/admin`), plus known `admin` credentials. Independent problems give parallel instances real work to split, unlike a single attack chain.

| Run | Instances | Transport | Purpose |
|---|---|---|---|
| A1 | 1 | stdio | Baseline: findings, wall clock, tokens |
| A2 | 3 | stdio | Coordination metrics, speedup over A1 |
| A3 | 3 | http, `--identity=false` (owner argument) | Same as A2 over the sessionless transport with agent-chosen owners |
| A4 | 3 | stdio, lease 2m, kill instance 2 after its first claim | Recovery and takeover |
| A5 | 3 | http (identities, the default) | Same as A2 with runner-issued identities; holder labels must match instance numbers (`holder_mismatches` 0) |

Run each of A1, A2, A3 and A5 three times; agent behavior varies run to run, so report the median and the worst coordination result, not the best. Same model for every run (`--model`), pinned in the scenario.

Exit criteria for phase 1: zero concurrent double claims across all runs, conflict handling at 90% or more, a successful takeover in A4, and no drop in findings against A1. If agents mishandle conflicts, fix the tool description or the orchestration guide in `artex://engagement` before moving to Winforge; that is the cheap place to iterate.

## 5. Phase 2: Winforge (realistic)

Winforge is the SSH-only Windows Server lab VM from the [Winforge CTF eval](https://artex-cc.com/evals/winforge-ctf/): the agent logs in as a non-admin user, and a vulnerable service listens on localhost only. Its existing scenario is one sequential chain (SQL injection, token theft, admin flag), which tests claim contention but leaves nothing to parallelize. Add a parallel variant:

- **Keep the existing chain** as one track.
- **Add two independent tracks**, each with its own flag, for example a writable scheduled-task script or service binary path that yields a SYSTEM-readable flag, and a credential left in a readable file or registry key that unlocks a second local account. Each track must be solvable without the others so instances can split them.
- **Seed one shared dependency**, such as the port-8080 discovery, so instances have a fact worth reading from `list_changes` instead of rediscovering.
- Reset between runs from a VM snapshot so every run starts identical, and keep the cleanup script from the original eval.

| Run | Instances | Transport | Purpose |
|---|---|---|---|
| W1 | 1 | http (Docker ARTEX, as in the original eval) | Baseline on the three-track scenario |
| W2 | 3 | http, identities | Coordination across tracks |
| W3 | 3 | stdio (native `artex-cc`, Homebrew or binary) | Session-based claims on the same target |
| W4 | 3 | http, lease 2m, kill one instance mid-track | Recovery against a real target |

Phase 2 also checks the evidence side: commands run over SSH from the host, not inside the ARTEX container, so note whether findings carry captured output or pasted text, as the original eval did.

## 6. Cost and safety

- Three Opus instances cost roughly three times a single run. Phase 1 runs locally against Docker targets; budget phase 2 after phase 1 passes.
- Runs use `--dangerously-skip-permissions` as the existing runner does, so they only ever point at the local Docker targets or the Winforge VM, never at anything else on the network. Scope rules in the scenario restrict targets as well.
- Winforge stays isolated as in the original eval: guest firewall allowing SSH only from the lab's bastion and management hosts.

## 7. Reporting

Each phase gets a write-up in the same shape as the existing evals (what the agents did, what ARTEX recorded, what went wrong), with the coordination scorecard and the single-instance comparison. If the results hold up, that write-up becomes the follow-up post on the site blog, and the launch post's "What's rough" section (which described multi-agent coordination as unbuilt, correct for v0.4.1, and now carries a dated note that v0.5.0 shipped it) can link to it.
