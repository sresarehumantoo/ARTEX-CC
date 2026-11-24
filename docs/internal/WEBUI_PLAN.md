# ARTEX Web UI Plan

## 1. ARTEX UI survey

ARTEX's web frontend is a Next.js 16 application (React 19, TypeScript, Tailwind CSS 4,
shadcn/ui, Zustand for state) with ~25 page routes, ~25 custom components, and a full
shadcn ui library. It talks to a Go HTTP API server via `/api/*` proxy rewrites and
receives real-time updates through an SSE-based Broadcaster.

### Screens and features (from screenshots and source)

**Sidebar navigation** splits into two groups:

Functions: Dashboard, Chat, Tasks, Findings, Traffic, Tool Execution, LLM Recording,
Assets, Asset Sync, Workspace.

System: LLM, Agent, MCP, Skill, Tools, Notifications, Interception Rules, Asset
Interception, Approval Records, Logs, System Settings.

**Dashboard** (not screenshotted but route exists): engagement overview, stats.

**Chat**: Multi-conversation interface with agent selector. Human-in-the-loop
messaging with the main agent. Shows tool executions inline (bash commands with
output). Supports `@` mentions to reference records. Side-question workspace for
one-shot Q&A without the agent harness.

**Tasks**: Table of tasks with ID, name, description, goal, status (Running/Paused/
Completed), goal progress (e.g., 2/5), finding counts by severity, and action buttons.
Task detail view has 11 tabs: Sessions, Overview, Exploration graph, Broadcast,
Findings, Retests, Test assets, Credentials, Asset coverage, Interception approval,
Report.

**Sessions** (task detail tab): Left panel shows the session tree: Main Agent with
sessions, Planner, System audit, Workers (numbered with status dots and "Live" badges).
Right panel shows the conversation for the selected session with token counts (In 1.3M,
Cache 890k, Out 96k), elapsed time, model indicator.

**Exploration graph** (task detail tab): Interactive canvas using `@xyflow/react`
(React Flow). Node types: Start (slate), Goal (green), Intent (blue), Fact (amber),
Vulnerability/Finding (rose), Hint (violet), Digest (teal). Edge types: spawns (green),
intent chain/derived_from (indigo), yields (amber), proves (red). Layered left-to-right
layout with auto-computed positions. Running intents have animated ping indicators and
animated edges. Click-to-select opens a detail drawer (Sheet) with properties, raw JSON,
and digest member navigation. MiniMap in corner. Legend panel. Digest nodes collapse
their covered members into a super-node. The layout algorithm uses longest-path
layering with barycenter sweep ordering, cycle-safe DFS.

**Findings**: Summary cards (Total/Pending/Critical/High/Medium/Low counts). Sortable
table with severity badges, vulnerability name, description, traffic evidence count,
affected asset, status (Confirmed/Pending), task, timestamp, and actions (Retest,
Deepen, Delete). Filterable by severity, status, verification state, type, task. Export
button. Views: All findings, By task, By asset.

**Traffic**: HTTP history browser (like Burp Suite). Recording indicator with port and
record count. Host/target selector, purge controls, search bar, method filter,
pagination. Table columns: Time, Host, Method (color-coded GET/POST badges), URL,
Status (color-coded), Content-Type, Response size. Advanced filters: response content
body keyword, path pattern, status filter, response size range. Flow selection with
"Link to finding" action.

**Assets**: Tabbed view by asset type (Companies, Root domains, IP, Subdomains, Apps,
Services, Endpoints) with counts. DSL search bar (e.g., `domain=example AND
status_code>=400`). Table displays type-specific columns (Domain, Root domain, Record
type, Record value for subdomains). Pagination. Add company button, refresh.

**Asset coverage** (task detail tab): Force-directed bubble chart using `@antv/g6`
showing asset relationships. Node types color-coded by asset type (Company, Root domain,
Subdomain, IP, Service, App, Endpoint). Legend shows In scope count, Tested count.
Tested nodes highlighted green, untested gray, out of scope faded. Drag and zoom.

**Agents**: Card grid of agent definitions (Finding retest, Goal decomposer, Planner,
Main agent, Work Agent, Recon bot). Each card shows name, slug, Built-in/Custom badge,
description, and tool/skill/MCP counts. Create agent button. Detail page for editing.

**LLM**: Model profile cards (Claude Opus 4.8, DeepSeek V4) with provider, model ID,
health status (Healthy/Tripped), rate counters, priority. Pool settings toggle, retry
and backoff configuration tab.

**Approval Records** (Interception): Table with pending/all records. Columns: #, Tool,
Source (task/worker), Matched rule, Arguments, Status (Pending/Allowed/Denied), Decided
by (Model/Legacy), timestamps. Filterable by approval status, decision source, actor.

**Logs**: Live log stream with level filtering (All/info/warn/error), text/tag filter,
Pause/Clear controls. Color-coded by source (engine, config, planner, guard, worker)
and severity (warnings orange, errors red).

**Other screens**: Tool Execution (command history), LLM Recording (inference
recording/replay), Asset Sync (ScopeSentry integration), Workspace (file browser),
MCP (MCP server configs), Skill (skill packs), Tools (tool registry), Notifications
(push channel config), System Settings, Interception Rules (rule editor).


## 2. Compliance analysis

ARTEX's compliance model: Claude Code is Anthropic's official client making inference
calls against the user's subscription. ARTEX is a tool server. The web UI is a data
viewer/editor for the same PostgreSQL and SQLite stores that the MCP tools read and
write. It never touches Claude's inference.

### Can port directly (pure data display/editing)

| ARTEX screen | Rationale |
|---|---|
| **Assets** | Reads/writes the `assets` table. No LLM involvement. |
| **Findings** | Reads `findings` table. Severity, evidence, status are all stored data. |
| **Traffic** | Reads the SQLite traffic index and blob store. Pure HTTP history viewer. |
| **Exploration graph** | Reads `exploration_nodes` and `exploration_edges`. The visualization is a React Flow canvas over stored graph data. |
| **Credentials** | Reads `task_credentials` (decrypted at the Go layer). |
| **Coverage** | Reads `coverage_ledger`. |
| **Dashboard** | Aggregates from PG: finding counts, asset counts, graph stats, task status. |
| **Tasks** (list and overview) | Reads `tasks` and `explorations`. |
| **Scope** | Reads/writes `task_scope` and `company_scope`. |
| **Companies** | Reads/writes `companies`. |
| **Intercept rules** | Reads/writes `intercept_rules` and `intercept_pending`. |
| **Activity log** | Reads the `activity` table. |
| **Report** | The report is generated server-side from stored data (findings + assets + coverage). No LLM. |

### Needs adaptation

| ARTEX screen | What changes |
|---|---|
| **Sessions/Broadcast** | ARTEX shows live agent sessions (planner, workers) with token counts and conversation transcripts. ARTEX has no agent loop; Claude Code IS the agent. The activity log and graph updates serve as the equivalent "live feed." Replace with an Activity Feed that shows tool calls, findings, graph mutations as they happen. |
| **Asset coverage graph** | ARTEX uses `@antv/g6` for the bubble chart. ARTEX can replicate this from the same underlying data (assets + coverage ledger), but the data is simpler (single-task, no multi-task overlay). |
| **Approval Records** | ARTEX's intercept review has an "ask the operator" flow with pending/allowed/denied. ARTEX has `intercept_pending` in the schema. The web UI could show pending intercepts and allow approve/deny, but the MCP protocol doesn't have a notification channel back to Claude Code, so the tool call just times out or uses the timeout_action. Still useful as audit trail and for pre-approving via rule changes. |

### Cannot port (requires LLM API access)

| ARTEX screen | Why |
|---|---|
| **Chat** | The chat interface sends messages to the LLM via the agent loop. ARTEX cannot relay inference. Claude Code's terminal IS the chat. |
| **LLM configuration** | Model profiles, provider pool, retry/backoff. Claude Code manages its own model; ARTEX has no LLM pool. |
| **Agent configuration** | Planner/worker agent definitions with prompt editing. Claude Code is the agent; there are no configurable agent roles. |
| **Side questions** | One-shot LLM Q&A. Would require an API call. |
| **MCP server config** | ARTEX configures outbound MCP connections. ARTEX IS an MCP server; it doesn't consume others. |
| **Skill management** | ARTEX's skill packs are agent-side features. Claude Code has its own skill system. |
| **Tool registry editor** | ARTEX lets you enable/disable tools for agents. ARTEX's tools are fixed at compile time. |
| **Notifications** | ARTEX pushes to Telegram/email/webhook. Could theoretically be ported (it's not LLM-dependent) but is low value for a single-user CLI tool. |
| **LLM Recording** | Records and replays LLM inference calls for debugging. ARTEX doesn't see inference. |
| **Sessions** (live conversation view) | Shows token-level agent conversations. ARTEX doesn't see Claude Code's conversation. |

### Not applicable

| ARTEX screen | Why |
|---|---|
| **Asset Sync** | ScopeSentry integration, specific to ARTEX's deployment. |
| **Workspace** | File browser for the agent's working directory. Claude Code uses the host filesystem directly. |
| **Tool Execution** (command viewer) | Shows bash command history from the agent. Claude Code's command execution is its own concern. Could show the activity log entries for tool calls instead. |


## 3. ARTEX web UI design

### 3.1 Dashboard

The landing page. Shows engagement health at a glance.

**Top bar**: Engagement name, goal text, task status badge (Running/Paused/Done), elapsed
time since creation.

**Stat cards row**: Total assets (with breakdown by type), Total findings (with severity
breakdown: Critical/High/Medium/Low), Coverage entries, Graph nodes (with breakdown:
goals met/total, intents open/running/done), Credentials found.

**Findings by severity**: Horizontal stacked bar or donut chart. Critical red, High
orange, Medium yellow, Low blue.

**Recent activity feed**: Last 20 activity entries in reverse chronological order. Each
entry shows timestamp, worker, tool name, summary, error indicator. Auto-updates via SSE.

**Graph overview mini**: Small version of the exploration graph (read-only, no
interaction needed) showing current shape and size. Click to navigate to the full graph
page.

**Coverage summary**: Table or heatmap of surface vs. risk_area with outcome coloring
(reported = green, no_issue_found = gray, needs_follow_up = amber, not tested = empty).

### 3.2 Exploration graph

The showcase feature. Full-page interactive canvas.

**Layout**: Same layered left-to-right layout as ARTEX. Port the layout algorithm
directly (it's pure geometry with no ARTEX-specific dependencies).

**Node types**: Same as ARTEX: Start (origin fact), Goal, Intent, Fact, Finding, Hint.
Color-coded icon badges. Node cards show kind icon, label text (from payload), state
badge (for goals and intents), priority indicator, node ID.

**Edge types**: spawns, derived_from, yields, proves. Color-coded with labels.

**Interactions**: Click node to open detail drawer. Drag nodes. Zoom and pan. MiniMap.
Fit-to-view. Legend panel.

**Detail drawer**: Properties (type, state, priority, origin, timestamp), payload data
(formatted JSON or extracted summary), raw JSON with copy button.

**Live updates**: SSE stream pushes graph mutations. New nodes appear with an entrance
animation. Running intents pulse.

**Digest collapsing**: Same algorithm as ARTEX. Active digests fold their covered
members; clicking a digest shows its members in the detail drawer.

### 3.3 Findings

**Summary row**: Stat cards for total, pending, by severity (matching ARTEX's layout).

**Table**: Severity badge, name, summary, evidence preview, affected asset(s), status,
task, timestamp. Sortable by severity, time. Filterable by severity buttons
(All/Critical/High/Medium/Low), status dropdown, search text.

**Detail view** (expand or navigate): Full finding record. Evidence text. Linked traffic
(if finding_traffic_bindings exist). Affected assets list. Status management
(confirm/dismiss via PATCH). Graph lineage: show the subgraph from origin to this
finding's node.

**Export**: CSV export. Markdown report generation (the report tool already exists in
ARTEX).

### 3.4 Assets

**Type tabs**: Companies, Root domains, IP, Subdomains, Apps, Services, Endpoints, each
with count badge. Same as ARTEX.

**DSL search**: The asset DSL search engine already exists in ARTEX
(`store/asset_dsl.go`). Wire it to a search bar.

**Table**: Type-specific columns (matching ARTEX). Company attribution column. Task
association.

**Actions**: Add company (opens a form), refresh. Click asset to see detail: all fields,
linked findings, coverage entries, graph anchors.

### 3.5 Traffic

**HTTP history table**: Time, Host, Method (color-coded badge), URL, Status (color-coded),
Content-Type, Response size. Sortable, paginated.

**Filters**: Host selector, method filter, status code filter, path search, response
content search, size range. These map directly to the existing `traffic.Search()` API.

**Detail view**: Click a row to see full request/response. Syntax-highlighted HTTP
headers, body viewer with auto-detection (JSON pretty-print, HTML, raw). Copy buttons.

**Recording indicator**: Show proxy status (recording/stopped), listen address, record
count.

**HAR export**: Button to export current filter results as HAR 1.2 (the HAR export
already exists in ARTEX's traffic package).

**Link to finding**: Select flows, link them to a finding via `finding_traffic_bindings`.

### 3.6 Coverage

**Ledger table**: Surface, risk area, outcome (color-coded badge), evidence, reason,
worker, intent ID, timestamp. Filterable by outcome.

**Heatmap view**: Matrix of surface (rows) vs. risk area (columns). Cell color by
outcome. Hover for details. Empty cells = not yet tested (the gaps).

**History**: Coverage ledger history table showing how assessments changed over time.

### 3.7 Credentials

**Table**: Domain, username, password (masked by default, click to reveal), source
(user/agent), note, timestamp. Filtered by domain search.

**Add credential**: Manual entry form.

**Security**: Passwords decrypted server-side, transmitted over localhost only. The UI
should show a warning if `ARTEX_CREDENTIAL_KEY` is not set.

### 3.8 Scope and constraints

**Task scope**: Table of scope rules (kind, domain/net/value, source, reason). Add/remove
rules. Same as the existing `set_task_scope` / `get_task_scope` tools.

**Company scope**: Per-company scope rules.

**Constraints**: Allow/deny constraints for the exploration. Add/edit/remove.

### 3.9 Intercept

**Rules table**: Name, enabled toggle, priority, match target, match type, pattern,
action (allow/deny/ask), timeout config. Add/edit/delete.

**Pending decisions**: Table of `intercept_pending` records with tool name, input, status,
timestamps. For "ask" actions, show approve/deny buttons (write back to the table, which
the intercept check in the tool pipeline reads).

### 3.10 Activity feed

Replaces ARTEX's Sessions/Broadcast/Logs screens with a unified activity stream.

**Live stream**: Auto-scrolling log of activity entries from the `activity` table. Each
entry shows: timestamp, worker, tool name, node ID (linkable to graph), summary, error
flag. Color-coded by kind (tool_use, tool_result, error).

**Filters**: By worker, by tool, by node, errors only, time range.

**SSE-driven**: New entries stream in real-time.

### 3.11 Report

**Preview**: Rendered markdown report (generated from findings + assets + coverage by
the existing `report/` package).

**Export**: Download as markdown file. PDF export (using the browser's print-to-PDF or a
server-side renderer).

### 3.12 Settings

**Engagement info**: Task ID, exploration ID, goal, description (read-only display of
`.artex.json`).

**Proxy config**: Current proxy address, CA cert path (informational).

**Database**: Connection status indicator.


## 4. Tech stack

### 4.1 ARTEX's stack (reference)

- Next.js 16 / React 19 / TypeScript
- Tailwind CSS 4 / shadcn/ui (radix-ui)
- `@xyflow/react` for the exploration graph
- `@antv/g6` for the asset coverage bubble chart
- Recharts for dashboard charts
- Zustand for client state
- Biome for linting
- Separate Node.js dev server, proxied to Go backend

### 4.2 Recommendation for ARTEX

**Embed the web UI in the Go binary using `embed.FS`.** Reasons:

1. ARTEX is distributed as a single binary. Users run `artex serve` and everything
   works. Adding a separate `npm install && npm run dev` step would be hostile to the
   target audience (pentesters who want to `go install` and go).

2. The web UI is a companion dashboard, not the primary interface. It doesn't need SSR,
   ISR, or any Next.js server features. A static SPA is sufficient.

3. The Go server already has an HTTP listener (for the MCP Streamable HTTP transport on
   the `--listen` address). Adding a file server handler for the embedded SPA is trivial.

4. ARTEX's Next.js stack is excellent for ARTEX (which is a full-time web application
   with auth, i18n, theme presets, etc.), but overkill for ARTEX's read-mostly
   dashboard.

**Frontend framework**: React with Vite (not Next.js).

- Vite produces a static `dist/` directory that embeds cleanly into Go.
- React because the exploration graph component (`@xyflow/react`) is React-only, and
  porting the graph visualization is the highest-value feature.
- TypeScript, Tailwind CSS, shadcn/ui for consistency with ARTEX's look and feel.

**Graph visualization**: `@xyflow/react` (React Flow).

- ARTEX already has a production-quality exploration graph component using React Flow.
- The component is ~670 lines of self-contained React/TypeScript with no ARTEX-specific
  backend dependencies (it takes `nodes: TaskNode[]` and `edges: Edge[]` as props).
- The layout algorithm (layered DAG with barycenter ordering) is embedded in the
  component. It can be ported directly.
- The minimap, controls, legend, and detail drawer all come along for free.

**Coverage visualization**: Replace `@antv/g6` with a simpler approach. The asset
coverage bubble chart is visually appealing but `@antv/g6` is a heavy dependency
(~500KB gzipped). Options:

- Use React Flow again (it handles arbitrary node graphs, not just DAGs).
- Use D3 directly for a force-directed layout (adds ~90KB).
- Start with a table/heatmap view (zero additional dependencies) and add the graph later.

Recommendation: table/heatmap for MVP, D3 force graph for the full version.

**Dashboard charts**: Recharts (same as ARTEX, ~45KB gzipped). Used for the severity
distribution chart and any time-series visualizations.

**State management**: Zustand or just React state/context. The UI is simple enough that
Zustand might be unnecessary for MVP; a few `useState` hooks and SWR-style data fetching
are sufficient.

**Data fetching**: The Go server exposes a REST API (new endpoints, separate from the
MCP protocol). The frontend fetches JSON.

**Real-time updates**: Server-Sent Events (SSE) from the Go server. One SSE endpoint
per entity type (or a single multiplexed stream with event types). The Go server watches
for database changes using PostgreSQL `LISTEN/NOTIFY` or periodic polling and pushes
events to connected browsers.

SSE is simpler than WebSocket for this use case (unidirectional server-to-client, text
events, auto-reconnect built into the EventSource API). It's what ARTEX's Broadcaster
already uses.

### 4.3 Build and embedding

```
web/
  src/
    main.tsx
    pages/
      Dashboard.tsx
      Graph.tsx
      Findings.tsx
      Traffic.tsx
      Assets.tsx
      Coverage.tsx
      Credentials.tsx
      Activity.tsx
      Report.tsx
      Settings.tsx
    components/
      exploration-graph.tsx   (ported from ARTEX)
      ...
  package.json
  vite.config.ts
  tsconfig.json

cmd/artex-cc/
  webui.go                   (embed.FS + http.FileServer)
```

Build process: `cd web && npm run build` produces `web/dist/`. The Go build embeds it:

```go
//go:embed web/dist
var webUI embed.FS

func webUIHandler() http.Handler {
    sub, _ := fs.Sub(webUI, "web/dist")
    return http.FileServerFS(sub)
}
```

The existing `--listen` address serves both MCP (on `/mcp`) and the web UI (on `/`).
Visiting `http://localhost:9400/` in a browser shows the dashboard. No new flags needed.

For development: `cd web && npm run dev` runs Vite's dev server with hot reload, proxying
`/api/*` to the Go server.

### 4.4 API endpoints

New REST endpoints on the Go server (alongside the existing MCP endpoint):

```
GET  /api/engagement              engagement info + summary stats
GET  /api/graph                   exploration nodes + edges
GET  /api/findings                paginated findings list
GET  /api/findings/:id            single finding detail
GET  /api/assets                  paginated, with DSL search query param
GET  /api/assets/:id              single asset detail
GET  /api/traffic                 paginated traffic search
GET  /api/traffic/:id             single exchange (request + response)
GET  /api/traffic/export/har      HAR export
GET  /api/coverage                coverage ledger
GET  /api/credentials             credentials list (decrypted)
GET  /api/scope                   task scope rules
GET  /api/constraints             exploration constraints
GET  /api/intercept/rules         intercept rules
GET  /api/intercept/pending       pending intercept decisions
POST /api/intercept/pending/:id   approve/deny a pending intercept
GET  /api/activity                paginated activity log
GET  /api/report                  generated markdown report
GET  /api/events                  SSE stream (graph updates, new findings, activity)
```

These endpoints reuse the existing `store.Store` and `traffic.Traffic` instances. No new
database connections. They are simple read handlers (mostly `SELECT` queries serialized
as JSON) with a few write endpoints for scope, constraints, intercept decisions, and
credential management.


## 5. The pitch

### How it works together

Claude Code runs in your terminal. You type `claude` and start a conversation. Claude
calls ARTEX's MCP tools to drive the pentest: inserting assets, adding intents,
recording facts, reporting findings, analyzing traffic.

Meanwhile, you have `http://localhost:9400` open in your browser. The web UI shows:

**The exploration graph grows in real-time.** You watch as Claude discovers subdomains
(new fact nodes appear), forms attack plans (intents branch off), and reports
vulnerabilities (finding nodes light up red). The graph is a live map of the
engagement's progress. Running intents pulse. Completed goals turn green.

**Findings appear as they're discovered.** A new critical finding pops up. You click it,
see the evidence, see the linked traffic (the actual HTTP requests that proved the
vulnerability). You can confirm it, mark it as false positive, or add a note.

**Traffic flows through the proxy.** Every HTTP request Claude's tools make is captured.
You can search through it like Burp Suite's HTTP history. Click a request, see the full
headers and body. Filter by host, method, status code. Export to HAR for your report.

**You steer the engagement from the browser.** Add a hint in the UI: "The admin panel
uses JWT auth, check for weak signing keys." Claude picks it up on its next planning
cycle. Adjust scope: add a new subdomain that's in-scope, remove one that isn't. Toggle
an intercept rule to block destructive commands.

**Coverage shows what's been tested and what hasn't.** The heatmap highlights gaps. You
see that authentication testing is done on the main app but not on the API. You tell
Claude in the terminal: "Focus on API authentication next."

### The value proposition

For the user, ARTEX's web UI provides what no other AI-powered pentest tool offers:

1. **Transparency.** You see exactly what the AI is doing, in real-time, with full
   detail. The exploration graph is not a black box; it's a live visualization of the
   AI's reasoning and progress.

2. **Control.** You can steer, constrain, and override the AI from a proper GUI without
   leaving the terminal conversation. Hints, scope changes, and intercept rules give you
   the knobs.

3. **Professional output.** The findings view, traffic browser, and report generator
   produce deliverables you can hand to a client. This is not a toy; it's a pentest
   workbench.

4. **No API key.** Your Claude Pro/Team/Enterprise subscription is the engine. The web
   UI is just a window into the data. No separate billing, no token counting, no API key
   management.

5. **Single binary.** `go install`, `artex init`, `artex serve`. Open the browser.
   The web UI is embedded in the binary. Zero frontend deployment.

### The marketing angle

> ARTEX gives Claude Code an exploration graph, a traffic proxy, and a web dashboard.
> Think of it as Burp Suite's data layer, operated by an AI agent, visible in your
> browser. Your Claude subscription is the only key you need.


## 6. MVP scope

### What ships first

The MVP web UI includes exactly five screens that demonstrate the core loop: Claude
acts, you watch, you steer.

1. **Dashboard**: Engagement summary, stat cards, recent activity feed. The "home" that
   proves the tool is alive and working.

2. **Exploration graph**: The full interactive React Flow graph. This is the demo
   feature, the one that makes people stop scrolling on GitHub. Node types, edge types,
   color coding, detail drawer, MiniMap, legend. Live updates via SSE.

3. **Findings**: Summary cards + sortable/filterable table + detail view. The
   deliverable-quality output.

4. **Traffic**: HTTP history table with search, filtering, and request/response detail
   viewer. Proves the proxy integration works.

5. **Assets**: Type-tabbed table with DSL search. Shows the inventory.

### What the MVP excludes

- Coverage heatmap (ship as a plain table first, heatmap later)
- Credentials page (low priority vs. the five above)
- Report preview (the tool generates markdown; rendering it in the browser can wait)
- Scope/constraint editing (use the MCP tools from the terminal for MVP)
- Intercept review (audit trail only, no approve/deny buttons)
- Activity feed as a standalone page (the dashboard includes a mini-feed)
- Settings page (informational, not critical)

### Effort estimate

| Component | Effort | Notes |
|---|---|---|
| Go API endpoints (REST handlers) | 2-3 days | Thin wrappers around existing store/traffic queries. |
| SSE event stream | 1 day | LISTEN/NOTIFY or poll-and-push. Single endpoint. |
| Vite + React scaffold | 0.5 days | Standard create-vite template + Tailwind + shadcn. |
| Exploration graph component | 1-2 days | Port from ARTEX. Mostly copy-paste with type adjustments. |
| Dashboard page | 1 day | Stat cards + activity feed. |
| Findings page | 1 day | Table + detail view. |
| Traffic page | 1-2 days | Table + detail viewer with syntax highlighting. |
| Assets page | 1 day | Tabbed table + DSL search wiring. |
| Go embed integration | 0.5 days | embed.FS, file server, build script. |
| Testing and polish | 1-2 days | Cross-browser, dark mode, responsive basics. |

**Total MVP: ~10-14 days of focused work.**

The exploration graph component is the highest-risk item because it's the most complex
React code, but ARTEX's implementation is a strong starting point; the layout algorithm
and React Flow configuration port with minimal changes.

### Build order

1. Go API endpoints (can test with curl immediately)
2. Vite scaffold + embed pipeline (verify the single-binary story works)
3. Exploration graph (the hero feature)
4. Dashboard (needs the graph mini-view and activity feed)
5. Findings
6. Traffic
7. Assets


## 7. Full vision

### Beyond MVP

**Coverage heatmap**: Interactive matrix visualization. Surface vs. risk area. Color by
outcome. Click to drill into the evidence. This shows engagement completeness at a
glance, which is high value for professional pentests.

**Report builder**: Live preview of the generated report. Drag to reorder findings.
Edit summaries inline. Export to markdown, HTML, or PDF. Include traffic evidence
screenshots.

**Credential vault**: Full credential management with search, manual entry, and
audit trail. Masked display with click-to-reveal. Source attribution (found by which
intent, at what time).

**Scope editor**: Visual scope management. Add/remove domains, IPs, CIDRs. See which
assets fall within scope and which don't. Toggle between company scope and task scope
views.

**Constraint editor**: Add/edit allow/deny constraints with the exploration graph
visible alongside, so you can see which parts of the graph a constraint affects.

**Intercept review**: Approve/deny pending tool calls from the browser. Push
notification (browser Notification API) when a new intercept is pending. This turns the
web UI into a true control panel.

**Activity timeline**: Full-page activity stream with filtering, search, and
timeline visualization. Tool call details (input/output), execution timing, error
details. Click an activity entry to jump to its graph node.

**Multi-engagement**: Engagement switcher in the top bar. Compare findings across
engagements. Archive completed engagements.

**Collaboration view**: When multiple Claude Code instances work the same engagement
(P4.6.3 in the roadmap), the web UI shows all of them: which intents each is working,
their activity streams interleaved, coordination status.

**Dark mode**: Both light and dark themes, respecting system preference. ARTEX supports
this via next-themes; ARTEX can use Tailwind's dark mode with a simple toggle.

**Mobile-responsive**: The dashboard and findings pages should be usable on a phone or
tablet for quick status checks during an engagement.

### The complete experience

A penetration tester opens three things:

1. A terminal running Claude Code with ARTEX connected.
2. A browser tab at `localhost:9400` showing the ARTEX dashboard.
3. Their browser configured to use the ARTEX proxy at `:8788`.

They type: "Start a penetration test against acme.com. The goal is to gain admin access
and demonstrate that user data can be exfiltrated."

Claude sets up the engagement, runs reconnaissance, discovers assets, plans attack paths,
and executes them. The web UI shows it all happening: the graph grows, findings appear,
traffic fills up. The tester watches, steers when needed, and at the end clicks "Export
Report" to get a client-ready deliverable.

That is the full vision: an AI-driven pentest with human oversight, professional
tooling, and zero API key friction.
