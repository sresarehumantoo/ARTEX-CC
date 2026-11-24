import { useQuery } from '@tanstack/react-query'
import type { GraphData } from '../lib/types'

const GRAPH_KEY = ['graph'] as const

async function fetchGraph(): Promise<GraphData> {
  const res = await fetch('/api/graph')
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json() as Promise<GraphData>
}

export function useGraphData() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: GRAPH_KEY,
    queryFn: fetchGraph,
    staleTime: 30_000,
  })

  return {
    data: data ?? { nodes: [], edges: [] },
    loading: isLoading,
    error: error ? (error instanceof Error ? error.message : 'Failed to fetch graph') : null,
    fetchGraph: refetch,
  }
}
