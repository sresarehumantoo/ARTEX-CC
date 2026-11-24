import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Lightbulb } from 'lucide-react'
import type { GraphNode } from '../../lib/types'
import { truncate } from '../../lib/utils'
import { NodeTooltip } from './NodeTooltip'

export const HintNode = memo(function HintNode({ data }: NodeProps & { data: GraphNode }) {
  const text = (data.payload.label ?? data.payload.text ?? data.payload.summary ?? '') as string
  return (
    <div className="group relative w-[180px] bg-card text-card-foreground rounded-lg shadow-md border-l-[3px] border-violet-500 overflow-hidden">
      <NodeTooltip lines={[`Hint #${data.id}`, truncate(text, 80)]} />
      <Handle type="target" position={Position.Left} className="!bg-violet-500 !w-2 !h-2" />
      <div className="p-2.5">
        <div className="flex items-center gap-1.5 mb-1">
          <Lightbulb size={14} className="text-violet-500 shrink-0" />
          <span className="text-[10px] text-muted-foreground">#{data.id}</span>
        </div>
        <p className="text-xs text-foreground leading-snug line-clamp-3">{text}</p>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-violet-500 !w-2 !h-2" />
    </div>
  )
})
