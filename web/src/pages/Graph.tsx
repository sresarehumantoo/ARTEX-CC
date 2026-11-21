import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  type ReactFlowInstance,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import '../graph.css'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { GraphSkeleton } from '../components/skeletons/GraphSkeleton'
import {
  GoalNode,
  IntentNode,
  FactNode,
  FindingNode,
  HintNode,
  CustomEdge,
  Legend,
  DetailDrawer,
  layoutGraph,
} from '../components/graph'
import { useGraphData } from '../hooks/useGraphData'
import type { GraphNode as GN } from '../lib/types'

const nodeTypes = {
  goal: GoalNode,
  intent: IntentNode,
  fact: FactNode,
  finding: FindingNode,
  hint: HintNode,
}

const edgeTypes = {
  custom: CustomEdge,
}

const miniMapColors: Record<string, string> = {
  goal: '#10b981',
  intent: '#3b82f6',
  fact: '#f59e0b',
  finding: '#ef4444',
  hint: '#8b5cf6',
}

function nodeColor(node: Node) {
  return miniMapColors[(node.data as unknown as GN)?.kind] ?? '#94a3b8'
}

export function GraphPage() {
  const { data, loading, error, fetchGraph } = useGraphData()
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([] as Node[])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([] as Edge[])
  const [selectedNode, setSelectedNode] = useState<GN | null>(null)
  const [rfInstance, setRfInstance] = useState<ReactFlowInstance | null>(null)
  const prevNodeIdsRef = useRef<Set<string>>(new Set())
  const prevCountRef = useRef(-1)

  useEffect(() => {
    if (data.nodes.length === 0) return
    const layout = layoutGraph(data)

    const prevIds = prevNodeIdsRef.current
    const enteringIds = new Set<string>()

    const layoutNodes = layout.nodes.map((n) => {
      const isNew = prevIds.size > 0 && !prevIds.has(n.id)
      if (isNew) enteringIds.add(n.id)
      return {
        ...n,
        className: isNew ? 'entering' : undefined,
      }
    })

    prevNodeIdsRef.current = new Set(layout.nodes.map((n) => n.id))
    setNodes(layoutNodes as unknown as Node[])
    setEdges(layout.edges as unknown as Edge[])

    if (enteringIds.size > 0) {
      setTimeout(() => {
        setNodes((nds) =>
          nds.map((n) => ({
            ...n,
            className: enteringIds.has(n.id) ? undefined : n.className,
          })),
        )
      }, 500)
    }
  }, [data, setNodes, setEdges])

  useEffect(() => {
    if (!rfInstance || nodes.length === 0) return
    if (prevCountRef.current === -1) {
      prevCountRef.current = nodes.length
      return
    }
    if (nodes.length !== prevCountRef.current) {
      prevCountRef.current = nodes.length
      setTimeout(() => rfInstance.fitView({ padding: 0.2, duration: 500 }), 50)
    }
  }, [nodes.length, rfInstance])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedNode(node.data as unknown as GN)
  }, [])

  const onPaneClick = useCallback(() => {
    setSelectedNode(null)
  }, [])

  const proofArrowMarker = useMemo(
    () => (
      <svg style={{ position: 'absolute', width: 0, height: 0 }}>
        <defs>
          <marker
            id="proves-arrow"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto-start-reverse"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#ef4444" />
          </marker>
        </defs>
      </svg>
    ),
    [],
  )

  if (loading) {
    return <GraphSkeleton />
  }

  if (error) {
    return (
      <div className="h-full flex items-center justify-center bg-slate-50 dark:bg-slate-900">
        <div className="text-center">
          <p className="text-rose-500 text-sm mb-2">Failed to load graph</p>
          <p className="text-slate-400 text-xs mb-3">{error}</p>
          <button
            onClick={() => fetchGraph()}
            className="px-3 py-1.5 text-xs rounded bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 hover:bg-slate-300 dark:hover:bg-slate-600"
          >
            Retry
          </button>
        </div>
      </div>
    )
  }

  if (data.nodes.length === 0) {
    return (
      <div className="h-full flex items-center justify-center bg-slate-50 dark:bg-slate-900">
        <div className="text-slate-400 dark:text-slate-500 text-sm">No exploration nodes yet.</div>
      </div>
    )
  }

  return (
    <div className="h-full w-full relative bg-slate-50 dark:bg-slate-900">
      {proofArrowMarker}
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        onInit={setRfInstance}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        minZoom={0.1}
        maxZoom={2}
        proOptions={{ hideAttribution: true }}
      >
        <Background variant={BackgroundVariant.Dots} gap={16} size={1} className="!bg-slate-50 dark:!bg-slate-900" />
        <Controls className="!bg-white dark:!bg-slate-800 !border-slate-200 dark:!border-slate-700 [&>button]:!bg-white dark:[&>button]:!bg-slate-800 [&>button]:!border-slate-200 dark:[&>button]:!border-slate-700 [&>button]:!text-slate-600 dark:[&>button]:!text-slate-300" />
        <MiniMap
          nodeColor={nodeColor}
          maskColor="rgba(0, 0, 0, 0.1)"
          className="!bg-white dark:!bg-slate-800 !border-slate-200 dark:!border-slate-700"
          pannable
          zoomable
        />
      </ReactFlow>
      <Legend />
      {selectedNode && (
        <DetailDrawer node={selectedNode} onClose={() => setSelectedNode(null)} />
      )}
    </div>
  )
}

export default GraphPage
