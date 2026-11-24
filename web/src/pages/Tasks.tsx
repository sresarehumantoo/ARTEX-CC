import { lazy, Suspense, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Crosshair, Layers, Server, Shield, Target } from 'lucide-react'
import { toast } from 'sonner'
import { Badge, severityVariant } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs'
import { StatusBadge } from '../components/StatusBadge'
import { getActivity, getEngagement } from '../lib/api'
import type { GraphNode } from '../lib/types'

const Findings = lazy(() => import('./Findings'))
const Assets = lazy(() => import('./Assets'))
const Coverage = lazy(() => import('./Coverage'))
const Activity = lazy(() => import('./Activity'))

interface TaskSummary {
  id: number
  title: string
  status: string
  created_at?: string
  stats: { findings: number; assets: number; coverage_pct: number }
}

async function getTasks(): Promise<TaskSummary[]> {
  const res = await fetch('/api/tasks')
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

async function addHint(text: string) {
  const res = await fetch('/api/hints', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text }),
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}

function hintText(h: GraphNode): string {
  const loose = h as GraphNode & { label?: string; text?: string }
  return loose.label || loose.text || (h.payload?.text as string) || `Hint #${h.id}`
}

function StatCard({ icon: Icon, title, children }: { icon: typeof Server; title: string; children: React.ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4 text-muted-foreground" />
          <CardTitle>{title}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  )
}

function OverviewTab({ task }: { task: TaskSummary | undefined }) {
  const engagement = useQuery({ queryKey: ['engagement'], queryFn: getEngagement, refetchInterval: 10_000 })
  const activity = useQuery({ queryKey: ['activity', 5], queryFn: () => getActivity({ limit: '5' }) })

  const e = engagement.data
  const counts: Record<string, number> = {}
  for (const f of e?.finding_list ?? []) {
    const sev = (f.severity as string) || 'info'
    counts[sev] = (counts[sev] || 0) + 1
  }
  const severities = ['critical', 'high', 'medium', 'low', 'info'].filter((s) => counts[s])
  const goals = e?.goals ?? []

  return (
    <div className="space-y-4">
      <Card>
        <CardContent>
          <div className="flex items-start gap-3">
            <Crosshair className="mt-1 h-6 w-6 shrink-0 text-emerald-500" />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-lg font-semibold text-foreground">{task?.title || e?.goal || 'Engagement'}</h2>
                {task && <StatusBadge domain="task" value={task.status} dot />}
              </div>
              {e?.description && <p className="mt-1 text-sm text-muted-foreground">{e.description}</p>}
              {task?.created_at && (
                <p className="mt-1 text-xs text-muted-foreground">
                  Started {new Date(task.created_at).toLocaleString()}
                </p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard icon={Shield} title="Findings">
          <div className="text-3xl font-bold text-foreground">{task?.stats.findings ?? 0}</div>
          {severities.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {severities.map((s) => (
                <Badge key={s} variant={severityVariant(s)}>
                  {counts[s]} {s}
                </Badge>
              ))}
            </div>
          )}
        </StatCard>
        <StatCard icon={Server} title="Assets">
          <div className="text-3xl font-bold text-foreground">{task?.stats.assets ?? 0}</div>
        </StatCard>
        <StatCard icon={Layers} title="Coverage">
          <div className="text-3xl font-bold text-foreground">{task?.stats.coverage_pct ?? 0}%</div>
          <div className="mt-2 h-2 overflow-hidden rounded-full bg-muted">
            <div className="h-full bg-emerald-500" style={{ width: `${task?.stats.coverage_pct ?? 0}%` }} />
          </div>
        </StatCard>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Target className="h-4 w-4 text-muted-foreground" />
              <CardTitle>Goals</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            {goals.length === 0 ? (
              <p className="text-sm text-muted-foreground">No goals recorded.</p>
            ) : (
              <div className="space-y-2">
                {goals.map((g) => (
                  <div key={g.id} className="flex items-start gap-3">
                    <p className="min-w-0 flex-1 text-sm text-foreground">
                      {(g as GraphNode & { label?: string }).label || (g.payload?.text as string) || `Goal #${g.id}`}
                    </p>
                    <StatusBadge domain="goal" value={g.state} dot />
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Recent Activity</CardTitle>
              <Link to="/activity" className="text-xs text-muted-foreground hover:text-foreground">
                View all
              </Link>
            </div>
          </CardHeader>
          <CardContent>
            {(activity.data ?? []).length === 0 ? (
              <p className="text-sm text-muted-foreground">No recent activity.</p>
            ) : (
              <ul className="space-y-1.5">
                {activity.data!.map((a) => (
                  <li key={a.id} className="flex items-baseline gap-2 text-sm">
                    <span className="shrink-0 text-xs font-medium text-foreground">{a.tool}</span>
                    <span className="truncate text-muted-foreground">{a.summary}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function HintsTab() {
  const qc = useQueryClient()
  const [text, setText] = useState('')
  const engagement = useQuery({ queryKey: ['engagement'], queryFn: getEngagement, refetchInterval: 10_000 })
  const add = useMutation({
    mutationFn: addHint,
    onSuccess: () => {
      setText('')
      qc.invalidateQueries({ queryKey: ['engagement'] })
      toast.success('Hint added')
    },
    onError: (err: Error) => toast.error(`Failed to add hint: ${err.message}`),
  })
  const hints = engagement.data?.hints ?? []

  return (
    <div className="space-y-4">
      <form
        className="flex gap-2"
        onSubmit={(ev) => {
          ev.preventDefault()
          const t = text.trim()
          if (t) add.mutate(t)
        }}
      >
        <Input value={text} onChange={(ev) => setText(ev.target.value)} placeholder="Add a hint for the agent" />
        <Button type="submit" disabled={add.isPending || !text.trim()}>
          Add
        </Button>
      </form>
      <Card>
        <CardContent>
          {hints.length === 0 ? (
            <p className="text-sm text-muted-foreground">No hints yet.</p>
          ) : (
            <ul className="divide-y divide-border">
              {hints.map((h) => (
                <li key={h.id} className="flex items-start justify-between gap-3 py-2 text-sm">
                  <span className="min-w-0 text-foreground">{hintText(h)}</span>
                  <StatusBadge domain="node" value={h.state} dot />
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function TabFallback() {
  return <div className="h-48 animate-pulse rounded-lg bg-muted" />
}

export default function Tasks() {
  const tasks = useQuery({ queryKey: ['tasks'], queryFn: getTasks, refetchInterval: 10_000 })
  const task = tasks.data?.[0]

  return (
    <div className="space-y-4 p-3 md:p-6">
      <h1 className="text-lg font-semibold text-foreground md:text-2xl">Tasks</h1>

      {tasks.isError && (
        <div className="rounded-lg border border-rose-200 bg-rose-50 p-4 dark:border-rose-800/50 dark:bg-rose-900/20">
          <p className="text-sm text-rose-600 dark:text-rose-400">
            Failed to load task: {(tasks.error as Error).message}
          </p>
        </div>
      )}

      <Tabs defaultValue="overview">
        <div className="overflow-x-auto">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="findings">Findings</TabsTrigger>
            <TabsTrigger value="assets">Assets</TabsTrigger>
            <TabsTrigger value="coverage">Coverage</TabsTrigger>
            <TabsTrigger value="activity">Activity</TabsTrigger>
            <TabsTrigger value="hints">Hints</TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="overview">
          <OverviewTab task={task} />
        </TabsContent>
        <TabsContent value="findings">
          <Suspense fallback={<TabFallback />}>
            <Findings />
          </Suspense>
        </TabsContent>
        <TabsContent value="assets">
          <Suspense fallback={<TabFallback />}>
            <Assets />
          </Suspense>
        </TabsContent>
        <TabsContent value="coverage">
          <Suspense fallback={<TabFallback />}>
            <Coverage />
          </Suspense>
        </TabsContent>
        <TabsContent value="activity">
          <Suspense fallback={<TabFallback />}>
            <Activity />
          </Suspense>
        </TabsContent>
        <TabsContent value="hints">
          <HintsTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
