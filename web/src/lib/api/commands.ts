export interface CommandEntry {
  seq: number
  conversation_id: string
  conversation_title: string
  tool_name: string
  input: Record<string, unknown>
  output_preview: string
  is_error: boolean
  time: string
}

export interface CommandStats {
  total: number
  by_tool: Record<string, number>
  by_conversation: Record<string, number>
  error_count: number
}

export interface CommandFilter {
  limit?: number
  offset?: number
  tool?: string
  conv?: string
  error?: boolean
}

async function request<T>(path: string, params?: Record<string, string>): Promise<T> {
  const url = new URL(path, window.location.origin)
  for (const [k, v] of Object.entries(params ?? {})) {
    if (v) url.searchParams.set(k, v)
  }
  const res = await fetch(url.toString())
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return (await res.json()) as T
}

export async function getCommands(filter: CommandFilter = {}) {
  const entries = await request<CommandEntry[] | null>('/api/commands', {
    limit: filter.limit ? String(filter.limit) : '',
    offset: filter.offset ? String(filter.offset) : '',
    tool: filter.tool ?? '',
    conv: filter.conv ?? '',
    error: filter.error ? 'true' : '',
  })
  return entries ?? []
}

export function getCommandStats() {
  return request<CommandStats>('/api/commands/stats')
}
