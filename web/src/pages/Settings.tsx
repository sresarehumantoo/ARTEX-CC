import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Settings as SettingsIcon, ShieldCheck, Lock, Info, Download } from 'lucide-react'
import { getEngagement, getConstraints } from '../lib/api'
import type { Constraint, EngagementData } from '../lib/types'
import { usePreferences } from '../stores/preferences'
import type { ThemeMode, ContentLayout } from '../stores/preferences'
import { Skeleton } from '../components/ui/skeleton'
import { SettingsSkeleton } from '../components/skeletons/SettingsSkeleton'
import { Button } from '../components/ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../components/ui/card'
import { SelectRoot, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../components/ui/select'
import { Switch } from '../components/ui/switch'
import { Label } from '../components/ui/label'
import { downloadFile } from '../lib/export'
import { toast } from 'sonner'
import { cn } from '../lib/utils'
import LlmTab from './LlmTab'
import { SOURCE_URL, sourceUrlFor } from '../lib/source'

interface SystemInfo {
  version: string
  go_version: string
  os: string
  arch: string
  data_dir: string
}

async function getSystem(): Promise<SystemInfo> {
  const res = await fetch('/api/system')
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

const KIND_COLORS: Record<string, string> = {
  time: 'bg-blue-500/10 text-blue-500 border-blue-500/20',
  scope: 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20',
  auth: 'bg-amber-500/10 text-amber-500 border-amber-500/20',
}

function KindBadge({ kind }: { kind: string }) {
  const color = KIND_COLORS[kind] ?? 'bg-slate-500/10 text-muted-foreground border-slate-500/20'
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
    <div className="border border-border rounded-lg">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex w-full items-center gap-2 px-3 md:px-4 py-3 text-left hover:bg-accent transition-colors rounded-lg"
      >
        {open ? <ChevronDown className="h-4 w-4 text-muted-foreground" /> : <ChevronRight className="h-4 w-4 text-muted-foreground" />}
        <Icon className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm font-medium text-foreground">{title}</span>
      </button>
      {open && <div className="px-3 md:px-4 pb-4">{children}</div>}
    </div>
  )
}

function ConstraintsList({ constraints }: { constraints: Constraint[] }) {
  if (constraints.length === 0) {
    return <p className="text-sm text-muted-foreground italic py-2">No constraints set</p>
  }

  return (
    <div className="space-y-2">
      {constraints.map(c => (
        <div key={c.id} className="flex items-start gap-3 px-3 py-2 rounded-md bg-muted">
          <KindBadge kind={c.kind} />
          <span className="text-sm text-foreground">{c.text}</span>
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

function SettingRow({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-6">
      <div className="space-y-1">
        <Label>{label}</Label>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
      {children}
    </div>
  )
}

function InfoRow({ label, value }: { label: string; value?: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 sm:flex-row sm:items-baseline sm:justify-between sm:gap-6">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="break-all font-mono text-sm text-foreground sm:text-right">{value || 'Unknown'}</span>
    </div>
  )
}

function AppearanceTab() {
  const prefs = usePreferences()
  return (
    <Card>
      <CardHeader>
        <CardTitle>Appearance</CardTitle>
        <CardDescription>Stored in this browser only.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <SettingRow label="Theme" hint="System follows your OS preference.">
          <SelectRoot value={prefs.theme_mode} onValueChange={v => prefs.setThemeMode(v as ThemeMode)}>
            <SelectTrigger aria-label="Theme">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="system">System</SelectItem>
              <SelectItem value="light">Light</SelectItem>
              <SelectItem value="dark">Dark</SelectItem>
            </SelectContent>
          </SelectRoot>
        </SettingRow>
        <SettingRow label="Sidebar collapsed by default">
          <Switch checked={prefs.sidebar_collapsed} onCheckedChange={prefs.setSidebarCollapsed} aria-label="Sidebar collapsed by default" />
        </SettingRow>
        <SettingRow label="Content layout">
          <SelectRoot value={prefs.content_layout} onValueChange={v => prefs.setContentLayout(v as ContentLayout)}>
            <SelectTrigger aria-label="Content layout">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="full-width">Full width</SelectItem>
              <SelectItem value="centered">Centered</SelectItem>
            </SelectContent>
          </SelectRoot>
        </SettingRow>
      </CardContent>
    </Card>
  )
}

function EngagementTab() {
  const { data: engagement, isLoading: engLoading, error: engError, refetch: refetchEng } = useQuery({
    queryKey: ['engagement'],
    queryFn: getEngagement,
  })
  const { data: constraints, isLoading: conLoading, error: conError, refetch: refetchCon } = useQuery({
    queryKey: ['constraints'],
    queryFn: getConstraints,
  })
  const { data: system } = useQuery({ queryKey: ['system'], queryFn: getSystem })

  return (
    <div className="space-y-4 md:space-y-6">
      <div className="flex justify-end">
        <Button
          variant="outline"
          size="sm"
          disabled={!engagement || !constraints}
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
          <Card>
            <CardHeader>
              <CardTitle>Current engagement</CardTitle>
              <CardDescription>Fixed at startup. Change it with CLI flags.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <InfoRow label="Goal" value={engagement?.goal} />
              <InfoRow label="Task ID" value={engagement?.task_id} />
              <InfoRow label="Exploration ID" value={engagement?.exploration_id} />
              <InfoRow label="Data directory" value={system?.data_dir} />
              {engagement?.description && <InfoRow label="Description" value={engagement.description} />}
            </CardContent>
          </Card>

          <CollapsibleSection title="Overview" icon={Info} defaultOpen>
            <div className="space-y-3">
              {engagement?.stats && (
                <div className="grid grid-cols-3 gap-3">
                  <div className="text-center">
                    <div className="text-lg font-semibold text-foreground">{engagement.stats.nodes}</div>
                    <div className="text-xs text-muted-foreground">Nodes</div>
                  </div>
                  <div className="text-center">
                    <div className="text-lg font-semibold text-foreground">{engagement.stats.findings}</div>
                    <div className="text-xs text-muted-foreground">Findings</div>
                  </div>
                  <div className="text-center">
                    <div className="text-lg font-semibold text-rose-500">{engagement.stats.intents_running}</div>
                    <div className="text-xs text-muted-foreground">Running</div>
                  </div>
                </div>
              )}
              {engagement?.stats && engagement.stats.intents_total > 0 && (
                <div>
                  <div className="flex items-center justify-between text-xs text-muted-foreground mb-1">
                    <span>Intent Progress</span>
                    <span>{engagement.stats.intents_done} / {engagement.stats.intents_total}</span>
                  </div>
                  <div className="h-1.5 rounded-full bg-muted overflow-hidden">
                    <div
                      className="h-full rounded-full bg-emerald-500 transition-all"
                      style={{ width: `${(engagement.stats.intents_done / engagement.stats.intents_total) * 100}%` }}
                    />
                  </div>
                </div>
              )}
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
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <span>{engagement?.task_credentials?.length ?? 0} credentials stored</span>
              <Link to="/credentials" className="text-emerald-500 hover:text-emerald-400 text-xs">View all</Link>
            </div>
          </CollapsibleSection>

          {engagement?.hints && engagement.hints.length > 0 && (
            <CollapsibleSection title="Hints" icon={Info} defaultOpen={false}>
              <div className="space-y-1">
                {engagement.hints.map(h => (
                  <div key={h.id} className="px-3 py-1.5 text-sm text-foreground rounded-md bg-muted">
                    {(h as any).label || h.payload?.text as string || JSON.stringify(h.payload ?? h)}
                  </div>
                ))}
              </div>
            </CollapsibleSection>
          )}
        </>
      )}
    </div>
  )
}

function AboutTab() {
  const { data, isLoading, error } = useQuery({ queryKey: ['system'], queryFn: getSystem })
  const linkClass = 'text-sm text-[oklch(0.65_0.15_250)] hover:underline'
  return (
    <div className="space-y-4 md:space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>ARTEX-CC</CardTitle>
          <CardDescription>Autonomous penetration testing platform powered by Claude Code.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <InfoRow label="Version" value={data?.version && data.version !== '(devel)' ? data.version : 'dev build'} />
          <InfoRow label="License" value="AGPL-3.0-only" />
          <div className="flex gap-4 pt-1">
            <a className={linkClass} href={`${SOURCE_URL}#readme`} target="_blank" rel="noreferrer">Docs</a>
            <a className={linkClass} href={sourceUrlFor(data?.version)} target="_blank" rel="noreferrer">Source code</a>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>System</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {isLoading ? (
            <Skeleton className="h-5" />
          ) : error ? (
            <p className="text-sm text-[oklch(0.63_0.22_25)]">Failed to load system info: {(error as Error).message}</p>
          ) : (
            <>
              <InfoRow label="Go version" value={data?.go_version} />
              <InfoRow label="OS" value={data ? `${data.os}/${data.arch}` : undefined} />
              <InfoRow label="Data directory" value={data?.data_dir} />
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export default function Settings() {
  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border bg-muted px-3 md:px-4 py-3">
        <h1 className="text-lg font-semibold text-foreground">Settings</h1>
      </div>

      <div className="flex-1 overflow-y-auto p-3 md:p-6">
        <Tabs defaultValue="appearance" className="mx-auto max-w-3xl">
          <TabsList className="max-w-full overflow-x-auto">
            <TabsTrigger value="appearance">Appearance</TabsTrigger>
            <TabsTrigger value="llm">LLM</TabsTrigger>
            <TabsTrigger value="engagement">Engagement</TabsTrigger>
            <TabsTrigger value="about">About</TabsTrigger>
          </TabsList>
          <TabsContent value="appearance"><AppearanceTab /></TabsContent>
          <TabsContent value="llm"><LlmTab /></TabsContent>
          <TabsContent value="engagement"><EngagementTab /></TabsContent>
          <TabsContent value="about"><AboutTab /></TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
