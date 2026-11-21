import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Server,
  Shield,
  GitBranch,
  Layers,
  AlertCircle,
  Wrench,
  CheckCircle,
  Target,
} from 'lucide-react'
import { DashboardSkeleton } from '../components/skeletons/DashboardSkeleton'
import { Spinner } from '../components/ui/spinner'
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card'
import { Badge, severityVariant } from '../components/ui/badge'
import { getEngagement, getActivity } from '../lib/api'
import type { Activity, EngagementData, GraphNode } from '../lib/types'

function relativeTime(iso: string): string {
  const seconds = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function StatCard({
  icon: Icon,
  title,
  value,
  to,
  children,
}: {
  icon: typeof Server
  title: string
  value: string | number
  to?: string
  children?: React.ReactNode
}) {
  const card = (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4 text-slate-500 dark:text-slate-400" />
          <CardTitle>{title}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold text-slate-900 dark:text-slate-100">{value}</div>
        {children && <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">{children}</div>}
      </CardContent>
    </Card>
  )

  if (!to) return card

  return (
    <Link to={to} className="block rounded-xl transition-transform hover:scale-[1.02]">
      {card}
    </Link>
  )
}

function SeverityBreakdown({ findings }: { findings: EngagementData['finding_list'] }) {
  const counts: Record<string, number> = {}
  for (const f of findings) {
    const sev = (f.severity as string) || 'unknown'
    counts[sev] = (counts[sev] || 0) + 1
  }
  const order = ['critical', 'high', 'medium', 'low']
  const present = order.filter((s) => counts[s])

  if (present.length === 0) return null

  return (
    <div className="flex gap-1.5 flex-wrap">
      {present.map((s) => (
        <Badge key={s} variant={severityVariant(s)}>
          {counts[s]} {s}
        </Badge>
      ))}
    </div>
  )
}

function ActivityFeed({ items }: { items: Activity[] }) {
  if (items.length === 0) {
    return <p className="text-sm text-slate-500">No recent activity.</p>
  }

  return (
    <div className="space-y-1">
      {items.map((a) => (
        <div
          key={a.id}
          className="flex items-start gap-3 rounded-md px-3 py-2 hover:bg-slate-100/50 dark:hover:bg-slate-800/50"
        >
          <div className="mt-0.5 shrink-0">
            {a.is_error ? (
              <AlertCircle className="h-4 w-4 text-rose-400" />
            ) : (
              <Wrench className="h-4 w-4 text-slate-500" />
            )}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-slate-700 dark:text-slate-300">{a.tool}</span>
              {a.created_at && <span className="text-xs text-slate-400 dark:text-slate-600">{relativeTime(a.created_at)}</span>}
            </div>
            <p className="truncate text-sm text-slate-500 dark:text-slate-400">{a.summary}</p>
          </div>
        </div>
      ))}
    </div>
  )
}

function goalStateDot(state: string) {
  if (state.includes('proven'))
    return 'bg-emerald-500'
  if (state === 'running' || state === 'in_progress')
    return 'bg-amber-500'
  return 'bg-slate-400 dark:bg-slate-500'
}

function goalStateLabel(state: string) {
  if (state.includes('proven')) return 'Proven'
  if (state === 'disproven') return 'Disproven'
  if (state === 'running' || state === 'in_progress') return 'In progress'
  return 'Open'
}

function GoalProgress({ goals }: { goals: GraphNode[] }) {
  if (goals.length === 0) return null

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Target className="h-4 w-4 text-slate-500 dark:text-slate-400" />
          <CardTitle>Goals</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {goals.map((g) => (
            <div key={g.id} className="flex items-start gap-3">
              <span className={`mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full ${goalStateDot(g.state)}`} />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-slate-700 dark:text-slate-300">
                  {(g.payload.text as string) || `Goal #${g.id}`}
                </p>
                <p className="text-xs text-slate-500 dark:text-slate-400">{goalStateLabel(g.state)}</p>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function RunningIntents({ intents }: { intents: GraphNode[] }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Wrench className="h-4 w-4 text-slate-500 dark:text-slate-400" />
          <CardTitle>Active Intents</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        {intents.length === 0 ? (
          <div className="flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400">
            <CheckCircle className="h-4 w-4" />
            <span>No active intents</span>
          </div>
        ) : (
          <div className="space-y-2">
            {intents.map((i) => (
              <div key={i.id} className="flex items-start gap-3">
                <span className="relative mt-1.5 flex h-2.5 w-2.5 shrink-0">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-blue-400 opacity-75" />
                  <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-blue-500" />
                </span>
                <p className="text-sm text-slate-700 dark:text-slate-300">
                  {(i.payload.text as string) || `Intent #${i.id}`}
                </p>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function CoverageRing({ coverage, frontierOpen }: { coverage: Record<string, unknown>; frontierOpen: number }) {
  const entries = (typeof coverage.entries === 'number' ? coverage.entries : 0) as number
  const gaps = (typeof coverage.gaps === 'number' ? coverage.gaps : 0) as number
  const tested = entries - gaps
  const total = entries + frontierOpen

  if (total === 0) {
    return (
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Layers className="h-4 w-4 text-slate-500 dark:text-slate-400" />
            <CardTitle>Coverage</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className="text-3xl font-bold text-slate-900 dark:text-slate-100">0</div>
          <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">No coverage data yet</div>
        </CardContent>
      </Card>
    )
  }

  const size = 120
  const strokeWidth = 14
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius

  const segments = [
    { value: Math.max(0, tested), color: 'text-emerald-500', label: 'Tested' },
    { value: gaps, color: 'text-amber-500', label: 'Needs follow-up' },
    { value: frontierOpen, color: 'text-slate-300 dark:text-slate-600', label: 'Unexamined' },
  ].filter(s => s.value > 0)

  let offset = 0
  const arcs = segments.map(s => {
    const pct = s.value / total
    const dash = pct * circumference
    const gap = circumference - dash
    const arc = { ...s, dashArray: `${dash} ${gap}`, dashOffset: -offset }
    offset += dash
    return arc
  })

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Layers className="h-4 w-4 text-slate-500 dark:text-slate-400" />
          <CardTitle>Coverage</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-6">
          <div className="relative shrink-0" style={{ width: size, height: size }}>
            <svg width={size} height={size} className="-rotate-90">
              {arcs.map((arc, idx) => (
                <circle
                  key={idx}
                  cx={size / 2}
                  cy={size / 2}
                  r={radius}
                  fill="none"
                  strokeWidth={strokeWidth}
                  className={`stroke-current ${arc.color}`}
                  strokeDasharray={arc.dashArray}
                  strokeDashoffset={arc.dashOffset}
                  strokeLinecap="round"
                />
              ))}
            </svg>
            <div className="absolute inset-0 flex items-center justify-center">
              <span className="text-2xl font-bold text-slate-900 dark:text-slate-100">{entries}</span>
            </div>
          </div>
          <div className="space-y-1.5 text-xs">
            {segments.map((s, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <span className={`inline-block h-2.5 w-2.5 rounded-full bg-current ${s.color}`} />
                <span className="text-slate-600 dark:text-slate-400">
                  {s.label}: {s.value}
                </span>
              </div>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function IntentProgressBar({ done, running, open }: { done: number; running: number; open: number }) {
  const total = done + running + open
  if (total === 0) return null

  const pctDone = (done / total) * 100
  const pctRunning = (running / total) * 100

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
        <span>Intent progress</span>
        <span>{done} done, {running} running, {open} open</span>
      </div>
      <div className="flex h-2 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700">
        {pctDone > 0 && (
          <div
            className="bg-emerald-500 transition-all"
            style={{ width: `${pctDone}%` }}
          />
        )}
        {pctRunning > 0 && (
          <div
            className="animate-pulse bg-blue-500 transition-all"
            style={{ width: `${pctRunning}%` }}
          />
        )}
      </div>
    </div>
  )
}

const ACTIVITY_PAGE_SIZE = 20

export function Dashboard() {
  const [activityLimit, setActivityLimit] = useState(ACTIVITY_PAGE_SIZE)

  const engagement = useQuery({
    queryKey: ['engagement'],
    queryFn: getEngagement,
    refetchInterval: 10_000,
  })

  const activity = useQuery({
    queryKey: ['activity', activityLimit],
    queryFn: () => getActivity({ limit: String(activityLimit) }),
    refetchInterval: 10_000,
  })

  const e = engagement.data

  return (
    <div className="space-y-4 md:space-y-6 p-3 md:p-6">
      <h1 className="text-lg md:text-2xl font-semibold text-slate-900 dark:text-slate-100">Dashboard</h1>

      {e?.goal && (
        <div>
          <p className="text-base md:text-lg text-slate-600 dark:text-slate-300">{e.goal}</p>
          {e.description && (
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{e.description}</p>
          )}
        </div>
      )}

      {engagement.isLoading && <DashboardSkeleton />}
      {engagement.isError && (
        <p className="text-sm text-rose-400">
          Failed to load engagement: {(engagement.error as Error).message}
        </p>
      )}

      {e && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard icon={Server} title="Assets" value={e.facts} to="/assets">
              {e.recent_facts.length > 0 && (
                <span>{e.recent_facts.length} recent</span>
              )}
            </StatCard>

            <StatCard icon={Shield} title="Findings" value={e.findings_total} to="/findings">
              <SeverityBreakdown findings={e.finding_list} />
            </StatCard>

            <StatCard icon={GitBranch} title="Graph" to="/graph" value={
              e.goals.length +
              e.open_intents.length +
              e.running_intents.length +
              e.done_intents_total +
              e.facts
            }>
              <span>
                {e.goals.length} goals, {e.open_intents.length + e.running_intents.length} active intents, {e.facts} facts
              </span>
            </StatCard>

            <CoverageRing coverage={e.coverage} frontierOpen={e.frontier_open} />
          </div>

          <IntentProgressBar
            done={e.done_intents_total}
            running={e.running_intents.length}
            open={e.open_intents.length}
          />

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <GoalProgress goals={e.goals} />
            <RunningIntents intents={e.running_intents} />
          </div>

          {e.hints.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>Hints</CardTitle>
              </CardHeader>
              <CardContent>
                <ul className="space-y-1 text-sm text-slate-700 dark:text-slate-300">
                  {e.hints.map((h) => (
                    <li key={h.id}>{h.payload.text as string || JSON.stringify(h.payload)}</li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          )}
        </>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Recent Activity</CardTitle>
        </CardHeader>
        <CardContent>
          {activity.isLoading && (
            <div className="flex items-center justify-center h-64"><Spinner className="h-8 w-8" /></div>
          )}
          {activity.isError && (
            <p className="text-sm text-rose-400">
              Failed to load activity: {(activity.error as Error).message}
            </p>
          )}
          {activity.data && (
            <>
              <ActivityFeed items={activity.data} />
              {activity.data.length >= activityLimit && (
                <div className="mt-3 text-center">
                  <button
                    type="button"
                    className="text-sm text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
                    onClick={() => setActivityLimit(prev => prev + ACTIVITY_PAGE_SIZE)}
                  >
                    Load more activity
                  </button>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
