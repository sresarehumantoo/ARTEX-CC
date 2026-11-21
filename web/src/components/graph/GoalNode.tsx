import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Target } from 'lucide-react'
import type { GraphNode } from '../../lib/types'

function stateBadge(state: string) {
  switch (state) {
    case 'proven':
      return <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-emerald-500 text-white">proven</span>
    case 'disproven':
      return <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-rose-500 text-white">disproven</span>
    default:
      return <span className="px-1.5 py-0.5 text-[10px] font-medium rounded border border-emerald-400 text-emerald-600 dark:text-emerald-400">{state}</span>
  }
}

export function GoalNode({ data }: NodeProps & { data: GraphNode }) {
  const text = (data.payload.text ?? data.payload.summary ?? '') as string
  return (
    <div className="node-enter w-[180px] bg-white dark:bg-slate-800 rounded-lg shadow-md border-l-[3px] border-emerald-500 overflow-hidden">
      <Handle type="target" position={Position.Left} className="!bg-emerald-500 !w-2 !h-2" />
      <div className="p-2.5">
        <div className="flex items-center gap-1.5 mb-1">
          <Target size={14} className="text-emerald-500 shrink-0" />
          <span className="text-[10px] text-slate-400 dark:text-slate-500">#{data.id}</span>
          <div className="ml-auto">{stateBadge(data.state)}</div>
        </div>
        <p className="text-xs text-slate-700 dark:text-slate-200 leading-snug line-clamp-3">{text}</p>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-emerald-500 !w-2 !h-2" />
    </div>
  )
}
