---
title: Tool reference
description: "All 62 ARTEX MCP tools by category, with a one-line summary of each."
sidebar:
  label: Overview
  order: 0
---

ARTEX registers 62 MCP tools across 10 categories. Claude calls them on its own based on what the engagement needs; you rarely call one by name. Each category page documents every parameter and return shape.

## Exploration graph

12 tools, [full parameters](/docs/tools/exploration-graph/).

| Tool | What it does |
|------|-------------|
| [`graph_overview`](/docs/tools/exploration-graph/#graph_overview) | Situational summary: asset counts, goals, hints, frontier, findings, coverage |
| [`add_intent`](/docs/tools/exploration-graph/#add_intent) | Plan a next step (queued for execution) |
| [`start_intent`](/docs/tools/exploration-graph/#start_intent) | Move a queued intent to in-progress |
| [`complete_intent`](/docs/tools/exploration-graph/#complete_intent) | Mark a running intent as done with a result summary |
| [`cancel_intent`](/docs/tools/exploration-graph/#cancel_intent) | Cancel an intent that is no longer needed |
| [`record_fact`](/docs/tools/exploration-graph/#record_fact) | Record a discovery or observation linked to an intent |
| [`list_facts`](/docs/tools/exploration-graph/#list_facts) | Page through exploration facts, newest first |
| [`node_detail`](/docs/tools/exploration-graph/#node_detail) | Inspect a specific graph node by ID |
| [`add_hint`](/docs/tools/exploration-graph/#add_hint) | Attach a strategic hint for the planner |
| [`set_goals`](/docs/tools/exploration-graph/#set_goals) | Define or update engagement goals |
| [`set_constraints`](/docs/tools/exploration-graph/#set_constraints) | Add allow/deny operation constraints |
| [`prove_goal`](/docs/tools/exploration-graph/#prove_goal) | Link evidence to a goal and mark it proved |

## Assets and scope

8 tools, [full parameters](/docs/tools/assets-and-scope/).

| Tool | What it does |
|------|-------------|
| [`insert_assets`](/docs/tools/assets-and-scope/#insert_assets) | Add domains, IPs, services, endpoints, or apps in bulk |
| [`list_assets`](/docs/tools/assets-and-scope/#list_assets) | Query assets by type, DSL filter, or ID |
| [`list_companies`](/docs/tools/assets-and-scope/#list_companies) | List tracked companies and their asset scopes |
| [`add_company_scope`](/docs/tools/assets-and-scope/#add_company_scope) | Add scope rules (domains, CIDRs) to a company |
| [`add_task_scope`](/docs/tools/assets-and-scope/#add_task_scope) | Add in-scope markers for the current task |
| [`list_untested_assets`](/docs/tools/assets-and-scope/#list_untested_assets) | Find assets with no coverage records |
| [`check_scope`](/docs/tools/assets-and-scope/#check_scope) | Check whether a target is within scope rules |
| [`record_coverage`](/docs/tools/assets-and-scope/#record_coverage) | Record what was tested, the outcome, and evidence |

## Findings

5 tools, [full parameters](/docs/tools/findings/).

| Tool | What it does |
|------|-------------|
| [`report_finding`](/docs/tools/findings/#report_finding) | Report a vulnerability with severity, evidence, and affected assets |
| [`list_findings`](/docs/tools/findings/#list_findings) | List findings for the current engagement |
| [`bind_finding_traffic`](/docs/tools/findings/#bind_finding_traffic) | Link a finding to captured traffic exchanges (requires `--proxy`) |
| [`merge_findings`](/docs/tools/findings/#merge_findings) | Merge duplicate findings into one |
| [`dismiss_finding`](/docs/tools/findings/#dismiss_finding) | Mark a finding as dismissed (false positive) |

## Credentials

2 tools, [full parameters](/docs/tools/credentials/).

| Tool | What it does |
|------|-------------|
| [`save_task_credential`](/docs/tools/credentials/#save_task_credential) | Store a discovered credential for the current task |
| [`list_task_credentials`](/docs/tools/credentials/#list_task_credentials) | List stored credentials |

## Evidence

4 tools, [full parameters](/docs/tools/evidence/).

| Tool | What it does |
|------|-------------|
| [`save_evidence`](/docs/tools/evidence/#save_evidence) | Store a proof-of-concept artifact (file, screenshot, or text) |
| [`list_evidence`](/docs/tools/evidence/#list_evidence) | List evidence items, optionally filtered by finding |
| [`get_evidence`](/docs/tools/evidence/#get_evidence) | Retrieve stored evidence content by ID |
| [`capture_command_output`](/docs/tools/evidence/#capture_command_output) | Run a command and save its output as evidence |

## Enrichment and ingestion

5 tools, [full parameters](/docs/tools/enrichment-and-ingestion/).

| Tool | What it does |
|------|-------------|
| [`enrich_domain`](/docs/tools/enrichment-and-ingestion/#enrich_domain) | DNS enrichment: A/AAAA/CNAME/MX/NS/TXT records and reverse DNS |
| [`enrich_ip`](/docs/tools/enrichment-and-ingestion/#enrich_ip) | Reverse DNS on an IP address |
| [`enrich_service`](/docs/tools/enrichment-and-ingestion/#enrich_service) | Probe an HTTP(S) service for headers, redirects, and fingerprints |
| [`ingest_scanner_output`](/docs/tools/enrichment-and-ingestion/#ingest_scanner_output) | Parse and import output from nmap, nuclei, httpx, or ffuf |
| [`ingest_masscan`](/docs/tools/enrichment-and-ingestion/#ingest_masscan) | Parse masscan XML or JSON and import hosts and open ports |

## Engagement management

10 tools, [full parameters](/docs/tools/engagement-management/).

| Tool | What it does |
|------|-------------|
| [`create_engagement`](/docs/tools/engagement-management/#create_engagement) | Create a new pentest engagement (task + exploration graph) |
| [`list_engagements`](/docs/tools/engagement-management/#list_engagements) | List all engagements with summary stats |
| [`switch_engagement`](/docs/tools/engagement-management/#switch_engagement) | Change the active engagement without restarting |
| [`update_engagement`](/docs/tools/engagement-management/#update_engagement) | Update the current engagement's goal or description |
| [`delete_engagement`](/docs/tools/engagement-management/#delete_engagement) | Permanently delete an engagement and all its data |
| [`engagement_status`](/docs/tools/engagement-management/#engagement_status) | Get goal, status, finding count, and coverage summary |
| [`engagement_activity`](/docs/tools/engagement-management/#engagement_activity) | Recent activity timeline: findings, assets, graph nodes |
| [`goal_met`](/docs/tools/engagement-management/#goal_met) | Mark the entire engagement as complete |
| [`list_coverage_gaps`](/docs/tools/engagement-management/#list_coverage_gaps) | Identify untested assets, unproven goals, and missing enrichment |
| [`list_sessions`](/docs/tools/engagement-management/#list_sessions) | List the MCP client sessions connected to this server |

## Activity

2 tools, [full parameters](/docs/tools/activity/).

| Tool | What it does |
|------|-------------|
| [`list_activity`](/docs/tools/activity/#list_activity) | List recent activity log entries for the engagement |
| [`log_activity`](/docs/tools/activity/#log_activity) | Record a manual activity entry |

## Reporting and export

3 tools, [full parameters](/docs/tools/reporting-and-export/).

| Tool | What it does |
|------|-------------|
| [`generate_report`](/docs/tools/reporting-and-export/#generate_report) | Generate a Markdown penetration test report from the current engagement |
| [`export_engagement`](/docs/tools/reporting-and-export/#export_engagement) | Export an engagement to a portable JSON archive |
| [`import_engagement`](/docs/tools/reporting-and-export/#import_engagement) | Import an engagement from a JSON archive |

## Traffic capture

11 tools, [full parameters](/docs/tools/traffic/). Requires `--proxy` on `serve`.

| Tool | What it does |
|------|-------------|
| [`traffic_search`](/docs/tools/traffic/#traffic_search) | Search captured exchanges by host, URL, or body content |
| [`traffic_get`](/docs/tools/traffic/#traffic_get) | Read the full request and response of a captured exchange |
| [`traffic_blob`](/docs/tools/traffic/#traffic_blob) | Page through oversized bodies by blob hash |
| [`traffic_messages`](/docs/tools/traffic/#traffic_messages) | List WebSocket or SSE messages of a captured stream |
| [`traffic_replay`](/docs/tools/traffic/#traffic_replay) | Replay a captured request with optional modifications |
| [`traffic_hosts`](/docs/tools/traffic/#traffic_hosts) | List all recorded target hosts with exchange counts |
| [`traffic_har_export`](/docs/tools/traffic/#traffic_har_export) | Export captured traffic as a HAR 1.2 JSON document |
| [`traffic_stats`](/docs/tools/traffic/#traffic_stats) | Aggregate statistics: hosts, status codes, methods, response sizes |
| [`traffic_timeline`](/docs/tools/traffic/#traffic_timeline) | Traffic activity over time, bucketed by minute, hour, or day |
| [`traffic_patterns`](/docs/tools/traffic/#traffic_patterns) | Most-requested paths, error-heavy hosts, and newly seen endpoints |
| [`traffic_clear`](/docs/tools/traffic/#traffic_clear) | Delete captured traffic for matching hosts |
