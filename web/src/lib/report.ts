import type { EngagementData, Finding } from './types'

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

  const severityOrder = ['critical', 'high', 'medium', 'low', 'info']
  const sorted = [...findings].sort((a, b) =>
    severityOrder.indexOf(a.severity) - severityOrder.indexOf(b.severity),
  )

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
