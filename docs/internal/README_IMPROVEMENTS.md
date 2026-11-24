# README improvement plan

## 1. Current state assessment

| Dimension | Score | Notes |
|-----------|-------|-------|
| First impression / hook | 4/5 | Strong tagline and value prop. "Use your Claude subscription to run pentests. No API key needed." grabs attention fast. "You bring the brain. cc-artex brings the hands." is memorable. Missing a demo GIF or terminal screenshot at the top to make it visual. |
| Visual appeal | 3/5 | Has logo SVG (dark-mode aware), architecture SVG, three badges. No demo GIF, no terminal screenshot, fewer badges than comparable projects. Page is predominantly text below the hero. |
| Quick start clarity | 3/5 | Two separate quick-start sections (native and Docker) with three different MCP config examples. Newcomer must choose a path with no guidance on which is recommended. Docker path is the lowest-friction but comes second. PostgreSQL prerequisite adds friction for the native path. |
| Feature showcase | 3/5 | 40+ tools listed in detailed tables, well-categorized but overwhelming. No "highlight reel" of key capabilities. The session transcript example is effective but buried after install. |
| Architecture understanding | 4/5 | Clean SVG diagram, good directory tree. The "How it works" prose explains the flow well. Missing data-flow detail for a pentest session. |
| Install options | 4/5 | Covers Homebrew, binaries, go install, Docker Compose. All common paths present. Binary section could use OS/arch guidance. |
| Comparison/positioning | 4/5 | "cc-artex vs ARTEX" table is excellent. "vs manual pentesting" paragraph is solid. No comparison with other AI security tools. |
| Community/contribution | 2/5 | Just a link to CONTRIBUTING.md. No Discord/Slack, no "get help" section, no contributor acknowledgments. Weakest area compared to every project surveyed. |
| SEO/discoverability | 4/5 | GitHub topics already applied and well-chosen. Repo description is concise. "Suggested GitHub topics" section in the README is redundant. |

**Overall: 3.4/5.** The content is strong and honest, but presentation and structure
leave value on the table. The README reads like documentation rather than a landing
page, which is what a GitHub README functionally is.

## 2. Priority improvements (ranked by impact)

### P1. Add a demo GIF or terminal screenshot near the top

**Why:** Every high-performing security tool README (Nuclei, httpx, Semgrep) has a
visual demo in the first scroll. A 15-20 second GIF of Claude calling tools in a
real session would be the single highest-impact change. It answers "what does this
actually look like?" before the reader has to imagine it.

**What to capture:** A terminal recording (asciinema or VHS) showing:
- Claude receiving the prompt "Pentest example.com"
- 3-4 tool calls executing with visible output
- A finding being reported
- The final report snippet

Place it immediately after the architecture diagram, before "Why this exists".

**Examples:**
- Nuclei uses YouTube thumbnail links
- httpx uses a feature banner image
- mcp-go uses a YouTube tutorial thumbnail

### P2. Move the session example above Install

**Why:** The "What a session looks like" section is the best piece of marketing copy
in the README, but it sits after two install sections and three MCP config blocks.
Most readers will not scroll that far without already being convinced. Move it to
right after "How it works" so the pitch flows: what it is, how it works, what it
looks like, then how to get it.

### P3. Consolidate install into a single section with tabs or clear ordering

**Why:** There are currently two quick-start sections with three MCP config examples
between them. A first-time reader has to absorb all three to pick the right one.

**Suggestion:** One "Install" section with four methods ordered by friction:
1. Docker Compose (recommended, one command)
2. Homebrew
3. Pre-built binaries
4. From source

Then one "Quick start" section with two paths clearly marked:
- **Docker** (copy-paste, works immediately)
- **Native** (requires PostgreSQL)

Each path: install command, init/start command, MCP config block. That is it.

### P4. Make the tool catalog collapsible

**Why:** The tool tables are ~100 lines of markdown. They are valuable reference
material but they push everything after them (architecture, comparison tables,
configuration) well below the fold. Nuclei wraps its flag reference in `<details>`;
mcp-go wraps its core concept examples the same way.

**How:** Wrap each tool category in a `<details>` block with the category name as the
`<summary>`. The first one (Assets and scope) can be open by default to show the
pattern.

### P5. Add more badges

**Why:** Badges signal project health at a glance. The current set (CI, Go version,
License) is a solid start, but comparable projects include: GitHub stars, latest
release, Docker pulls, and community links.

**Add:**
- `![GitHub stars](https://img.shields.io/github/stars/SREsAreHumanToo/cc-artex?style=flat&logo=github)`
- `![Release](https://img.shields.io/github/v/release/SREsAreHumanToo/cc-artex?logo=github)`
- `![Docker Pulls](https://img.shields.io/docker/pulls/SREsAreHumanToo/cc-artex?logo=docker)` (if published)

### P6. Add a community / get help section

**Why:** This is the weakest area versus every comparable project. Even Metasploit's
minimal README has explicit support channels (Slack, Discussions, Twitter). A "Get
help" section reduces issue noise by routing questions to the right place, and it
signals an active community.

**Suggestion (even before channels exist):**
```markdown
## Get help

- [GitHub Issues](link) for bugs and feature requests
- [GitHub Discussions](link) for questions and ideas
- [CONTRIBUTING.md](CONTRIBUTING.md) for development setup
```

### P7. Remove the "Suggested GitHub topics" section

**Why:** The topics are already applied to the repo. This section in the README has no
function for the reader and reads like a TODO that was never cleaned up.

## 3. Visual improvements

### Logo
The existing SVG is clean, monochrome-compatible, and dark-mode aware. No changes
needed. Consider adding width="200" to the README img tag (already present).

### Demo GIF
**What to record** (using `vhs` or `asciinema` + `agg`):
- 800x600 terminal, dark theme
- Show `claude` starting, a pentest prompt, 5-6 tool calls with output
- Crop to under 20 seconds
- Save as `docs/demo.gif` and optionally upload to a release for CDN speed

### Architecture diagram
The current SVG is good. One improvement: add a "Target" box on the right side of
the Recording Proxy, with an arrow, so the data flow is complete: Claude -> cc-artex
-> PostgreSQL/Proxy -> Target.

### Badge additions
See P5 above. Keep them on a single centered line.

## 4. Content gaps

### Missing sections
- **Get help / Community** (see P6)
- **Supported platforms** (macOS, Linux; Windows status unclear)
- **Security policy** (SECURITY.md link, responsible disclosure for a security tool)
- **Acknowledgments** (ARTEX upstream credit is mentioned in context but not formally)

### Unclear explanations
- The relationship between `.cc-artex.json`, the data directory, and the engagement
  is spread across multiple sections. A "How state is managed" paragraph would help.
- The CA certificate trust step in Quick start says "prints trust instructions for
  your OS" but doesn't show what those look like. A sample output or a note about
  `mkcert` compatibility would reduce anxiety.

### Marketing copy improvements
- The "Why this exists" section is honest but could be tighter. Lead with the user
  benefit ("Run full pentests from your terminal with just a Claude subscription"),
  not the technical comparison with ARTEX's agent loop.
- Consider a one-line "elevator pitch" after the logo that works without any context:
  "Turn Claude Code into a penetration testing platform. 40+ MCP tools for
  reconnaissance, exploitation, traffic capture, and reporting."

## 5. Recommended README structure

```
1.  Logo (centered, 200px)
2.  Project name + one-line elevator pitch
3.  Badge row (CI, Release, Go, License, Stars)
4.  Authorized-use callout (keep it, but as a blockquote, not a section)
5.  How it works (2-3 sentences + architecture SVG)
6.  What a session looks like (the transcript example, MOVED UP)
7.  Demo GIF (when available)
8.  Why cc-artex (the three bullet points, tightened)
9.  Install (single section: Docker / Homebrew / Binary / Source)
10. Quick start
    - Docker path (recommended, 3 steps)
    - Native path (requires PostgreSQL, 4 steps)
11. Tool catalog (collapsible by category)
12. Architecture (directory tree, brief explanation)
13. Comparison tables (vs ARTEX, vs manual)
14. Configuration reference
15. Examples (link to walkthrough)
16. Get help (Issues, Discussions, Contributing)
17. License
```

This reorders for the reader's journey: hook them, show them, convince them, let
them install, give them reference material.
