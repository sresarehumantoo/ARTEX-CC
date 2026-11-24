import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Target } from 'lucide-react'
import type { GraphNode } from '../../lib/types'
import { truncate } from '../../lib/utils'
import { StatusBadge } from '../StatusBadge'
import { NodeTooltip } from './NodeTooltip'

export const GoalNode = memo(function GoalNode({ data }: NodeProps & { data: GraphNode }) {
  const text = (data.payload.label ?? data.payload.text ?? data.payload.summary ?? '') as string
  return (
    <div className="group relative w-[180px] bg-card text-card-foreground rounded-lg shadow-md border-l-[3px] border-emerald-500 overflow-hidden">
      <NodeTooltip lines={[`Goal: ${truncate(text, 80)}`, `State: ${data.state}`]} />
      <Handle type="target" position={Position.Left} className="!bg-emerald-500 !w-2 !h-2" />
      <div className="p-2.5">
        <div className="flex items-center gap-1.5 mb-1">
          <Target size={14} className="text-emerald-500 shrink-0" />
          <span className="text-[10px] text-muted-foreground">#{data.id}</span>
          <div className="ml-auto"><StatusBadge domain="goal" value={data.state} className="px-1.5 text-[10px]" /></div>
        </div>
        <p className="text-xs text-foreground leading-snug line-clamp-3">{text}</p>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-emerald-500 !w-2 !h-2" />
    </div>
  )
})
