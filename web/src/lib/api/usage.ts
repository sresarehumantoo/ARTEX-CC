export interface UsageSummary {
  conversation_id: string
  conversation_title: string
  model: string
  total_input_tokens: number
  total_output_tokens: number
  total_cache_read_tokens: number
  total_cache_creation_tokens: number
  total_cost_usd: number
  total_duration_ms: number
  total_turns: number
  result_count: number
  first_time: string
  last_time: string
}

export interface ModelUsage {
  input_tokens: number
  output_tokens: number
  cost_usd: number
}

export interface UsageTotals {
  total_input_tokens: number
  total_output_tokens: number
  total_cache_read_tokens: number
  total_cache_creation_tokens: number
  total_cost_usd: number
  total_duration_ms: number
  total_turns: number
  conversation_count: number
  by_model: Record<string, ModelUsage>
}

async function request<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return (await res.json()) as T
}

export async function getUsage(conv = '') {
  const q = conv ? `?conv=${encodeURIComponent(conv)}` : ''
  return (await request<UsageSummary[] | null>(`/api/usage${q}`)) ?? []
}

export function getUsageTotals() {
  return request<UsageTotals>('/api/usage/totals')
}
