import { Target, Zap, FileText, ShieldAlert, Lightbulb } from 'lucide-react'
import { cn } from '../../lib/utils'

const nodeKinds = [
  { kind: 'Goal', icon: Target, color: 'text-emerald-500' },
  { kind: 'Intent', icon: Zap, color: 'text-blue-500' },
  { kind: 'Fact', icon: FileText, color: 'text-amber-500' },
  { kind: 'Finding', icon: ShieldAlert, color: 'text-rose-500' },
  { kind: 'Hint', icon: Lightbulb, color: 'text-violet-500' },
]

const edgeTypes = [
  { rel: 'spawns', color: '#10b981', label: 'Spawns' },
  { rel: 'derived_from', color: '#6366f1', label: 'Derived from' },
  { rel: 'yields', color: '#f59e0b', label: 'Yields' },
  { rel: 'proves', color: '#ef4444', label: 'Proves' },
]

interface LegendProps {
  nodeCounts: Record<string, number>
  activeKind: string
  onKindClick: (kind: string) => void
}

export function Legend({ nodeCounts, activeKind, onKindClick }: LegendProps) {
  return (
    <div className="absolute bottom-3 left-3 z-10 bg-white/90 dark:bg-slate-800/90 backdrop-blur rounded-lg shadow-lg p-3 text-xs select-none">
      <div className="font-medium text-foreground mb-1.5">Nodes</div>
      <div className="space-y-0.5 mb-2.5">
        {nodeKinds.map(({ kind, icon: Icon, color }) => {
          const kindLower = kind.toLowerCase()
          const count = nodeCounts[kindLower] ?? 0
          const isActive = activeKind === 'all' || activeKind === kindLower
          return (
            <button
              key={kind}
              type="button"
              onClick={() => onKindClick(activeKind === kindLower ? 'all' : kindLower)}
              className={cn(
                'flex w-full items-center gap-1.5 rounded px-1.5 py-1 transition-colors hover:bg-accent',
                !isActive && 'opacity-40',
              )}
            >
              <Icon size={12} className={color} />
              <span className="text-slate-600 dark:text-slate-300 flex-1 text-left">{kind}</span>
              <span className="text-muted-foreground tabular-nums">{count}</span>
            </button>
          )
        })}
      </div>
      <div className="font-medium text-foreground mb-1.5">Edges</div>
      <div className="space-y-1">
        {edgeTypes.map(({ rel, color, label }) => (
          <div key={rel} className="flex items-center gap-1.5 px-1.5">
            <svg width="16" height="8" className="shrink-0">
              <line x1="0" y1="4" x2="16" y2="4" stroke={color} strokeWidth="2" />
            </svg>
            <span className="text-slate-600 dark:text-slate-300">{label}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
