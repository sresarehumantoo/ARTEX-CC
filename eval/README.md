# ARTEX Evaluation Harness

Self-hosted evaluation harness for testing ARTEX against controlled vulnerable
targets. No HTB dependency; all targets run locally in Docker.

## Targets

| Target | Port | Description |
|--------|------|-------------|
| DVWA | 8081 | Damn Vulnerable Web App, security level set to `low` |
| Juice Shop | 8082 | OWASP Juice Shop with `NODE_ENV=unsafe` |
| VulnApp | 8083 | Custom app with SQLi, directory traversal, SSRF, IDOR, open admin |

VulnApp vulnerabilities:

- **SQL injection** in `/api/search?q=` and `/login` (string interpolation)
- **Directory traversal** in `/files?path=` (unsanitized path join)
- **SSRF** in `/fetch?url=` (fetches arbitrary URLs server-side)
- **Broken access control**: unauthenticated admin panel at `/admin`, IDOR at `/api/users/:id` (returns passwords)
- **Known credentials**: `admin` / `admin123`

## Quick start

```
cd eval

# Start targets (Postgres for ARTEX state is included)
docker compose up -d

# Wait for health checks, then run a scenario
go run ../eval/cmd/eval-runner \
  --pg 'postgres://ccartex_eval:ccartex_eval@localhost:5433/ccartex_eval?sslmode=disable' \
  scenarios/vulnapp-full.yml

# Dry-run (set up engagement, skip Claude execution, score empty state)
go run ../eval/cmd/eval-runner --dry-run scenarios/vulnapp-full.yml

# Tear down
docker compose down -v
```

The eval Postgres is published on `127.0.0.1:5433`, so it can run next to the
dev stack's Postgres on 5432. Without `--pg`, the runner uses `ARTEX_PG` if set,
then the DSN above, so unset `ARTEX_PG` if it points at your dev database.

The runner calls `claude` (Claude Code CLI) in `--print --dangerously-skip-permissions`
mode with an auto-generated MCP config pointing ARTEX at the eval Postgres. It also passes
`--strict-mcp-config`, so instances load only that config: none of your own MCP servers or
claude.ai connectors (mail, calendar, docs, DNS) are reachable from an unattended run. Each
instance runs with `--output-format stream-json --verbose`, and its transcript is
saved as `transcripts-<scenario>-<timestamp>/instance-<i>.jsonl` next to the
scorecard, so every tool call, argument and result is on record.

Each instance also gets its own temp directory,
`transcripts-<scenario>-<timestamp>/tmp-instance-<i>` (mode 0700), set as
`TMPDIR`, `TMP` and `TEMP` in its environment, so instances cannot clobber each
other's helper scripts and nothing lands in your `/tmp`. Because agents often
hardcode `/tmp` in commands, every prompt (including N=1) ends its base text
with "Keep scratch files (scripts, payloads, downloads, keys) in $TMPDIR, not
/tmp." An `--env TMPDIR=...` overrides the directory. The directories are kept
after the run; each entry in the scorecard's `runs` records `temp_dir` and a
`scratch` section with the number and total size of files left there, the
first 50 by path with their sizes, and `private_keys`: files whose first MiB
holds a PEM `-----BEGIN ... PRIVATE KEY-----` block. The runner also logs a
warning for those; delete them before sharing or committing results. Files an
agent writes to `/tmp` anyway are not tracked.

## Multiple instances

`--instances N` starts N `claude` processes at once on the same engagement, with
the same goal. When N is above 1, each prompt ends with "You are instance i of
N; other agents are working this engagement at the same time" and a pointer to
intent claims and `list_changes`. With N=1 that sentence is left out.

`--transport` picks how instances reach ARTEX:

- `stdio` (default): every `claude` process launches its own
  `artex-cc serve --stdio`, so each instance has its own MCP session. Claims
  belong to that session. All of them share the eval Postgres.
- `http`: the runner starts one `artex-cc serve --listen` on a free loopback
  port and points every instance at it. The server log is written to
  `server.log` in the transcripts directory. By default each instance gets its
  own signed `X-Artex-Identity`, named `eval/<run>/i<n>` (`<run>` is the run's
  start time, `20060102-150405`), so claims belong to the instance without any
  `owner` argument and holder labels in the transcripts read
  `eval/<run>/i<n>`. The runner mints the tokens in process with the temporary
  data directory's `identity.key`, which the server verifies against, with a
  TTL of the scenario timeout plus the lease (`--lease`, else 30m). Each token
  is written only into that instance's own MCP config (`mcp-i<n>.json`, mode
  0600) inside the temporary data directory, which is deleted when the run
  ends; tokens are never logged or copied into the scorecard or transcripts.
- `http` with `--identity=false`: the anonymous path. Every instance shares one
  MCP config without an identity, and each prompt tells the agent to pick a
  random hex owner label once and pass it as `owner` on `start_intent`,
  `complete_intent`, `cancel_intent` and `list_changes`, which is how
  anonymous claims are told apart on MCP protocol 2026-07-28. Holder labels
  are hashed (`h-...`).

`--identity` has no effect with `stdio` or with `--mcp-config`; the scorecard's
`identity` field says whether identities were used.

`ARTEX_REQUIRE_IDENTITY` in the runner's environment passes through to the
server only with identities, where every instance satisfies it. The anonymous
modes (`stdio`, and `http` with `--identity=false`) are what it forbids, so the
runner clears it for the servers it starts in those modes. With
`--mcp-config` the runner leaves the environment alone.

`--kill-after INSTANCE:DURATION` kills one instance (its whole process group,
including its `serve --stdio` child) after the delay, leaving its claims live,
for the recovery test. The kill time is recorded in the scorecard's `runs`.

`--lease DURATION` sets `ARTEX_INTENT_LEASE` on every `serve` process the runner
starts, so a takeover test does not wait the default 30 minutes. It needs an
`artex-cc` built with lease override support; older servers ignore it.

`--env KEY=VALUE` (repeatable) adds a variable to every `claude` instance's
environment, for example `--env TELEPORT_HOME=/path/to/empty/dir` so `tsh` in the
agents' shell cannot see your own Teleport profile, plus a variable naming a
narrowly scoped identity file the scenario tells agents to use. That keeps your
login out of accidental reach; it is not a sandbox, since the agents still run
as your OS user.

The scenario timeout covers the whole run: when it passes, every instance still
running is killed. The runner waits for all instances before scoring.

```
# Three instances over stdio, lease 2m, kill instance 2 after 3 minutes
go run ../eval/cmd/eval-runner --instances 3 --transport stdio \
  --lease 2m --kill-after 2:3m scenarios/vulnapp-full.yml

# Three instances sharing one HTTP server, one identity each
go run ../eval/cmd/eval-runner --instances 3 --transport http scenarios/vulnapp-full.yml

# The same over the anonymous owner argument
go run ../eval/cmd/eval-runner --instances 3 --transport http --identity=false scenarios/vulnapp-full.yml
```

## What it measures

The scorecard (written to stdout and a JSON file) includes:

- **Finding type score** (40%): fraction of expected vulnerability types found
- **Finding count score** (25%): fraction of expected minimum finding count reached
- **Asset score** (15%): whether assets were discovered and recorded
- **Intent score** (20%): fraction of created intents that reached completion

Additional data: total findings, total assets, coverage records, goals set,
intents created/finished, wall-clock duration, timeout flag, and per instance
the transcript path, duration, exit error, kill time, temp directory and the
scratch files left in it.

## Coordination scorecard

After every run that started `claude`, the runner reads the transcripts and the
final engagement and adds a `coordination` section to the scorecard. It also
prints a readable summary to stderr and writes it to `coordination.txt` in the
transcripts directory. Claim history is not stored in the database, so the
transcripts are the source of truth for claims.

The claim metrics share one replay of claim windows, in time order (tool
result timestamps). A successful `start_intent` opens or renews a window until
the expiry in its result text (else `--lease` or 30m), and a successful
`complete_intent` or `cancel_intent` closes it. Servers that renew claims
implicitly also extend the window when the instance holding the intent records
work against it: a successful `record_fact` (each fact's `intent_id`),
`report_finding` with `intent_id`, or `record_coverage` with `intent_id`. As in
the server, only the instance that made the intent's latest claim renews it,
the window then runs to call time + lease, a call with more than half a lease
left changes nothing, and a lapsed claim nobody else has taken since is
revived. `implicit_renewal` says whether this was modeled and
`implicit_renewals` how many renewals it added. It is on when the `artex-cc`
binary's `tools --json` catalog says recorded work renews `start_intent`
claims (not probed with `--mcp-config`), or when any transcript has the
renewal sentence that such servers put in every "is claimed by" refusal.
Transcripts from older servers have neither and score as before. The replay
cannot tell an anonymous holder that has no MCP session (`owner` only, which
the server does not renew) from one that has, so such runs on a renewing
server overstate renewals.

| Metric | How it is computed |
|--------|--------------------|
| `double_claims` | From the claim replay: a successful claim by a second instance before the previous holder's window ends, implicit renewals included. Pass: 0 |
| `unclaimed_work` | Per instance: `work_calls` is its work done after the first intent of the run was created (bootstrap recon before that is skipped, since nothing could be claimed). Work is every Bash call (failed commands included) and every successful `report_finding`, `record_fact`, `record_coverage`, `insert_assets`, `save_evidence`, `capture_command_output`, `ingest_scanner_output` or `ingest_masscan`; planning, reads and claim bookkeeping are not work. `calls` (split into `bash` and `artex_records`) is the part done while the instance held no live claim on any intent in the claim replay, so with a short `--lease` work after a lapse counts, while work after an implicit renewal (the renewing call included) does not. Holding any claim is enough, whatever intent the work was for. `finished_unclaimed_intents` lists intents it completed without ever having claimed them, and intents created by another instance that it canceled without claiming. Canceling an unclaimed intent it created itself is the duplicate cleanup `add_intent` asks for when it reports an overlapping open intent, so those go in `own_duplicate_cancels` instead. Totals are `unclaimed_work_calls`, `unclaimed_finishes` and `own_duplicate_cancels`. Informational |
| `concurrent_intent_work` | Pairs of instances whose work windows on one intent overlap. A window runs from the instance's first claim of the intent or first fact/finding with that `intent_id`, whichever is earlier, to its last such record or its successful `complete_intent`/`cancel_intent`, and needs at least one record or finish. Leases, and so renewals, are ignored, so this catches an instance that kept working after its lease lapsed while another took the intent over, and an instance recording against an intent another one holds, which `double_claims` misses when only one of them claimed. Windows that only touch are not counted. Pass: 0 |
| `conflicts_handled` | `start_intent` results saying the intent "is claimed by" another holder, counted as unhandled when one of the next 5 tool calls of that instance retries `start_intent`, completes, cancels or records work against the same intent. A retry counts as handled once the expiry the refusal stated has passed and, with implicit renewal, the holder's window in the claim replay has also ended, since the holder may have renewed after the refusal. Pass: 90% or more |
| `refused_completions` | `complete_intent` or `cancel_intent` refused because another holder has a live claim |
| `holders`, `holder_instance` | Holder labels each instance claimed under, and for each unhandled conflict the instance holding the intent: read from the label for `eval/<run>/i<n>` identities, else the instance whose own claims carried that hashed label |
| `holder_mismatches` | Successful claims whose `eval/<run>/i<n>` label names a different instance than the transcript it appears in, which means the per-instance configs were crossed. Pass: 0 |
| `duplicate_findings` | Findings grouped by normalized vulnclass and asset (asset 0 when a finding has none) that were reported by more than one instance, which comes from the `report_finding` results. `raw` counts every extra finding; `after_merges` ignores merged and dismissed ones. Pass: 0 after merges |
| `redundant_recon` | Bash commands, whitespace-normalized, run by more than one instance; `redundant_runs` counts the instances beyond the first per command |
| `list_changes_calls` | Per instance |
| `cross_instance_parents` | Graph edges whose parent node was created by a different instance than the child, by child kind. Node creators come from the `add_intent`, `record_fact` and `report_finding` results in the transcripts |
| `takeovers` | With `--kill-after`: each intent the killed instance still held in the claim replay when it died (`lease_expires` includes its implicit renewals), the survivor that started it next and when, whether that was after the lease lapsed, and how many of the killed instance's facts for it are still attached in the graph |
| `usage`, `cost_usd` | Per instance and total, from the stream-json `result` message. A killed instance has no `result` message, so its token counts are summed from its assistant messages and its cost is 0 |

## Adding scenarios

Create a YAML file in `scenarios/`:

```yaml
name: "My Scenario"
description: "What this scenario tests"
model: claude-opus-5-5  # optional; overridden by --model flag
target: "http://localhost:8083"
goal: "The goal prompt given to Claude"
scope:
  - "localhost:8083"
credentials:
  username: "admin"
  password: "admin123"
expected_findings:
  min_count: 2
  must_include:
    - type: "sql_injection"
      description: "Human-readable description"
      endpoints:
        - "/api/search?q="
  may_include:
    - type: "xss"
timeout: 10m
```

`must_include` types are scored; `may_include` types are informational. The
matching is fuzzy: `sql_injection` matches findings with vulnclass or summary
containing "sqli", "sql injection", etc.

## Model selection

Scenarios can specify which Claude model the runner passes to `claude --model`:

```yaml
model: claude-opus-5-5
```

The field is optional. When omitted, the runner uses whatever model Claude Code
defaults to. The `--model` CLI flag overrides the scenario value.

Opus 4.8's cyber safeguards block security-oriented prompts, so eval scenarios
targeting live hosts should set `model: claude-opus-5-5` (or another model
without those restrictions). Local-only targets like VulnApp are less likely to
trigger safeguards, but pinning the model keeps runs reproducible.

## Auto-mode allow rule

The project's `.claude/settings.json` includes an allow rule for
`Bash(claude --print --dangerously-skip-permissions:*)`. The eval runner spawns
`claude --print --dangerously-skip-permissions` as a subprocess; without this
rule, Claude Code's auto-mode classifier blocks it as "Undermining Oversight".
The rule is scoped to that exact command prefix.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--pg` | `ARTEX_PG` or localhost eval DB | PostgreSQL connection string |
| `--artex` | `artex-cc` | Path to the `artex-cc` binary |
| `--claude` | `claude` | Path to Claude Code CLI binary |
| `--model` | (scenario or CLI default) | Claude model override |
| `--target` | (scenario value) | Target URL or IP override |
| `--mcp-config` | (generated) | Existing MCP config file to use instead of generating one |
| `--skip-health` | false | Skip the target health check |
| `--output` | `.` | Directory for scorecard JSON files |
| `--dry-run` | false | Set up engagement but skip Claude execution |
| `--instances` | 1 | Number of concurrent `claude` instances on the engagement |
| `--transport` | `stdio` | `stdio` (one `serve --stdio` per instance) or `http` (one shared `serve --listen`) |
| `--kill-after` | (none) | Kill one instance after a delay, as `INSTANCE:DURATION` (e.g. `2:3m`) |
| `--lease` | (server default) | Intent lease passed to every `serve` process as `ARTEX_INTENT_LEASE` |
| `--env` | (none) | `KEY=VALUE` added to every `claude` instance's environment; repeatable |
| `--identity` | true | With `--transport http`, give each instance its own `X-Artex-Identity`; `false` keeps the anonymous `owner` argument path |

## Windows target testing (manual)

For testing against Windows targets, use the homenet-winforge workbench
(snapshot-restorable via `qm snapshot 3601 toolchain-clean`). This is not
automated in the harness because it requires a live Proxmox host and manual
network setup. Use it for ad-hoc validation of Windows-specific findings
(SMB, Active Directory, PowerShell-based attacks).
