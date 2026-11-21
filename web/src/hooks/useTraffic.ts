import { useQuery } from '@tanstack/react-query'
import type { TrafficDetail, TrafficPage } from '../lib/types'

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json() as Promise<T>
}

export interface TrafficFilters {
  host: string
  method: string
  status: string
  query: string
  page: number
  size: number
}

export function useTraffic(filters: TrafficFilters) {
  const params = new URLSearchParams()
  if (filters.host) params.set('host', filters.host)
  if (filters.method) params.set('method', filters.method)
  if (filters.status) params.set('status', filters.status)
  if (filters.query) params.set('query', filters.query)
  params.set('page', String(filters.page))
  params.set('size', String(filters.size))

  return useQuery({
    queryKey: ['traffic', filters],
    queryFn: () => fetchJSON<TrafficPage>(`/api/traffic?${params}`),
    staleTime: 10_000,
  })
}

export function useTrafficDetail(id: string | null) {
  return useQuery({
    queryKey: ['traffic', 'detail', id],
    queryFn: () => fetchJSON<TrafficDetail>(`/api/traffic/${id}`),
    enabled: id !== null,
    staleTime: 60_000,
  })
}
