import { useQuery } from '@tanstack/react-query'
import type { Finding, FindingDetail } from '../lib/types'

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json() as Promise<T>
}

export function useFindings() {
  return useQuery({
    queryKey: ['findings'],
    queryFn: () => fetchJSON<Finding[]>('/api/findings'),
    staleTime: 30_000,
  })
}

export function useFindingDetail(id: number | null) {
  return useQuery({
    queryKey: ['findings', id],
    queryFn: () => fetchJSON<FindingDetail>(`/api/findings/${id}`),
    enabled: id !== null,
    staleTime: 30_000,
  })
}
