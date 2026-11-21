import { Handle, Position, type NodeProps } from '@xyflow/react'
import { FileText } from 'lucide-react'
import type { GraphNode } from '../../lib/types'

export function FactNode({ data }: NodeProps & { data: GraphNode }) {
  const text = (data.payload.text ?? data.payload.summary ?? '') as string
  return (
    <div className="node-enter w-[180px] bg-white dark:bg-slate-800 rounded-lg shadow-md border-l-[3px] border-amber-500 overflow-hidden">
      <Handle type="target" position={Position.Left} className="!bg-amber-500 !w-2 !h-2" />
      <div className="p-2.5">
        <div className="flex items-center gap-1.5 mb-1">
          <FileText size={14} className="text-amber-500 shrink-0" />
          <span className="text-[10px] text-slate-400 dark:text-slate-500">#{data.id}</span>
          {data.origin && (
            <span className="ml-auto px-1.5 py-0.5 text-[10px] font-medium rounded bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 truncate max-w-[80px]">
              {data.origin}
            </span>
          )}
        </div>
        <p className="text-xs text-slate-700 dark:text-slate-200 leading-snug line-clamp-3">{text}</p>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-amber-500 !w-2 !h-2" />
    </div>
  )
}
