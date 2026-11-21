import { X } from 'lucide-react'
import type { GraphNode } from '../../lib/types'

const kindColors: Record<string, string> = {
  goal: 'bg-emerald-500',
  intent: 'bg-blue-500',
  fact: 'bg-amber-500',
  finding: 'bg-rose-500',
  hint: 'bg-violet-500',
}

interface Props {
  node: GraphNode
  onClose: () => void
}

function PayloadField({ label, value }: { label: string; value: unknown }) {
  if (value == null || value === '') return null
  const display = typeof value === 'object' ? JSON.stringify(value, null, 2) : String(value)
  return (
    <div className="mb-2">
      <dt className="text-[10px] uppercase tracking-wide text-slate-400 dark:text-slate-500 mb-0.5">{label}</dt>
      <dd className="text-sm text-slate-700 dark:text-slate-200 whitespace-pre-wrap break-words">{display}</dd>
    </div>
  )
}

export function DetailDrawer({ node, onClose }: Props) {
  const payload = node.payload
  const priorityKeys = ['text', 'summary', 'name', 'title', 'severity', 'evidence', 'description']
  const shownKeys = new Set(priorityKeys)
  const otherKeys = Object.keys(payload).filter((k) => !shownKeys.has(k))

  return (
    <div className="drawer-enter absolute top-0 right-0 z-20 h-full w-80 bg-white dark:bg-slate-800 shadow-2xl border-l border-slate-200 dark:border-slate-700 flex flex-col">
      <div className="flex items-center gap-2 p-3 border-b border-slate-200 dark:border-slate-700">
        <span className={`w-2.5 h-2.5 rounded-full ${kindColors[node.kind] ?? 'bg-slate-400'}`} />
        <span className="text-sm font-medium text-slate-800 dark:text-slate-100 capitalize">{node.kind}</span>
        <span className="text-xs text-slate-400 dark:text-slate-500">#{node.id}</span>
        <span className="ml-auto px-1.5 py-0.5 text-[10px] font-medium rounded bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300">
          {node.state}
        </span>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-400 dark:text-slate-500"
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
      </div>
    </div>
  )
}
