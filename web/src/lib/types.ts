export interface EngagementData {
  task_id: number
  exploration_id: number
  description: string
  goal: string
  goals: GraphNode[]
  open_intents: IntentSummary[]
  running_intents: IntentSummary[]
  recent_done_intents: GraphNode[]
  done_intents_total: number
  frontier_open: number
  findings_total: number
  findings_dismissed: number
  findings_merged: number
  facts: number
  finding_list: Record<string, unknown>[]
  recent_facts: GraphNode[]
  coverage: Record<string, unknown>
  task_credentials: Record<string, unknown>[]
  hints: GraphNode[]
  stats?: {
    nodes: number
    findings: number
    intents_running: number
    intents_done: number
    intents_total: number
  }
}

export interface IntentSummary {
  id: number
  state: string
  summary?: string
  label?: string
  text?: string
  claimed_by?: string
  claim_expires_at?: string
  claim_expired?: boolean
  previous_owner?: string
}

export interface GraphNode {
  id: number
  kind: 'goal' | 'hint' | 'intent' | 'fact' | 'finding'
  state: string
  payload: Record<string, unknown>
  origin?: string
}

export interface GraphEdge {
  from: number
  rel: 'spawns' | 'derived_from' | 'yields' | 'proves'
  to: number
}

export interface GraphData {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info'

export interface Finding {
  id: number
  node_id: number
  vulnclass: string
  severity: Severity
  summary: string
  state: string
  status: string
  merged_into: number
  dismissed: boolean
  dismiss_reason: string
}

export interface TrafficBinding {
  id: number
  finding_id: number
  snapshot_id: string
  role: string
  note: string
  position: number
}

export interface FindingDetail extends Finding {
  created_at?: string
  traffic_bindings: TrafficBinding[]
}

export interface Asset {
  id: number
  type: string
  label: string
  data: Record<string, unknown>
}

export interface TrafficExchange {
  id: string
  ts: number
  host: string
  method: string
  url_template: string
  url: string
  status: number
  content_type: string
  resp_len: number
  rewritten: boolean
}

export interface Activity {
  id: number
  exploration_id: number
  node_id?: number
  worker?: string
  kind: string
  tool?: string
  is_error?: boolean
  summary?: string
  detail?: string
  created_at?: string
}

export interface TrafficPage {
  rows: TrafficExchange[]
  total: number
}

export interface TrafficDetail {
  id: string
  request: string
  response: string
}

export interface CoverageEntry {
  surface: string
  risk_area: string
  outcome: string
  evidence: string
  reason: string
  worker?: string
  updated_at: string
}

export interface CoverageGap {
  kind: string
  surface?: string
  risk_area: string
}

export interface CoverageStats {
  entries: CoverageEntry[]
  gaps: CoverageGap[]
  unexamined: string[]
}

export interface CoverageGaps {
  untargeted_assets: Asset[]
  unresolved_intents: GraphNode[]
  unproven_goals: GraphNode[]
  unenriched_services: Asset[]
}

export interface Credential {
  id: number
  domain: string
  username: string
  password?: string
  source?: string
  note?: string
}

export interface Constraint {
  id: number
  kind: string
  text: string
}
