import type { Finding } from './types'

function downloadFile(content: string, filename: string, mimeType: string) {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export function exportCSV(findings: Finding[], filename: string) {
  const headers = ['ID', 'Severity', 'Vuln Class', 'Summary', 'Status', 'Dismissed', 'Node ID', 'Merged Into']
  const rows = findings.map(f => [
    f.id,
    f.severity,
    f.vulnclass,
    `"${f.summary.replace(/"/g, '""')}"`,
    f.status,
    f.dismissed,
    f.node_id,
    f.merged_into,
  ])
  const csv = [headers.join(','), ...rows.map(r => r.join(','))].join('\n')
  downloadFile(csv, filename, 'text/csv')
}

export function exportJSON(findings: Finding[], filename: string) {
  const json = JSON.stringify(findings, null, 2)
  downloadFile(json, filename, 'application/json')
}

export function exportAssetCSV(assets: { id: number; type: string; label: string; data: Record<string, unknown> }[], filename: string) {
  const headers = ['ID', 'Type', 'Label']
  const rows = assets.map(a => [
    a.id,
    a.type,
    `"${a.label.replace(/"/g, '""')}"`,
  ])
  const csv = [headers.join(','), ...rows.map(r => r.join(','))].join('\n')
  downloadFile(csv, filename, 'text/csv')
}
