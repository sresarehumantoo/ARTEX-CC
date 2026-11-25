import type { Activity, Asset, EngagementData, Finding } from './types'

const SEVERITY_ORDER = ['critical', 'high', 'medium', 'low', 'info'] as const

function sortBySeverity(findings: Finding[]): Finding[] {
  return [...findings].sort(
    (a, b) => SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity),
  )
}

export function generateReport(findings: Finding[], engagementData?: EngagementData): string {
  const now = new Date().toISOString().split('T')[0]
  const critical = findings.filter(f => f.severity === 'critical')
  const high = findings.filter(f => f.severity === 'high')
  const medium = findings.filter(f => f.severity === 'medium')
  const low = findings.filter(f => f.severity === 'low')
  const info = findings.filter(f => f.severity === 'info')

  let md = `# Penetration Test Findings Report\n\n`
  md += `**Generated:** ${now}\n\n`

  if (engagementData) {
    if (engagementData.goal) md += `**Goal:** ${engagementData.goal}\n\n`
  }

  md += `## Executive Summary\n\n`
  md += `| Severity | Count |\n|----------|-------|\n`
  md += `| Critical | ${critical.length} |\n`
  md += `| High | ${high.length} |\n`
  md += `| Medium | ${medium.length} |\n`
  md += `| Low | ${low.length} |\n`
  md += `| Info | ${info.length} |\n`
  md += `| **Total** | **${findings.length}** |\n\n`

  md += `## Findings\n\n`

  const sorted = sortBySeverity(findings)

  for (const f of sorted) {
    md += `### ${f.vulnclass} [${f.severity.toUpperCase()}]\n\n`
    md += `${f.summary}\n\n`
    md += `- **Status:** ${f.status}\n`
    if (f.node_id) md += `- **Node:** ${f.node_id}\n`
    if (f.dismissed) md += `- **Dismissed**\n`
    if (f.merged_into) md += `- **Merged into:** #${f.merged_into}\n`
    md += `\n---\n\n`
  }

  return md
}

export function generateFullReport(
  engagement: EngagementData,
  findings: Finding[],
  assets: Asset[],
  activity: Activity[],
): string {
  const now = new Date().toISOString().split('T')[0]
  const sorted = sortBySeverity(findings)
  const active = sorted.filter(f => !f.dismissed && !f.merged_into)
  const critical = active.filter(f => f.severity === 'critical')
  const high = active.filter(f => f.severity === 'high')
  const medium = active.filter(f => f.severity === 'medium')
  const low = active.filter(f => f.severity === 'low')
  const info = active.filter(f => f.severity === 'info')

  const goalsProven = engagement.goals.filter(g => g.state === 'proven').length
  const goalsTotal = engagement.goals.length
  const intentsComplete = engagement.done_intents_total
  const intentsOpen = engagement.open_intents.length
  const intentsRunning = engagement.running_intents.length
  const intentsTotal = intentsComplete + intentsOpen + intentsRunning

  let md = ''

  md += `# Engagement Report\n\n`
  if (engagement.goal) md += `**Goal:** ${engagement.goal}\n\n`
  if (engagement.description) md += `${engagement.description}\n\n`
  md += `**Generated:** ${now}\n\n`

  md += `## Table of Contents\n\n`
  md += `1. Executive Summary\n`
  md += `2. Findings\n`
  md += `3. Asset Inventory\n`
  md += `4. Coverage and Methodology\n`
  md += `5. Activity Summary\n\n`

  md += `---\n\n`

  // Executive Summary
  md += `## 1. Executive Summary\n\n`
  md += `### Findings by Severity\n\n`
  md += `| Severity | Count |\n|----------|-------|\n`
  md += `| Critical | ${critical.length} |\n`
  md += `| High | ${high.length} |\n`
  md += `| Medium | ${medium.length} |\n`
  md += `| Low | ${low.length} |\n`
  md += `| Info | ${info.length} |\n`
  md += `| **Total (active)** | **${active.length}** |\n\n`

  if (findings.length > active.length) {
    const dismissed = findings.filter(f => f.dismissed).length
    const merged = findings.filter(f => f.merged_into).length
    md += `*${dismissed} dismissed, ${merged} merged (${findings.length} total including inactive)*\n\n`
  }

  md += `### Key Statistics\n\n`
  md += `| Metric | Value |\n|--------|-------|\n`
  md += `| Assets discovered | ${assets.length} |\n`
  md += `| Goals proven / total | ${goalsProven} / ${goalsTotal} |\n`
  md += `| Intents completed / total | ${intentsComplete} / ${intentsTotal} |\n`
  md += `| Open intents | ${intentsOpen} |\n`
  md += `| Running intents | ${intentsRunning} |\n\n`

  md += `---\n\n`

  // Findings
  md += `## 2. Findings\n\n`

  if (active.length === 0) {
    md += `No active findings.\n\n`
  }

  md += `| # | Severity | Vuln Class | Summary | Status |\n`
  md += `|---|----------|------------|---------|--------|\n`
  for (let i = 0; i < active.length; i++) {
    const f = active[i]
    const summary = f.summary.replace(/\|/g, '\\|').replace(/\n/g, ' ')
    md += `| ${i + 1} | ${f.severity.toUpperCase()} | ${f.vulnclass} | ${summary} | ${f.status} |\n`
  }
  md += `\n`

  for (const f of active) {
    md += `### ${f.vulnclass} [${f.severity.toUpperCase()}]\n\n`
    md += `${f.summary}\n\n`
    md += `- **Status:** ${f.status}\n`
    if (f.node_id) md += `- **Node:** ${f.node_id}\n`
    md += `\n`
  }

  md += `---\n\n`

  // Asset Inventory
  md += `## 3. Asset Inventory\n\n`

  if (assets.length === 0) {
    md += `No assets discovered.\n\n`
  } else {
    const assetsByType = new Map<string, Asset[]>()
    for (const a of assets) {
      const list = assetsByType.get(a.type) ?? []
      list.push(a)
      assetsByType.set(a.type, list)
    }

    md += `| Type | Count |\n|------|-------|\n`
    for (const [type, list] of assetsByType) {
      md += `| ${type} | ${list.length} |\n`
    }
    md += `\n`

    for (const [type, list] of assetsByType) {
      md += `### ${type}\n\n`
      md += `| ID | Label |\n|----|-------|\n`
      for (const a of list) {
        md += `| ${a.id} | ${a.label.replace(/\|/g, '\\|')} |\n`
      }
      md += `\n`
    }
  }

  md += `---\n\n`

  // Coverage and Methodology
  md += `## 4. Coverage and Methodology\n\n`

  if (engagement.goals.length > 0) {
    md += `### Goals\n\n`
    md += `| Goal | Status |\n|------|--------|\n`
    for (const g of engagement.goals) {
      const label = (g.payload.label as string) ?? (g.payload.description as string) ?? `Goal #${g.id}`
      md += `| ${label.replace(/\|/g, '\\|')} | ${g.state} |\n`
    }
    md += `\n`
  }

  if (engagement.hints.length > 0) {
    md += `### Hints\n\n`
    for (const h of engagement.hints) {
      const text = (h.payload.text as string) ?? (h.payload.label as string) ?? `Hint #${h.id}`
      md += `- ${text}\n`
    }
    md += `\n`
  }

  const coverage = engagement.coverage
  if (coverage && Object.keys(coverage).length > 0) {
    md += `### Coverage Data\n\n`
    md += `| Area | Status |\n|------|--------|\n`
    for (const [area, status] of Object.entries(coverage)) {
      md += `| ${area} | ${String(status)} |\n`
    }
    md += `\n`
  }

  md += `---\n\n`

  // Activity Summary
  md += `## 5. Activity Summary\n\n`

  const significant = activity.filter(
    a => a.kind !== 'noop' && (a.summary ?? '').trim().length > 0,
  )

  if (significant.length === 0) {
    md += `No significant activity recorded.\n\n`
  } else {
    md += `| Time | Tool | Kind | Summary |\n`
    md += `|------|------|------|---------|\n`
    for (const a of significant.slice(0, 50)) {
      const ts = a.created_at ? a.created_at.split('T')[0] : ''
      const summary = (a.summary ?? '').replace(/\|/g, '\\|').replace(/\n/g, ' ')
      md += `| ${ts} | ${a.tool} | ${a.kind}${a.is_error ? ' (error)' : ''} | ${summary} |\n`
    }
    md += `\n`
  }

  return md
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function markdownTableToHtml(block: string): string {
  const lines = block.trim().split('\n')
  if (lines.length < 2) return `<p>${escapeHtml(block)}</p>`

  const parseRow = (line: string) =>
    line.split('|').slice(1, -1).map(cell => cell.trim())

  const headers = parseRow(lines[0])
  const rows = lines.slice(2).map(parseRow)

  let html = '<table><thead><tr>'
  for (const h of headers) {
    html += `<th>${inlineMarkdown(h)}</th>`
  }
  html += '</tr></thead><tbody>'
  for (const row of rows) {
    html += '<tr>'
    for (const cell of row) {
      html += `<td>${inlineMarkdown(cell)}</td>`
    }
    html += '</tr>'
  }
  html += '</tbody></table>'
  return html
}

function inlineMarkdown(text: string): string {
  let out = escapeHtml(text)
  out = out.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  out = out.replace(/\*(.+?)\*/g, '<em>$1</em>')
  out = out.replace(/`(.+?)`/g, '<code>$1</code>')
  return out
}

export function markdownToHtml(md: string): string {
  const lines = md.split('\n')
  const htmlParts: string[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]

    if (line.startsWith('### ')) {
      htmlParts.push(`<h3>${inlineMarkdown(line.slice(4))}</h3>`)
      i++
    } else if (line.startsWith('## ')) {
      htmlParts.push(`<h2>${inlineMarkdown(line.slice(3))}</h2>`)
      i++
    } else if (line.startsWith('# ')) {
      htmlParts.push(`<h1>${inlineMarkdown(line.slice(2))}</h1>`)
      i++
    } else if (line.startsWith('|') && lines[i + 1]?.match(/^\|[\s-|]+$/)) {
      let tableBlock = line
      let j = i + 1
      while (j < lines.length && lines[j].startsWith('|')) {
        tableBlock += '\n' + lines[j]
        j++
      }
      htmlParts.push(markdownTableToHtml(tableBlock))
      i = j
    } else if (line.startsWith('- ')) {
      const items: string[] = []
      let j = i
      while (j < lines.length && lines[j].startsWith('- ')) {
        items.push(inlineMarkdown(lines[j].slice(2)))
        j++
      }
      htmlParts.push('<ul>' + items.map(it => `<li>${it}</li>`).join('') + '</ul>')
      i = j
    } else if (line.match(/^\d+\. /)) {
      const items: string[] = []
      let j = i
      while (j < lines.length && lines[j].match(/^\d+\. /)) {
        items.push(inlineMarkdown(lines[j].replace(/^\d+\. /, '')))
        j++
      }
      htmlParts.push('<ol>' + items.map(it => `<li>${it}</li>`).join('') + '</ol>')
      i = j
    } else if (line === '---') {
      htmlParts.push('<hr>')
      i++
    } else if (line.trim() === '') {
      i++
    } else {
      htmlParts.push(`<p>${inlineMarkdown(line)}</p>`)
      i++
    }
  }

  return htmlParts.join('\n')
}

const HTML_STYLES = `
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  max-width: 900px;
  margin: 2rem auto;
  padding: 0 1.5rem;
  color: #1e293b;
  line-height: 1.6;
}
h1 { font-size: 1.75rem; margin: 1.5rem 0 0.75rem; }
h2 { font-size: 1.35rem; margin: 1.5rem 0 0.5rem; border-bottom: 1px solid #e2e8f0; padding-bottom: 0.25rem; }
h3 { font-size: 1.1rem; margin: 1rem 0 0.4rem; }
table { border-collapse: collapse; width: 100%; margin: 0.75rem 0; }
th, td { border: 1px solid #cbd5e1; padding: 0.4rem 0.75rem; text-align: left; font-size: 0.875rem; }
th { background: #f1f5f9; font-weight: 600; }
tr:nth-child(even) td { background: #f8fafc; }
ul, ol { margin: 0.5rem 0; padding-left: 1.5rem; }
li { margin: 0.2rem 0; }
hr { border: none; border-top: 1px solid #e2e8f0; margin: 1.5rem 0; }
p { margin: 0.5rem 0; }
code { background: #f1f5f9; padding: 0.1rem 0.3rem; border-radius: 3px; font-size: 0.85em; }
strong { font-weight: 600; }
@media print {
  body { margin: 0; padding: 0; max-width: none; }
  table { page-break-inside: auto; }
  tr { page-break-inside: avoid; }
  h2, h3 { page-break-after: avoid; }
}
`

export function wrapHtmlDocument(bodyHtml: string, title: string): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>${escapeHtml(title)}</title>
<style>${HTML_STYLES}</style>
</head>
<body>
${bodyHtml}
</body>
</html>`
}
