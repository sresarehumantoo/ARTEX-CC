import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Zap } from 'lucide-react'
import type { GraphNode } from '../../lib/types'
import { truncate } from '../../lib/utils'
import { NodeTooltip } from './NodeTooltip'

function stateBadge(state: string) {
  switch (state) {
    case 'running':
      return <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-blue-500 text-white animate-pulse">running</span>
    case 'done':
      return <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-slate-400 text-white">done</span>
    default:
      return <span className="px-1.5 py-0.5 text-[10px] font-medium rounded border border-blue-400 text-blue-600 dark:text-blue-400">{state}</span>
  }
}

export function IntentNode({ data }: NodeProps & { data: GraphNode }) {
  const text = (data.payload.text ?? data.payload.summary ?? '') as string
  const isRunning = data.state === 'running'
  return (
    <div className={`group relative w-[180px] bg-white dark:bg-slate-800 rounded-lg shadow-md border-l-[3px] border-blue-500 overflow-hidden ${isRunning ? 'node-running' : ''}`}>
      <NodeTooltip lines={[`Intent #${data.id}`, truncate(text, 80), `State: ${data.state}`]} />
      <Handle type="target" position={Position.Left} className="!bg-blue-500 !w-2 !h-2" />
      <div className="p-2.5">
        <div className="flex items-center gap-1.5 mb-1">
          <Zap size={14} className="text-blue-500 shrink-0" />
          <span className="text-[10px] text-slate-400 dark:text-slate-500">#{data.id}</span>
          <div className="ml-auto">{stateBadge(data.state)}</div>
        </div>
        <p className="text-xs text-slate-700 dark:text-slate-200 leading-snug line-clamp-3">{text}</p>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-blue-500 !w-2 !h-2" />
    </div>
  )
}
