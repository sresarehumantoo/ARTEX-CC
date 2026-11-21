import { useQuery } from '@tanstack/react-query'
import {
  Server,
  Shield,
  GitBranch,
  Layers,
  AlertCircle,
  Wrench,
} from 'lucide-react'
import { Spinner } from '../components/ui/spinner'
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card'
import { Badge, severityVariant } from '../components/ui/badge'
import { getEngagement, getActivity } from '../lib/api'
import type { Activity, EngagementData } from '../lib/types'

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
  children,
}: {
  icon: typeof Server
  title: string
  value: string | number
  children?: React.ReactNode
}) {
  return (
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
              <span className="text-xs text-slate-400 dark:text-slate-600">{relativeTime(a.created_at)}</span>
            </div>
            <p className="truncate text-sm text-slate-500 dark:text-slate-400">{a.summary}</p>
          </div>
        </div>
      ))}
    </div>
  )
}

export function Dashboard() {
  const engagement = useQuery({
    queryKey: ['engagement'],
    queryFn: getEngagement,
    refetchInterval: 10_000,
  })

  const activity = useQuery({
    queryKey: ['activity'],
    queryFn: () => getActivity({ limit: '20' }),
    refetchInterval: 10_000,
  })

  const e = engagement.data

  return (
    <div className="space-y-6 p-6">
      <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Dashboard</h1>

      {engagement.isLoading && (
        <div className="flex items-center justify-center h-64"><Spinner className="h-8 w-8" /></div>
      )}
      {engagement.isError && (
        <p className="text-sm text-rose-400">
          Failed to load engagement: {(engagement.error as Error).message}
        </p>
      )}

      {e && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard icon={Server} title="Assets" value={e.facts}>
              {e.recent_facts.length > 0 && (
                <span>{e.recent_facts.length} recent</span>
              )}
            </StatCard>

            <StatCard icon={Shield} title="Findings" value={e.findings_total}>
              <SeverityBreakdown findings={e.finding_list} />
            </StatCard>

            <StatCard icon={GitBranch} title="Graph" value={
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

            <StatCard icon={Layers} title="Coverage" value={
              Object.keys(e.coverage).length
            }>
              <span>{e.frontier_open} frontier open</span>
            </StatCard>
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
          {activity.data && <ActivityFeed items={activity.data} />}
        </CardContent>
      </Card>
    </div>
  )
}
