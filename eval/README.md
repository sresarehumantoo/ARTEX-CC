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
  --pg 'postgres://ccartex_eval:ccartex_eval@localhost:5432/ccartex_eval?sslmode=disable' \
  scenarios/vulnapp-full.yml

# Dry-run (set up engagement, skip Claude execution, score empty state)
go run ../eval/cmd/eval-runner --dry-run scenarios/vulnapp-full.yml

# Tear down
docker compose down -v
```

The runner calls `claude` (Claude Code CLI) in `--print --dangerously-skip-permissions`
mode with an auto-generated MCP config pointing ARTEX at the eval Postgres. It also passes
`--strict-mcp-config`, so instances load only that config: none of your own MCP servers or
claude.ai connectors (mail, calendar, docs, DNS) are reachable from an unattended run. Each
instance runs with `--output-format stream-json --verbose`, and its transcript is
saved as `transcripts-<scenario>-<timestamp>/instance-<i>.jsonl` next to the
scorecard, so every tool call, argument and result is on record.

## Multiple instances

`--instances N` starts N `claude` processes at once on the same engagement, with
the same goal. When N is above 1, each prompt ends with "You are instance i of
N; other agents are working this engagement at the same time" and a pointer to
intent claims and `list_changes`. With N=1 the prompt is unchanged.

`--transport` picks how instances reach ARTEX:

- `stdio` (default): every `claude` process launches its own
  `artex-cc serve --stdio`, so each instance has its own MCP session. Claims
  belong to that session. All of them share the eval Postgres.
- `http`: the runner starts one `artex-cc serve --listen` on a free loopback
  port and points every instance at it. Each prompt also tells the agent to pick
  a random hex owner label once and pass it as `owner` on `start_intent`,
  `complete_intent`, `cancel_intent` and `list_changes`, which is how claims are
  told apart on MCP protocol 2026-07-28. The server log is written to
  `server.log` in the transcripts directory.

`--kill-after INSTANCE:DURATION` kills one instance (its whole process group,
including its `serve --stdio` child) after the delay, leaving its claims live,
for the recovery test. The kill time is recorded in the scorecard's `runs`.

`--lease DURATION` sets `ARTEX_INTENT_LEASE` on every `serve` process the runner
starts, so a takeover test does not wait the default 30 minutes. It needs an
`artex-cc` built with lease override support; older servers ignore it.

The scenario timeout covers the whole run: when it passes, every instance still
running is killed. The runner waits for all instances before scoring.

```
# Three instances over stdio, lease 2m, kill instance 2 after 3 minutes
go run ../eval/cmd/eval-runner --instances 3 --transport stdio \
  --lease 2m --kill-after 2:3m scenarios/vulnapp-full.yml

# Three instances sharing one HTTP server
go run ../eval/cmd/eval-runner --instances 3 --transport http scenarios/vulnapp-full.yml
```

## What it measures

The scorecard (written to stdout and a JSON file) includes:

- **Finding type score** (40%): fraction of expected vulnerability types found
- **Finding count score** (25%): fraction of expected minimum finding count reached
- **Asset score** (15%): whether assets were discovered and recorded
- **Intent score** (20%): fraction of created intents that reached completion

Additional data: total findings, total assets, coverage records, goals set,
intents created/finished, wall-clock duration, timeout flag, and per instance
the transcript path, duration, exit error and kill time.

## Coordination scorecard

After every run that started `claude`, the runner reads the transcripts and the
final engagement and adds a `coordination` section to the scorecard. It also
prints a readable summary to stderr and writes it to `coordination.txt` in the
transcripts directory. Claim history is not stored in the database, so the
transcripts are the source of truth for claims.

| Metric | How it is computed |
|--------|--------------------|
| `double_claims` | Successful `start_intent` results replayed in time order (tool result timestamps); a claim by a second instance before the previous holder's expiry (from the result text, else `--lease` or 30m) and with no `complete_intent`/`cancel_intent` in between. Pass: 0 |
| `conflicts_handled` | `start_intent` results saying the intent "is claimed by" another holder, counted as handled when one of the next 5 tool calls of that instance is `start_intent` on a different intent. Pass: 90% or more |
| `refused_completions` | `complete_intent` or `cancel_intent` refused because another holder has a live claim |
| `duplicate_findings` | Findings grouped by normalized vulnclass and asset (asset 0 when a finding has none) that were reported by more than one instance, which comes from the `report_finding` results. `raw` counts every extra finding; `after_merges` ignores merged and dismissed ones. Pass: 0 after merges |
| `redundant_recon` | Bash commands, whitespace-normalized, run by more than one instance; `redundant_runs` counts the instances beyond the first per command |
| `list_changes_calls` | Per instance |
| `cross_instance_parents` | Graph edges whose parent node was created by a different instance than the child, by child kind. Node creators come from the `add_intent`, `record_fact` and `report_finding` results in the transcripts |
| `takeovers` | With `--kill-after`: each intent the killed instance still held when it died, the survivor that started it next and when, whether that was after the lease lapsed, and how many of the killed instance's facts for it are still attached in the graph |
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

## Windows target testing (manual)

For testing against Windows targets, use the homenet-winforge workbench
(snapshot-restorable via `qm snapshot 3601 toolchain-clean`). This is not
automated in the harness because it requires a live Proxmox host and manual
network setup. Use it for ad-hoc validation of Windows-specific findings
(SMB, Active Directory, PowerShell-based attacks).
