import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Server,
  Shield,
  GitBranch,
  Layers,
  AlertCircle,
  Clock,
  Wrench,
  CheckCircle,
  Target,
  Crosshair,
} from 'lucide-react'
import { DashboardSkeleton } from '../components/skeletons/DashboardSkeleton'
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card'
import { Badge, severityVariant } from '../components/ui/badge'
import { getEngagement, getActivity, getAssets } from '../lib/api'
import { cn } from '../lib/utils'
import { StatusBadge } from '../components/StatusBadge'
import type { Activity, EngagementData, GraphNode, IntentSummary } from '../lib/types'

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
          <Icon className="h-4 w-4 text-muted-foreground" />
          <CardTitle>{title}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold text-foreground">{value}</div>
        {children && <div className="mt-2 text-sm text-muted-foreground">{children}</div>}
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
    return <p className="text-sm text-muted-foreground">No recent activity.</p>
  }

  return (
    <div className="space-y-1">
      {items.map((a) => {
        const content = (
          <div className="flex items-start gap-3 rounded-md px-3 py-2 hover:bg-accent">
            <div className="mt-0.5 shrink-0">
              {a.is_error ? (
                <AlertCircle className="h-4 w-4 text-rose-400" />
              ) : (
                <Wrench className="h-4 w-4 text-muted-foreground" />
              )}
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-xs font-medium text-foreground">{a.tool}</span>
                {a.created_at && <span className="text-xs text-muted-foreground dark:text-slate-600">{relativeTime(a.created_at)}</span>}
              </div>
              <p className="truncate text-sm text-muted-foreground">{a.summary}</p>
            </div>
          </div>
        )
        return a.node_id ? (
          <Link key={a.id} to={`/graph?focus=${a.node_id}`} className="block">
            {content}
          </Link>
        ) : (
          <div key={a.id}>{content}</div>
        )
      })}
    </div>
  )
}

function GoalProgress({ goals }: { goals: GraphNode[] }) {
  if (goals.length === 0) return null

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Target className="h-4 w-4 text-muted-foreground" />
          <CardTitle>Goals</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {goals.map((g) => (
            <div key={g.id} className="flex items-start gap-3">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-foreground">
                  {(g as any).label || (g.payload?.text as string) || `Goal #${g.id}`}
                </p>
              </div>
              <StatusBadge domain="goal" value={g.state.includes('proven') && g.state !== 'disproven' ? 'proven' : g.state} dot />
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function shortHolder(name: string): string {
  const m = name.match(/^chat\/(.{8}).{2,}$/)
  return m ? `chat/${m[1]}…` : name
}

function timeLeft(iso: string): string {
  const minutes = Math.floor((new Date(iso).getTime() - Date.now()) / 60_000)
  if (minutes < 1) return 'expiring'
  if (minutes < 60) return `${minutes}m left`
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m left`
}

export function RunningIntents({ intents }: { intents: IntentSummary[] }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Wrench className="h-4 w-4 text-muted-foreground" />
          <CardTitle>Active Intents</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        {intents.length === 0 ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <CheckCircle className="h-4 w-4" />
            <span>No active intents</span>
          </div>
        ) : (
          <div className="space-y-2">
            {intents.map((i) => (
              <div key={i.id} className="flex items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="text-sm text-foreground">
                    {i.label || i.summary || i.text || `Intent #${i.id}`}
                  </p>
                  {i.claimed_by && (
                    <p className="text-xs text-muted-foreground">
                      <span className="font-mono" title={i.claimed_by}>{shortHolder(i.claimed_by)}</span>
                      {i.claim_expires_at && <> · {timeLeft(i.claim_expires_at)}</>}
                    </p>
                  )}
                  {i.claim_expired && (
                    <p className="text-xs text-muted-foreground">
                      claim lapsed
                      {i.previous_owner && (
                        <> (<span className="font-mono" title={i.previous_owner}>{shortHolder(i.previous_owner)}</span>)</>
                      )}
                    </p>
                  )}
                </div>
                <StatusBadge domain="intent" value={i.state} dot />
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
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Layers className="h-4 w-4 text-muted-foreground" />
              <CardTitle>Coverage</CardTitle>
            </div>
            <Link
              to="/coverage"
              className="text-xs text-muted-foreground hover:text-foreground"
            >
              View all
            </Link>
          </div>
        </CardHeader>
        <CardContent>
          <div className="text-3xl font-bold text-foreground">0</div>
          <div className="mt-2 text-sm text-muted-foreground">No coverage data yet</div>
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
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Layers className="h-4 w-4 text-muted-foreground" />
            <CardTitle>Coverage</CardTitle>
          </div>
          <Link
            to="/coverage"
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            View all
          </Link>
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
              <span className="text-2xl font-bold text-foreground">{entries}</span>
            </div>
          </div>
          <div className="space-y-1.5 text-xs">
            {segments.map((s, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <span className={`inline-block h-2.5 w-2.5 rounded-full bg-current ${s.color}`} />
                <span className="text-muted-foreground">
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

interface TimeBucket {
  label: string
  count: number
}

function FindingsTimeline({ activities }: { activities: Activity[] }) {
  const buckets = useMemo(() => {
    const findings = activities
      .filter(a => a.kind === 'finding' && a.created_at)
      .sort((a, b) => new Date(a.created_at!).getTime() - new Date(b.created_at!).getTime())

    if (findings.length < 2) return null

    const first = new Date(findings[0].created_at!).getTime()
    const last = new Date(findings[findings.length - 1].created_at!).getTime()
    const rangeMs = last - first

    const HOUR = 3_600_000
    const DAY = 86_400_000

    let bucketMs: number
    let formatLabel: (d: Date) => string
    if (rangeMs < DAY) {
      bucketMs = HOUR
      formatLabel = d => d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    } else if (rangeMs < 7 * DAY) {
      bucketMs = 6 * HOUR
      formatLabel = d => {
        const day = d.toLocaleDateString([], { weekday: 'short' })
        const time = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        return `${day} ${time}`
      }
    } else {
      bucketMs = DAY
      formatLabel = d => d.toLocaleDateString([], { month: 'short', day: 'numeric' })
    }

    const bucketStart = Math.floor(first / bucketMs) * bucketMs
    const bucketEnd = Math.floor(last / bucketMs) * bucketMs + bucketMs
    const bucketCount = Math.ceil((bucketEnd - bucketStart) / bucketMs)

    const result: TimeBucket[] = []
    for (let i = 0; i < bucketCount; i++) {
      result.push({
        label: formatLabel(new Date(bucketStart + i * bucketMs)),
        count: 0,
      })
    }

    for (const f of findings) {
      const t = new Date(f.created_at!).getTime()
      const idx = Math.min(Math.floor((t - bucketStart) / bucketMs), bucketCount - 1)
      result[idx].count++
    }

    return result
  }, [activities])

  if (!buckets) return null

  const maxCount = Math.max(...buckets.map(b => b.count))
  const totalFindings = buckets.reduce((s, b) => s + b.count, 0)

  const chartHeight = 160
  const chartPadding = 24
  const barGap = 2
  const labelCount = Math.min(5, buckets.length)
  const labelIndices: number[] = []
  if (labelCount >= 2) {
    for (let i = 0; i < labelCount; i++) {
      labelIndices.push(Math.round((i / (labelCount - 1)) * (buckets.length - 1)))
    }
  } else if (buckets.length > 0) {
    labelIndices.push(0)
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Clock className="h-4 w-4 text-muted-foreground" />
            <CardTitle>Finding Timeline</CardTitle>
          </div>
          <span className="text-sm text-muted-foreground">
            {totalFindings} finding{totalFindings !== 1 ? 's' : ''}
          </span>
        </div>
      </CardHeader>
      <CardContent>
        <svg
          width="100%"
          height={chartHeight + chartPadding}
          viewBox={`0 0 ${buckets.length * 20} ${chartHeight + chartPadding}`}
          preserveAspectRatio="none"
          className="overflow-visible"
        >
          {buckets.map((b, i) => {
            const barWidth = 20 - barGap
            const barHeight = maxCount > 0 ? (b.count / maxCount) * chartHeight : 0
            const x = i * 20 + barGap / 2
            const y = chartHeight - barHeight
            return (
              <rect
                key={i}
                x={x}
                y={y}
                width={barWidth}
                height={Math.max(barHeight, b.count > 0 ? 3 : 0)}
                rx={2}
                className="fill-emerald-500/80"
              />
            )
          })}
          {labelIndices.map(idx => (
            <text
              key={idx}
              x={idx * 20 + (20 - barGap) / 2}
              y={chartHeight + chartPadding - 2}
              textAnchor="middle"
              className="fill-slate-500 dark:fill-slate-400"
              style={{ fontSize: 10 }}
            >
              {buckets[idx].label}
            </text>
          ))}
        </svg>
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
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>Intent progress</span>
        <span>{done} done, {running} running, {open} open</span>
      </div>
      <div className="flex h-2 overflow-hidden rounded-full bg-muted">
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

const SEVERITY_DOT: Record<string, string> = {
  critical: 'bg-rose-500',
  high: 'bg-orange-500',
  medium: 'bg-amber-500',
  low: 'bg-blue-500',
  info: 'bg-slate-400',
}

function RecentFindings({ findings }: { findings: Record<string, unknown>[] }) {
  const recent = findings.slice(0, 5)
  if (recent.length === 0) return null

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Shield className="h-4 w-4 text-muted-foreground" />
            <CardTitle>Recent Findings</CardTitle>
          </div>
          <Link
            to="/findings"
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            View all
          </Link>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {recent.map((f) => {
            const id = f.id as number
            const severity = (f.severity as string) ?? 'info'
            const summary = (f.label as string) ?? (f.summary as string) ?? 'Untitled finding'
            const vulnclass = f.vulnclass as string | undefined
            return (
              <Link
                key={id}
                to={`/findings?id=${id}`}
                className="flex items-start gap-3 rounded-md px-3 py-2 hover:bg-accent transition-colors group"
              >
                <span className={cn('mt-1.5 h-2 w-2 shrink-0 rounded-full', SEVERITY_DOT[severity] ?? 'bg-slate-400')} />
                <div className="min-w-0 flex-1">
                  <p className="text-sm text-foreground group-hover:text-slate-900 dark:group-hover:text-slate-100 truncate">
                    {summary}
                  </p>
                  <div className="flex items-center gap-2 mt-0.5">
                    <span className="text-[11px] font-medium text-muted-foreground capitalize">{severity}</span>
                    {vulnclass && (
                      <span className="text-[11px] text-muted-foreground truncate">{vulnclass}</span>
                    )}
                  </div>
                </div>
              </Link>
            )
          })}
        </div>
      </CardContent>
    </Card>
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

  const assets = useQuery({
    queryKey: ['assets', 'dashboard-total'],
    queryFn: () => getAssets({ limit: '1' }),
    refetchInterval: 10_000,
  })

  const activity = useQuery({
    queryKey: ['activity', activityLimit],
    queryFn: () => getActivity({ limit: String(activityLimit) }),
    refetchInterval: 10_000,
  })

  const raw = engagement.data
  const e = raw ? {
    ...raw,
    running_intents: raw.running_intents ?? [],
    open_intents: raw.open_intents ?? [],
    goals: raw.goals ?? [],
    finding_list: raw.finding_list ?? [],
    recent_facts: raw.recent_facts ?? [],
    hints: raw.hints ?? [],
    task_credentials: raw.task_credentials ?? [],
  } : undefined

  return (
    <div className="space-y-4 md:space-y-6 p-3 md:p-6">
      <h1 className="text-lg md:text-2xl font-semibold text-foreground">Dashboard</h1>

      {engagement.isLoading && <DashboardSkeleton />}
      {engagement.isError && (
        <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
          <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load engagement: {(engagement.error as Error).message}</p>
          <button
            type="button"
            onClick={() => engagement.refetch()}
            className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
          >
            Try again
          </button>
        </div>
      )}

      {e && (
        <>
          {e.goal && (
            <div className="rounded-lg border border-border bg-background p-3 md:p-4">
              <div className="flex items-start gap-3">
                <Crosshair className="h-5 w-5 text-emerald-500 shrink-0 mt-0.5" />
                <div className="min-w-0">
                  <h2 className="text-sm font-semibold text-foreground">{e.goal}</h2>
                  {e.description && (
                    <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">{e.description}</p>
                  )}
                </div>
              </div>
            </div>
          )}

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard icon={Server} title="Assets" value={assets.data?.total ?? 0} to="/assets" />

            <StatCard icon={Shield} title="Findings" value={e.findings_total} to="/findings">
              <SeverityBreakdown findings={e.finding_list} />
            </StatCard>

            <StatCard icon={GitBranch} title="Graph" to="/graph" value={
              e.goals.length +
              e.open_intents.length +
              e.running_intents.length +
              e.done_intents_total +
              e.facts +
              e.findings_total
            }>
              <span>
                <span className="whitespace-nowrap">{e.goals.length} goals,</span>{' '}
                <span className="whitespace-nowrap">{e.open_intents.length + e.running_intents.length} active intents,</span>{' '}
                <span className="whitespace-nowrap">{e.facts} facts,</span>{' '}
                <span className="whitespace-nowrap">{e.findings_total} findings</span>
              </span>
            </StatCard>

            <CoverageRing coverage={e.coverage} frontierOpen={e.frontier_open} />
          </div>

          <IntentProgressBar
            done={e.done_intents_total}
            running={e.running_intents.length}
            open={e.open_intents.length}
          />

          <FindingsTimeline activities={activity.data ?? []} />

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <GoalProgress goals={e.goals} />
            <RunningIntents intents={[...e.running_intents, ...e.open_intents.filter((i) => i.claim_expired)]} />
          </div>

          <RecentFindings findings={e.finding_list} />

          {e.hints.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>Hints</CardTitle>
              </CardHeader>
              <CardContent>
                <ul className="space-y-1 text-sm text-foreground">
                  {e.hints.map((h) => (
                    <li key={h.id}>{(h as any).label || (h as any).text || (h.payload?.text as string) || JSON.stringify(h.payload ?? h)}</li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          )}
        </>
      )}

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Recent Activity</CardTitle>
            <Link
              to="/activity"
              className="text-xs text-muted-foreground hover:text-foreground"
            >
              View all
            </Link>
          </div>
        </CardHeader>
        <CardContent>
          {activity.isLoading && (
            <div className="space-y-3">
              {[1, 2, 3, 4, 5].map(i => (
                <div key={i} className="animate-pulse flex items-start gap-3 py-2">
                  <div className="h-2 w-2 rounded-full bg-muted mt-1.5 shrink-0" />
                  <div className="flex-1 space-y-1.5">
                    <div className="h-3.5 w-3/4 rounded bg-muted" />
                    <div className="h-3 w-1/3 rounded bg-muted" />
                  </div>
                </div>
              ))}
            </div>
          )}
          {activity.isError && (
            <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
              <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load activity: {(activity.error as Error).message}</p>
              <button
                type="button"
                onClick={() => activity.refetch()}
                className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
              >
                Try again
              </button>
            </div>
          )}
          {activity.data && (
            <>
              <ActivityFeed items={activity.data} />
              {activity.data.length >= activityLimit && (
                <div className="mt-3 text-center">
                  <button
                    type="button"
                    className="text-sm text-muted-foreground hover:text-foreground"
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
