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
import { useSearchParams } from 'react-router-dom'
import { Search, X, Filter } from 'lucide-react'
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
  const [searchParams, setSearchParams] = useSearchParams()
  const focusParam = searchParams.get('focus')
  const [focusedNodeId, setFocusedNodeId] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [kindFilter, setKindFilter] = useState<string>('all')

  const matchingNodeIds = useMemo(() => {
    const query = searchQuery.trim().toLowerCase()
    if (!query && kindFilter === 'all') return null
    return new Set(
      data.nodes
        .filter((gn) => {
          if (kindFilter !== 'all' && gn.kind !== kindFilter) return false
          if (!query) return true
          const p = gn.payload
          const haystack = [
            String(p.text ?? ''),
            String(p.summary ?? ''),
            String(p.vulnclass ?? ''),
            String(p.name ?? ''),
            gn.kind,
            gn.state,
          ].join(' ').toLowerCase()
          return haystack.includes(query)
        })
        .map((gn) => String(gn.id)),
    )
  }, [data.nodes, searchQuery, kindFilter])

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

  useEffect(() => {
    setNodes((nds) =>
      nds.map((n) => {
        const isFocused = focusedNodeId === n.id
        return {
          ...n,
          style: {
            ...n.style,
            opacity: matchingNodeIds === null || matchingNodeIds.has(n.id) ? 1 : 0.2,
            transition: 'opacity 0.2s ease, filter 0.3s ease',
            filter: isFocused ? 'drop-shadow(0 0 8px rgba(59, 130, 246, 0.7)) drop-shadow(0 0 16px rgba(59, 130, 246, 0.4))' : undefined,
          },
        }
      }),
    )
  }, [matchingNodeIds, focusedNodeId, setNodes])

  useEffect(() => {
    if (!rfInstance || !matchingNodeIds || matchingNodeIds.size !== 1) return
    const nodeId = [...matchingNodeIds][0]
    rfInstance.fitView({ nodes: [{ id: nodeId }], padding: 0.5, duration: 500 })
  }, [matchingNodeIds, rfInstance])

  useEffect(() => {
    if (!focusParam || data.nodes.length === 0 || !rfInstance) return
    const target = data.nodes.find((n) => String(n.id) === focusParam)
    if (!target) return

    setSelectedNode(target)
    const nodeId = String(target.id)
    setFocusedNodeId(nodeId)
    rfInstance.fitView({ nodes: [{ id: nodeId }], padding: 0.5, duration: 500 })

    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      next.delete('focus')
      return next
    }, { replace: true })

    const timer = setTimeout(() => setFocusedNodeId(null), 2000)
    return () => clearTimeout(timer)
  }, [focusParam, data.nodes, rfInstance, setSearchParams])

  const focusMatches = useCallback(() => {
    if (!rfInstance || !matchingNodeIds || matchingNodeIds.size === 0) return
    rfInstance.fitView({
      nodes: [...matchingNodeIds].map((id) => ({ id })),
      padding: 0.3,
      duration: 500,
    })
  }, [rfInstance, matchingNodeIds])

  const clearSearch = useCallback(() => {
    setSearchQuery('')
    setKindFilter('all')
  }, [])

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
      <div className="absolute top-3 right-3 z-10 flex items-center gap-2">
        <div className="flex items-center gap-1.5 bg-white/90 dark:bg-slate-800/90 backdrop-blur-sm rounded-lg border border-slate-200 dark:border-slate-700 shadow-sm px-2.5 py-1.5">
          <Search className="w-3.5 h-3.5 text-slate-400 shrink-0" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search nodes..."
            className="bg-transparent text-sm text-slate-700 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-500 outline-none w-40"
          />
          {(searchQuery || kindFilter !== 'all') && (
            <button onClick={clearSearch} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
              <X className="w-3.5 h-3.5" />
            </button>
          )}
          <div className="w-px h-4 bg-slate-200 dark:bg-slate-600 mx-0.5" />
          <Filter className="w-3.5 h-3.5 text-slate-400 shrink-0" />
          <select
            value={kindFilter}
            onChange={(e) => setKindFilter(e.target.value)}
            className="bg-transparent text-xs text-slate-600 dark:text-slate-300 outline-none cursor-pointer"
          >
            <option value="all">All</option>
            <option value="goal">Goals</option>
            <option value="intent">Intents</option>
            <option value="fact">Facts</option>
            <option value="finding">Findings</option>
            <option value="hint">Hints</option>
          </select>
          {matchingNodeIds !== null && (
            <>
              <span className="text-xs text-slate-400 dark:text-slate-500 ml-1 whitespace-nowrap">
                {matchingNodeIds.size} of {nodes.length}
              </span>
              {matchingNodeIds.size > 0 && (
                <button
                  onClick={focusMatches}
                  className="text-xs text-blue-500 hover:text-blue-600 dark:text-blue-400 dark:hover:text-blue-300 font-medium ml-1"
                >
                  Focus
                </button>
              )}
            </>
          )}
        </div>
      </div>
      <Legend />
      {selectedNode && (
        <DetailDrawer node={selectedNode} onClose={() => setSelectedNode(null)} />
      )}
    </div>
  )
}

export default GraphPage
