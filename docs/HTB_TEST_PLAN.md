# HTB test plan for ARTEX

Validation plan using five Hack The Box machines, ordered to build coverage
incrementally. Each box targets a different slice of the tool surface; together
they exercise every tool at least once.

## Quick start

### Initialize the engagement

```
createdb ccartex_htb
artex-cc init \
  --pg "postgres://localhost/ccartex_htb" \
  --data ~/htb/management \
  --goal "HTB validation: Management (Easy, Linux)"
```

### Add the box to /etc/hosts

```
htb-hosts add <SPAWN_IP> management.htb
```

### MCP config

Add to `.mcp.json` in the working directory (or `~/.claude.json` globally):

```json
{
  "mcpServers": {
    "artex": {
      "command": "artex-cc",
      "args": [
        "serve", "--stdio",
        "--pg", "postgres://localhost/ccartex_htb",
        "--data", "/path/to/htb/management",
        "--proxy", ":8788"
      ]
    }
  }
}
```

Trust the CA cert (printed by `artex-cc init`) so the recording proxy can
intercept HTTPS, then start Claude Code.

### Hands-free mode

```
/loop 5m Continue the engagement: call graph_overview, assess progress, dispatch subagents for open intents, record facts, report findings, prove goals when evidence exists, and generate_report when all goals are met or all surfaces tested.
```

---

## Box 1: Management (Easy, Linux, ID 981)

**Difficulty:** Easy | **OS:** Linux | **Blood time:** 6 min

**Why this box:** Baseline end-to-end lifecycle test. The short, structured path
lets you verify every phase of a ARTEX engagement without debugging complex
attack chains. If the full init-to-report cycle works here, the plumbing is
sound.

### What it validates

Full engagement lifecycle: initialization through report generation, with every
core tool exercised at least once.

### Tool sequence

| Phase | Tools |
|-------|-------|
| Setup | `create_engagement`, `set_goals`, `set_constraints`, `add_task_scope` |
| Recon | `insert_assets`, `enrich_domain`, `enrich_ip`, `enrich_service` |
| Plan | `graph_overview`, `add_intent`, `start_intent` |
| Execute | `record_fact`, `record_coverage`, `save_task_credential` |
| Report | `report_finding`, `prove_goal`, `generate_report`, `goal_met` |

### Pass criteria

- `graph_overview` shows goals, intents progress through open/running/done.
- `generate_report` produces a Markdown report with at least one finding.
- `engagement_status` shows the engagement complete.
- Credentials stored with `save_task_credential` appear in `list_task_credentials`.

---

## Box 2: Cohort (Easy, Linux, ID 933)

**Difficulty:** Easy | **OS:** Linux

**Why this box:** SSRF, WebSocket traffic, and PacketKit privilege escalation
across ports 22/80/443. Tests the exploration graph under branching conditions
and validates multi-scanner ingestion.

### What it validates

Branching exploration graph, scanner output ingestion (nmap + ffuf),
coverage tracking across multiple risk areas, WebSocket message capture.

### Tool sequence

| Phase | Tools |
|-------|-------|
| Recon | `insert_assets`, `enrich_domain`, `enrich_ip`, `enrich_service` |
| Ingest | `ingest_scanner_output` (nmap XML), `ingest_scanner_output` (ffuf JSON) |
| Explore | `graph_overview`, `add_intent` (branching: SSRF path vs. direct), `start_intent`, `complete_intent` |
| Traffic | `traffic_search`, `traffic_get`, `traffic_messages` (WebSocket frames) |
| Coverage | `record_coverage` (multiple risk areas: ssrf, privesc, websocket) |
| Findings | `report_finding`, `bind_finding_traffic` |

### Pass criteria

- `ingest_scanner_output` correctly parses nmap XML and ffuf JSON, creating
  assets for discovered services and endpoints.
- `traffic_messages` returns WebSocket frames with correct direction filtering.
- `record_coverage` tracks at least three distinct risk areas.
- The exploration graph shows a branching structure (multiple intents from the
  same parent fact).

---

## Box 3: Layover (Medium, Linux, ID 984)

**Difficulty:** Medium | **OS:** Linux

**Why this box:** Multi-host engagement with given credentials
(contractor/Contractor2026!), RDP jump host, Wi-Fi traffic capture,
portal.international.htb (Craft CMS RCE), SSH lateral movement, and CUPS
privilege escalation. The most complex asset graph in this plan.

### What it validates

Credential management across 3+ sets of credentials, traffic capture and
analysis, multi-host asset tracking, scope expansion mid-engagement.

### Setup

```
htb-hosts add <SPAWN_IP> layover.htb portal.international.htb
```

Initialize with given credentials:

```
artex-cc init \
  --pg "postgres://localhost/ccartex_htb" \
  --data ~/htb/layover \
  --goal "HTB validation: Layover (Medium, Linux) - multi-host lateral movement"
```

### Tool sequence

| Phase | Tools |
|-------|-------|
| Setup | `add_task_scope` (initial host), `save_task_credential` (contractor/Contractor2026!) |
| Recon | `insert_assets` (multiple hosts), `enrich_domain`, `enrich_service` |
| Credential mgmt | `save_task_credential` (3+ times as creds are discovered), `list_task_credentials` |
| Traffic analysis | `traffic_search`, `traffic_get`, `traffic_blob`, `traffic_stats`, `traffic_timeline` |
| Scope expansion | `add_task_scope` (add new hosts as lateral movement progresses), `check_scope` |
| Lateral movement | `add_intent` with `parent_ids` linking to discovery facts, `record_fact` per hop |
| Findings | `report_finding` (Craft CMS RCE, CUPS privesc), `bind_finding_traffic` |
| Wrap-up | `prove_goal`, `generate_report`, `export_engagement` |

### Pass criteria

- `list_task_credentials` returns 3+ distinct credentials.
- `list_assets` shows hosts from at least two different IPs.
- `check_scope` correctly gates replay and enrichment against the defined scope.
- `traffic_stats` and `traffic_timeline` return meaningful aggregates.
- `export_engagement` produces a JSON archive that can be re-imported.

---

## Box 4: Touch (Easy, Windows, ID 987)

**Difficulty:** Easy | **OS:** Windows

**Why this box:** Linked to Layover via the Jenny Crawford / KS7X2M booking
code. Tests cross-engagement credential reuse and Windows-specific service
enumeration.

### What it validates

Cross-engagement data (credentials discovered in Layover applied here), Windows
service and asset tracking, `import_engagement` for credential continuity.

### Setup

Import Layover engagement data for credential continuity:

```
artex-cc init \
  --pg "postgres://localhost/ccartex_htb" \
  --data ~/htb/touch \
  --goal "HTB validation: Touch (Easy, Windows) - cross-engagement credential reuse"
```

### Tool sequence

| Phase | Tools |
|-------|-------|
| Setup | `import_engagement` (Layover export), `list_task_credentials` (verify imported creds) |
| Recon | `insert_assets` (Windows host), `enrich_ip`, `enrich_service` |
| Windows enum | `ingest_scanner_output` (nmap with Windows services: SMB, RDP, WinRM) |
| Explore | `graph_overview`, `add_intent`, `start_intent`, `record_fact` |
| Findings | `report_finding`, `record_coverage` |
| Activity | `log_activity`, `list_activity` |

### Pass criteria

- `import_engagement` loads the Layover export without errors.
- Credentials from Layover are available via `list_task_credentials` in the new
  engagement.
- `insert_assets` handles Windows service types (SMB, RDP, WinRM).
- `list_activity` shows a readable audit trail of the engagement.

---

## Box 5: DutyBound (Medium, Linux, ID 999)

**Difficulty:** Medium | **OS:** Linux | **Blood time:** 9 min

**Why this box:** Medium stretch target that exercises the full traffic tool
suite: replay with modifications, HAR export, blob reads for large responses,
and timeline analysis. Also tests parallel subagent dispatch.

### What it validates

Parallel subagent dispatch, traffic replay with request modifications,
HAR export, full traffic tool coverage, concurrent intent execution.

### Tool sequence

| Phase | Tools |
|-------|-------|
| Recon | `insert_assets`, `enrich_domain`, `enrich_ip`, `enrich_service` |
| Parallel dispatch | `add_intent` (multiple), `start_intent` (concurrent), `graph_overview` (after subagents) |
| Traffic capture | `traffic_search`, `traffic_get`, `traffic_hosts` |
| Traffic replay | `traffic_replay` (with `modify_headers`, `modify_body`, `modify_url`) |
| Traffic analysis | `traffic_blob` (oversized responses), `traffic_har_export`, `traffic_stats`, `traffic_timeline` |
| Hints | `add_hint` (steer subagents mid-engagement) |
| Advanced explore | `node_detail`, `list_facts`, `cancel_intent` (abandon dead-end paths) |
| Findings | `report_finding`, `bind_finding_traffic`, `list_findings` |
| Report | `generate_report`, `goal_met` |

### Pass criteria

- At least two intents run concurrently (visible in `graph_overview` as
  multiple `running` intents).
- `traffic_replay` successfully replays a captured request with modified
  headers or body and records the new exchange.
- `traffic_har_export` produces valid HAR 1.2 JSON.
- `traffic_blob` retrieves segments of an oversized response body.
- `cancel_intent` correctly marks a dead-end intent without losing its
  recorded facts.

---

## Tool coverage matrix

Each cell marks which box first exercises the tool (1-5) or confirms it again
(+). Every tool appears at least once.

| Tool | 1 Mgmt | 2 Cohort | 3 Layover | 4 Touch | 5 DutyBound |
|------|:------:|:--------:|:---------:|:-------:|:-----------:|
| **Asset tools** | | | | | |
| `insert_assets` | 1 | + | + | + | + |
| `list_assets` | 1 | + | + | + | + |
| `list_companies` | | | | | |
| `add_company_scope` | | | | | |
| `add_task_scope` | 1 | | + | | |
| `list_untested_assets` | 1 | + | | | |
| **Enrichment tools** | | | | | |
| `enrich_domain` | 1 | + | + | | + |
| `enrich_ip` | 1 | | | 4 | + |
| `enrich_service` | 1 | + | + | + | + |
| **Exploration tools** | | | | | |
| `graph_overview` | 1 | + | + | + | + |
| `add_intent` | 1 | + | + | + | + |
| `record_fact` | 1 | + | + | + | + |
| `list_facts` | | | | | 5 |
| `node_detail` | | | | | 5 |
| `add_hint` | | | | | 5 |
| `set_goals` | 1 | | | | |
| `set_constraints` | 1 | | | | |
| `prove_goal` | 1 | | + | | + |
| **Intent tools** | | | | | |
| `start_intent` | 1 | + | | + | + |
| `complete_intent` | | 2 | | | + |
| `cancel_intent` | | | | | 5 |
| **Finding tools** | | | | | |
| `report_finding` | 1 | + | + | + | + |
| `list_findings` | | | | | 5 |
| `bind_finding_traffic` | | 2 | + | | + |
| **Credential tools** | | | | | |
| `save_task_credential` | 1 | | + | | |
| `list_task_credentials` | 1 | | + | + | |
| **Scope tools** | | | | | |
| `check_scope` | | | 3 | | |
| `record_coverage` | 1 | + | | + | |
| **Ingest tools** | | | | | |
| `ingest_scanner_output` | | 2 | | + | |
| **Traffic tools** | | | | | |
| `traffic_search` | | 2 | + | | + |
| `traffic_get` | | 2 | + | | + |
| `traffic_blob` | | | 3 | | + |
| `traffic_messages` | | 2 | | | |
| `traffic_replay` | | | | | 5 |
| `traffic_hosts` | | | | | 5 |
| `traffic_har_export` | | | | | 5 |
| `traffic_stats` | | | 3 | | + |
| `traffic_timeline` | | | 3 | | + |
| **Report tools** | | | | | |
| `generate_report` | 1 | | + | | + |
| **Engagement tools** | | | | | |
| `create_engagement` | 1 | | | | |
| `list_engagements` | | | | 4 | |
| `goal_met` | 1 | | | | + |
| `engagement_status` | 1 | | | | |
| **Activity tools** | | | | | |
| `log_activity` | | | | 4 | |
| `list_activity` | | | | 4 | |
| **Export tools** | | | | | |
| `export_engagement` | | | 3 | | |
| `import_engagement` | | | | 4 | |

**Not exercised:** `list_companies`, `add_company_scope` (company-scoped features;
add a multi-company box or synthetic test if needed).

---

## Notes

- Each box gets its own `artex-cc init` and database (or at least a fresh
  engagement in the same database). Layover and Touch share credential data
  via `export_engagement`/`import_engagement`.
- Run boxes in order. Management validates the basics; later boxes assume those
  tools already work and focus on what is new.
- The proxy (`--proxy :8788`) is required for boxes 2-5. Box 1 can run without
  it but should include it to verify proxy startup.
- Blood times are from the HTB API and indicate relative complexity. A
  6-minute blood on Management means a skilled human solves it fast; ARTEX
  will be slower but should complete it in a single session.
