export interface Agent {
  id: string
  name: string
  description: string
  system_prompt: string
  model: string
  allowed_tools: string[]
  created_at: string
  updated_at: string
}

export type AgentInput = Partial<Pick<Agent, 'name' | 'description' | 'system_prompt' | 'model' | 'allowed_tools'>>

const BASE = '/api/agents'

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

export async function getAgents() {
  return (await request<Agent[] | null>(BASE)) ?? []
}

export function getAgent(id: string) {
  return request<Agent>(`${BASE}/${encodeURIComponent(id)}`)
}

export function createAgent(input: AgentInput) {
  return request<Agent>(BASE, jsonInit('POST', input))
}

export function updateAgent(id: string, input: AgentInput) {
  return request<Agent>(`${BASE}/${encodeURIComponent(id)}`, jsonInit('PATCH', input))
}

export function deleteAgent(id: string) {
  return request<void>(`${BASE}/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
