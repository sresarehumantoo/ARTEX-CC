import { X, ExternalLink, ArrowRight } from 'lucide-react'
import { Link } from 'react-router-dom'
import { StatusBadge } from '../StatusBadge'
import type { GraphNode, GraphEdge } from '../../lib/types'

const kindColors: Record<string, string> = {
  goal: 'bg-emerald-500',
  intent: 'bg-blue-500',
  fact: 'bg-amber-500',
  finding: 'bg-rose-500',
  hint: 'bg-violet-500',
}

interface Props {
  node: GraphNode
  edges: GraphEdge[]
  allNodes: GraphNode[]
  onClose: () => void
  onSelectNode: (nodeId: number) => void
}

function PayloadField({ label, value }: { label: string; value: unknown }) {
  if (value == null || value === '') return null
  const display = typeof value === 'object' ? JSON.stringify(value, null, 2) : String(value)
  return (
    <div className="mb-2">
      <dt className="text-[10px] uppercase tracking-wide text-muted-foreground mb-0.5">{label}</dt>
      <dd className="text-sm text-foreground whitespace-pre-wrap break-words">{display}</dd>
    </div>
  )
}

function nodeLabel(node: GraphNode): string {
  const p = node.payload
  const raw = p.text ?? p.summary ?? p.name ?? p.title
  if (raw != null && String(raw).length > 0) {
    const s = String(raw)
    return s.length > 60 ? s.slice(0, 57) + '...' : s
  }
  return `Node #${node.id}`
}

interface Neighbor {
  node: GraphNode
  rel: string
  direction: 'outgoing' | 'incoming'
}

export function DetailDrawer({ node, edges, allNodes, onClose, onSelectNode }: Props) {
  const payload = node.payload
  const priorityKeys = ['text', 'summary', 'name', 'title', 'severity', 'evidence', 'description']
  const shownKeys = new Set(priorityKeys)
  const otherKeys = Object.keys(payload).filter((k) => !shownKeys.has(k))

  const nodeMap = new Map(allNodes.map((n) => [n.id, n]))

  const neighbors: Neighbor[] = []
  for (const e of edges) {
    if (e.from === node.id) {
      const target = nodeMap.get(e.to)
      if (target) neighbors.push({ node: target, rel: e.rel, direction: 'outgoing' })
    } else if (e.to === node.id) {
      const source = nodeMap.get(e.from)
      if (source) neighbors.push({ node: source, rel: e.rel, direction: 'incoming' })
    }
  }

  const groupedByRel = new Map<string, Neighbor[]>()
  for (const n of neighbors) {
    const list = groupedByRel.get(n.rel) ?? []
    list.push(n)
    groupedByRel.set(n.rel, list)
  }

  const hasLinks =
    (node.kind === 'finding' && typeof payload.finding_id === 'number') ||
    (node.kind === 'fact' && typeof payload.asset_id === 'number')

  return (
    <div className="drawer-enter absolute top-0 right-0 z-20 h-full w-80 bg-card text-card-foreground shadow-2xl border-l border-border flex flex-col">
      <div className="flex items-center gap-2 p-3 border-b border-border">
        <span className={`w-2.5 h-2.5 rounded-full ${kindColors[node.kind] ?? 'bg-slate-400'}`} />
        <span className="text-sm font-medium text-foreground capitalize">{node.kind}</span>
        <span className="text-xs text-muted-foreground">#{node.id}</span>
        <StatusBadge
          domain={node.kind === 'intent' ? 'intent' : node.kind === 'goal' ? 'goal' : 'node'}
          value={node.state}
          className="ml-auto px-1.5 text-[10px]"
        />
        <button
          onClick={onClose}
          aria-label="Close detail panel"
          className="p-1 rounded hover:bg-accent text-muted-foreground"
        >
          <X size={16} />
        </button>
      </div>
      <div className="flex-1 overflow-y-auto p-3">
        <dl>
          {priorityKeys.map((key) => (
            <PayloadField key={key} label={key} value={payload[key]} />
          ))}
          {node.origin && <PayloadField label="origin" value={node.origin} />}
          {otherKeys.map((key) => (
            <PayloadField key={key} label={key} value={payload[key]} />
          ))}
        </dl>

        {hasLinks && (
          <div className="mt-3 pt-3 border-t border-border">
            <h4 className="text-[10px] uppercase tracking-wide text-muted-foreground mb-2">Links</h4>
            {node.kind === 'finding' && typeof payload.finding_id === 'number' && (
              <Link
                to={`/findings?id=${payload.finding_id}`}
                className="inline-flex items-center gap-1 px-2 py-1 text-xs rounded-full bg-muted text-slate-600 dark:text-slate-300 hover:text-emerald-600 dark:hover:text-emerald-400 hover:bg-emerald-50 dark:hover:bg-emerald-900/30 transition-colors"
              >
                <ExternalLink size={12} />
                Finding #{payload.finding_id}
              </Link>
            )}
            {node.kind === 'fact' && typeof payload.asset_id === 'number' && (
              <Link
                to={`/assets?id=${payload.asset_id}`}
                className="inline-flex items-center gap-1 px-2 py-1 text-xs rounded-full bg-muted text-slate-600 dark:text-slate-300 hover:text-emerald-600 dark:hover:text-emerald-400 hover:bg-emerald-50 dark:hover:bg-emerald-900/30 transition-colors"
              >
                <ExternalLink size={12} />
                Asset #{payload.asset_id}
              </Link>
            )}
          </div>
        )}

        {neighbors.length > 0 && (
          <div className="mt-3 pt-3 border-t border-border">
            <h4 className="text-[10px] uppercase tracking-wide text-muted-foreground mb-2">Connections</h4>
            {[...groupedByRel.entries()].map(([rel, items]) => (
              <div key={rel} className="mb-2">
                <span className="text-[10px] font-medium text-muted-foreground">{rel}</span>
                <div className="mt-1 space-y-0.5">
                  {items.map((neighbor) => (
                    <button
                      key={`${neighbor.node.id}-${neighbor.direction}`}
                      onClick={() => onSelectNode(neighbor.node.id)}
                      className="w-full flex items-center gap-1.5 px-2 py-1 rounded text-left hover:bg-accent transition-colors group"
                    >
                      <span className={`w-2 h-2 rounded-full shrink-0 ${kindColors[neighbor.node.kind] ?? 'bg-slate-400'}`} />
                      <span className="text-xs text-slate-600 dark:text-slate-300 truncate flex-1">
                        {nodeLabel(neighbor.node)}
                      </span>
                      <ArrowRight size={12} className="text-slate-300 dark:text-slate-600 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" />
                    </button>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
