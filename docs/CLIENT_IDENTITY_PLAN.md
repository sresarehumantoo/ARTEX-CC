# Client identity plan

Status: proposal for maintainer review. No code yet. Citations are against `main` at `df802eec` (after #213, #215, #216) and mcp-go v1.6.0 in the module cache.

## Summary

Generalize the web chat's signed conversation header (#212) into one server-signed identity header, `X-Artex-Identity`, that any HTTP client can carry. The signing key lives in the data directory, so every process serving that directory verifies the same tokens and a restart keeps identities valid. Tokens are minted by `artex-cc identity issue` (operators, external clients), by the web chat runner (one per conversation), and by the eval runner (one per instance). A verified identity becomes the claim holder `id:<name>`, the `owner` argument may still subdivide it, and the holder label shows the readable name. Requests without the header behave exactly as today, so no existing claim changes format.

## Current state (verified)

- Claim holders come from `tools.ClaimOwner` (`tools/claim.go:64`): session alone, `session/owner:<x>`, or `owner:<x>` when there is no session. Labels are `h-` plus 6 bytes of HMAC-SHA256 under `owner-label.key` (`tools/claim.go:90`, `config/labelkey.go:18`), installed by `serve` at `cmd/artex-cc/main.go:233`.
- The session comes from `callerSessionID` (`mcpserver/handler.go:31`). stdio uses a random per-process ID (`mcpserver/handler.go:22`). Legacy HTTP uses mcp-go's session ID; with no session it falls back to `conversation:<id>` (`mcpserver/handler.go:37`, `mcpserver/conversation.go:74`).
- Legacy HTTP session IDs are bearer values. mcp-go defaults to `StatelessGeneratingSessionIdManager` (`server/streamable_http.go:400`), whose `Validate` only checks the `mcp-session-` prefix and UUID syntax (`server/streamable_http.go:2447`). Any token holder who learns another client's `Mcp-Session-Id` can act as that session. #215 stopped leaking them; nothing binds them to a client.
- On 2026-07-28 the session header is ignored and each request gets an ephemeral session with no ID (`server/streamable_http.go:779-786`). Clients must pass `owner` (`tools/claim.go:39-45`), and any token holder can pass the same string.
- mcp-go builds the tool context from the HTTP request's context for both eras, then applies `WithHTTPContextFunc` (`server/streamable_http.go:860-864`, `server/streamable_http_handle.go:45`). So an ordinary `http.Handler` middleware that stores a value in `r.Context()` reaches the tool handler in both eras, which is how #212's middleware works today. `HTTPContextFunc` (`server/http_transport_options.go:11`) can only return a context; it cannot reject a request with 401.
- #212: `ConversationSigner` (`mcpserver/conversation.go:21`) uses a random in-memory key, token `<id>.<hex hmac>`, no expiry, no version, no domain separation. Its middleware is mounted only on the `--web` path (`cmd/artex-cc/main.go:359`); the plain `--listen` path (`cmd/artex-cc/main.go:382-388`, `mcpserver/transport_http.go:12`) has none. The runner writes `chat-mcp/<id>.json` with the header (`api/chat_runner.go:148-175`). A server restart invalidates every conversation token.
- `/mcp` auth: `--web` generates a bearer token per start (`cmd/artex-cc/main.go:309-315`); without `--web`, non-loopback listeners require `ARTEX_MCP_TOKEN` (`cmd/artex-cc/main.go:112`) checked by `bearerAuth` (`cmd/artex-cc/main.go:128`). Everyone with the token is one principal.
- `list_sessions` lists only sessions registered through mcp-go hooks in this process (`mcpserver/server.go:77-84`, `133-170`). Modern HTTP clients and conversation callers never appear.
- The eval runner gives all HTTP instances one shared `mcp.json` without auth (`eval/cmd/eval-runner/instances.go:114-138`, `248-251`) and tells each agent to invent an owner (`instances.go:81-84`). The coordination scorer compares holder labels parsed from transcripts (`eval/cmd/eval-runner/coordination.go:480-500`).
- `exploration_nodes.owner` is free `TEXT` (`store/schema.sql:136`); exports and change rows show only labels (`store/export.go:121`, `store/changes.go:136`, `store/exploration.go:342`).

## 1. Goals and non-goals

Goals:

- G1. Cooperating clients that share one bearer token cannot hold, renew, complete or cancel each other's claims by accident or by copying a value they saw (an owner string, a label, a session ID).
- G2. Claim holders are stable across reconnects, server restarts and protocol eras for any client that carries an identity.
- G3. One mechanism for web chat, eval instances and external clients.
- G4. Holders are readable in `claimed_by`, `list_changes`, exports and eval scoring when an identity is present.

Non-goals:

- Not authorization. A token holder still has every tool: it can delete nodes, read credentials, or `cancel_intent` through any path the tools allow to an unclaimed node. Identity only decides who a claim belongs to.
- Not protection against a malicious token holder who can also read the data directory (it can read the key) or the other client's MCP config (it can copy the identity token).
- Not per-client bearer tokens (Option D). Identity sits on top of the shared bearer token.
- Not stdio. A stdio server process is spawned by its client and holds the DB DSN and the data dir, so a header-style identity adds nothing it could not forge.

## 2. Requirements

Issuance

- R1. `artex-cc identity issue --data <dir> --name <name> [--ttl <dur>] [--format token|header|claude-json]` prints one token to stdout and nothing else on stdout. It runs offline: it needs only the data dir, not a server or database.
- R2. Names match `^[a-z0-9][a-z0-9._/-]{0,63}$`. The prefixes `chat/` and `eval/` are used by the built-in issuers, which mint in process; the CLI always refuses them.
- R3. Default TTL is 7 days for the CLI, settable with `--ttl` up to a maximum of 30 days, conversation lifetime plus a margin for chat (see R14), and the run timeout plus the lease for eval. `--ttl 0` is rejected; every token expires.
- R4. `--format claude-json [--url <mcp url>] [--bearer-env VAR]` prints an `mcpServers.artex` block, ready for `claude --mcp-config` or `.mcp.json`. `--url` defaults to `http://localhost:9400/mcp`. With `--bearer-env VAR` the block also sends `Authorization: Bearer ${VAR}`, which Claude Code expands from the environment; without it no `Authorization` header is written, for a tokenless loopback listener.
- R5. Revocation is by rotation: `artex-cc identity rotate --data <dir>` replaces the key, which invalidates every token from that directory, including chat and eval ones. Running servers pick up the new key without restart (R9). Per-token revocation is an open question (Q2).

Verification

- R6. An `/mcp` request with `X-Artex-Identity` that fails to parse, fails the MAC, names an unknown key ID, or is expired gets HTTP 400 with a short reason, before mcp-go sees it, and the server logs a warning with the reason class and the unverified name and key ID (never the token). Failing closed means a client misconfigured with a stale token notices at once instead of silently claiming anonymously. Not 401 or 403: tested against Claude Code 2.1.332, a 401 starts OAuth discovery and replaces the reason with a Dynamic Client Registration error, and a 403 marks the server as needing authentication in Claude Code's needs-auth cache, so it is skipped even after the token is fixed. A 400 shows the reason directly with no cached state.
- R7. Verification is constant time on the MAC and allocates no state per token. It runs after bearer auth, so an unauthenticated request learns nothing about identity validity.
- R8. Identity verification is mounted on every HTTP `/mcp` path: `--web` and plain `--listen`.
- R9. Servers reread the key file when its inode, mtime or size changes (checked at most once per second). `rotate` writes a temp file and renames it into place, so the change is atomic and always visible. There is no grace period: per Q2, rotation is the revocation tool and every old token stops verifying at once. A `--grace` option keeping the old key as `identity.key.prev` can be added if a deployment needs zero-downtime rotation; nothing in the token format prevents it.

Composition

- R10. Holder resolution, first match wins:
  1. identity and `owner`: `id:<name>/owner:<x>`
  2. identity: `id:<name>`
  3. today's rules (`session/owner:<x>`, `session`, `owner:<x>`), unchanged.
  The legacy session is not part of an identity holder, so the same identity keeps its claims across reconnects and across eras.
- R11. Two clients given the same identity are one holder. That is the issuer's decision; the CLI keeps no record of issued names, so the docs say "one name per concurrently running agent".
- R12. `tools.SessionID(ctx)` keeps its meaning (transport session). A new `tools.IdentityName(ctx)` carries the verified name. `ClaimOwner` reads both.

Key

- R13. A separate key file `<data>/identity.key`, 32 random bytes, hex, mode 0600, created on first use with the same atomic link-publish as `owner-label.key` (`config/labelkey.go:33-47`). Justification: the two keys have different lifecycles. Rotating `identity.key` is the revocation tool and must not change every label already in exports, logs and eval reports; rotating the label key must not log every client out. Domain separation also means a token MAC can never equal a label HMAC. The key ID is the first 8 hex characters of SHA-256 of the key.

Per client kind

- R14. Web chat: the runner mints `chat/<conversation id>` with the persisted key, replacing `ConversationSigner` and `X-Artex-Conversation`. TTL 7 days; the runner re-mints when it starts or resumes a conversation process, since it rewrites `chat-mcp/<id>.json` each start anyway (`api/chat_runner.go:239`). A resumed conversation keeps its claims across a server restart, which #212 could not do.
- R15. Eval runner (HTTP transport): one token per instance, name `eval/<run id>/i<n>`, minted in process through the `config` package (the runner is in the same module), one MCP config file per instance. The "choose an owner" prompt line is dropped when identities are on. A flag `--identity=false` keeps today's anonymous owner path so scenario A3 remains measurable.
- R16. Eval runner (stdio): unchanged.
- R17. External clients: unchanged unless they add the header. Documented in `mcp-clients.md`.
- R18. Anonymous clients (no header): byte-for-byte today's behavior, including `ErrNoClaimOwner` on 2026-07-28 without `owner`.
- R19. Multi-process: every process with the same `--data` shares `identity.key`, so a token verifies on any of them, and a claim made through one is the same holder on another. Processes that share a database but not a data dir reject each other's tokens with "unknown key ID" (R6); the docs say processes serving one database must share `--data` (already true for labels).

Presentation

- R20. The label for `id:<name>` is `<name>`; for `id:<name>/owner:<x>` it is `<name>/h-<6 bytes>` with the HMAC over the owner part only. Anonymous holders keep `h-` labels. The name is not secret, because knowing it does not let anyone forge the token.
- R21. `list_sessions` adds an `identities` array of identities seen by this process in the last lease period (`name`, `last_active`, `self`). The existing `sessions` array is unchanged.
- R22. Tokens are never written to logs, tool results, change rows or exports. The redacting logger from #218 is extended to redact anything matching the `ai1.` token format, as defense in depth.

## 3. Design

### Token format

```
X-Artex-Identity: ai1.<payload>.<mac>
payload = base64url(JSON {"n": name, "k": key id, "iat": unix, "exp": unix})
mac     = base64url(HMAC-SHA256(identity.key, "artex-identity-v1\x00" + payload))
```

- The `ai1` prefix versions the format and makes the value recognizable to log redaction.
- The domain string separates it from any other MAC the key might ever sign.
- `k` names the signing key and gives a precise error for rotated keys and cross-data-dir use.
- No engagement binding by default: claims are per node, nodes are global in the database, and `switch_engagement` changes engagement mid-session. See Q4.
- No nonce or replay cache: the token is a bearer credential scoped to a holder name, and the threat model is "copied a value", which expiry plus non-disclosure (R22) handles. A replayed token is the identity, by design.

### Components and file-level changes

| File | Change |
|------|--------|
| `config/identitykey.go` (new) | `IdentityKey(dataDir)`; shares create logic with `labelkey.go` via an unexported `loadOrCreateKey(path)` |
| `config/labelkey.go` | Move body into `loadOrCreateKey`; behavior unchanged |
| `identity/identity.go` (new package) | `Signer{Issue(name, ttl), Verify(token) (Claims, error)}`, key reload by mtime, name validation |
| `mcpserver/identity.go` (new) | `IdentityMiddleware(verifier)`: 400 on bad header, stores the name in the request context; `seen` map feeding `list_sessions` |
| `mcpserver/conversation.go` | Deleted after phase 3; `conversationOwner` replaced by identity lookup |
| `mcpserver/handler.go` | `callerSessionID` drops the conversation fallback; handler also sets `tools.WithIdentity(ctx, name)` |
| `mcpserver/transport_http.go` | Accept the identity middleware (it already takes a middleware list, line 12) |
| `mcpserver/server.go` | `list_sessions` adds `identities` (R21) |
| `tools/claim.go` | `WithIdentity`, `IdentityName`, new `ClaimOwner` order (R10), label rule (R20), owner arg description mentions identity |
| `cmd/artex-cc/main.go` | `identity` subcommand (`issue`, `rotate`, `show`); mount the middleware on both HTTP paths; pass the signer to the runner |
| `cmd/artex-cc/identity.go` (new) | Subcommand implementation |
| `api/chat_runner.go` | `SetIdentitySigner`, header `X-Artex-Identity`, name `chat/<id>` |
| `eval/cmd/eval-runner/instances.go` | Per-instance config and token, prompt change, `--identity` flag |
| `site/src/content/docs/docs/mcp-clients.md`, `CLAUDE.md`, `CHANGELOG.md`, completion script | Docs |

The identity middleware is a plain `http.Handler` wrapper, not `WithHTTPContextFunc`, because only a handler can return an error status (R6). mcp-go derives the tool context from `r.Context()` in both eras (`server/streamable_http.go:860`), so the effect on tool calls is the same. `WithHTTPContextFunc` stays available if a future mcp-go stops propagating the request context.

### Data flow for a tool call

1. Request hits `/mcp`. `bearerAuth` checks `Authorization` (unchanged).
2. Identity middleware: no header, pass through. Header present: parse, select key by `k`, verify MAC, check `exp` with 60 s skew, else 400 and a warning log. On success put `name` in the context and touch `seen[name]`.
3. mcp-go routes by era, builds the context from `r.Context()`, calls the tool handler.
4. Handler sets `tools.WithSessionID` (unchanged) and `tools.WithIdentity`.
5. `ClaimOwner` returns `id:<name>[/owner:x]` or falls back to today's rules. `store.ClaimIntent` is unchanged (`store/exploration.go:126`); the owner column is opaque text.
6. Results render `OwnerLabel(owner)`, which now returns `<name>` or `<name>/h-...` for identity holders.

### Interaction with bearer auth

Identity is not authentication and never substitutes for the bearer token. On a tokenless loopback listener the identity header still works and still prevents accidental collisions between local agents, which is the eval case. A future Option D can map a per-client bearer token to an implied identity name using the same holder format, so this plan does not block it.

## 4. Migration and compatibility

- Anonymous holders keep their format, so no live claim is orphaned by the upgrade for stdio, legacy HTTP, or `owner`-only clients. This removes most of the "orphan live claims" cost noted for Option C, which assumed the session would be part of every holder.
- Web chat holders change from `conversation:<id>` to `id:chat/<id>`. Today every conversation token dies with the server process and the runner stops all conversations on shutdown (`cmd/artex-cc/main.go:380`), so their claims are already orphaned by any restart. The upgrade costs nothing extra; claims lapse at lease expiry as today.
- `X-Artex-Conversation` is removed rather than kept as an alias, since only the runner sends it and the runner ships in the same binary. Requests still carrying it are rejected with a clear error telling the caller to start a new conversation. In practice none should arrive: the old signer's key lived only in server memory and the chat `claude` processes are children of the server, so neither survives the upgrade restart.
- Exports and change rows keep showing labels only. Identity labels become readable names; anonymous labels are unchanged because `owner-label.key` is untouched.
- No schema migration.
- `mcp-clients.md` gains a "Separate claim holders" section after the bearer token section (line 21-42): when to use it, `identity issue --format claude-json`, one name per running agent, rotation, and that the header is not a credential for anything except claims.
- Eval: reports gain readable holder names; the A3 scenario passes `--identity=false`. `docs/MULTI_AGENT_EVAL_PLAN.md` lines 9, 39, 67, 86 are updated.

## 5. Test plan and acceptance criteria

Unit (`identity`, `tools`, `config`):

- Issue then verify round trip; name validation table; reserved prefixes.
- Forged: flipped payload byte, flipped MAC byte, MAC from another key, payload with `exp` edited, truncated token, wrong prefix (`ai0`), empty segments, oversized header (> 2 KiB). All rejected.
- Expired: `exp` in the past beyond skew rejected; within skew accepted.
- Cross-data-dir: token from dir A rejected by dir B with "unknown key ID".
- Rotation: after `rotate`, old tokens rejected and new ones verify; the key file stays 0600 and gets a new inode; reload picked up without restart.
- `ClaimOwner` table for every combination of identity, session, owner, including whitespace owners.
- Labels: identity, identity plus owner, anonymous unchanged against a fixed key (golden values so a change to anonymous labels fails the test).
- Key file created 0600, concurrent first creation yields one key (mirror the labelkey test).

Integration (`mcpserver/claim_http_test.go` style, both eras):

- Two modern clients with different identities: second `start_intent` gets a conflict naming the first by name.
- Same identity on a legacy client, reconnect with a new `Mcp-Session-Id`: renew succeeds, `Renewed` is true.
- Adversarial copy: client B sends A's `owner` string and B's own identity: conflict. Client B sends A's session ID (legacy) and B's identity: conflict, because identity wins.
- Bad header: 400 on both `--web` and plain `--listen` stacks, and the tool never runs.
- Header without bearer on a token-protected listener: 401 from bearer auth, identical body whether the identity is valid, invalid or absent.
- Two server processes on one data dir: claim through one, conflict through the other with the same label.
- Web chat: conversation claims labeled `chat/<id>`, survive a runner restart of that conversation.
- `list_sessions` shows identities with `self`.
- Log capture: no token substring in any log line at debug level.

Eval:

- `eval-runner --transport http --instances 3` against the vulnapp scenario: zero double claims, holder names match instance numbers in the coordination report.

Acceptance: all of the above pass in CI with `ARTEX_TEST_PG`; `go vet` clean; anonymous behavior tests from #212 to #216 pass unmodified except the conversation test, which is rewritten for the new header.

## 6. Phasing

| PR | Scope | Effort |
|----|-------|--------|
| 1 | `config` key loader refactor plus `identity.key`; `identity` package with issue, verify, reload; unit tests | S, about 0.5 day |
| 2 | Middleware on both HTTP paths, `tools.WithIdentity`, `ClaimOwner` order, labels, `list_sessions`; integration tests | M, about 1 day |
| 3 | `artex-cc identity issue/rotate/show`, completion, `mcp-clients.md`, CHANGELOG | S, about 0.5 day |
| 4 | Web chat moves to the identity signer; delete `conversation.go` | S, about 0.5 day |
| 5 | Eval runner per-instance identities, `--identity` flag, eval plan doc | S, about 0.5 day |
| 6 (optional) | Per-token revocation or `--require-identity`, per Q2 and Q3. `--require-identity` is done (see Q3); per-token revocation is not planned | S each |

PR 2 depends on the other agent's work in `transport_http.go` and `main.go` landing first; it should rebase on it rather than conflict.

## Decisions

Recorded 2026-11-25 by the maintainer.

- Q1. Holder composition: drop the session from identity holders (R10). Issue one name per agent.
- Q2. Revocation: rotation plus short TTLs only. No per-token revocation list until someone needs it.
- Q3. `--require-identity`: add later in phase 6, off by default. Done: `serve --require-identity` or `ARTEX_REQUIRE_IDENTITY` makes `start_intent`, `complete_intent`, `cancel_intent` and the `my_claims` part of `list_changes` refuse callers without a verified identity; session and `owner` holders do not count. It applies to HTTP only, and `serve --stdio` with it set exits at startup, since a stdio client cannot send the header. The eval runner clears an inherited value in its anonymous modes.
- Q4. Engagement binding: no. Tokens bind to the data dir only.
- Q5. Default CLI TTL: 7 days, with `--ttl` allowing up to 30 days. Short-lived users such as the eval runner issue their own per-run tokens instead of relying on the default.
- Q6. Issuing endpoint: deferred until the web auth story supports it.
- Q7. `X-Artex-Conversation`: rejected with a clear error, not ignored, so a stale header fails loudly instead of silently moving claims to a different holder.
