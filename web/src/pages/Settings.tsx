import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Settings as SettingsIcon, ShieldCheck, Lock, Info, Download } from 'lucide-react'
import { getEngagement, getConstraints } from '../lib/api'
import type { Constraint, EngagementData } from '../lib/types'
import { Skeleton } from '../components/ui/skeleton'
import { SettingsSkeleton } from '../components/skeletons/SettingsSkeleton'
import { Button } from '../components/ui/button'
import { downloadFile } from '../lib/export'
import { toast } from 'sonner'
import { cn } from '../lib/utils'

const KIND_COLORS: Record<string, string> = {
  time: 'bg-blue-500/10 text-blue-500 border-blue-500/20',
  scope: 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20',
  auth: 'bg-amber-500/10 text-amber-500 border-amber-500/20',
}

function KindBadge({ kind }: { kind: string }) {
  const color = KIND_COLORS[kind] ?? 'bg-slate-500/10 text-slate-400 border-slate-500/20'
  return (
    <span className={cn('inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium', color)}>
      {kind}
    </span>
  )
}

function CollapsibleSection({ title, icon: Icon, defaultOpen, children }: {
  title: string
  icon: typeof SettingsIcon
  defaultOpen?: boolean
  children: React.ReactNode
}) {
  const [open, setOpen] = useState(defaultOpen ?? true)

  return (
    <div className="border border-slate-200 dark:border-slate-700 rounded-lg">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex w-full items-center gap-2 px-3 md:px-4 py-3 text-left hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors rounded-lg"
      >
        {open ? <ChevronDown className="h-4 w-4 text-slate-400" /> : <ChevronRight className="h-4 w-4 text-slate-400" />}
        <Icon className="h-4 w-4 text-slate-500" />
        <span className="text-sm font-medium text-slate-800 dark:text-slate-200">{title}</span>
      </button>
      {open && <div className="px-3 md:px-4 pb-4">{children}</div>}
    </div>
  )
}

function ConstraintsList({ constraints }: { constraints: Constraint[] }) {
  if (constraints.length === 0) {
    return <p className="text-sm text-slate-500 italic py-2">No constraints set</p>
  }

  return (
    <div className="space-y-2">
      {constraints.map(c => (
        <div key={c.id} className="flex items-start gap-3 px-3 py-2 rounded-md bg-slate-50 dark:bg-slate-800/50">
          <KindBadge kind={c.kind} />
          <span className="text-sm text-slate-700 dark:text-slate-300">{c.text}</span>
        </div>
      ))}
    </div>
  )
}

function exportEngagementJSON(engagement: EngagementData, constraints: Constraint[]) {
  const payload = {
    engagement: {
      goal: engagement.goal,
      description: engagement.description,
      findings_total: engagement.findings_total,
      facts: engagement.facts,
      goals: engagement.goals.length,
      open_intents: engagement.open_intents.length,
      running_intents: engagement.running_intents.length,
      done_intents: engagement.done_intents_total,
    },
    constraints: constraints.map(c => ({ kind: c.kind, text: c.text })),
  }
  const json = JSON.stringify(payload, null, 2)
  downloadFile(json, `engagement-${new Date().toISOString().slice(0, 10)}.json`, 'application/json')
}

export default function Settings() {
  const { data: engagement, isLoading: engLoading, error: engError, refetch: refetchEng } = useQuery({
    queryKey: ['engagement'],
    queryFn: getEngagement,
  })

  const { data: constraints, isLoading: conLoading, error: conError, refetch: refetchCon } = useQuery({
    queryKey: ['constraints'],
    queryFn: getConstraints,
  })

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 md:px-4 py-3">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Settings</h1>
          <div className="ml-auto">
            <Button
              variant="outline"
              size="sm"
              disabled={!engagement}
              onClick={() => {
                if (engagement && constraints) {
                  exportEngagementJSON(engagement, constraints)
                  toast.success('Exported engagement data')
                }
              }}
            >
              <Download className="h-4 w-4 mr-1" />
              Export JSON
            </Button>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-3 md:p-6 space-y-4 md:space-y-6">
        {engError ? (
          <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
            <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load engagement: {(engError as Error).message}</p>
            <button
              type="button"
              onClick={() => refetchEng()}
              className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
            >
              Try again
            </button>
          </div>
        ) : engLoading ? (
          <SettingsSkeleton />
        ) : (
          <>
            <CollapsibleSection title="Engagement" icon={Info} defaultOpen>
              <div className="space-y-3">
                {engagement?.goal && (
                  <p className="text-base font-medium text-slate-900 dark:text-slate-100">
                    {engagement.goal}
                  </p>
                )}
                {engagement?.description && (
                  <p className="text-sm text-slate-600 dark:text-slate-400">
                    {engagement.description}
                  </p>
                )}
                {engagement?.stats && (
                  <div className="grid grid-cols-3 gap-3 mt-3 pt-3 border-t border-slate-100 dark:border-slate-800">
                    <div className="text-center">
                      <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">{engagement.stats.nodes}</div>
                      <div className="text-xs text-slate-500">Nodes</div>
                    </div>
                    <div className="text-center">
                      <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">{engagement.stats.findings}</div>
                      <div className="text-xs text-slate-500">Findings</div>
                    </div>
                    <div className="text-center">
                      <div className="text-lg font-semibold text-rose-500">{engagement.stats.intents_running}</div>
                      <div className="text-xs text-slate-500">Running</div>
                    </div>
                  </div>
                )}
                {engagement?.stats && engagement.stats.intents_total > 0 && (
                  <div className="mt-3">
                    <div className="flex items-center justify-between text-xs text-slate-500 mb-1">
                      <span>Intent Progress</span>
                      <span>{engagement.stats.intents_done} / {engagement.stats.intents_total}</span>
                    </div>
                    <div className="h-1.5 rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
                      <div
                        className="h-full rounded-full bg-emerald-500 transition-all"
                        style={{ width: `${(engagement.stats.intents_done / engagement.stats.intents_total) * 100}%` }}
                      />
                    </div>
                  </div>
                )}
                <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs text-slate-500 dark:text-slate-500">
                  <span>Task {engagement?.task_id}</span>
                  <span>Exploration {engagement?.exploration_id}</span>
                </div>
              </div>
            </CollapsibleSection>

            <CollapsibleSection title="Constraints" icon={ShieldCheck} defaultOpen={(constraints?.length ?? 0) > 0}>
              {conError ? (
                <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
                  <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load constraints: {(conError as Error).message}</p>
                  <button
                    type="button"
                    onClick={() => refetchCon()}
                    className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
                  >
                    Try again
                  </button>
                </div>
              ) : conLoading ? (
                <div className="space-y-2">
                  <Skeleton className="h-8 rounded-md" />
                  <Skeleton className="h-8 rounded-md" />
                </div>
              ) : (
                <ConstraintsList constraints={constraints ?? []} />
              )}
            </CollapsibleSection>

            <CollapsibleSection title="Credentials" icon={Lock} defaultOpen={false}>
              <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-400">
                <span>{engagement?.task_credentials?.length ?? 0} credentials stored</span>
                <Link to="/credentials" className="text-emerald-500 hover:text-emerald-400 text-xs">View all</Link>
              </div>
            </CollapsibleSection>

            {engagement?.hints && engagement.hints.length > 0 && (
              <CollapsibleSection title="Hints" icon={Info} defaultOpen={false}>
                <div className="space-y-1">
                  {engagement?.hints.map(h => (
                    <div key={h.id} className="px-3 py-1.5 text-sm text-slate-700 dark:text-slate-300 rounded-md bg-slate-50 dark:bg-slate-800/50">
                      {(h as any).label || h.payload?.text as string || JSON.stringify(h.payload ?? h)}
                    </div>
                  ))}
                </div>
              </CollapsibleSection>
            )}
          </>
        )}
      </div>
    </div>
  )
}
