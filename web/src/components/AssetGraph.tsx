import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ReactFlow,
  Controls,
  Background,
  BackgroundVariant,
  Handle,
  Position,
  useNodesState,
  useEdgesState,
  BaseEdge,
  getSmoothStepPath,
  type Node,
  type Edge,
  type NodeProps,
  type EdgeProps,
  type ReactFlowInstance,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import dagre from 'dagre'
import type { Asset } from '../lib/types'

const NODE_WIDTH = 200
const NODE_HEIGHT = 52

const KIND_COLORS: Record<string, { border: string; bg: string; text: string; hex: string }> = {
  root_domain: { border: 'border-l-emerald-500', bg: 'bg-emerald-500/10', text: 'text-emerald-400', hex: '#10b981' },
  subdomain: { border: 'border-l-teal-500', bg: 'bg-teal-500/10', text: 'text-teal-400', hex: '#14b8a6' },
  ip: { border: 'border-l-blue-500', bg: 'bg-blue-500/10', text: 'text-blue-400', hex: '#3b82f6' },
  service: { border: 'border-l-violet-500', bg: 'bg-violet-500/10', text: 'text-violet-400', hex: '#8b5cf6' },
  endpoint: { border: 'border-l-amber-500', bg: 'bg-amber-500/10', text: 'text-amber-400', hex: '#f59e0b' },
  app: { border: 'border-l-rose-500', bg: 'bg-rose-500/10', text: 'text-rose-400', hex: '#f43f5e' },
}

const DEFAULT_COLOR = { border: 'border-l-slate-500', bg: 'bg-slate-500/10', text: 'text-muted-foreground', hex: '#64748b' }

function inferEdges(assets: Asset[]): Array<{ from: number; to: number }> {
  const edges: Array<{ from: number; to: number }> = []
  const byType = (t: string) => assets.filter((a) => a.type === t)

  const domains = byType('root_domain')
  const subdomains = byType('subdomain')
  const services = byType('service')
  const endpoints = byType('endpoint')

  for (const sub of subdomains) {
    const parent = domains.find((d) => sub.label.endsWith('.' + d.label))
    if (parent) edges.push({ from: parent.id, to: sub.id })
  }

  for (const svc of services) {
    const host = svc.label.split(':')[0]
    const parent = [...domains, ...subdomains].find((d) => d.label === host)
    if (parent) edges.push({ from: parent.id, to: svc.id })
  }

  for (const ep of endpoints) {
    const svc = services.find((s) => ep.label.startsWith(s.label))
    if (svc) edges.push({ from: svc.id, to: ep.id })
  }

  return edges
}

function layoutAssets(
  assets: Asset[],
  edges: Array<{ from: number; to: number }>,
): { nodes: Node[]; edges: Edge[] } {
  const g = new dagre.graphlib.Graph()
  g.setGraph({ rankdir: 'LR', nodesep: 40, ranksep: 100 })
  g.setDefaultEdgeLabel(() => ({}))

  for (const a of assets) {
    g.setNode(String(a.id), { width: NODE_WIDTH, height: NODE_HEIGHT })
  }
  for (const e of edges) {
    g.setEdge(String(e.from), String(e.to))
  }

  dagre.layout(g)

  return {
    nodes: assets.map((a) => {
      const pos = g.node(String(a.id))
      return {
        id: String(a.id),
        type: 'asset',
        position: { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 },
        data: { asset: a },
      }
    }),
    edges: edges.map((e, i) => ({
      id: `ae-${i}`,
      source: String(e.from),
      target: String(e.to),
      type: 'assetEdge',
    })),
  }
}

function AssetNode({ data }: NodeProps & { data: { asset: Asset } }) {
  const asset = data.asset
  const colors = KIND_COLORS[asset.type] ?? DEFAULT_COLOR
  return (
    <div
      className={`w-[180px] rounded-lg border border-l-[3px] px-3 py-2 text-xs shadow-sm bg-card text-card-foreground border-border ${colors.border}`}
    >
      <Handle type="target" position={Position.Left} className="!bg-slate-400 !w-1.5 !h-1.5" />
      <div className="font-mono truncate max-w-[160px] text-foreground">
        {asset.label}
      </div>
      <div className={`text-[10px] mt-0.5 ${colors.text}`}>{asset.type}</div>
      <Handle type="source" position={Position.Right} className="!bg-slate-400 !w-1.5 !h-1.5" />
    </div>
  )
}

function AssetEdge(props: EdgeProps) {
  const [edgePath] = getSmoothStepPath({
    sourceX: props.sourceX,
    sourceY: props.sourceY,
    targetX: props.targetX,
    targetY: props.targetY,
    sourcePosition: props.sourcePosition,
    targetPosition: props.targetPosition,
    borderRadius: 12,
  })

  return (
    <BaseEdge
      id={props.id}
      path={edgePath}
      style={{ stroke: '#475569', strokeWidth: 1.5 }}
    />
  )
}

const nodeTypes = { asset: AssetNode }
const edgeTypes = { assetEdge: AssetEdge }

export function AssetGraph({
  assets,
  onSelectAsset,
}: {
  assets: Asset[]
  onSelectAsset: (asset: Asset) => void
}) {
  const inferred = useMemo(() => inferEdges(assets), [assets])
  const layout = useMemo(() => layoutAssets(assets, inferred), [assets, inferred])

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([] as Node[])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([] as Edge[])
  const [rfInstance, setRfInstance] = useState<ReactFlowInstance | null>(null)

  useEffect(() => {
    setNodes(layout.nodes)
    setEdges(layout.edges)
  }, [layout, setNodes, setEdges])

  useEffect(() => {
    if (rfInstance && nodes.length > 0) {
      setTimeout(() => rfInstance.fitView({ padding: 0.2, duration: 300 }), 50)
    }
  }, [rfInstance, nodes.length])

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      const asset = (node.data as { asset: Asset }).asset
      onSelectAsset(asset)
    },
    [onSelectAsset],
  )

  const legend = useMemo(() => {
    const typesPresent = [...new Set(assets.map((a) => a.type))]
    return typesPresent.map((t) => {
      const c = KIND_COLORS[t] ?? DEFAULT_COLOR
      return { type: t, hex: c.hex }
    })
  }, [assets])

  if (assets.length === 0) return null

  return (
    <div className="h-full w-full relative">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        onInit={setRfInstance}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        minZoom={0.1}
        maxZoom={2}
        proOptions={{ hideAttribution: true }}
      >
        <Background
          variant={BackgroundVariant.Dots}
          gap={16}
          size={1}
          className="!bg-muted dark:!bg-slate-900"
        />
        <Controls className="!bg-white dark:!bg-slate-800 !border-border dark:!border-slate-700 [&>button]:!bg-white dark:[&>button]:!bg-slate-800 [&>button]:!border-border dark:[&>button]:!border-slate-700 [&>button]:!text-slate-600 dark:[&>button]:!text-slate-300" />
      </ReactFlow>
      {legend.length > 0 && (
        <div className="absolute bottom-3 left-3 flex gap-3 bg-white/90 dark:bg-slate-800/90 backdrop-blur-sm rounded-lg px-3 py-2 border border-border text-xs">
          {legend.map((l) => (
            <div key={l.type} className="flex items-center gap-1.5">
              <span
                className="w-2.5 h-2.5 rounded-sm"
                style={{ backgroundColor: l.hex }}
              />
              <span className="text-muted-foreground">{l.type}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
