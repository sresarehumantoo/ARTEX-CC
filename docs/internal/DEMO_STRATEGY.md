# Demo GIF Strategy

## Target demo

A real ARTEX session against an HTB Easy box (Management), showing the full tool lifecycle from initialization through report generation. The goal is a concise terminal recording that demonstrates ARTEX's exploration graph keeping Claude oriented across the engagement.

## Recording tool

**VHS by Charm** (charmbracelet/vhs) for reproducible, scripted terminal recordings rendered to GIF and MP4. Advantages over asciinema: deterministic output, renders directly to GIF without a separate conversion step, and the .tape script lives in the repo for future re-recording.

Fallback: asciinema + agg for a lightweight cast-to-GIF pipeline if VHS font rendering causes issues.

## Script: command sequence

The recording walks through the core ARTEX workflow. Each step shows a tool call and its output.

1. **`artex init`** with `--goal "Pentest Management (10.129.x.x)"`. Shows the engagement being created and the MCP config printed.
2. **`htb-hosts add`** for the target IP, demonstrating the host setup step.
3. **Claude Code starting**, reading the engagement resource. Claude sees the goal and the empty graph.
4. **`graph_overview`** showing the goal, zero assets, zero findings, the blank slate.
5. **`enrich_domain` / `enrich_ip`** running recon. Output shows discovered services populating the asset graph.
6. **`insert_assets`** adding the discovered services (HTTP, SSH, etc.) to the graph.
7. **`add_intent`** planning attack paths ("test HTTP for default credentials", "enumerate web directories").
8. **`report_finding`** documenting a vulnerability with severity, evidence, and affected asset.
9. **`generate_report`** producing the final Markdown report with finding details and coverage summary.

## Timing and length

- **Target: 60-90 seconds.** Each step gets 3-8 seconds of visible output. Use VHS `Sleep` and `Type` speed settings to keep the pace brisk but readable.
- Trim or skip verbose output (e.g., full Nmap results). Focus on the tool call and the graph state changing.

## Output formats

- **GIF** for the README hero image (placed right after the badges, before "How it works").
- **MP4** for social media (Twitter/X, LinkedIn) and YouTube. Same recording, higher quality.

## Compliance

- No actual HTB flags or writeup-level spoilers shown. The demo shows tool interactions (asset insertion, intent planning, finding reporting), not exploit details.
- The target IP should be anonymized or use a retired/Easy box where solution details are already public.
- Findings shown should be generic enough to not constitute a walkthrough (e.g., "default credentials on management interface" rather than exact creds).

## Hosting

- GIF: committed to `docs/demo.gif` and referenced from README via relative path. If the GIF exceeds 10 MB, host on GitHub Releases and reference via the release asset URL.
- MP4: uploaded to GitHub Releases for linking from blog posts and social media.

## VHS tape file location

`docs/demo.tape` in the repo root. Include a comment header explaining how to re-record:

```
# Re-record: vhs docs/demo.tape
# Requires: vhs, ARTEX binary, running PostgreSQL, htb-hosts
```

## Production steps

1. Set up a clean ARTEX instance against a local PostgreSQL.
2. Write the .tape file with the scripted commands.
3. Test-render at 2x speed to check timing.
4. Final render at target speed, review for readability.
5. Optimize GIF size with gifsicle if needed.
6. Add to README and commit.
