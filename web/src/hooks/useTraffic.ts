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
  body: string
  respMin: string
  respMax: string
  sort: string
  order: 'asc' | 'desc' | ''
  page: number
  size: number
}

export function useTraffic(filters: TrafficFilters) {
  const params = new URLSearchParams()
  if (filters.host) params.set('host', filters.host)
  if (filters.method) params.set('method', filters.method)
  if (filters.status) params.set('status', filters.status)
  if (filters.query) params.set('q', filters.query)
  if (filters.body) params.set('body', filters.body)
  if (filters.respMin) params.set('resp_min', filters.respMin)
  if (filters.respMax) params.set('resp_max', filters.respMax)
  if (filters.sort) params.set('sort', filters.sort)
  if (filters.order) params.set('order', filters.order)
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
