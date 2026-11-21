import { BaseEdge, getSmoothStepPath, type EdgeProps } from '@xyflow/react'

const edgeStyles: Record<string, { stroke: string; animated: boolean }> = {
  spawns: { stroke: '#10b981', animated: false },
  derived_from: { stroke: '#6366f1', animated: false },
  yields: { stroke: '#f59e0b', animated: false },
  proves: { stroke: '#ef4444', animated: false },
}

export function CustomEdge(props: EdgeProps & { data?: { rel?: string; sourceRunning?: boolean } }) {
  const rel = props.data?.rel ?? 'derived_from'
  const sourceRunning = props.data?.sourceRunning ?? false
  const style = edgeStyles[rel] ?? edgeStyles.derived_from

  const [edgePath] = getSmoothStepPath({
    sourceX: props.sourceX,
    sourceY: props.sourceY,
    targetX: props.targetX,
    targetY: props.targetY,
    sourcePosition: props.sourcePosition,
    targetPosition: props.targetPosition,
    borderRadius: 12,
  })

  const isAnimated = style.animated || (rel === 'spawns' && sourceRunning)

  return (
    <g className={isAnimated ? 'edge-animated' : undefined}>
      <BaseEdge
        id={props.id}
        path={edgePath}
        style={{
          stroke: style.stroke,
          strokeWidth: 2,
          ...(rel === 'proves' ? { markerEnd: 'url(#proves-arrow)' } : {}),
        }}
      />
    </g>
  )
}
