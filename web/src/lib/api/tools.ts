export interface ToolInfo {
  name: string
  description: string
  read_only: boolean
  category: string
  schema?: Record<string, unknown>
}

export interface McpServer {
  name: string
  type: 'stdio' | 'http'
  command?: string
  args?: string[]
  url?: string
  env_keys: string[]
  built_in: boolean
}

export interface McpServerInput {
  name: string
  type: 'stdio' | 'http'
  command?: string
  args?: string[]
  url?: string
  env?: Record<string, string>
}

export interface Skill {
  name: string
  description: string
  content: string
  updated_at: string
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

export async function getTools() {
  return (await request<ToolInfo[] | null>('/api/tools')) ?? []
}

export async function getMcpServers() {
  return (await request<McpServer[] | null>('/api/mcp/servers')) ?? []
}

export function addMcpServer(input: McpServerInput) {
  return request<{ name: string }>('/api/mcp/servers', jsonInit('POST', input))
}

export function deleteMcpServer(name: string) {
  return request<void>(`/api/mcp/servers/${encodeURIComponent(name)}`, { method: 'DELETE' })
}

export async function getSkills() {
  return (await request<Skill[] | null>('/api/skills')) ?? []
}

export function saveSkill(name: string, input: { description: string; content: string }) {
  return request<Skill>(`/api/skills/${encodeURIComponent(name)}`, jsonInit('PUT', input))
}

export function deleteSkill(name: string) {
  return request<void>(`/api/skills/${encodeURIComponent(name)}`, { method: 'DELETE' })
}
