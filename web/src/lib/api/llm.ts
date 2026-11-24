export interface LLMSettings {
  backend: string
  default_model: string
  master_model: string
  subagent_model: string
  subagent_strategy: string
  max_parallel_subagents: number
  max_turns: number
  max_conversations: number
  thinking: string
  global_system_prompt: string
}

export interface BackendStatus {
  id: string
  name: string
  available: boolean
  path?: string
  version?: string
  error?: string
  enabled: boolean
}

export interface ModelOption {
  id: string
  label: string
  group: string
}

export interface LLMStatus {
  backends: BackendStatus[]
  models: ModelOption[]
}

export const FALLBACK_MODELS: ModelOption[] = [
  { id: 'opus', label: 'Opus', group: 'Aliases' },
  { id: 'sonnet', label: 'Sonnet', group: 'Aliases' },
  { id: 'haiku', label: 'Haiku', group: 'Aliases' },
  { id: 'fable', label: 'Fable', group: 'Aliases' },
]

export const llmSettingsKey = ['llm-settings'] as const
export const llmStatusKey = ['llm-status'] as const

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

export function groupModels(models: ModelOption[]) {
  return Object.entries(
    models.reduce<Record<string, ModelOption[]>>((acc, m) => {
      ;(acc[m.group] ??= []).push(m)
      return acc
    }, {}),
  )
}

export function modelLabel(id: string): string {
  return id.replace(/^claude-/, '')
}

export function getLLMSettings() {
  return request<LLMSettings>('/api/llm/settings')
}

export function updateLLMSettings(settings: Partial<LLMSettings>) {
  return request<LLMSettings>('/api/llm/settings', jsonInit('PUT', settings))
}

export function getLLMStatus() {
  return request<LLMStatus>('/api/llm/status')
}
