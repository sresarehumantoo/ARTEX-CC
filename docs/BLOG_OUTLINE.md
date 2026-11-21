# Launch Blog Post Outline

**Platform:** dev.to (primary), cross-post to Hashnode.
**Target length:** ~1500 words.

## Title options

1. "Autonomous Pentesting with Claude Code: How an Exploration Graph Keeps the AI on Track"
2. "47 MCP Tools for Pentesting: Turning Claude Code into a Red Team Partner"
3. "I Replaced My Pentest Agent Loop with Claude Code and 47 MCP Tools"
4. "cc-artex: State-Aware Pentesting Through Claude Code's MCP Tools"

## Section outline

### Hook (150 words)
The problem: LLM-driven pentesting tools lose state. They forget what they scanned, repeat work, and have no concept of "where was I?" Open with a concrete example of this failure mode: an AI agent re-running Nmap on the same target because it lost its scan results.

### What cc-artex is (200 words)
An MCP server that gives Claude Code 47 pentest tools backed by a PostgreSQL exploration graph. No separate API key, no token budget. Your Claude subscription is the entire AI engine.

Include the architecture diagram (docs/architecture.svg) showing the data flow.

### The exploration graph, explained (300 words)
This is the core differentiator. Walk through what the graph tracks:
- Assets (domains, IPs, services, endpoints)
- Intents (planned next steps)
- Facts (observations and discoveries)
- Findings (vulnerabilities with severity and evidence)
- Coverage (what was tested, with what result)

Explain how `graph_overview` reconstructs context after hours away. Include a screenshot of the graph overview output.

### A real session (400 words)
Walk through a session against an HTB box. Show the tool calls in sequence:
1. Init and goal setting
2. Recon with enrich tools
3. Asset insertion as services are discovered
4. Intent planning
5. Traffic capture through the recording proxy
6. Finding reporting with evidence chains
7. Report generation

Include 2-3 terminal screenshots showing key moments. Reference the demo GIF.

### Why not just use PentestGPT / HexStrike? (200 words)
Brief, factual comparison. Lead with what cc-artex does differently (state management, evidence chains), not what competitors lack. Include the comparison table from the README.

### Getting started (150 words)
Three install paths: Homebrew, Docker Compose, from source. Show the minimal `docker compose up` path. Link to the README for full instructions.

### What's next (100 words)
Roadmap highlights: web UI for the exploration graph, additional tool integrations, community-contributed skills/recipes. Invite contributions and feedback. Link to the GitHub repo.

## Screenshots and GIFs to include

1. Architecture diagram (docs/architecture.svg)
2. Demo GIF (the README hero GIF, once produced)
3. Terminal screenshot: `graph_overview` output showing a populated engagement
4. Terminal screenshot: `report_finding` with evidence chain
5. Terminal screenshot: the generated report

## Tags for dev.to

`security`, `ai`, `opensource`, `tutorial`
