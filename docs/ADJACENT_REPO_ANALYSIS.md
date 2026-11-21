# Adjacent Repo Analysis

Cross-repo survey looking for patterns, techniques, and ideas worth bringing
into cc-artex. Examined November 2026.

## 1. wsmancer (`~/projects/wsmancer`)

**What it is.** A Go WinRM shell speaking the PowerShell Remoting Protocol end
to end. Typed JSON output, NTLM/Kerberos message sealing, session recovery,
SOCKS5 pivoting, and the full evil-winrm post-exploitation command set, all in a
single static binary.

### Relevant patterns

**NDJSON structured output contract (`internal/output/`).** wsmancer's `-o json`
emits one self-describing JSON object per command, with a `type` discriminator
(`result`, `error`, `event`), correlation IDs, PowerShell stream separation, and
a `had_errors` verdict distinct from the HTTP status. This maps directly to MCP
tool responses. The pattern of normalizing protocol-specific objects (PSObjects)
into plain JSON with reserved metadata keys (`__type`, `__string`) is reusable
for any tool that wraps typed protocol data: WinRM results, LDAP entries, raw
packet fields.

**Batch mode with session-level events (`internal/shell/batch.go`).** Batch
mode runs multiple commands over a single authenticated connection, with the
runspace persisting. Session-level events (shell recovered, state lost) are
emitted in-band as typed objects so an automation consumer knows when to
re-establish state. cc-artex's traffic replay and multi-step exploration chains
face the same problem: a long-running operation whose intermediate state can
break.

**Connection-loss classification (`internal/shell/connlost_test.go`).** The
connection-loss classifier distinguishes transport errors from command errors
using real observed error strings, not invented patterns. Tests assert both
true positives (verbatim strings from live sessions) and false negatives. This
is worth adopting anywhere cc-artex classifies errors to decide retry vs. fail,
especially in proxy and enrichment probes where network errors and application
errors need different handling.

**Safe file-path handling in transfers (`internal/transfer/`).** Remote
filenames are attacker-controlled. `safeLocalPath` strips directory components,
rejects traversal, and belt-and-braces confirms the joined path is inside the
destination. cc-artex ingests target-supplied filenames (HAR imports, file
downloads during exploration) and should apply the same discipline.

**Session Renew pattern (`internal/session/session.go`).** The Session.Renew
method swaps in a freshly built client under a lock while keeping the old one
alive until the swap succeeds, so a failure leaves the caller no worse off.
cc-artex's proxy sessions and enrichment clients could use the same atomic
swap-on-reconnect.

**SOCKS5 dialer as a first-class option (`internal/proxydial/`).** The proxy
dialer is passed into every auth provider as a constructor argument; a nil
dialer dials directly. This is cleaner than environment-variable proxying and
already the pattern cc-artex uses for its own SOCKS5 proxy, but the nil-is-valid
design (every call site holds one unconditionally) is worth adopting more
broadly.

### Ideas worth considering

1. **Correlation IDs in batch tool calls.** wsmancer's batch protocol lets a
   caller send `{"id": ..., "command": "..."}` and get the id echoed on the
   response. cc-artex tools that run multi-step operations (exploration chains,
   enrichment pipelines) could benefit from request/response correlation so the
   agent can match results to the step that produced them.

2. **State-loss signaling.** When wsmancer's shell is recovered after a target
   restart, it emits `state_lost: true` so the consumer knows to reload scripts
   and re-apply patches. For cc-artex, any long-lived stateful operation (a
   proxy session, an active SOCKS tunnel, a recording session) should signal
   state loss to the agent rather than silently reconnecting with a clean slate.

3. **SHA-256 transfer verification.** Every file transfer is verified with a
   hash comparison. cc-artex's HAR export, report generation, and any
   file-producing tool should do the same, especially since the data may cross
   a proxy.

### Not applicable

The Windows-specific details (WinRM sealing, PSRP protocol, NTLM/Kerberos
crypto, AppLocker paths, AMSI bypass) are deep protocol work specific to Windows
remote management. The line editor (reeflective/readline integration, ghost-text
autosuggestions) is a TUI concern cc-artex doesn't have.


## 2. bypass-clm (`~/projects/bypass-clm`)

**What it is.** A tool to bypass PowerShell Constrained Language Mode. Two
variants: a modern .NET 10 self-contained executable that patches
`GetSystemLockdownPolicy` and `AmsiScanBuffer` in memory, and a .NET Framework
4.x version compilable on-target with csc.exe.

### Relevant patterns

**None that transfer.** This is a focused, single-purpose tool. The code is
roughly 150 lines of C# that patches function pointers in memory. The build
system (`build.sh`, Docker, multi-variant modes) is clean but not novel.

### Ideas worth considering

**Build-variant matrix for target-environment constraints.** bypass-clm builds
four variants (standard, slim, lite, dll) from the same source via preprocessor
directives, trading size against capability for different target constraints.
cc-artex's tooling doesn't produce deployable payloads, but the concept of
variant generation based on target constraints (what's available, what's
restricted) has a loose analogy to cc-artex's enrichment probes. When probing a
target, the enrichment pipeline could select probe strategies based on what the
target exposes, similar to how bypass-clm selects a variant based on what the
target has installed. This is a stretch, though: the enrichment pipeline already
selects probes dynamically, and the analogy is thin.

### Not applicable

The core technique (memory patching of CLM/AMSI/ETW via VirtualProtect and
Marshal.Copy) is Windows internals work that has no bearing on cc-artex. The
XOR-encoded string obfuscation pattern is specific to AV evasion. The on-target
compilation workflow (csc.exe, no SDK) is irrelevant to a server-side tool.


## 3. dotfiles / dfinstall (`~/projects/dotfiles`)

**What it is.** A Go CLI that manages dotfiles across machines: idempotent
symlink creation, module-based architecture, backup/restore, drift detection,
dry-run preview, and cross-platform support (Linux, WSL, macOS). It is a real
tool, not a pile of shell scripts.

### Relevant patterns

**Module system with single source of truth (`core/module.go`, `core/link.go`).**
Every module exposes a `Links()` method that is the single source of truth for
Install, Uninstall, Status, and diff. This eliminated a class of bugs where
the four methods independently spelled out paths and drifted. cc-artex's MCP
tools have a similar structure: each tool has a schema, a handler, and
validation logic that could drift. A single-source-of-truth pattern for tool
definitions (schema, handler, and test expectations all derived from one
declaration) would prevent the same kind of inconsistency.

**Install session with process lock and failure tracking
(`core/install_session.go`).** The install session takes a process-wide lock,
tracks whether the operation failed (to avoid persisting bad state), and refuses
to mutate anything under dry-run. cc-artex doesn't have dry-run, but the
session-lock and failure-tracking patterns are relevant to any operation that
mutates the database (inserting findings, updating coverage, recording traffic).

**Idempotent operations with change tracking (`core/output.go`).** The output
system distinguishes `Changed` (something the install did to the machine) from
`Ok` (found it already right). The install report then shows only what actually
changed. cc-artex's exploration tools could benefit from this: when re-running
enrichment or re-scanning an asset, distinguishing "new finding" from
"confirmed existing" is useful signal for the agent.

**Drift detection (`core/drift.go`, `core/canonical.go`).** dfinstall detects
when managed symlinks point at different clones, or when managed files have been
edited outside the tool. This is a form of configuration drift detection.
cc-artex's coverage ledger tracks what has been tested, but it doesn't track
whether the target's state has changed since the last test. Drift detection for
the exploration graph (has the target changed since we last probed it?) would
help the agent decide whether to re-probe.

**Subprocess execution with tiered deadlines (`core/exec.go`).** Three wrappers
(`runProbe`, `runNetProbe`, `runCmd`) apply different timeouts for different
classes of work (quick queries, network reads, long installs). cc-artex's
enrichment probes (DNS, HTTP, TLS, tech fingerprint) would benefit from the same
tiered-deadline approach: a DNS lookup should time out much faster than a full
HTTP probe.

### Ideas worth considering

1. **Tiered probe timeouts.** cc-artex enrichment probes currently share timeout
   configuration. Adopting dfinstall's tiered model (fast probes like DNS get
   short timeouts, slow probes like full HTTP crawls get longer ones) would make
   enrichment more responsive and prevent a hung probe from blocking the pipeline.

2. **Change-vs-confirmation tracking.** When enrichment or exploration re-visits
   a target, distinguishing "new information" from "confirmed what we already
   knew" is valuable signal. The agent can prioritize genuinely new findings
   over confirmations.

### Not applicable

The symlink management, dotfile linking, backup/restore, OS detection,
package-manager abstraction, and cross-platform concerns are all infrastructure
management patterns that don't apply to a pentest tool. The toolkit registry
(external JSON catalog of installable tools) is interesting engineering but
solves a different problem. The UI patterns (spinner, colored output, interactive
menus) are CLI concerns cc-artex doesn't have as an MCP server.


## Summary: top ideas ranked by value to cc-artex

### 1. State-loss signaling for long-lived operations
**What:** When a stateful session (proxy, SOCKS tunnel, recording) reconnects
after a failure, signal to the agent that accumulated state was lost, rather than
silently presenting a clean session as though it were the old one.
**Why it matters:** An agent that doesn't know state was lost will issue commands
that assume the previous state still exists (e.g., replaying traffic through a
proxy that no longer has the original TLS context). wsmancer learned this the
hard way with AMSI patches surviving shell recovery.
**Effort:** Low. Add a `state_lost` field to relevant tool responses and emit it
on reconnection. The infrastructure already exists in the exploration graph's
fact tracking.

### 2. Tiered timeouts for enrichment probes
**What:** Apply different timeouts to different classes of enrichment work: fast
for DNS (2-5s), medium for HTTP probes (15-30s), long for full tech
fingerprinting (60s+).
**Why it matters:** A single timeout that accommodates the slowest probe makes
fast probes wait too long on failure. A DNS lookup hanging for 30 seconds before
falling back is wasted time the agent could spend elsewhere.
**Effort:** Low. The enrichment pipeline already dispatches probes individually;
this is configuration, not architecture.

### 3. Connection-loss classification with real error strings
**What:** When a network operation fails, classify the error as "transport
failure" (retry) vs. "application error" (report) using patterns captured from
real observed failures, not invented heuristics.
**Why it matters:** Proxy connections, enrichment probes, and database
connections all fail in ways that look similar but need different responses. A
TCP reset needs a retry; a 403 needs to be reported as a finding. Getting this
wrong either retries forever or misses transient failures.
**Effort:** Low-medium. Write the classifier, seed it with real error strings
from testing, and add the true-positive/false-negative test pattern from
wsmancer.

### 4. Change-vs-confirmation tracking for enrichment
**What:** When enrichment revisits a target, distinguish "new finding" from
"confirmed existing finding" in the tool response, so the agent knows what
genuinely changed.
**Why it matters:** An autonomous agent re-running enrichment on a target it has
already probed should focus on deltas, not re-process the same DNS records and
TLS certificates. This is especially important for long engagements where
targets evolve.
**Effort:** Medium. The store already tracks findings; the enrichment layer
needs to compare before inserting and annotate the response accordingly.

### 5. Single-source-of-truth tool definitions
**What:** Derive each MCP tool's schema, handler registration, and test
expectations from one declaration, so they cannot drift apart.
**Why it matters:** As the tool count grows, maintaining consistency between the
JSON schema exposed to the agent, the handler that implements it, and the tests
that exercise it becomes a real maintenance burden. dfinstall's LinkSet pattern
(one `Links()` call drives Install, Uninstall, Status, and diff) is the right
shape.
**Effort:** Medium-high. This is a structural refactor of the tool registration
system, best done incrementally as tools are added or modified.
