import dagre from 'dagre'
import type { GraphData, GraphNode, GraphEdge } from '../../lib/types'

const NODE_WIDTH = 200
const NODE_HEIGHT = 80

export interface LayoutResult {
  nodes: Array<{
    id: string
    type: string
    position: { x: number; y: number }
    data: GraphNode
  }>
  edges: Array<{
    id: string
    source: string
    target: string
    type: string
    data: { rel: string; sourceRunning: boolean }
  }>
}

export function layoutGraph(data: GraphData): LayoutResult {
  const g = new dagre.graphlib.Graph()
  g.setGraph({ rankdir: 'LR', nodesep: 60, ranksep: 120 })
  g.setDefaultEdgeLabel(() => ({}))

  data.nodes.forEach((n: GraphNode) => {
    g.setNode(String(n.id), { width: NODE_WIDTH, height: NODE_HEIGHT })
  })

  data.edges.forEach((e: GraphEdge) => {
    g.setEdge(String(e.from), String(e.to))
  })

  dagre.layout(g)

  const nodeMap = new Map(data.nodes.map((n: GraphNode) => [n.id, n]))

  return {
    nodes: data.nodes.map((n: GraphNode) => {
      const pos = g.node(String(n.id))
      return {
        id: String(n.id),
        type: n.kind,
        position: { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 },
        data: n,
      }
    }),
    edges: data.edges.map((e: GraphEdge, i: number) => {
      const sourceNode = nodeMap.get(e.from)
      return {
        id: `e-${i}`,
        source: String(e.from),
        target: String(e.to),
        type: 'custom',
        data: { rel: e.rel, sourceRunning: sourceNode?.state === 'running' },
      }
    }),
  }
}
