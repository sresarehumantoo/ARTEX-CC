import { describe, it, expect } from 'vitest'
import { generateFullReport } from '../report'
import type { Activity, EngagementData } from '../types'

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
