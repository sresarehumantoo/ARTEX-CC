import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Settings as SettingsIcon, ShieldCheck, Lock, Info } from 'lucide-react'
import { getEngagement, getConstraints } from '../lib/api'
import type { Constraint } from '../lib/types'
import { Spinner } from '../components/ui/spinner'
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

export default function Settings() {
  const { data: engagement, isLoading: engLoading, error: engError } = useQuery({
    queryKey: ['engagement'],
    queryFn: getEngagement,
  })

  const { data: constraints, isLoading: conLoading, error: conError } = useQuery({
    queryKey: ['constraints'],
    queryFn: getConstraints,
  })

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 md:px-4 py-3">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Settings</h1>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-3 md:p-6 space-y-4 md:space-y-6">
        {engError ? (
          <div className="text-sm text-rose-400">Failed to load engagement: {(engError as Error).message}</div>
        ) : engLoading ? (
          <div className="flex items-center justify-center h-48"><Spinner className="h-8 w-8" /></div>
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
                <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs text-slate-500 dark:text-slate-500">
                  <span>Task {engagement?.task_id}</span>
                  <span>Exploration {engagement?.exploration_id}</span>
                </div>
              </div>
            </CollapsibleSection>

            <CollapsibleSection title="Constraints" icon={ShieldCheck} defaultOpen={(constraints?.length ?? 0) > 0}>
              {conError ? (
                <div className="text-sm text-rose-400">Failed to load constraints: {(conError as Error).message}</div>
              ) : conLoading ? (
                <div className="flex items-center justify-center h-16"><Spinner className="h-6 w-6" /></div>
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

            <CollapsibleSection title="Hints" icon={SettingsIcon} defaultOpen={(engagement?.hints?.length ?? 0) > 0}>
              {engagement?.hints && engagement.hints.length > 0 ? (
                <div className="space-y-2">
                  {engagement.hints.map(h => {
                    const text = (h.payload.text as string) ?? (h.payload.label as string) ?? `Hint #${h.id}`
                    return (
                      <div key={h.id} className="px-3 py-2 rounded-md bg-slate-50 dark:bg-slate-800/50 text-sm text-slate-700 dark:text-slate-300">
                        {text}
                      </div>
                    )
                  })}
                </div>
              ) : (
                <p className="text-sm text-slate-500 italic py-2">No hints added</p>
              )}
            </CollapsibleSection>
          </>
        )}
      </div>
    </div>
  )
}
