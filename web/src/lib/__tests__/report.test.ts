import { describe, it, expect } from 'vitest'
import { generateFullReport, generateReport } from '../report'
import type { Activity, EngagementData, Finding } from '../types'

describe('generateFullReport', () => {
  it('handles activity entries the server sent without a summary', () => {
    const engagement: EngagementData = {
      task_id: 1,
      exploration_id: 1,
      description: '',
      goal: 'test',
      goals: [],
      open_intents: [],
      running_intents: [],
      recent_done_intents: [],
      done_intents_total: 0,
      frontier_open: 0,
      findings_total: 0,
      findings_dismissed: 0,
      findings_merged: 0,
      facts: 0,
      finding_list: [],
      recent_facts: [],
      coverage: {},
      task_credentials: [],
      hints: [],
    }
    const activity: Activity[] = [
      { id: 1, exploration_id: 1, kind: 'scan', created_at: '2026-01-01T00:00:00Z' },
      { id: 2, exploration_id: 1, kind: 'scan', summary: 'nmap | top ports', created_at: '2026-01-01T00:00:00Z' },
    ]
    const md = generateFullReport(engagement, [], [], activity)
    expect(md).toContain('nmap \\| top ports')
  })
})

describe('generateReport', () => {
  it('keeps a superseded finding and marks it', () => {
    const findings: Finding[] = [
      { id: 1, node_id: 11, vulnclass: 'xss', severity: 'high', summary: 'Reflected XSS', state: 'confirmed', status: 'confirmed', merged_into: 0, dismissed: false, dismiss_reason: '', superseded_by_reopen: 7 },
      { id: 2, node_id: 12, vulnclass: 'sqli', severity: 'low', summary: 'Blind SQLi', state: 'confirmed', status: 'confirmed', merged_into: 0, dismissed: false, dismiss_reason: '' },
    ]
    const md = generateReport(findings)
    expect(md).toContain('Reflected XSS')
    expect(md.match(/\*\*Superseded:\*\* intent #7 was reopened for a redo/g)).toHaveLength(1)
  })
})
