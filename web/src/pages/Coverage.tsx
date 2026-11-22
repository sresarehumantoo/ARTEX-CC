import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Target, AlertTriangle, CheckCircle2, XCircle, HelpCircle, MinusCircle } from 'lucide-react'
import { getCoverage, getCoverageGaps } from '../lib/api'
import type { CoverageEntry, CoverageGap, CoverageGaps } from '../lib/types'
import { Badge } from '../components/ui/badge'
import { Skeleton } from '../components/ui/skeleton'
import { CoverageSkeleton } from '../components/skeletons/CoverageSkeleton'
import { cn } from '../lib/utils'

const OUTCOME_CONFIG: Record<string, { label: string; className: string; icon: typeof CheckCircle2 }> = {
  reported: {
    label: 'Reported',
    className: 'bg-rose-500/10 text-rose-500 border-rose-500/20',
    icon: AlertTriangle,
  },
  no_issue_found: {
    label: 'No Issue',
    className: 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20',
    icon: CheckCircle2,
  },
  ruled_out: {
    label: 'Ruled Out',
    className: 'bg-slate-500/10 text-slate-400 border-slate-500/20',
    icon: XCircle,
  },
  not_applicable: {
    label: 'N/A',
    className: 'bg-slate-500/10 text-slate-500 border-slate-500/20',
    icon: MinusCircle,
  },
  needs_follow_up: {
    label: 'Follow Up',
    className: 'bg-amber-500/10 text-amber-500 border-amber-500/20',
    icon: HelpCircle,
  },
}

const GAP_KIND_LABELS: Record<string, string> = {
  needs_follow_up: 'Needs Follow-Up',
  unrecorded_risk_class: 'Unrecorded Risk Class',
  missing_evidence: 'Missing Evidence',
  missing_reason: 'Missing Reason',
}

function OutcomeBadge({ outcome }: { outcome: string }) {
  const config = OUTCOME_CONFIG[outcome]
  if (!config) return <Badge variant="secondary">{outcome}</Badge>
  const Icon = config.icon
  return (
    <span className={cn('inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium', config.className)}>
      <Icon className="h-3 w-3" />
      {config.label}
    </span>
  )
}

function GapKindBadge({ kind }: { kind: string }) {
  const label = GAP_KIND_LABELS[kind] ?? kind
  const colorMap: Record<string, string> = {
    needs_follow_up: 'bg-amber-500/10 text-amber-500 border-amber-500/20',
    unrecorded_risk_class: 'bg-violet-500/10 text-violet-400 border-violet-500/20',
    missing_evidence: 'bg-sky-500/10 text-sky-400 border-sky-500/20',
    missing_reason: 'bg-orange-500/10 text-orange-400 border-orange-500/20',
  }
  return (
    <span className={cn('inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium', colorMap[kind] ?? 'bg-slate-500/10 text-slate-400 border-slate-500/20')}>
      {label}
    </span>
  )
}

function CoverageMatrix({ entries }: { entries: CoverageEntry[] }) {
  const surfaces = [...new Set(entries.map(e => e.surface))]
  const riskAreas = [...new Set(entries.map(e => e.risk_area))]

  const entryMap = new Map<string, CoverageEntry>()
  for (const e of entries) {
    entryMap.set(`${e.surface}|${e.risk_area}`, e)
  }

  if (surfaces.length > 20 || riskAreas.length > 20) {
    return <CoverageList entries={entries} />
  }

  if (surfaces.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-slate-500">
        <Target className="h-10 w-10 text-slate-400 dark:text-slate-600 mb-2" />
        <p className="text-sm">No coverage data recorded yet</p>
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-slate-200 dark:border-slate-800">
            <th className="px-3 py-2 text-left text-xs font-medium text-slate-500 uppercase tracking-wider sticky left-0 bg-white dark:bg-slate-900">
              Surface
            </th>
            {riskAreas.map(ra => (
              <th key={ra} className="px-3 py-2 text-left text-xs font-medium text-slate-500 uppercase tracking-wider whitespace-nowrap">
                {ra}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {surfaces.map(surface => (
            <tr key={surface} className="border-b border-slate-200/50 dark:border-slate-800/50">
              <td className="px-3 py-2 text-slate-800 dark:text-slate-200 font-medium text-sm whitespace-nowrap sticky left-0 bg-white dark:bg-slate-900">
                {surface}
              </td>
              {riskAreas.map(ra => {
                const entry = entryMap.get(`${surface}|${ra}`)
                return (
                  <td key={ra} className="px-3 py-2">
                    {entry ? <OutcomeBadge outcome={entry.outcome} /> : (
                      <span className="text-slate-300 dark:text-slate-700">-</span>
                    )}
                  </td>
                )
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function CoverageList({ entries }: { entries: CoverageEntry[] }) {
  const grouped = new Map<string, CoverageEntry[]>()
  for (const e of entries) {
    const list = grouped.get(e.surface) ?? []
    list.push(e)
    grouped.set(e.surface, list)
  }

  return (
    <div className="space-y-4">
      {[...grouped.entries()].map(([surface, items]) => (
        <div key={surface}>
          <h4 className="text-sm font-medium text-slate-800 dark:text-slate-200 mb-2">{surface}</h4>
          <div className="space-y-1">
            {items.map(e => (
              <div key={`${e.surface}-${e.risk_area}`} className="flex items-center gap-2 md:gap-3 px-2 md:px-3 py-1.5 rounded-md bg-slate-50 dark:bg-slate-800/50 flex-wrap min-w-0">
                <OutcomeBadge outcome={e.outcome} />
                <span className="text-sm text-slate-600 dark:text-slate-400 min-w-0 truncate">{e.risk_area}</span>
                {e.evidence && (
                  <span className="text-xs text-slate-500 truncate max-w-[200px] md:max-w-xs" title={e.evidence}>{e.evidence}</span>
                )}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

function GapsList({ gaps }: { gaps: CoverageGap[] }) {
  if (gaps.length === 0) {
    return <p className="text-sm text-slate-500 italic py-4">No coverage gaps detected</p>
  }

  return (
    <div className="space-y-1">
      {gaps.map((g, i) => (
        <div key={i} className="flex items-center gap-2 md:gap-3 px-2 md:px-3 py-2 rounded-md bg-slate-50 dark:bg-slate-800/50 flex-wrap min-w-0">
          <GapKindBadge kind={g.kind} />
          {g.surface && <span className="text-sm text-slate-700 dark:text-slate-300 min-w-0 truncate">{g.surface}</span>}
          <span className="text-sm text-slate-500 dark:text-slate-400 min-w-0 truncate">{g.risk_area}</span>
        </div>
      ))}
    </div>
  )
}

function UnexaminedList({ items }: { items: string[] }) {
  if (items.length === 0) {
    return <p className="text-sm text-slate-500 italic py-4">All risk classes have been examined</p>
  }

  return (
    <div className="flex flex-wrap gap-2">
      {items.map(item => (
        <span
          key={item}
          className="inline-flex items-center rounded-md border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 px-2.5 py-1 text-sm text-slate-700 dark:text-slate-300"
        >
          {item}
        </span>
      ))}
    </div>
  )
}

function CollapsibleSection({ title, count, defaultOpen, children }: {
  title: string
  count: number
  defaultOpen?: boolean
  children: React.ReactNode
}) {
  const [open, setOpen] = useState(defaultOpen ?? count > 0)

  return (
    <div className="border border-slate-200 dark:border-slate-700 rounded-lg">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex w-full items-center gap-2 px-3 md:px-4 py-3 text-left hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors rounded-lg"
      >
        {open ? <ChevronDown className="h-4 w-4 text-slate-400" /> : <ChevronRight className="h-4 w-4 text-slate-400" />}
        <span className="text-sm font-medium text-slate-800 dark:text-slate-200">{title}</span>
        <span className="ml-auto text-xs text-slate-500">{count}</span>
      </button>
      {open && <div className="px-3 md:px-4 pb-4">{children}</div>}
    </div>
  )
}

function StructuralGaps({ gaps }: { gaps: CoverageGaps }) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const sections: { title: string; items: any[]; render: (item: any) => string }[] = [
    { title: 'Untargeted Assets', items: gaps.untargeted_assets, render: (a: CoverageGaps['untargeted_assets'][0]) => `${a.type}: ${a.label}` },
    { title: 'Unresolved Intents', items: gaps.unresolved_intents, render: (n: CoverageGaps['unresolved_intents'][0]) => (n.payload.label as string) || `Intent #${n.id}` },
    { title: 'Unproven Goals', items: gaps.unproven_goals, render: (n: CoverageGaps['unproven_goals'][0]) => (n.payload.label as string) || `Goal #${n.id}` },
    { title: 'Unenriched Services', items: gaps.unenriched_services, render: (a: CoverageGaps['unenriched_services'][0]) => a.label },
  ]

  const totalGaps = sections.reduce((sum, s) => sum + s.items.length, 0)
  if (totalGaps === 0) {
    return <p className="text-sm text-slate-500 italic py-4">No structural gaps found</p>
  }

  return (
    <div className="space-y-3">
      {sections.map(({ title, items, render }) => (
        items.length > 0 && (
          <CollapsibleSection key={title} title={title} count={items.length}>
            <div className="space-y-1">
              {items.map((item, i) => (
                <div key={i} className="px-2 md:px-3 py-1.5 text-sm text-slate-700 dark:text-slate-300 rounded-md bg-slate-50 dark:bg-slate-800/50 break-words">
                  {render(item)}
                </div>
              ))}
            </div>
          </CollapsibleSection>
        )
      ))}
    </div>
  )
}

export default function Coverage() {
  const { data: stats, isLoading: statsLoading, error: statsError } = useQuery({
    queryKey: ['coverage'],
    queryFn: getCoverage,
  })

  const { data: gaps, isLoading: gapsLoading, error: gapsError } = useQuery({
    queryKey: ['coverage-gaps'],
    queryFn: getCoverageGaps,
  })

  const entryCount = stats?.entries.length ?? 0
  const gapCount = stats?.gaps.length ?? 0
  const unexaminedCount = stats?.unexamined.length ?? 0

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 md:px-4 py-3">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Coverage</h1>
          {entryCount > 0 && (
            <span className="inline-flex items-center rounded-full bg-slate-200 dark:bg-slate-700 px-2 py-0.5 text-xs font-medium text-slate-600 dark:text-slate-300">
              {entryCount}
            </span>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-3 md:p-6 space-y-4 md:space-y-6">
        {statsError ? (
          <div className="text-sm text-rose-400">Failed to load coverage: {(statsError as Error).message}</div>
        ) : statsLoading ? (
          <CoverageSkeleton />
        ) : (
          <>
            <section>
              <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3">
                Coverage Matrix
              </h2>
              <div className="border border-slate-200 dark:border-slate-700 rounded-lg overflow-hidden">
                <CoverageMatrix entries={stats?.entries ?? []} />
              </div>
            </section>

            <section>
              <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3">
                Gaps
                {gapCount > 0 && (
                  <span className="ml-2 inline-flex items-center rounded-full bg-amber-100 dark:bg-amber-900/30 px-2 py-0.5 text-xs font-medium text-amber-700 dark:text-amber-300">
                    {gapCount}
                  </span>
                )}
              </h2>
              <GapsList gaps={stats?.gaps ?? []} />
            </section>

            <section>
              <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3">
                Unexamined Risk Classes
                {unexaminedCount > 0 && (
                  <span className="ml-2 inline-flex items-center rounded-full bg-violet-100 dark:bg-violet-900/30 px-2 py-0.5 text-xs font-medium text-violet-700 dark:text-violet-300">
                    {unexaminedCount}
                  </span>
                )}
              </h2>
              <UnexaminedList items={stats?.unexamined ?? []} />
            </section>
          </>
        )}

        <section>
          <h2 className="text-sm font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3">
            Structural Gaps
          </h2>
          {gapsError ? (
            <div className="text-sm text-rose-400">Failed to load gaps: {(gapsError as Error).message}</div>
          ) : gapsLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 rounded-md" />
              <Skeleton className="h-8 rounded-md" />
              <Skeleton className="h-8 rounded-md" />
            </div>
          ) : (
            <StructuralGaps gaps={gaps!} />
          )}
        </section>
      </div>
    </div>
  )
}
