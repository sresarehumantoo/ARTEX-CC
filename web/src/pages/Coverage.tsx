import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { ArrowRight, ChevronDown, ChevronRight, Download, Target, AlertTriangle, CheckCircle2, XCircle, HelpCircle, MinusCircle } from 'lucide-react'
import { getCoverage, getCoverageGaps } from '../lib/api'
import type { CoverageEntry, CoverageGap, CoverageGaps } from '../lib/types'
import { StatusBadge } from '../components/StatusBadge'
import { Button } from '../components/ui/button'
import { Skeleton } from '../components/ui/skeleton'
import { CoverageSkeleton } from '../components/skeletons/CoverageSkeleton'
import { toast } from 'sonner'
import { downloadFile } from '../lib/export'
import { cn } from '../lib/utils'

const OUTCOME_ICONS: Record<string, typeof CheckCircle2> = {
  reported: AlertTriangle,
  no_issue_found: CheckCircle2,
  ruled_out: XCircle,
  not_applicable: MinusCircle,
  needs_follow_up: HelpCircle,
}

const OUTCOME_CONFIG: Record<string, { icon: typeof CheckCircle2; className: string; label: string }> = {
  reported: { icon: AlertTriangle, className: 'text-yellow-600 dark:text-yellow-400', label: 'Reported' },
  no_issue_found: { icon: CheckCircle2, className: 'text-green-600 dark:text-green-400', label: 'No Issue' },
  ruled_out: { icon: XCircle, className: 'text-red-600 dark:text-red-400', label: 'Ruled Out' },
  not_applicable: { icon: MinusCircle, className: 'text-slate-500', label: 'N/A' },
  needs_follow_up: { icon: HelpCircle, className: 'text-blue-600 dark:text-blue-400', label: 'Follow Up' },
}

const GAP_KIND_LABELS: Record<string, string> = {
  needs_follow_up: 'Needs Follow-Up',
  unrecorded_risk_class: 'Unrecorded Risk Class',
  missing_evidence: 'Missing Evidence',
  missing_reason: 'Missing Reason',
}

function OutcomeBadge({ outcome }: { outcome: string }) {
  const Icon = OUTCOME_ICONS[outcome]
  return (
    <StatusBadge domain="outcome" value={outcome} className="gap-1 rounded-full">
      {Icon && <Icon className="h-3 w-3" />}
    </StatusBadge>
  )
}

function OutcomeFilter({ entries, active, onToggle }: {
  entries: CoverageEntry[]
  active: string | null
  onToggle: (outcome: string | null) => void
}) {
  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const e of entries) {
      c[e.outcome] = (c[e.outcome] ?? 0) + 1
    }
    return c
  }, [entries])

  const total = entries.length

  return (
    <div className="flex flex-wrap items-center gap-2 px-3 md:px-4 py-2 border-b border-border">
      <button
        type="button"
        onClick={() => onToggle(null)}
        className={cn(
          'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-colors',
          active === null
            ? 'border-slate-400 dark:border-slate-500 bg-muted text-foreground'
            : 'border-border text-muted-foreground hover:border-slate-300 dark:hover:border-slate-600',
        )}
      >
        All <span className="text-muted-foreground">{total}</span>
      </button>
      {Object.entries(OUTCOME_CONFIG).map(([key, config]) => {
        const count = counts[key] ?? 0
        if (count === 0) return null
        const Icon = config.icon
        return (
          <button
            key={key}
            type="button"
            onClick={() => onToggle(active === key ? null : key)}
            className={cn(
              'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-colors',
              active === key
                ? config.className + ' border-current'
                : 'border-border text-muted-foreground hover:border-slate-300 dark:hover:border-slate-600',
            )}
          >
            <Icon className="h-3 w-3" />
            {config.label} <span className="opacity-60">{count}</span>
          </button>
        )
      })}
    </div>
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
    <span className={cn('inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium', colorMap[kind] ?? 'bg-slate-500/10 text-muted-foreground border-slate-500/20')}>
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
      <div className="flex h-64 flex-col items-center justify-center gap-3 text-muted-foreground">
        <Target className="h-12 w-12 text-slate-300 dark:text-slate-700" />
        <div className="text-center">
          <p className="text-lg font-medium text-muted-foreground">No coverage data</p>
          <p className="mt-1 text-sm text-muted-foreground">Coverage tracking will show tested and untested attack surfaces</p>
        </div>
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border">
            <th className="px-3 py-2 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider sticky left-0 bg-background">
              Surface
            </th>
            {riskAreas.map(ra => (
              <th key={ra} className="px-3 py-2 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider whitespace-nowrap">
                {ra}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {surfaces.map(surface => (
            <tr key={surface} className="border-b border-slate-200/50 dark:border-slate-800/50">
              <td className="px-3 py-2 text-foreground font-medium text-sm whitespace-nowrap sticky left-0 bg-background">
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
          <h4 className="text-sm font-medium text-foreground mb-2">{surface}</h4>
          <div className="space-y-1">
            {items.map(e => (
              <div key={`${e.surface}-${e.risk_area}`} className="flex items-center gap-2 md:gap-3 px-2 md:px-3 py-1.5 rounded-md bg-muted flex-wrap min-w-0">
                <OutcomeBadge outcome={e.outcome} />
                <span className="text-sm text-muted-foreground min-w-0 truncate">{e.risk_area}</span>
                {e.evidence && (
                  <span className="text-xs text-muted-foreground truncate max-w-[200px] md:max-w-xs" title={e.evidence}>{e.evidence}</span>
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
    return <p className="text-sm text-muted-foreground italic py-4">No coverage gaps detected</p>
  }

  return (
    <div className="space-y-1">
      {gaps.map((g, i) => (
        <div key={i} className="flex items-center gap-2 md:gap-3 px-2 md:px-3 py-2 rounded-md bg-muted flex-wrap min-w-0">
          <GapKindBadge kind={g.kind} />
          {g.surface && <span className="text-sm text-foreground min-w-0 truncate">{g.surface}</span>}
          <span className="text-sm text-muted-foreground min-w-0 truncate">{g.risk_area}</span>
        </div>
      ))}
    </div>
  )
}

function UnexaminedList({ items }: { items: string[] }) {
  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground italic py-4">All risk classes have been examined</p>
  }

  return (
    <div className="flex flex-wrap gap-2">
      {items.map(item => (
        <span
          key={item}
          className="inline-flex items-center rounded-md border border-border bg-muted dark:bg-slate-800 px-2.5 py-1 text-sm text-foreground"
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
    <div className="border border-border rounded-lg">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex w-full items-center gap-2 px-3 md:px-4 py-3 text-left hover:bg-accent transition-colors rounded-lg"
      >
        {open ? <ChevronDown className="h-4 w-4 text-muted-foreground" /> : <ChevronRight className="h-4 w-4 text-muted-foreground" />}
        <span className="text-sm font-medium text-foreground">{title}</span>
        <span className="ml-auto text-xs text-muted-foreground">{count}</span>
      </button>
      {open && <div className="px-3 md:px-4 pb-4">{children}</div>}
    </div>
  )
}

function StructuralGaps({ gaps }: { gaps: CoverageGaps }) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const sections: { title: string; items: any[]; render: (item: any) => string; link: (item: any) => string }[] = [
    { title: 'Untargeted Assets', items: gaps.untargeted_assets, render: (a: CoverageGaps['untargeted_assets'][0]) => `${a.type}: ${a.label}`, link: (a) => `/assets?id=${a.id}` },
    { title: 'Unresolved Intents', items: gaps.unresolved_intents, render: (n: CoverageGaps['unresolved_intents'][0]) => (n.payload.label as string) || `Intent #${n.id}`, link: (n) => `/graph?focus=${n.id}` },
    { title: 'Unproven Goals', items: gaps.unproven_goals, render: (n: CoverageGaps['unproven_goals'][0]) => (n.payload.label as string) || `Goal #${n.id}`, link: (n) => `/graph?focus=${n.id}` },
    { title: 'Unenriched Services', items: gaps.unenriched_services, render: (a: CoverageGaps['unenriched_services'][0]) => a.label, link: (a) => `/assets?id=${a.id}` },
  ]

  const totalGaps = sections.reduce((sum, s) => sum + s.items.length, 0)
  if (totalGaps === 0) {
    return <p className="text-sm text-muted-foreground italic py-4">No structural gaps found</p>
  }

  return (
    <div className="space-y-3">
      {sections.map(({ title, items, render, link }) => (
        items.length > 0 && (
          <CollapsibleSection key={title} title={title} count={items.length}>
            <div className="space-y-1">
              {items.map((item, i) => (
                <Link
                  key={i}
                  to={link(item)}
                  className="flex items-center gap-2 px-2 md:px-3 py-1.5 text-sm text-foreground rounded-md bg-muted break-words hover:bg-accent transition-colors group"
                >
                  <span className="min-w-0 truncate">{render(item)}</span>
                  <ArrowRight className="h-3 w-3 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
                </Link>
              ))}
            </div>
          </CollapsibleSection>
        )
      ))}
    </div>
  )
}

function exportCoverageCSV(entries: CoverageEntry[]) {
  const headers = ['Surface', 'Risk Area', 'Outcome', 'Evidence', 'Reason', 'Worker', 'Updated']
  const rows = entries.map(e => [
    e.surface, e.risk_area, e.outcome, e.evidence, e.reason, e.worker ?? '', e.updated_at,
  ])
  const csv = [headers, ...rows].map(r => r.map(v => `"${(v ?? '').replace(/"/g, '""')}"`).join(',')).join('\n')
  downloadFile(csv, `coverage-${new Date().toISOString().slice(0, 10)}.csv`, 'text/csv')
}

export default function Coverage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [outcomeFilter, setOutcomeFilter] = useState<string | null>(() => searchParams.get('outcome'))

  useEffect(() => {
    const timer = setTimeout(() => {
      const next = new URLSearchParams()
      if (outcomeFilter) next.set('outcome', outcomeFilter)
      setSearchParams(next, { replace: true })
    }, 300)
    return () => clearTimeout(timer)
  }, [outcomeFilter, setSearchParams])

  const { data: stats, isLoading: statsLoading, error: statsError, refetch: refetchStats } = useQuery({
    queryKey: ['coverage'],
    queryFn: getCoverage,
  })

  const { data: gaps, isLoading: gapsLoading, error: gapsError, refetch: refetchGaps } = useQuery({
    queryKey: ['coverage-gaps'],
    queryFn: getCoverageGaps,
  })

  const entries = stats?.entries ?? []
  const filteredEntries = useMemo(() => {
    if (!outcomeFilter) return entries
    return entries.filter(e => e.outcome === outcomeFilter)
  }, [entries, outcomeFilter])

  const entryCount = stats?.entries.length ?? 0
  const gapCount = stats?.gaps.length ?? 0
  const unexaminedCount = stats?.unexamined.length ?? 0

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border bg-muted px-3 md:px-4 py-3">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-semibold text-foreground">Coverage</h1>
          {entryCount > 0 && (
            <span className="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-slate-600 dark:text-slate-300">
              {entryCount}
            </span>
          )}
          <div className="ml-auto">
            <Button
              variant="outline"
              size="sm"
              disabled={!entries.length}
              onClick={() => {
                exportCoverageCSV(filteredEntries)
                toast.success('Coverage exported')
              }}
            >
              <Download className="h-4 w-4 mr-1" />
              Download
            </Button>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-3 md:p-6 space-y-4 md:space-y-6">
        {statsError ? (
          <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
            <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load coverage: {(statsError as Error).message}</p>
            <button
              type="button"
              onClick={() => refetchStats()}
              className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
            >
              Try again
            </button>
          </div>
        ) : statsLoading ? (
          <CoverageSkeleton />
        ) : (
          <>
            <section>
              <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3">
                Coverage Matrix
              </h2>
              <div className="border border-border rounded-lg overflow-hidden">
                {entries.length > 0 && (
                  <OutcomeFilter entries={entries} active={outcomeFilter} onToggle={setOutcomeFilter} />
                )}
                <CoverageMatrix entries={filteredEntries} />
              </div>
            </section>

            <section>
              <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3">
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
              <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3">
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
          <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3">
            Structural Gaps
          </h2>
          {gapsError ? (
            <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
              <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load gaps: {(gapsError as Error).message}</p>
              <button
                type="button"
                onClick={() => refetchGaps()}
                className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
              >
                Try again
              </button>
            </div>
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
