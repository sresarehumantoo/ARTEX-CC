export type RuleScope = 'tool_name' | 'input'
export type RuleAction = 'allow' | 'ask' | 'deny'
export type ApprovalStatus = 'pending' | 'approved' | 'denied'

export interface InterceptRule {
  id: string
  pattern: string
  scope: RuleScope
  action: RuleAction
  description: string
  enabled: boolean
  created_at: string
}

export type RuleInput = Partial<Pick<InterceptRule, 'pattern' | 'scope' | 'action' | 'description' | 'enabled'>>

export interface Approval {
  id: string
  conversation_id: string
  tool_name: string
  tool_input: unknown
  rule_id: string
  status: ApprovalStatus
  decided_by: string
  decided_at: string
  created_at: string
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`
    try {
      const body = await res.json()
      if (body?.error) message = body.error
    } catch {
      // keep the status text
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

function jsonInit(method: string, body: unknown): RequestInit {
  return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
}

export async function getInterceptRules() {
  return (await request<InterceptRule[] | null>('/api/intercept/rules')) ?? []
}

export function createInterceptRule(input: RuleInput) {
  return request<InterceptRule>('/api/intercept/rules', jsonInit('POST', input))
}

export function updateInterceptRule(id: string, input: RuleInput) {
  return request<InterceptRule>(`/api/intercept/rules/${encodeURIComponent(id)}`, jsonInit('PATCH', input))
}

export function deleteInterceptRule(id: string) {
  return request<void>(`/api/intercept/rules/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export async function getApprovals(status?: ApprovalStatus) {
  const qs = status ? `?status=${status}` : ''
  return (await request<Approval[] | null>(`/api/approvals${qs}`)) ?? []
}

export function decideApproval(id: string, action: 'approve' | 'deny') {
  return request<Approval>(`/api/approvals/${encodeURIComponent(id)}/decide`, jsonInit('POST', { action }))
}
