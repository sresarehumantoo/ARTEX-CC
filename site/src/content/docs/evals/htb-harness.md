---
title: HTB harness assessment
description: "Whether ARTEX should automate Hack The Box runs: compliance, architecture and risk."
sidebar:
  order: 3
---

Should ARTEX include a test harness that automates spawning HTB boxes, running
ARTEX against them, and validating results? This document evaluates compliance,
architecture, and practical risk.

## 1. Compliance summary

The analysis is based on the HTB Acceptable Use Policy (effective 01 April 2026),
the Platform Rules (help.hackthebox.com, article 12325897), and the
Streaming/Writeup Guidelines (article 5188925).

### 1.1 Automated access (AUP Section 6)

**Section 6.1 (No automated access):** "Do not deploy, operate, or permit any bot,
crawler, scraper, AI agent, large language model, reinforcement learning agent,
multi-agent system, or similar technology ('Automated Systems') to directly access,
interact with, or extract data from any part of the Services, except where
expressly authorised under a Service Agreement with Hack The Box."

The `htb` CLI tool uses HTB's API v4 to spawn/stop machines. That API access is
authorized for authenticated users with a valid API token. Using the HTB MCP server
(which HTB itself ships and promotes) is explicitly within the platform's intended
use. However, an AI agent autonomously driving the HTB platform API in a loop is a
gray area: the AUP bans "Automated Systems" from directly accessing the Services,
with an exception only for "expressly authorised" access under a Service Agreement.
A personal subscription is not a Service Agreement in the B2B sense.

**Section 6.3 (Programmatic Access Through APIs):** "Where Hack The Box authorises
access through its APIs, MCP, or integration protocols, such access is subject to
the Service Agreement and this AUP." This confirms that HTB acknowledges API and
MCP access, but ties it to the Service Agreement and AUP compliance.

**Verdict:** Using `htb` CLI to spawn/stop machines manually or semi-automatically
(human-in-the-loop) is likely fine. Fully autonomous orchestration (harness spawns
box, runs agent, scores, shuts down, next box) is at best ambiguous, at worst a
violation of 6.1. HTB's own MCP feature blurs the line, but the AUP language is
broad enough that a fully automated eval harness could be flagged.

### 1.2 AI training and benchmarking (AUP Section 6.2, Platform Rules)

**Section 6.2 (Prohibition on AI training and content exploitation):** "You shall
not use any content from the Services, including challenges, solutions, write-ups,
hints, flags, methods, or any content or data output by the Services, to train,
evaluate, fine-tune, test, benchmark, or develop any machine learning model,
artificial intelligence system, large language model, or similar technology."

**Platform Rules (AI):** "It is strictly prohibited to use, reproduce, adapt,
compile, or otherwise exploit any Hack The Box content to train, evaluate,
fine-tune, test, benchmark, or develop AI or machine learning systems."

**Verdict:** This is the critical blocker. Using HTB boxes to "evaluate" or
"benchmark" ARTEX (an AI-assisted pentest system) is squarely within the
prohibition. The rule bans using HTB content to "evaluate" or "test" any AI system,
"regardless of the means of acquisition, format, or whether the use is commercial
or non-commercial." Even internal-only, never-published eval runs appear to
violate this clause as written.

**Exception:** "This restriction does not apply where expressly authorised under a
Service Agreement with Hack The Box." A B2B Service Agreement could unlock this.
HTB's own MCP feature and "Attack of the Agents" CTF suggest they are building
toward sanctioned AI-on-HTB workflows, but as of the current AUP, the default is
prohibition.

### 1.3 Content publication (AUP Section 8)

**Section 8.2 (Active content):** "Do not share, publish, sell, or disclose
solutions, flags, answers, write-ups, hints, or spoilers for any active content or
event."

**Section 8.3 (Retired content):** Solutions, write-ups, walkthroughs, videos, and
streams may be published for Retired Content only, "and only for the purpose of
community knowledge-sharing, education, and discussion."

**Verdict for the repo:**
- Flags, root proofs, and specific exploit chains for ANY box: never commit.
- Box names and IDs (Management, Cohort, etc.) in a test plan: acceptable (this
  is metadata, not a solution).
- Generic tool sequences and pass criteria: acceptable (describes what ARTEX
  should do, not how to solve the box).
- Specific attack paths ("SSRF on port X leads to ...") with enough detail to
  reproduce the solve: not acceptable for active boxes, acceptable for retired
  boxes if framed as educational.

### 1.4 Fair play (AUP Section 11)

**Section 11.3(b):** Cheating includes "having another person, automated system, or
any combination of these produce, obtain, or submit work on your behalf."

**Section 11.4:** "No enabling cheating. Do not help others cheat or build the
infrastructure for cheating." This includes "developing or distributing tools
purpose-built to automate the completion of Hack The Box assessments."

**Verdict:** A published, automated HTB-solving harness could be classified as a
tool "purpose-built to automate the completion of Hack The Box assessments."
ARTEX itself is a general-purpose pentest tool, not an HTB-specific cheating
tool, but an HTB-specific eval harness with box-specific scoring strays closer to
that line.

### 1.5 HTB's own MCP feature

HTB launched an official MCP Server in mid-2025, allowing AI agents to browse CTF
events, control challenge environments, and submit flags. Admin controls exist for
enabling/disabling MCP access. This signals that HTB is actively building toward
AI-on-HTB workflows, but the AUP has not caught up. The MCP feature appears
designed for CTF events (where event-specific rules may override the AUP), not
general lab use.

## 2. Recommendation: Option C (scripts in docs), with caveats

**Do not build an automated harness in the repo.** The compliance picture is too
hostile. AUP Section 6.2 explicitly prohibits using HTB content to evaluate or
benchmark AI systems. Section 11.4 prohibits distributing tools that automate HTB
assessments. Publishing an automated harness in a public GitHub repo combines both
violations.

**What to do instead:**

### For private development (not in the repo)

Keep the eval workflow manual and local. The existing `docs/HTB_TEST_PLAN.md`
already describes the right approach: human-initiated, one box at a time, with
Claude Code's `/loop` for the agent loop. The human is in the loop (spawns the box,
starts the engagement, monitors progress, decides when to stop). This is closer to
"using an AI coding assistant while hacking" than "automated benchmarking."

### For the public repo

Publish only what does not depend on HTB content:

1. **Generic orchestration eval harness** (already planned in `EVAL_PLAN.md`): The
   custom vulnerable app, DVWA, and Juice Shop targets are self-hosted, have no
   ToS restrictions, and can be automated freely. Invest here.
2. **MCP protocol conformance tests**: Test that ARTEX's tools accept valid
   input, reject invalid input, and return well-formed output. These are pure unit
   and integration tests with no external dependency.
3. **Orchestration pattern tests**: Transcript-based evals (Section 2.3 of
   EVAL_PLAN.md) that verify Claude calls tools in the right order, with no real
   target. These are safe to automate and publish.

### If HTB terms change

HTB's MCP feature and "Attack of the Agents" CTF suggest the platform is moving
toward sanctioned AI workflows. If HTB publishes an AI evaluation policy, a
developer program, or a B2B agreement that explicitly allows using lab machines to
evaluate AI tools, revisit this decision. Watch for:
- Changes to AUP Section 6.2 (AI training/benchmarking exception for authorized
  API users).
- An HTB developer or partner program for tool authors.
- Explicit guidance on the MCP feature's scope (CTF-only vs. general labs).

## 3. What belongs in the ARTEX repo

### Safe to include

- `docs/HTB_TEST_PLAN.md` (already merged): Box names, IDs, difficulty, tool
  sequences, and pass criteria. This is a test plan, not a solution. It describes
  what ARTEX should do, not how to solve the box.
- Generic eval infrastructure (`eval/` directory): Docker-based vulnerable apps,
  the eval runner, the scorer, ground-truth manifests for self-hosted targets.
- MCP config examples showing how to point ARTEX at any target.
- Tool sequence diagrams and expected call patterns (generic, not box-specific).

### Borderline (keep but monitor)

- Box-specific tool sequences in `HTB_TEST_PLAN.md`: Currently safe because they
  describe ARTEX's behavior, not the box's vulnerabilities. If the boxes retire
  and the plan starts including specific exploit details, that is still fine under
  Section 8.3. While boxes are active, keep descriptions at the "what tools to
  exercise" level, not the "here is the attack chain" level.
- Given credentials in the test plan (e.g., contractor/Contractor2026!): These are
  provided by HTB as part of the box and are not discovered through exploitation.
  Borderline. The credential is in the box's starting material (like a username on
  a login page), but publishing it for an active box could be seen as a hint. When
  the box retires, this is unambiguously fine.

### Must not include

- HTB flags (user.txt, root.txt contents).
- Automated flag submission code.
- Expected flag values in test assertions.
- Full exploit code specific to an HTB box (as opposed to generic tool usage).
- An automated harness that spawns boxes, runs the agent, and scores results
  without human intervention.
- HTB API tokens or credentials (even in examples).

## 4. What belongs elsewhere

### Private local workflow (never committed)

- The actual eval runs: spawn box, start ARTEX, let Claude work, review
  results.
- Box-specific scoring notes: "Management should find at least 1 SQLi" with
  expected flag hashes or proof strings.
- Performance data: tool call counts, timing, token usage per run.
- Comparison data: ARTEX vs. manual pentester findings on the same box.

### Potential future separate repo (if terms change)

If HTB publishes a developer/partner program that allows AI tool evaluation:
- `artex-eval` repo with HTB-specific harness code.
- Box-specific scenarios with ground-truth manifests (retired boxes only).
- Automated spawn, run, score, teardown cycle.
- Scoring logic that checks findings against expected vulnerability classes (not
  flags).

This repo does not exist today and should not be created until the compliance
picture changes.

## 5. Minimum viable approach (what to build now)

### In the ARTEX repo

**Phase 1 (immediate, already in EVAL_PLAN.md):**
- Tool-level unit tests with mockStore (extend existing `tools/tools_test.go`).
- Store integration tests against Postgres (extend `store/store_test.go`).
- Schema conformance tests (verify declared JSON Schema matches runtime).

**Phase 2 (next):**
- Custom vulnerable Docker app with 3-5 planted vulns and a ground-truth
  manifest.
- End-to-end eval harness: `docker compose up`, seed database, run ARTEX, run
  Claude Code, score results against ground truth.
- This is the HTB-equivalent workflow but against a target we control, with no
  ToS constraints and full reproducibility.

**Phase 3 (stretch):**
- Orchestration transcript evals (pre-seeded database, no real target).
- Safety evals (scope enforcement, constraint adherence).

### For HTB testing (private, manual)

Follow `docs/HTB_TEST_PLAN.md` as written. The plan is already well-structured for
manual execution:
1. Spawn the box (`htb spawn`).
2. Add hosts (`htb-hosts add`).
3. Init the engagement (`artex-cc init`).
4. Start Claude Code with the MCP config.
5. Use `/loop` for semi-autonomous execution.
6. Review results manually.
7. Tear down (`htb stop`, `htb-hosts clean`).

No harness code needed. The human is the harness.

## 6. Risk assessment

### Legal risk: MODERATE

The AUP language is broad and explicitly covers AI benchmarking. Publishing an
automated HTB eval harness in a public repo creates a clear paper trail of
prohibited use. Even private use technically violates Section 6.2, though
enforcement against a single user running boxes they are paying for is unlikely.
The risk increases sharply if the harness is public and could be used by others to
automate HTB assessments (Section 11.4).

### Ethical risk: LOW

ARTEX is a legitimate pentest tool. Using HTB to validate it is conceptually no
different from a pentester practicing on HTB. The ethical concern is about
automation and scale, not intent. Running 5 boxes manually is ethically
unproblematic. Automating the same workflow at scale (or enabling others to) is
where it gets questionable.

### Practical risk: LOW-MODERATE

- HTB machine availability depends on subscription tier and server load.
- Machines can be reset by other users mid-test.
- VPN connectivity adds a failure mode.
- Results are non-deterministic (LLM behavior varies between runs).
- The custom Docker app from EVAL_PLAN.md is strictly better for automated,
  reproducible testing. HTB adds real-world variety but worse reproducibility.

### Account risk: LOW-MODERATE

If HTB detects automated API usage patterns (rapid spawn/stop cycles, API calls
from AI agents), they could warn or suspend the account. The `htb` CLI tool uses
the same API as the web UI, so individual spawn/stop calls are indistinguishable
from normal use. But a repeated, patterned sequence (spawn, wait, stop, spawn next)
could be flagged by monitoring.

### Reputational risk: LOW if private, MODERATE if public

Publishing a "here is how to automate HTB box solving with AI" harness in a public
repo could draw negative attention from the HTB community, which values manual
skill-building. This is separate from the legal question. Even if HTB's terms
allowed it, the community norms lean against automation.

## References

- HTB Acceptable Use Policy (01 April 2026):
  https://resources.hackthebox.com/hubfs/Legal/AUP.pdf
- HTB Platform Rules:
  https://help.hackthebox.com/en/articles/12325897-hack-the-box-platform-rules
- HTB Streaming/Writeup Guidelines:
  https://help.hackthebox.com/en/articles/5188925-streaming-writeups-walkthrough-guidelines
- HTB User Agreement:
  https://resources.hackthebox.com/hubfs/Legal/UA.pdf
- HTB MCP Blog Post:
  https://www.hackthebox.com/blog/model-context-protocol
