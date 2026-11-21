import { useCallback, useState } from 'react'
import type { GraphData, GraphNode } from '../lib/types'
import { useSSE } from './useSSE'

export function useGraphData() {
  const [data, setData] = useState<GraphData>({ nodes: [], edges: [] })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchGraph = useCallback(async () => {
    try {
      const res = await fetch('/api/graph')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const json = (await res.json()) as GraphData
      setData(json)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch graph')
    } finally {
      setLoading(false)
    }
  }, [])

  const handleSSE = useCallback((event: { type: string; data: unknown }) => {
    if (event.type === 'node_added') {
      const node = event.data as GraphNode
      setData((prev) => ({
        ...prev,
        nodes: [...prev.nodes.filter((n) => n.id !== node.id), node],
      }))
    } else if (event.type === 'node_updated') {
      const node = event.data as GraphNode
      setData((prev) => ({
        ...prev,
        nodes: prev.nodes.map((n) => (n.id === node.id ? node : n)),
      }))
    } else if (event.type === 'edge_added') {
      const edge = event.data as GraphData['edges'][0]
      setData((prev) => ({
        ...prev,
        edges: [...prev.edges, edge],
      }))
    } else if (event.type === 'full_refresh') {
      const graph = event.data as GraphData
      setData(graph)
    }
  }, [])

  useSSE('/api/events', handleSSE)

  return { data, loading, error, fetchGraph }
}
