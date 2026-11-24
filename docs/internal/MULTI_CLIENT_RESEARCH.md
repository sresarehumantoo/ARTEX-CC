# Multi-Client Compatibility Research

Evaluating whether ARTEX can support MCP clients beyond Claude Code,
including open-source alternatives with local/self-hosted LLM backends.

Research date: 2026-11-21.

---

## 1. OpenCode deep-dive

### What it is

OpenCode was a Go-based, MIT-licensed, terminal-first AI coding agent with a
TUI, CLI, beta desktop app, web UI, and IDE extension. It supported MCP over
both stdio and SSE, and was model-agnostic (OpenAI, Anthropic, Google Gemini,
AWS Bedrock, Groq, Azure OpenAI, OpenRouter, GitHub Copilot, Vertex AI, plus
any OpenAI-compatible endpoint via `LOCAL_ENDPOINT`).

MCP servers are configured in `opencode.jsonc` under a `mcpServers` key, with
`stdio` or `sse` transport types. This is the same pattern Claude Code uses,
so ARTEX's existing stdio and HTTP transports would work without changes.

### Current status: archived, succeeded by Crush

The OpenCode GitHub repository (`opencode-ai/opencode`) is archived and
read-only. The project moved to **Crush**, developed by the Charm team
(`charmbracelet/crush`). Crush is also Go-based, terminal-native, and supports
MCP servers in a `crush.json` config file with the same stdio transport model.

Crush's license is FSL-1.1-MIT (source-available; restricts competing
commercial use until each release converts to MIT after two years). This is
_not_ OSI-approved open source, which matters for some users.

### Could a user run ARTEX with OpenCode/Crush?

Yes. Both support MCP stdio and SSE/HTTP transports. A user would add
ARTEX's command to their config file and the tools would appear. No changes
to ARTEX required for basic tool calling.

The `artex://engagement` resource is the open question: OpenCode's docs don't
confirm whether it reads MCP resources or only calls tools. If a client ignores
resources, the orchestration guide won't be injected, and the LLM won't know
the orient/plan/execute/loop workflow. See section 4 for the fix.

### Practical limits with local models

A DEV Community guide on OpenCode + Ollama reports that file writes are
unreliable on 7B models and recommends 14B+ for anything beyond plan-only mode.
OpenCode's own docs warn that token-heavy MCP servers can exceed context limits
on small local models.


## 2. MCP client landscape

### Compatibility matrix

| Client | MCP Tools | MCP Resources | Transport | Local models | License | Notes |
|---|---|---|---|---|---|---|
| **Claude Code** | Yes | Yes | stdio | No (Anthropic only) | Proprietary | Native MCP; reads resources automatically |
| **Crush** (ex-OpenCode) | Yes | Unclear | stdio, SSE | Yes (any OpenAI-compat) | FSL-1.1-MIT | Successor to OpenCode |
| **Cursor** | Yes | Yes | stdio | Partial (via API proxy) | Proprietary | IDE-based; has approval model for tool execution |
| **Windsurf** | Yes | Yes | stdio | Partial | Proprietary | IDE-based |
| **Cline** | Yes | Yes | stdio | Yes (Ollama, LM Studio) | Apache-2.0 | VS Code/JetBrains extension; also standalone CLI |
| **Continue** | Yes | Unclear | stdio | Yes (Ollama, llama.cpp) | Apache-2.0 | VS Code/JetBrains extension |
| **Zed** | Yes | No (as of late 2026) | stdio | Yes (custom provider) | Varies | Only tools and prompts confirmed |
| **Gemini CLI** | Yes | Yes | stdio | No (Google only) | Apache-2.0 | Google's terminal agent |

### Key findings

1. **Tools are universal.** Every MCP client listed supports tool calling. ARTEX's 30+ tools will appear in any of them.

2. **Resources are inconsistent.** Claude Code reads resources automatically. Zed explicitly does not support resources. Others are unclear. This means the `artex://engagement` orchestration guide (the orient/plan/execute/loop instructions) cannot be reliably delivered through MCP resources alone.

3. **Cline is the strongest open-source option** for local model + MCP tool use. It's Apache-2.0, runs in VS Code or standalone, supports Ollama natively, and has MCP tool support. If we target one alternative client first, Cline is it.

4. **MCP went stateless in July 2026.** Protocol revision 2026-07-28 removed session tracking at the protocol layer. ARTEX already manages sessions internally (task_id/exploration_id), so this shouldn't matter.


## 3. Local model assessment

### Which models work for tool calling?

Based on the TrustedSec benchmark (8 challenges, 100 runs each, all via Ollama)
and multiple comparison guides:

| Model | Size | Tool-call pass rate | Notes |
|---|---|---|---|
| Gemma 4 | 31B | 98.5% | Best overall in TrustedSec benchmark |
| Qwen 3.5 | 27B | 97.5% | Strong; widely recommended for tool calling |
| Devstral Small 2 | 24B | 95.6% | Fastest (22.9s avg per run) |
| Nemotron 3 Super | ~27B | 92.0% | Decent |
| Qwen 3 Coder | ~32B | 86.5% | Worse than smaller Qwen 3.5 |
| Qwen 3 | 32B | 85.4% | Lower than expected for its size |

### Minimum requirements

- **14B+** for reliable single-step tool calls.
- **27-32B** for multi-step tool use with acceptable reliability.
- **70B** (Llama 3.3) for the highest reported tool-call accuracy (~97%
  well-formed calls), but needs 48GB+ VRAM.
- 24GB VRAM is the practical floor for a useful model (runs 24-32B quantized).

### Multi-step and pentesting viability

The TrustedSec benchmark is the most relevant data point. Key findings:

- Local 27-32B models handle single-step tool calls well (85-98% pass rate).
- **None of the local models completed multi-step challenges** that required
  extraction or complex attack chains (JWT algorithm confusion). They succeeded
  only at simpler challenges or through token replay rather than the intended
  attack path.
- Claude Code (informal test, not part of the benchmark) solved all challenges
  in under 10 minutes. The gap is attributed to both the model and the richer
  harness.

Academic research corroborates this:
- An ALA 2026 paper showed Qwen-14B/32B executing real exploits in a
  structured multi-agent framework, but with significant orchestration
  scaffolding.
- An IJCAI 2026 paper (AutoPentest-Bench) concluded that fully autonomous
  IP-to-root penetration testing "remains challenging" across all 10 LLMs
  tested.
- Common failure modes: hallucinated commands, tool misuse, redundant looping,
  state fragmentation across long sequences.

### Honest assessment

**A 70B local model cannot match Claude's tool-use capability for pentesting
today.** The gap is real and not just about raw intelligence:

1. **Multi-step planning.** ARTEX's workflow (orient/plan/execute/loop with
   subagent dispatch) requires sustained coherent reasoning across dozens of
   tool calls. Local models degrade noticeably after 5-10 steps.

2. **Tool description comprehension.** ARTEX has 30+ tools with detailed
   schemas. Smaller models struggle with the working memory needed to select
   the right tool with the right parameters when the catalog is this large.

3. **Context management.** The orchestration guide alone is ~1200 tokens. Add
   graph_overview output, tool schemas, and conversation history, and a 32B
   model's effective context fills fast.

4. **Structured output reliability.** Tool calling requires well-formed JSON
   arguments. At 27-32B, this works ~90% of the time. At the ARTEX scale
   (complex nested schemas like `insert_assets`), the error rate would be
   higher.

**However:** a local model could plausibly handle a subset of ARTEX's
workflow: single-intent execution (run one tool, record one fact), read-only
queries (graph_overview, list_assets, list_findings), and report generation.
The coordinator/planner role is the part that needs frontier-class reasoning.


## 4. ARTEX changes needed

### What is client-specific today

After reading the codebase (`mcpserver/server.go`, `mcpserver/handler.go`,
`tools/exploration_tools.go`, and all tool registration files):

1. **The MCP protocol layer is already generic.** ARTEX uses `mcp-go`
   (`github.com/mark3labs/mcp-go`), the standard Go MCP library. Tools are
   registered via `mcp.NewToolWithRawSchema()` with standard JSON Schema.
   The handler converts to/from `mcp.CallToolRequest`/`mcp.CallToolResult`.
   Nothing here is Claude-specific.

2. **Transport is already dual.** `transport_stdio.go` for Claude Code,
   `transport_http.go` (Streamable HTTP) for everything else. Both work today.

3. **Tool descriptions are model-agnostic.** I read every tool description
   (`graph_overview`, `add_intent`, `record_fact`, `report_finding`,
   `insert_assets`, etc.). None mention Claude by name. They use generic
   imperative language: "Call this FIRST", "Batch preferred", "Returns an ids
   array". These read as standard tool documentation that any capable LLM
   should follow.

4. **The orchestration guide IS Claude-centric in delivery, not content.** The
   `artex://engagement` resource (`server.go:127-165`) contains the
   orient/plan/execute/loop workflow. The text itself is model-agnostic (it
   says "you" not "Claude"), but it's delivered as an MCP resource. If a client
   doesn't read resources, the LLM never sees these instructions.

5. **The README and marketing are Claude-branded.** "Use your Claude
   subscription", "Claude Code is the LLM", "No API key needed". This is
   positioning, not a technical constraint.

### What needs to change for multi-client support

**Minimal changes (low effort, high value):**

- **Move the orchestration guide into tool descriptions.** Instead of relying
  on the `artex://engagement` resource (which not all clients read), embed a
  condensed version of the workflow in `graph_overview`'s description. This is
  the tool users call first anyway. The description already says "Call this
  FIRST at the start of every planning cycle"; extend it with the
  orient/plan/execute/loop pattern. This one change makes ARTEX work with
  any MCP client, resource support or not.

- **Add a `--guide` flag or `get_guide` tool.** For clients that support
  resources, keep the resource. For others, add a simple tool that returns the
  orchestration guide text. The LLM can call it at the start of a session.

- **Update README with multi-client setup instructions.** Add Cline, Crush,
  and generic MCP client configuration examples alongside the existing Claude
  Code instructions.

**No changes needed:**

- Tool schemas: already standard JSON Schema.
- Tool descriptions: already model-agnostic.
- Transport: stdio and HTTP both work.
- Session management: task_id/exploration_id are passed explicitly, not tied to
  MCP session state.
- The `switch_engagement` tool: works via tool call, not client-specific.

**Moderate effort (if local model support becomes a priority):**

- **Add a "simple mode" tool subset.** Register only the 8-10 most essential
  tools (graph_overview, add_intent, record_fact, report_finding,
  insert_assets, list_assets, list_findings, prove_goal) to reduce schema
  overhead for smaller models.

- **Shorter tool descriptions.** The current descriptions are thorough (good
  for Claude) but long. A `--compact-descriptions` flag could swap in terse
  one-liners for local models that struggle with long tool catalogs.

- **Simpler schemas for critical tools.** `insert_assets` has a complex nested
  schema with many optional fields. A simplified `add_host`/`add_service`
  wrapper would help smaller models.


## 5. Competitive analysis

### AI pentest tool landscape

| Tool | Approach | Backend | MCP | Open source | Maturity |
|---|---|---|---|---|---|
| **ARTEX** | MCP tool server; external LLM drives | Any MCP client | Yes (is a server) | Yes | Active, 30+ tools |
| **HexStrike AI** | MCP tool server; external LLM drives | Any MCP client | Yes (is a server) | Yes (Apache-2.0) | Active, 127-150 tools |
| **PentestGPT** | Copilot; advises, human executes | GPT-4 (original) | No | Yes | Pioneer (~12.5K stars), slowing |
| **AutoPentest** (JuliusHenke) | Autonomous agent | GPT-4o via LangChain | No | Yes | Research prototype |
| **Strix** | Autonomous agentic platform | Unspecified | No | Yes (Apache-2.0) | Active, proxy + browser |
| **CAI** | Autonomous framework | 300+ backends, Ollama | No | Yes | Air-gap capable |
| **PentAGI** | Autonomous multi-agent | LiteLLM (12+ providers) | No | Yes | 4 sub-agents, Docker sandboxes |
| **AutoPentest-AI** (bhavsec) | MCP server | Any MCP client | Yes | Yes | 68+ tools, 109 WSTG tests |
| **PentestMCP** | MCP server library | Tested with o3, Gemini | Yes | Yes | Wraps nmap/nuclei/metasploit |
| **Raptor** | Claude Code native | Claude | No | Unclear | AFL + CodeQL integration |

### ARTEX's positioning

**ARTEX's unique advantage is the exploration graph.** Most competitors are
either tool collections (HexStrike, PentestMCP) or autonomous agents with
their own LLM loop (PentAGI, Strix, CAI).

ARTEX is neither. It is a **stateful task-tracking layer** that any LLM can
drive. The exploration graph (intents, facts, findings, goals, coverage)
solves the core problem of autonomous pentesting: maintaining coherent state
across long engagements where context windows fill up. No other MCP-based
pentest tool has this.

Specific advantages:
- **State persistence.** The graph survives context exhaustion, session
  restarts, and model swaps. HexStrike has no equivalent.
- **Coverage tracking.** ARTEX knows what has been tested and what hasn't.
  Pure tool servers don't track this.
- **Multi-agent coordination.** The intent system supports parallel subagent
  dispatch with results flowing back through the graph. This is designed into
  the data model, not bolted on.
- **Evidence chain.** Findings link to traffic snapshots, facts link to
  intents, goals link to evidence. This produces auditable results.

If ARTEX supports multiple clients, its positioning becomes: "the pentest
task-tracking layer for any AI agent", which is a broader market than "Claude
Code pentest tools."

### HexStrike AI as direct competitor

HexStrike is the closest competitor: also an MCP server, also open source, also
wraps offensive tools. But it's a **tool collection**, not a task manager. It
has ~150 tools (nmap, burp, metasploit wrappers) but no exploration graph, no
intent tracking, no coverage ledger, no finding-to-traffic evidence binding.

A combined deployment (HexStrike for raw tool execution, ARTEX for
orchestration and state) would actually make sense.


## 6. Recommendation

### Should we invest in multi-client support?

**Yes, but incrementally, and not because of local models.**

The case for multi-client support:

1. **Low technical cost.** ARTEX's MCP implementation is already
   model-agnostic. The orchestration guide delivery is the only real gap,
   and it's a small fix (move guide text into tool descriptions or add a
   `get_guide` tool).

2. **Market expansion.** Cline has a large user base of developers who
   already use local models or non-Anthropic APIs. Supporting them costs
   nearly nothing if the tools already work over MCP.

3. **Resilience.** Single-vendor lock-in is a risk. If Anthropic changes
   pricing, rate limits, or the Claude Code model, users have no fallback.

4. **HexStrike is already there.** It supports any MCP client. If ARTEX
   doesn't, users who want multi-client support will use HexStrike instead,
   even though ARTEX's exploration graph is a better foundation.

The case against local models as a priority:

1. **Tool use quality gap is real.** No local model today can reliably run a
   multi-step pentest through 30+ tools. The coordinator/planner role needs
   frontier reasoning.

2. **The gap is closing but slowly.** 27-32B models went from unusable to
   85-98% single-step tool-call accuracy in the past year. Multi-step
   reliability is still poor.

3. **The payoff is low.** Users who need local models (air-gapped
   environments, cost sensitivity) are a small segment, and the ones who
   need autonomous pentesting are a small segment of that.

### Recommended plan

**Phase 1 (1-2 days, do now):**
- Move the orchestration guide into `graph_overview`'s description (or add a
  `get_guide` tool).
- Add Cline and generic MCP client configuration to the README.
- Test with Cline + Claude API key to verify tool calling works end-to-end.

**Phase 2 (1 week, when a user asks):**
- Add `--compact-descriptions` mode for smaller models.
- Add simplified tool wrappers (`add_host`, `add_service`) for the most
  complex schemas.
- Test with Cline + Ollama + Qwen 3.5 27B on a simple engagement (single
  target, 2-3 intents).

**Phase 3 (defer until local models improve):**
- Full local-model testing matrix.
- Optimized tool subsets for different model tiers.
- Local-model-specific orchestration prompts (simpler, more directive).

### Bottom line

ARTEX is 95% client-agnostic already. The 5% gap is the orchestration guide
delivery mechanism. Fix that, update the README, and the tool works with any
MCP client today. Local model support is a "nice to have" that isn't worth
significant investment until 70B+ models can reliably chain 10+ tool calls
without degrading.
