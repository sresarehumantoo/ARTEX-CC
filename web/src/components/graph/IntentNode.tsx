import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Zap } from 'lucide-react'
import type { GraphNode } from '../../lib/types'
import { truncate } from '../../lib/utils'
import { StatusBadge } from '../StatusBadge'
import { NodeTooltip } from './NodeTooltip'

export const IntentNode = memo(function IntentNode({ data }: NodeProps & { data: GraphNode }) {
  const text = (data.payload.label ?? data.payload.text ?? data.payload.summary ?? '') as string
  const isRunning = data.state === 'running'
  return (
    <div className={`group relative w-[180px] bg-card text-card-foreground rounded-lg shadow-md border-l-[3px] border-blue-500 overflow-hidden ${isRunning ? 'node-running' : ''}`}>
      <NodeTooltip lines={[`Intent #${data.id}`, truncate(text, 80), `State: ${data.state}`]} />
      <Handle type="target" position={Position.Left} className="!bg-blue-500 !w-2 !h-2" />
      <div className="p-2.5">
        <div className="flex items-center gap-1.5 mb-1">
          <Zap size={14} className="text-blue-500 shrink-0" />
          <span className="text-[10px] text-muted-foreground">#{data.id}</span>
          <div className="ml-auto"><StatusBadge domain="intent" value={data.state} className={`px-1.5 text-[10px] ${isRunning ? 'animate-pulse' : ''}`} /></div>
        </div>
        <p className="text-xs text-foreground leading-snug line-clamp-3">{text}</p>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-blue-500 !w-2 !h-2" />
    </div>
  )
})
