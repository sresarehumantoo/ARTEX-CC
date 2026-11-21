import { useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { GraphData, GraphNode } from '../lib/types'
import { useSSE } from './useSSE'

const GRAPH_KEY = ['graph'] as const

async function fetchGraph(): Promise<GraphData> {
  const res = await fetch('/api/graph')
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json() as Promise<GraphData>
}

export function useGraphData() {
  const queryClient = useQueryClient()

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: GRAPH_KEY,
    queryFn: fetchGraph,
    staleTime: 30_000,
  })

  const handleSSE = useCallback(
    (event: { type: string; data: unknown }) => {
      if (event.type === 'node_added') {
        const node = event.data as GraphNode
        queryClient.setQueryData<GraphData>(GRAPH_KEY, (prev) =>
          prev
            ? { ...prev, nodes: [...prev.nodes.filter((n) => n.id !== node.id), node] }
            : prev,
        )
      } else if (event.type === 'node_updated') {
        const node = event.data as GraphNode
        queryClient.setQueryData<GraphData>(GRAPH_KEY, (prev) =>
          prev
            ? { ...prev, nodes: prev.nodes.map((n) => (n.id === node.id ? node : n)) }
            : prev,
        )
      } else if (event.type === 'edge_added') {
        const edge = event.data as GraphData['edges'][0]
        queryClient.setQueryData<GraphData>(GRAPH_KEY, (prev) =>
          prev ? { ...prev, edges: [...prev.edges, edge] } : prev,
        )
      } else if (event.type === 'full_refresh') {
        queryClient.invalidateQueries({ queryKey: GRAPH_KEY })
      }
    },
    [queryClient],
  )

  useSSE('/api/events', handleSSE)

  return {
    data: data ?? { nodes: [], edges: [] },
    loading: isLoading,
    error: error ? (error instanceof Error ? error.message : 'Failed to fetch graph') : null,
    fetchGraph: refetch,
  }
}
