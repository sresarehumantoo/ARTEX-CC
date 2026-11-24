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
mode with an auto-generated MCP config pointing ARTEX at the eval Postgres.

## What it measures

The scorecard (written to stdout and a JSON file) includes:

- **Finding type score** (40%): fraction of expected vulnerability types found
- **Finding count score** (25%): fraction of expected minimum finding count reached
- **Asset score** (15%): whether assets were discovered and recorded
- **Intent score** (20%): fraction of created intents that reached completion

Additional data: total findings, total assets, coverage records, goals set,
intents created/finished, wall-clock duration, timeout flag.

## Adding scenarios

Create a YAML file in `scenarios/`:

```yaml
name: "My Scenario"
description: "What this scenario tests"
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

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--pg` | `ARTEX_PG` or localhost eval DB | PostgreSQL connection string |
| `--artex` | `artex` | Path to artex binary |
| `--claude` | `claude` | Path to Claude Code CLI binary |
| `--output` | `.` | Directory for scorecard JSON files |
| `--dry-run` | false | Set up engagement but skip Claude execution |

## Windows target testing (manual)

For testing against Windows targets, use the homenet-winforge workbench
(snapshot-restorable via `qm snapshot 3601 toolchain-clean`). This is not
automated in the harness because it requires a live Proxmox host and manual
network setup. Use it for ad-hoc validation of Windows-specific findings
(SMB, Active Directory, PowerShell-based attacks).
