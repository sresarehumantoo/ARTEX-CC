import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { FileText } from 'lucide-react'
import type { GraphNode } from '../../lib/types'
import { truncate } from '../../lib/utils'
import { NodeTooltip } from './NodeTooltip'

export const FactNode = memo(function FactNode({ data }: NodeProps & { data: GraphNode }) {
  const text = (data.payload.label ?? data.payload.text ?? data.payload.summary ?? '') as string
  return (
    <div className="group relative w-[180px] bg-card text-card-foreground rounded-lg shadow-md border-l-[3px] border-amber-500 overflow-hidden">
      <NodeTooltip lines={[`Fact #${data.id}`, truncate(text, 80), `State: ${data.state}`]} />
      <Handle type="target" position={Position.Left} className="!bg-amber-500 !w-2 !h-2" />
      <div className="p-2.5">
        <div className="flex items-center gap-1.5 mb-1">
          <FileText size={14} className="text-amber-500 shrink-0" />
          <span className="text-[10px] text-muted-foreground">#{data.id}</span>
          {data.origin && (
            <span className="ml-auto px-1.5 py-0.5 text-[10px] font-medium rounded bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 truncate max-w-[80px]">
              {data.origin}
            </span>
          )}
        </div>
        <p className="text-xs text-foreground leading-snug line-clamp-3">{text}</p>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-amber-500 !w-2 !h-2" />
    </div>
  )
})
