import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { ShieldAlert } from 'lucide-react'
import type { GraphNode } from '../../lib/types'
import { truncate } from '../../lib/utils'
import { NodeTooltip } from './NodeTooltip'

const severityColors: Record<string, string> = {
  critical: 'bg-rose-500 text-white',
  high: 'bg-orange-500 text-white',
  medium: 'bg-amber-500 text-white',
  low: 'bg-blue-500 text-white',
  info: 'bg-slate-400 text-white',
}

export const FindingNode = memo(function FindingNode({ data }: NodeProps & { data: GraphNode }) {
  const name = (data.payload.name ?? data.payload.title ?? 'Finding') as string
  const summary = (data.payload.label ?? data.payload.summary ?? data.payload.text ?? '') as string
  const severity = ((data.payload.severity ?? 'medium') as string).toLowerCase()
  const colorClass = severityColors[severity] ?? severityColors.medium
  return (
    <div className="group relative w-[180px] bg-card text-card-foreground rounded-lg shadow-md border-l-[3px] border-rose-500 overflow-hidden">
      <NodeTooltip lines={[`Finding #${data.id}: ${truncate(name, 60)}`, truncate(summary, 80), `Severity: ${severity}`]} />
      <Handle type="target" position={Position.Left} className="!bg-rose-500 !w-2 !h-2" />
      <div className="p-2.5">
        <div className="flex items-center gap-1.5 mb-1">
          <ShieldAlert size={14} className="text-rose-500 shrink-0" />
          <span className="text-[10px] text-muted-foreground">#{data.id}</span>
          <span className={`ml-auto px-1.5 py-0.5 text-[10px] font-medium rounded ${colorClass}`}>
            {severity}
          </span>
        </div>
        <p className="text-xs font-medium text-foreground truncate mb-0.5">{name}</p>
        <p className="text-[11px] text-muted-foreground leading-snug line-clamp-2">{summary}</p>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-rose-500 !w-2 !h-2" />
    </div>
  )
})
