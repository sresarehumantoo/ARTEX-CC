# Changelog

All notable changes to ARTEX are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.7.0] - 2026-11-26

### Upgrade notes
- Migrations 0004 through 0008 apply at the first start against an existing database. 0006 reopens met goals whose only proofs are dismissed or merged findings. 0007 rewrites stored credential domains to their normalized form (trimmed, lowercased, one trailing dot dropped) and merges rows that then collide (same task, domain and username). 0008 rebuilds the target change journal's live-change index so POSIX paths match case-sensitively.
- At the first `serve` after upgrading, asset URLs are normalized once: userinfo is stripped and saved as an `asset_url` credential, and scheme, host case and default ports are normalized. Load `ARTEX_CREDENTIAL_KEY` as usual before that start. If this backfill fails, `serve` exits instead of starting.
- The web UI workspace is now rooted at `<data>/workspace` instead of the data directory itself. Nothing is moved: files you kept at the top of the data directory no longer show in the workspace pane until you move them into `workspace/`. The chat agent still runs with the data directory as its working directory, so files it writes there do not appear in the workspace pane either.
- At the first `serve` or `init` after upgrading, in a data directory an earlier ARTEX version already used, group and other access is removed once from the files and directories ARTEX owns there (see Security). This rewrites their modes, so it also clears a setgid bit on those directories. Run `init` and `serve` as the same user; a walk that cannot change an entry logs a warning and is retried at the next start.
- A request arriving on a loopback address with a Host other than `localhost` or a loopback IP gets 421, and a browser write (POST, PUT, PATCH, DELETE) whose `Origin` does not match the Host, or whose `Sec-Fetch-Site` is `cross-site` or `same-site`, gets 403. A reverse proxy on the same host that forwards the public Host hits the first; one that rewrites Host to `localhost` hits the second for web UI writes. There is no allow-list; see [Troubleshooting](https://artex-cc.com/docs/troubleshooting/) and reach the server on a non-loopback address instead.
- The REST API no longer sends `Access-Control-Allow-Origin: *`. Browser pages on another origin, including another port on localhost, can no longer call it. Responses also carry `X-Frame-Options: DENY`, so the web UI cannot be embedded in a frame.
- Setting the first web UI password needs a one-time setup token that the server prints to its logs at startup (`docker compose logs artex` for the Docker image). Scripts calling `POST /api/auth/init` must send it as `setup_token`.
- `artex-cc init` prints the DSN password as `***` in the MCP config snippet and in `--dry-run --format json`. Substitute the real password, or pass the DSN via `ARTEX_PG`.
- `docker-compose.yml` now builds the default `ARTEX_PG` from `POSTGRES_USER`, `POSTGRES_PASSWORD` and `POSTGRES_DB`. A password with characters that are special in a URL (`@`, `:`, `/`, `?`, `#`, `%`) must be URL-encoded, or set `ARTEX_PG` yourself.
- Company scope is now enforced. An engagement scoped only by `company` entries goes from allowing every target to allowing only those companies' domain, IP and CIDR lines, and a `company` entry whose company has none of those lines fails closed. Add the missing lines with `add_company_scope`.
- Company scope lines written as `*.example.com` now match subdomains only, never the apex. A `*` anywhere other than a leading `*.` is rejected, as is a wildcard `root_domain` task scope entry.
- `engagement_status` reports `open` or `achieved` instead of always `active`.
- `complete_intent` and `cancel_intent` on an open intent now claim it first, so they need a claim holder (session, identity or `owner`) and fail if another holder claimed it. `add_intent` skips an intent that duplicates an open or running one, returning 0 in its `ids` slot.
- `record_fact`, `report_finding` and `record_coverage` take an optional `owner`. Clients that claim only with `owner` (no session or identity, as on MCP protocol 2026-07-28) should pass the same `owner` so recording results renews their claims.
- Eval harness only: the eval Postgres in `eval/docker-compose.yml` is now published on `127.0.0.1:5433`, and the eval runner's default DSN uses that port.

### Added
- Target change journal (migration 0005). `record_target_change` records a change to a target (target, resource, kind, original fingerprint, backup, intent) before it is made; when another unrestored change on the same target and resource exists, it returns a warning naming who recorded it and the baseline, the first recorded original fingerprint, which is what to restore to. `record_target_restore` takes change ids and the fingerprint observed after restoring, and marks changes restored only on an exact match with the baseline; a mismatch is recorded as a failed attempt. `graph_overview` and `list_changes` list `unrestored_target_changes`, `generate_report` has a Target changes section, and engagement export and import carry the journal. The orchestration guide tells agents to record changes before making them and never to report a host restored while unrestored changes remain.
- CI runs `govulncheck` against the module on every push and pull request.
- Eval runner: each instance gets its own temp directory (`transcripts-<scenario>-<timestamp>/tmp-instance-<i>`, set as `TMPDIR`, `TMP` and `TEMP`), every prompt tells agents to keep scratch files there instead of `/tmp`, and each scorecard run records `temp_dir` and a `scratch` section listing the files left behind, flagging any holding a PEM private key.
- Eval runner: the coordination scorecard adds `unclaimed_work` (per instance work done while holding no live claim, and intents finished without being claimed) and `concurrent_intent_work` (pairs of instances whose work on one intent overlaps). Canceling an unclaimed intent the instance created itself is listed as `own_duplicate_cancels`, not as an unclaimed finish.
- Eval runner: the claim replay models implicit claim renewal when the server supports it, reported as `implicit_renewal` and `implicit_renewals`.
- Eval runner: the Winforge three-track target and the `winforge-multitrack.yml` scenario.

### Changed
- A claim on a running intent renews when its holder records results against it: `record_fact`, `report_finding` with `intent_id`, and `record_coverage` with the new `intent_id` argument extend the lease. Only the holder renews, a claim with more than half its lease left is not rewritten, and a lapsed claim nobody has taken over is revived. The three tools also take `owner` and renew the claims of the holder it resolves to, exactly as `start_intent` resolves it, so a client that claimed with `owner` and has no session or identity (MCP HTTP protocol 2026-07-28), or a subagent that claimed with its own `owner` inside a session, is renewed as that holder. Without `owner` they renew the session's or identity's claims and those of every `owner` within it.
- `complete_intent` and `cancel_intent` on an open intent now claim it first and then close it, refusing if another holder got there first, and record the closer's holder label as `completed_by` or `cancelled_by`. Completing an intent that was never started also returns a reminder to call `start_intent` before working an intent. Closing an open intent now needs a claim holder (session, identity or `owner`).
- `add_intent` serializes per engagement and skips an intent whose normalized summary and `asset_ids` match an open or running intent, returning it in `skipped` with its `existing_id` (its slot in `ids` is 0). It also lists up to ten unclaimed `open_intents` so agents can pick up existing work. An unknown `asset_id` is a per-item error.

### Fixed
- Importing an export failed with a not-null violation on `technologies` for any HTTP service asset without detected technologies, since export omits the empty list. Imported assets now default to an empty list.
- The MCP server reported version `0.1.0` to clients in every release. It now reports the binary's version (`dev` for local builds).
- Company scope was never enforced. Scope checks only read the company in `tasks.company_id`, which engagements created by `init` never set, and ignored `add_task_scope` entries of kind `company`, so the domain, IP and CIDR lines recorded with `add_company_scope` restricted nothing, and an engagement whose only scope entries were companies allowed every target. Each `company` entry now contributes its company's domain, IP and CIDR lines to `check_scope`, to the tools that enforce scope and to the `in_scope` rules plugins receive. A `company` entry whose company has no domain, IP or CIDR lines fails closed: targets that match no other scope entry are refused with a reason naming the company and `add_company_scope`, instead of the engagement falling back to allowing everything.
- The `add_task_scope` description called task scope "coverage scope, not enforced", but it is what `insert_assets`, the enrich tools, `traffic_replay` and `capture_command_output` check targets against. The description and the hands-free docs now say so, including that a task with no domain, IP, CIDR or company scope lines allows every target (so the first entry narrows scope and later ones widen it), that `icp` and `keyword` entries are not matched against targets, and that `add_company_scope` alone does not change what is in scope.
- Company scope lines written as `*.example.com` matched no host. They now match subdomains only (`a.example.com`, `a.b.example.com`), never the apex `example.com`, which must be listed separately to be in scope. A `*` anywhere other than a leading `*.` is rejected, as is a wildcard `root_domain` task scope entry.
- `export_engagement` failed with "converting NULL to string is unsupported" for any engagement with scope entries, and exports did not carry companies: a `company` scope entry lost its company, and the company's scope lines, the engagement's company and asset company links were not exported at all. Exports now include a `companies` list (each referenced company with its scope lines), a top-level `company_id`, and `company_id` on `company` scope entries and assets. Import gets or creates each company by name, adds its scope lines, and links the new engagement, its `company` scope entries and its assets to it; an asset already linked to a company keeps that link. Exports without `companies`, such as those from v0.6.1, import as before. The export format version stays 1.
- Importing an engagement export dropped each finding's `merged_into`, so merged findings came back live (merged into nothing) while their graph nodes stayed dismissed, and `merge_findings` and goal proofs treated them as live. Import now remaps `merged_into` to the new finding ids. Exports that lack the field import as before, with no finding merged.
- A goal marked met by `prove_goal` stayed met after its proving finding was dismissed. Dismissing a finding now removes its `proves` edges and reopens any goal left without another proof from a live finding or fact. Merging findings moves the secondary's `proves` edges to the primary, so the goal stays met. Concurrent dismissals of a goal's last proofs no longer leave it met, and `prove_goal` refuses a dismissed or merged finding. For databases that already hold such goals, migration 0006 moves the `proves` edges of merged findings to the live finding at the end of their merge chain, deletes those of dismissed findings and merged ones with no live root, and reopens each met goal left without a live proof.
- Two agents merging the same findings into each other at once could leave both findings merged and neither live, reopening the goal they proved. `merge_findings` now locks the findings and the primary's merge chain in a fixed order, follows an already merged `primary_id` to the live finding it points to (returned as `id`, with `requested_primary_id`), refuses a dismissed primary, and skips secondaries that are already merged or dismissed, listing them in `skipped`. The `dismiss_finding` description now points duplicates to `merge_findings`.
- Two concurrent `merge_findings` calls whose secondaries proved the same goals in opposite order could deadlock, and so could `merge_findings` racing `prove_goal` on one of its secondaries; PostgreSQL aborted one call with "deadlock detected". A merge now locks the nodes of all its secondaries and then every goal they prove, each in id order, before changing anything, which matches the order `prove_goal` and `dismiss_finding` use.
- Asset URLs kept any `user:pass@` userinfo, so credentials surfaced in the graph, web UI and reports, and URLs differing only in scheme or host case or a default port (`:80` for http, `:443` for https) were stored as separate assets. URLs are now stored without userinfo (including a password containing a raw `/`, `?` or `#`), with a lowercase scheme and host and no default port; path, query and a Unicode host are kept as given, and `url==` and `url!=` filters match the same normalized form. Userinfo is recorded as a task credential for the host with source `asset_url` (migration 0004; percent-decoded; a username without a password is kept), and never replaces an existing credential for the same host and username. Importing an engagement export captures userinfo from its asset URLs the same way, after the exported credentials, so an exported credential for the same host and username is kept. Tool errors that echo an asset URL no longer include its userinfo. Asset URLs stored before this are rewritten once when `serve` starts, after the credential key is loaded, and their userinfo is recorded as an `asset_url` credential for each of the asset's tasks. A URL whose normalized form already belongs to another asset only has its userinfo stripped, and is left unchanged when that also collides; such assets are not merged. `serve` exits if this rewrite fails.
- Credential domains were stored with their original case, so `Example.com` and `example.com` were separate credentials. Saved and imported credential domains are now normalized (trimmed, lowercased, one trailing dot dropped). Migration 0007 rewrites existing rows the same way and merges rows that then collide: the most recently updated row keeps its password and source (taking the newest non-empty password if its own is empty, copied without decryption), and every distinct note is kept.
- The target change journal folded case on the whole resource, so `/srv/App.conf` and `/srv/app.conf` on a Linux host shared one baseline and restoring one marked the other restored. Targets still match ignoring case, but resources now do only when they are Windows paths or registry keys (a drive letter, a UNC or other backslash path, or an `HKLM`, `HKCU`, `HKCR`, `HKU`, `HKCC` or `HKEY_` key); POSIX paths and other resources match exactly. Migration 0008 rebuilds the live-change index with the new key.
- `engagement_status` always reported `"status": "active"`, even after `goal_met`, because `graph_overview` never loaded the engagement status. `graph_overview` now returns `task.status` from the exploration, and `engagement_status` reports it: `open`, or `achieved` once `goal_met` has run.
- `artex-cc init --data DIR` failed with "failed to save config" when `DIR` did not exist, after it had already created the engagement in PostgreSQL, leaving an orphan engagement behind. `init` now creates a missing data directory 0711, as `serve` does, and generates the proxy CA before creating the engagement, once PostgreSQL is reachable. If writing `.artex.json` still fails, the new engagement is deleted again.
- With `serve --proxy 127.0.0.1:0` (as in the Docker stdio config), plugins received `config.proxy.url` as `http://127.0.0.1:0`, so their traffic never reached the recording proxy. Request replay and the startup log used the same address. A port of 0 is now resolved to the port the proxy actually listens on before anything reports it.
- Opening the store (`serve` or any CLI command) skips re-running `schema.sql` when the database already has the same schema, so starting a server or CLI against a database another server is using no longer risks `deadlock detected` errors in either one.
- `docker-compose.yml` built the default `ARTEX_PG` with a hardcoded `ccartex:ccartex` login, so overriding `POSTGRES_PASSWORD` (or `POSTGRES_USER`/`POSTGRES_DB`) left ARTEX unable to connect. The default DSN now uses the same variables and defaults as the postgres service.
- Web UI chat: a message sent just as a conversation's Claude process exits now starts a new process once the old one has saved its final status, instead of failing with a 500 "file already closed" or having the new process's status overwritten with idle.
- The web Graph page crashed with "Cannot read properties of null" on an engagement with no edges yet, because `GET /api/graph` returned `null` for empty `nodes` or `edges`; both are now always arrays.
- `POST /api/hints` (the web UI's Add hint) returned 500 because it stored hints in the `open` state, which the database rejects; it now stores them `active`, the same as `add_hint`.
- The eval harness Postgres in `eval/docker-compose.yml` published no port, so the eval README's Quick start and the eval runner's default DSN (`localhost:5432`) could not reach it. It is now published on `127.0.0.1:5433`, which stays clear of the dev stack's Postgres on 5432, and the README and the runner's default DSN use that port.
- Eval runner: an instance's `cost_usd` is taken from its last `result` line, which is the session's running total, instead of summing every `result` line.

### Security
- The REST API answered every origin with `Access-Control-Allow-Origin: *` and decoded JSON whatever the Content-Type, so with auth off (the default on a loopback `--web` listener) any web page open in the operator's browser could read and change the engagement, credentials and MCP server entries. The wildcard CORS headers are gone, and POST, PUT, PATCH and DELETE requests are refused with 403 when `Sec-Fetch-Site` is `cross-site` or `same-site` or `Origin` does not match the request's Host. This also covers other ports on localhost, which a `SameSite=Lax` session cookie does not. Requests without either header (CLI and MCP clients) are unaffected.
- A loopback listener accepted any Host header, so a DNS-rebinding page could reach it as same-origin. Requests that arrive on a loopback address, including a `0.0.0.0` listener reached through `127.0.0.1`, are now refused with 421 unless their Host is `localhost` or a loopback IP. Connections on other addresses are unaffected, so remote access by hostname keeps working.
- Anyone who reached a fresh server before the operator, including a DNS-rebinding page on a Docker container whose published port hides whether the client is local, could set the first web UI password. While no password is set, the server now prints a one-time setup token and the setup URL to its logs at startup, and `POST /api/auth/init` returns 403 unless the request carries that token as `setup_token`. The setup screen asks for it. The token lives only in memory and is discarded once the password is set.
- Every HTTP response, including the web UI and `/mcp`, carries `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Content-Security-Policy: frame-ancestors 'none'` and `Referrer-Policy: no-referrer`.
- The web UI workspace (`/api/workspace`) was rooted at the data directory itself, so anyone who could reach the API (with auth off, the default on a loopback listener) could read, overwrite or delete ARTEX's own secrets there: `auth.json` (password hash and session key), `identity.key`, `owner-label.key`, `mcp-config.json` (the `/mcp` bearer token), `chat-mcp/`, `mcp_servers.json` and the proxy CA key under `traffic/_ca`. The workspace now has its own root, `workspace/` in the data directory, created 0700, with new files 0600 and folders 0700. Paths that leave it, including through a symlink that resolves outside it, are refused with 400. Nothing is moved on upgrade, since the data directory gives no reliable way to tell your files from ARTEX's own: the new workspace starts empty, and files you kept at the top of the data directory stay there until you move them into `workspace/`.
- Files in the data directory were world-readable: `auth.json` (password hash and session key), `mcp_servers.json` (server env values such as API keys), chat conversations and their event logs, approvals, intercept rules, LLM settings, agent presets, skills and `.artex.json` were written 0644 in 0755 directories. They are now written 0600 in 0700 directories. Captured traffic blobs, the traffic index directory and saved evidence, which can hold credentials and session tokens, are owner-only too. When `serve` has to create the data directory it creates it 0711, so other users can still reach the public CA certificate in `traffic/_ca` without listing anything else; an existing data directory keeps its mode. At the first startup after upgrading, `serve` or `init` removes group and other access from the files and directories ARTEX writes in the data directory (`.artex.json`, `identity.key`, `owner-label.key`, `auth.json`, `mcp_servers.json`, `mcp-config.json`, `approvals.json`, `intercept_rules.json`, `chat-mcp/`, `conversations/`, `settings/`, `agents/`, `skills/`, `workspace/` and `evidence/`), skipping symlinks and leaving everything else in the directory alone, then writes `.perms-tightened` so later starts skip the walk. This only happens when the directory already holds one of the files only ARTEX writes there (`.artex.json`, `identity.key`, `owner-label.key`, `auth.json`, `mcp_servers.json`, `mcp-config.json`, `approvals.json`, `intercept_rules.json` or `chat-mcp/`). A fresh data directory, such as a project or home directory used with the default `--data .`, just gets the marker, so its own `workspace/`, `skills/` or `evidence/` keeps its permissions. A walk that cannot reach or restrict an entry logs a warning and is retried at the next start. `traffic/` tightens its own tree once (including host directories captured by older versions, but not `_ca`). The eval runner writes `mcp.json` (which holds the PostgreSQL DSN), its engagement config, transcripts, `server.log` and `coordination.txt` 0600 and creates the transcripts directory 0700.
- `artex-cc init` printed the PostgreSQL DSN, password included, in the MCP config snippet and in `--dry-run --format json`, so in Docker the password ended up in `docker logs`. The password is now shown as `***` with a note to substitute it or pass it via `ARTEX_PG`; DSNs without a password print unchanged.
- Bumped `github.com/gorilla/websocket` to v1.5.4-0.20240701034025-d67f41855da4 (GO-2026-6278, reached through the recording proxy) and `github.com/klauspost/compress` to v1.20.2 (GO-2026-6865).

## [0.6.1] - 2026-11-25

### Added
- `serve --require-identity` (or `ARTEX_REQUIRE_IDENTITY=1`), off by default, makes `start_intent`, `complete_intent` and `cancel_intent` refuse callers without a verified `X-Artex-Identity`, whatever session or `owner` they have, with an error pointing at `artex-cc identity issue`. `list_changes` returns `claims_error` instead of `my_claims` for such callers; other tools are unaffected. `start_intent`'s description and the orchestration guide state the requirement when it is on. It applies to HTTP only: `serve --stdio` with it set exits at startup, and an `ARTEX_REQUIRE_IDENTITY` value that is not a boolean is a startup error. The eval runner clears an inherited value for the servers it starts in its anonymous modes (stdio and `--identity=false`).
- The web UI Dashboard's Active Intents card shows who holds each running intent's claim and how long it has left (`chat/<conversation id>` names shortened, full name on hover), and lists intents whose claim lapsed with a "claim lapsed" hint naming the previous holder.
- Eval runner: `--env KEY=VALUE` (repeatable) adds variables to every Claude instance's environment.

### Changed
- A request whose `X-Artex-Identity` fails to verify gets HTTP 400 instead of 401, with the same reason in the body. Claude Code reads a 401 as an OAuth challenge and hid the reason behind a Dynamic Client Registration error; a 403 would have cached the server as needing authentication. Bearer token failures still get 401.
- Each rejected identity logs one warning with the reason (`malformed`, `bad_signature`, `expired`, `unknown_key` or `header`), the claimed name and key ID from the unverified token, and the remote address. The token is never logged.

### Fixed
- Eval runner: scenario `scope` entries are now registered with kinds the store accepts (`cidr`, `ip`, or `subdomain` for an exact host), and a run stops if any entry fails to apply. Previously scope was silently not applied.

## [0.6.0] - 2026-11-25

### Added
- HTTP MCP clients can send an `X-Artex-Identity` token signed with the data directory's `identity.key`, on both `--web` and plain `--listen`. A verified identity holds claims as `id:<name>` (or `id:<name>/owner:<x>` with `owner`), independent of the MCP session, so it keeps its claims across reconnects, server restarts and protocol eras, and needs no `owner` on protocol 2026-07-28. A header that fails to verify (malformed, forged, expired, or from another data directory or a rotated key) gets HTTP 401 after bearer auth. Requests without the header behave as before.
- `list_sessions` returns an `identities` array (`name`, `self`, `last_active`) of identities that made requests to this server process within the claim lease, so clients on protocol 2026-07-28 show up.
- `artex-cc identity issue` prints a signed `X-Artex-Identity` token for an HTTP MCP client (`--format token|header|claude-json`, `--ttl` default 7d, max 30d), offline from the data directory's `identity.key`. Names under `chat/` and `eval/` are refused.
- `artex-cc identity rotate` replaces `identity.key` atomically, revoking every token issued from that data directory.
- `artex-cc identity show` prints the current key ID, or decodes a token (argument or `-` for stdin) and reports its name, key ID, issue and expiry times and whether it verifies, without printing the token.
- Shell completion for the `identity` subcommands.
- Eval runner: with `--transport http`, each instance gets its own `X-Artex-Identity` named `eval/<run>/i<n>`, minted per run with a TTL of the scenario timeout plus the lease and written only to that instance's 0600 MCP config in the temporary data directory. Prompts no longer ask HTTP instances to pick an `owner`. `--identity=false` keeps the anonymous owner path. The coordination scorecard parses identity holder labels, lists each instance's `holders`, attributes unhandled conflicts to the holding instance, and counts `holder_mismatches`.

### Changed
- Claims held by an identity are labeled with the identity name in `claimed_by`, `list_changes`, claim errors and exports (`<name>/h-...` when `owner` subdivides it). Anonymous holders keep their hashed labels.
- The MCP HTTP transport's logs redact anything shaped like an identity token.
- `serve` over HTTP (`--web` or `--listen`) loads or creates `identity.key` in the data directory at startup and exits if it cannot; stdio is unaffected.
- Web UI chat conversations hold claims as the identity `chat/<conversation id>`, minted from `identity.key` each time a conversation starts or resumes, so labels show that name and a resumed conversation keeps its claims across a server restart. This replaces the in-memory `X-Artex-Conversation` signer; a request still sending `X-Artex-Conversation` gets HTTP 400 telling the caller to start a new conversation.

## [0.5.1] - 2026-11-25

### Added
- Web UI chat conversations hold intent claims automatically through a server-signed per-conversation identity, so their first `start_intent` succeeds without an `owner` argument and separate conversations never share claims.
- Eval runner: `--instances`, `--transport stdio|http`, `--kill-after` and `--lease` run several Claude instances on one engagement, save each instance's transcript, and score coordination (claims, conflicts, duplicates, takeover) in the scorecard and `coordination.txt`.

### Changed
- `set_goals` sets goals once per engagement: concurrent calls serialize, later calls return `existing_goals` instead of adding more unless `append: true`, and goals matching an existing one are skipped. The response is now always JSON.
- `possible_duplicates` is scoped to the engagement and matches findings on a normalized vulnerability class (with aliases such as BOLA and IDOR), excluding dismissed and merged findings, and returns `{id, vulnclass, severity, summary}` entries.
- The orchestration guide tells parallel agents to adopt existing goals, check for duplicate findings before reporting, and poll `list_changes` instead of sleeping when every open intent is claimed.
- `serve --listen` without `--web` refuses to start on a non-loopback address (anything other than `localhost`, `127.0.0.0/8` or `::1`, including `:9400` and `0.0.0.0`) unless `ARTEX_MCP_TOKEN` is set, and then requires it as a bearer token on `/mcp`. Previously such a listener exposed every tool without authentication. Loopback listeners are unchanged, and honor `ARTEX_MCP_TOKEN` when it is set.
- `list_sessions` returns each session's hashed holder `label` and a `self` flag instead of its raw session ID.
- Claim holder labels are keyed with a random secret stored in `owner-label.key` in the data directory (created on first start, mode 0600), so processes sharing a data directory show the same labels. Labels shown before this change no longer match.

### Fixed
- Any MCP client could read other clients' session IDs from `list_sessions` and replay one as `Mcp-Session-Id` to complete or cancel intents claimed by that session.
- A low-entropy `owner` could be recovered from its claim label by guessing, since the label was an unkeyed hash.
- Session register and unregister debug logs, which reach `docker logs` and the web Logs page, carried raw session IDs; they now log the holder label.
- The MCP HTTP transport's own logs (rejected GETs, expired session sweeps, sampling responses, stream resume and event purge failures, session termination failures) carried raw session IDs; they now log the holder label. With `--web` these logs also reach the web Logs page.

## [0.5.0] - 2026-11-25

### Added
- Exec plugins can return `assets`, `facts` and `findings`, persisted through `insert_assets`, `record_fact` and `report_finding` with scope enforcement, and each invocation receives the active engagement, its scope, the data directory and the recording proxy address.
- Example plugin `examples/plugins/httpprobe.py` and a [Plugins](https://artex-cc.com/docs/plugins/) docs page.
- Intent claiming: `start_intent` takes a renewable 30-minute lease for the calling session or `owner`, other holders are refused until it expires, and `graph_overview` shows hashed holder labels. `cancel_intent` and `complete_intent` respect live claims.
- `list_changes` tool for polling new and updated findings, facts, intents, goals, hints and assets since a cursor, plus the caller's held claims; `graph_overview` returns a starting `changes_cursor`.
- Cross-process change notifications over PostgreSQL LISTEN/NOTIFY, so `serve --web` shows changes made by other artex-cc processes on the same database.
- Orchestration guide and hands-free docs cover running several agents on one engagement.
- `ARTEX_INTENT_LEASE` sets the claim lease in whole minutes (default 30m), for tests and evals that need claims to lapse quickly.

### Changed
- Plugin limits: 8 MiB stdout, 500 structured items per result, 1 minute to persist results, 4 KiB of error text.
- Web UI refreshes the views each mutating tool affects (including intent and plugin tools) and coalesces bursts of changes into one refresh per event type. The "Data refreshed" toast is gone.
- Migrations add `exploration_nodes.claim_expires_at` and `findings.updated_at`. Intents already running before the upgrade have no lease and are immediately claimable.

### Fixed
- Data races on the MCP server's active engagement during `switch_engagement`.
- Plugin results after `switch_engagement` were written into the startup engagement.
- The example plugin probed `file://` and `ftp://` URLs on in-scope hosts.
- Web UI: the Dashboard's Assets card showed the fact count, and active intents showed `Intent #N` instead of their summary.

### Removed
- Retired Go Report Card badge from the README.

## [0.4.1] - 2026-11-24

### Added
- `artex-cc tools [--json]` prints the MCP tool catalog (name, category, description, input schema, read-only and proxy-only flags) without a database.
- Project site at https://artex-cc.com: docs, evals, changelog, a blog, a tool reference generated from `artex-cc tools --json`, and a static live demo of the web UI at `/demo/`.
- Homebrew cask: releases publish `artex-cc` to `SREsAreHumanToo/homebrew-tap`. Install with `brew install --cask sresarehumantoo/tap/artex-cc` on macOS or Linux (Homebrew 7.0.11 or later).
- Docker image ships a pentest tooling layer (nmap, impacket, evil-winrm, nuclei, ffuf and more); opt out with `--build-arg PENTEST_TOOLS=0`.
- Eval runner: `--target`, `--model`, `--mcp-config` and `--skip-health` flags, and TCP health checks for non-HTTP targets.

### Changed
- `artex-cc` is the command name throughout the CLI help, init output and docs. Shell completions now register for `artex-cc` (they targeted `artex`, so they never attached to the installed binary).
- README trimmed to an overview; full documentation moved to https://artex-cc.com.

### Fixed
- `insert_assets` failed with a NOT NULL violation when re-inserting a known HTTP service with no technologies.
- Web UI: the Report page crashed on activity entries without a summary, and Activity search crashed on entries without a tool or worker.
- Web UI: doubled page titles on several pages, and the command palette needing Escape twice to close.

## [0.4.0] - 2026-11-24

### Breaking
- Project renamed from cc-artex to ARTEX-CC. The binary is now `artex-cc` (was `cc-artex`) and the Go module is `github.com/SREsAreHumanToo/ARTEX-CC`. MCP tool names (`mcp__artex__*`) and `ARTEX_*` environment variables are unchanged.
- Relicensed from MIT to AGPL-3.0-only. Parts of ARTEX-CC are derived from AGPL-3.0 [ARTEX](https://github.com/BaaRai/ARTEX); see NOTICE. Releases v0.1.0 through v0.3.0 carried an MIT license file in error and are AGPL-3.0 as well.

### Added
- Web UI (`serve --web`, enabled in the Docker image): chat with a Claude subprocess runner, agents and presets, engagement tasks, live logs, commands log, intercept approvals, MCP server config, tool catalog, skills, token usage, workspace browser, LLM settings, and password auth.
- Exec-based plugin system for registering external tools.
- Multi-client MCP session tracking and the `list_sessions` tool.
- `traffic_patterns` tool for temporal traffic analysis and `traffic_clear` to delete captured traffic by host.
- Request size statistics in `traffic_stats`.
- `list_activity` `--since` filter and CSV export.
- `artex-cc init --dry-run` and `--format json`.
- `ARTEX_LISTEN` environment variable for the listen address.
- SQLite traffic DB health check.
- Docker image builds the web frontend and installs the Claude CLI.
- ARTEX-CC logo, favicon and brand mark; web UI links to its source code (AGPL section 13).

### Fixed
- `report_finding` and scanner ingest now create a graph node for each finding, so findings show up in `graph_overview`, the dashboard and the graph, and can be passed to `prove_goal`. `report_finding` returns the `node_id` to use.
- Dismissed findings and merged duplicates no longer count in the graph overview.
- `traffic_search` and `traffic_har_export` only ever returned empty-body responses.
- `traffic_stats` failed whenever average sizes were not whole numbers.
- `bind_finding_traffic` accepted snapshot ids that were never captured; it now validates them and is registered only with `--proxy`.
- Docker docs: HTTP MCP endpoint is `/mcp` with a bearer token (not `/sse`), the binary path is `/app/artex-cc`, and the stdio example no longer collides with the main proxy port.
- SSE connections flush on connect so the web UI's `EventSource.onopen` fires.
- Goreleaser v2 deprecation warnings.
- Windows builds failed because the web chat runner used Unix-only process-group calls.

## [0.3.0] - 2026-11-22

### Added
- `artex health --format json` for structured health check output (#105)
- `artex health --timeout` for configurable health check timeout (#108)
- Shell completions for `--format` and `--timeout` health flags in bash/zsh/fish (#106)
- Web UI: search page, coverage filter, traffic explorer with URL params
- Web UI: finding detail, asset chart, interactive legend, activity indicator
- Web UI: credentials export, activity export, report executive summary
- Web UI: 404 page, loading skeletons, settings stats and progress
- Web UI: timeline chart, severity bar, traffic export, empty states
- Web UI: connection banner, recent findings, syntax highlighting
- Web UI: graph tooltips, palette view-all, dashboard banner
- API handler tests (0% to 74% coverage)
- Tools test coverage pushed from 57.6% to 82.5% with evidence, ingest, traffic, export, and report tests
- Go Report Card, Go Reference, and License badges in README
- Traffic rewrite rule validation tests (`rule_test.go`)
- Good-first-issue templates (#105-#109)
- GitHub repository topics and metadata

### Changed
- Migrated `log.Printf` to structured `slog` in traffic/ and egress/
- 910+ test runs across all packages (up from 480+)

### Fixed
- `gofmt -s` struct alignment in health.go
- Goreleaser web build hook (`bash -c` wrapper)
- Missing `Link` import in Settings.tsx
- Homebrew tap upload permission (skipped until PAT configured)
- Ingest test data corrections

## [0.2.0] - 2026-11-21

### Added
- `artex health` subcommand for PostgreSQL connectivity checks
- `artex completion bash/zsh/fish` for shell completions
- `artex version --json` for structured version output
- `artex serve --timeout` for configurable graceful shutdown deadline
- Codecov coverage reporting in CI
- `gofmt -s` check in CI pipeline
- Tool reference documentation (docs/TOOLS.md)
- MCP client configuration guide (docs/MCP_CLIENTS.md)
- CHANGELOG.md
- AGENTS.md contributor guide
- CODE_OF_CONDUCT.md

### Improved
- Error message when ARTEX_PG is not set now shows both options with examples
- CONTRIBUTING.md with dev setup, testing, code style sections
- Roadmap refreshed with current stats
- Test coverage: report 96.1%, egress 98.9%

## [0.1.1] - 2026-11-21

### Added
- .deb package support via goreleaser nfpms
- Demo GIF in README
- Social preview image for GitHub SEO
- Homebrew tap shared with dfinstall at SREsAreHumanToo/homebrew-tap

## [0.1.0] - 2026-11-21

### Added
- 59 MCP tools across 10 categories
- PostgreSQL-backed exploration graph (goals, intents, facts, findings, hints, constraints)
- Asset management with bulk insert, DSL-based queries, and 6 asset types
- Finding triage: merge duplicates, dismiss false positives, duplicate detection
- Credential storage with AES-256-GCM encryption at rest
- Evidence capture (save, list, get, capture_command_output)
- Coverage ledger with gap analysis
- Report generation (markdown and CSV export)
- Passive reconnaissance (enrich_domain, enrich_ip, enrich_service)
- Scanner ingestion (nmap, nuclei, httpx, ffuf, masscan)
- Scope enforcement with override mechanism
- Multi-engagement switching
- Activity logging and audit trail
- Full MITM traffic recording proxy with SQLite index, HAR export, request replay
- Traffic rewrite engine with custom hook VM
- Egress governor with rate limiting profiles
- SOCKS5 proxy with UDP relay
- Raw TCP/UDP capture
- Transparent proxy (Linux)
- Intercept rules with allow/deny/ask actions
- Web UI with dashboard, graph visualization, findings, assets, and traffic views
- Self-hosted evaluation harness (Docker-based with DVWA, Juice Shop)
- Engagement export/import to portable JSON archives
- Cross-platform binaries via goreleaser
- Dockerfile for container deployments
- GitHub Actions CI with PostgreSQL service container

[Unreleased]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/SREsAreHumanToo/ARTEX-CC/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/SREsAreHumanToo/ARTEX-CC/releases/tag/v0.1.0
