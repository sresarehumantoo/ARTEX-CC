import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, Printer } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Badge, severityVariant } from '../components/ui/badge'

import { getEngagement, getFindings, getAssets, getActivity } from '../lib/api'
import { generateFullReport, markdownToHtml, wrapHtmlDocument } from '../lib/report'
import { downloadFile } from '../lib/export'
import { toast } from 'sonner'
import type { Activity, Asset, EngagementData, Finding } from '../lib/types'

const SEVERITY_ORDER = ['critical', 'high', 'medium', 'low', 'info'] as const

function sortBySeverity(findings: Finding[]): Finding[] {
  return [...findings].sort(
    (a, b) => SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity),
  )
}

function SeverityTable({ findings }: { findings: Finding[] }) {
  const active = findings.filter(f => !f.dismissed && !f.merged_into)
  const counts = {
    critical: active.filter(f => f.severity === 'critical').length,
    high: active.filter(f => f.severity === 'high').length,
    medium: active.filter(f => f.severity === 'medium').length,
    low: active.filter(f => f.severity === 'low').length,
    info: active.filter(f => f.severity === 'info').length,
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border">
            <th className="text-left py-2 px-3 font-semibold text-foreground">Severity</th>
            <th className="text-right py-2 px-3 font-semibold text-foreground">Count</th>
          </tr>
        </thead>
        <tbody>
          {SEVERITY_ORDER.map(sev => (
            <tr key={sev} className="border-b border-border">
              <td className="py-1.5 px-3">
                <Badge variant={severityVariant(sev)}>{sev}</Badge>
              </td>
              <td className="py-1.5 px-3 text-right font-medium text-foreground">
                {counts[sev]}
              </td>
            </tr>
          ))}
          <tr className="font-semibold">
            <td className="py-1.5 px-3 text-foreground">Total (active)</td>
            <td className="py-1.5 px-3 text-right text-foreground">{active.length}</td>
          </tr>
        </tbody>
      </table>
    </div>
  )
}

function StatsTable({ engagement, assets }: { engagement: EngagementData; assets: Asset[] }) {
  const goalsProven = engagement.goals.filter(g => g.state === 'proven').length
  const intentsTotal = engagement.done_intents_total + engagement.open_intents.length + engagement.running_intents.length

  const rows = [
    ['Assets discovered', String(assets.length)],
    ['Goals proven / total', `${goalsProven} / ${engagement.goals.length}`],
    ['Intents completed / total', `${engagement.done_intents_total} / ${intentsTotal}`],
    ['Open intents', String(engagement.open_intents.length)],
    ['Running intents', String(engagement.running_intents.length)],
  ]

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border">
            <th className="text-left py-2 px-3 font-semibold text-foreground">Metric</th>
            <th className="text-right py-2 px-3 font-semibold text-foreground">Value</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(([label, value]) => (
            <tr key={label} className="border-b border-border">
              <td className="py-1.5 px-3 text-muted-foreground">{label}</td>
              <td className="py-1.5 px-3 text-right font-medium text-foreground">{value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function ExecutiveSummary({ findings, assetCount }: { findings: Finding[]; assetCount: number }) {
  const active = findings.filter(f => !f.dismissed && !f.merged_into)
  const counts = {
    critical: active.filter(f => f.severity === 'critical').length,
    high: active.filter(f => f.severity === 'high').length,
    medium: active.filter(f => f.severity === 'medium').length,
    low: active.filter(f => f.severity === 'low').length,
    info: active.filter(f => f.severity === 'info').length,
  }
  const total = active.length

  const SEVERITY_COLORS: Record<string, string> = {
    critical: 'bg-rose-500',
    high: 'bg-orange-500',
    medium: 'bg-amber-500',
    low: 'bg-blue-500',
    info: 'bg-slate-400',
  }

  return (
    <div className="rounded-lg border border-border bg-background p-4">
      <h3 className="text-sm font-semibold text-foreground mb-3">Executive Summary</h3>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <div className="text-center">
          <div className="text-2xl font-bold text-foreground">{total}</div>
          <div className="text-xs text-muted-foreground">Active Findings</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-bold text-rose-500">{counts.critical + counts.high}</div>
          <div className="text-xs text-muted-foreground">Critical + High</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-bold text-foreground">{assetCount}</div>
          <div className="text-xs text-muted-foreground">Assets</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-bold text-foreground">{findings.length - total}</div>
          <div className="text-xs text-muted-foreground">Dismissed</div>
        </div>
      </div>
      {total > 0 && (
        <div className="mt-3 flex h-2 rounded-full overflow-hidden bg-muted">
          {(['critical', 'high', 'medium', 'low', 'info'] as const).map(sev => {
            const pct = (counts[sev] / total) * 100
            if (pct === 0) return null
            return <div key={sev} className={`${SEVERITY_COLORS[sev]}`} style={{ width: `${pct}%` }} />
          })}
        </div>
      )}
    </div>
  )
}

function FindingsTable({ findings }: { findings: Finding[] }) {
  const active = sortBySeverity(findings.filter(f => !f.dismissed && !f.merged_into))

  if (active.length === 0) {
    return <p className="text-sm text-muted-foreground">No active findings.</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border">
            <th className="text-left py-2 px-3 font-semibold text-foreground">#</th>
            <th className="text-left py-2 px-3 font-semibold text-foreground">Severity</th>
            <th className="text-left py-2 px-3 font-semibold text-foreground">Vuln Class</th>
            <th className="text-left py-2 px-3 font-semibold text-foreground">Summary</th>
            <th className="text-left py-2 px-3 font-semibold text-foreground">Status</th>
          </tr>
        </thead>
        <tbody>
          {active.map((f, i) => (
            <tr key={f.id} className="border-b border-border">
              <td className="py-1.5 px-3 text-muted-foreground">{i + 1}</td>
              <td className="py-1.5 px-3">
                <Badge variant={severityVariant(f.severity)}>{f.severity}</Badge>
              </td>
              <td className="py-1.5 px-3 font-medium text-foreground">{f.vulnclass}</td>
              <td className="py-1.5 px-3 text-muted-foreground max-w-md">{f.summary}</td>
              <td className="py-1.5 px-3 text-muted-foreground">{f.status}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function AssetInventory({ assets }: { assets: Asset[] }) {
  const byType = useMemo(() => {
    const map = new Map<string, Asset[]>()
    for (const a of assets) {
      const list = map.get(a.type) ?? []
      list.push(a)
      map.set(a.type, list)
    }
    return map
  }, [assets])

  if (assets.length === 0) {
    return <p className="text-sm text-muted-foreground">No assets discovered.</p>
  }

  return (
    <div className="space-y-4">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border">
              <th className="text-left py-2 px-3 font-semibold text-foreground">Type</th>
              <th className="text-right py-2 px-3 font-semibold text-foreground">Count</th>
            </tr>
          </thead>
          <tbody>
            {Array.from(byType).map(([type, list]) => (
              <tr key={type} className="border-b border-border">
                <td className="py-1.5 px-3 text-foreground">{type}</td>
                <td className="py-1.5 px-3 text-right font-medium text-foreground">{list.length}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {Array.from(byType).map(([type, list]) => (
        <div key={type}>
          <h4 className="text-sm font-semibold text-foreground mb-1">{type}</h4>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-1.5 px-3 font-semibold text-foreground">ID</th>
                  <th className="text-left py-1.5 px-3 font-semibold text-foreground">Label</th>
                </tr>
              </thead>
              <tbody>
                {list.map(a => (
                  <tr key={a.id} className="border-b border-border">
                    <td className="py-1 px-3 text-muted-foreground">{a.id}</td>
                    <td className="py-1 px-3 text-foreground">{a.label}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ))}
    </div>
  )
}

function ActivitySummary({ activity }: { activity: Activity[] }) {
  const significant = activity.filter(a => a.kind !== 'noop' && (a.summary ?? '').trim().length > 0).slice(0, 50)

  if (significant.length === 0) {
    return <p className="text-sm text-muted-foreground">No significant activity recorded.</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border">
            <th className="text-left py-2 px-3 font-semibold text-foreground">Time</th>
            <th className="text-left py-2 px-3 font-semibold text-foreground">Tool</th>
            <th className="text-left py-2 px-3 font-semibold text-foreground">Kind</th>
            <th className="text-left py-2 px-3 font-semibold text-foreground">Summary</th>
          </tr>
        </thead>
        <tbody>
          {significant.map(a => (
            <tr key={a.id} className="border-b border-border">
              <td className="py-1.5 px-3 text-muted-foreground whitespace-nowrap">
                {a.created_at ? a.created_at.split('T')[0] : ''}
              </td>
              <td className="py-1.5 px-3 text-foreground">{a.tool}</td>
              <td className="py-1.5 px-3">
                <Badge variant={a.is_error ? 'critical' : 'default'}>{a.kind}</Badge>
              </td>
              <td className="py-1.5 px-3 text-muted-foreground max-w-lg truncate">{a.summary}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function GoalsTable({ engagement }: { engagement: EngagementData }) {
  if (engagement.goals.length === 0) return null

  return (
    <div>
      <h3 className="text-base font-semibold text-foreground mb-2">Goals</h3>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border">
              <th className="text-left py-2 px-3 font-semibold text-foreground">Goal</th>
              <th className="text-left py-2 px-3 font-semibold text-foreground">Status</th>
            </tr>
          </thead>
          <tbody>
            {engagement.goals.map(g => {
              const label = (g as any).label || (g.payload?.label as string) || (g.payload?.description as string) || `Goal #${g.id}`
              return (
                <tr key={g.id} className="border-b border-border">
                  <td className="py-1.5 px-3 text-foreground">{label}</td>
                  <td className="py-1.5 px-3">
                    <Badge variant={g.state === 'proven' ? 'get' : 'default'}>{g.state}</Badge>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default function Report() {
  const engagementQ = useQuery({ queryKey: ['engagement'], queryFn: getEngagement })
  const findingsQ = useQuery({ queryKey: ['findings'], queryFn: getFindings })
  const assetsQ = useQuery({
    queryKey: ['assets', 'report'],
    queryFn: () => getAssets({ limit: '1000' }),
  })
  const activityQ = useQuery({
    queryKey: ['activity', 'report'],
    queryFn: () => getActivity({ limit: '200' }),
  })

  const isLoading = engagementQ.isLoading || findingsQ.isLoading || assetsQ.isLoading || activityQ.isLoading
  const isError = engagementQ.isError || findingsQ.isError || assetsQ.isError || activityQ.isError

  const rawEngagement = engagementQ.data
  const engagement = rawEngagement ? {
    ...rawEngagement,
    running_intents: rawEngagement.running_intents ?? [],
    open_intents: rawEngagement.open_intents ?? [],
    goals: rawEngagement.goals ?? [],
    finding_list: rawEngagement.finding_list ?? [],
    recent_facts: rawEngagement.recent_facts ?? [],
    hints: rawEngagement.hints ?? [],
    task_credentials: rawEngagement.task_credentials ?? [],
  } : undefined
  const findings = findingsQ.data ?? []
  const assets = assetsQ.data?.assets ?? []
  const activity = activityQ.data ?? []
  const now = new Date().toISOString().split('T')[0]

  function handleDownloadMarkdown() {
    if (!engagement) return
    const md = generateFullReport(engagement, findings, assets, activity)
    downloadFile(md, `engagement-report-${now}.md`, 'text/markdown')
    toast.success('Report downloaded as Markdown')
  }

  function handleDownloadHtml() {
    if (!engagement) return
    const md = generateFullReport(engagement, findings, assets, activity)
    const bodyHtml = markdownToHtml(md)
    const html = wrapHtmlDocument(bodyHtml, 'Engagement Report')
    downloadFile(html, `engagement-report-${now}.html`, 'text/html')
    toast.success('Report downloaded as HTML')
  }

  if (isLoading) {
    return (
      <div className="p-6 space-y-6 animate-pulse">
        <div className="h-6 w-48 rounded bg-muted" />
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {[1, 2, 3, 4].map(i => (
            <div key={i} className="h-16 rounded-lg bg-muted" />
          ))}
        </div>
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-20 rounded-lg bg-muted" />
          ))}
        </div>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6">
        <p className="text-sm text-rose-400">
          Failed to load report data. Check that all API endpoints are reachable.
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-6 p-3 md:p-6">
      <div className="no-print flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between print:hidden">
        <h1 className="text-lg md:text-2xl font-semibold text-foreground">
          Engagement Report
        </h1>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={handleDownloadMarkdown} disabled={!engagement}>
            <Download className="h-4 w-4 mr-1.5" />
            Download Markdown
          </Button>
          <Button variant="outline" size="sm" onClick={handleDownloadHtml} disabled={!engagement}>
            <Download className="h-4 w-4 mr-1.5" />
            Download HTML
          </Button>
          <Button variant="outline" size="sm" onClick={() => window.print()}>
            <Printer className="h-4 w-4 mr-1.5" />
            Print
          </Button>
        </div>
      </div>

      {findings.length > 0 && (
        <ExecutiveSummary findings={findings} assetCount={assets.length} />
      )}

      <div className="report-preview rounded-lg border border-border bg-background p-4 md:p-8 space-y-8">
        {/* Title Block */}
        <div className="border-b border-border pb-6">
          <h1 className="text-2xl font-bold text-foreground">Engagement Report</h1>
          {engagement?.goal && (
            <p className="mt-2 text-lg text-foreground">{engagement.goal}</p>
          )}
          {engagement?.description && (
            <p className="mt-1 text-sm text-muted-foreground">{engagement.description}</p>
          )}
          <p className="mt-2 text-xs text-muted-foreground">Generated: {now}</p>
        </div>

        {/* Executive Summary */}
        <section>
          <h2 className="text-xl font-semibold text-foreground mb-4">Executive Summary</h2>
          <div className="grid gap-6 md:grid-cols-2">
            <div className="rounded-lg border border-border overflow-hidden">
              <div className="bg-muted dark:bg-slate-800 px-3 py-2">
                <h3 className="text-sm font-semibold text-foreground">Findings by Severity</h3>
              </div>
              <SeverityTable findings={findings} />
            </div>
            {engagement && (
              <div className="rounded-lg border border-border overflow-hidden">
                <div className="bg-muted dark:bg-slate-800 px-3 py-2">
                  <h3 className="text-sm font-semibold text-foreground">Key Statistics</h3>
                </div>
                <StatsTable engagement={engagement} assets={assets} />
              </div>
            )}
          </div>
        </section>

        {/* Findings */}
        <section>
          <h2 className="text-xl font-semibold text-foreground mb-4">Findings</h2>
          <div className="rounded-lg border border-border overflow-hidden">
            <FindingsTable findings={findings} />
          </div>
        </section>

        {/* Asset Inventory */}
        <section>
          <h2 className="text-xl font-semibold text-foreground mb-4">Asset Inventory</h2>
          <div className="rounded-lg border border-border p-3 md:p-4 overflow-hidden">
            <AssetInventory assets={assets} />
          </div>
        </section>

        {/* Coverage and Methodology */}
        {engagement && (
          <section>
            <h2 className="text-xl font-semibold text-foreground mb-4">Coverage and Methodology</h2>
            <div className="rounded-lg border border-border p-3 md:p-4 space-y-4">
              <GoalsTable engagement={engagement} />
              {engagement.hints.length > 0 && (
                <div>
                  <h3 className="text-base font-semibold text-foreground mb-2">Hints</h3>
                  <ul className="list-disc pl-5 text-sm text-muted-foreground space-y-1">
                    {engagement.hints.map(h => {
                      const text = (h as any).label || (h as any).text || (h.payload?.text as string) || (h.payload?.label as string) || `Hint #${h.id}`
                      return <li key={h.id}>{text}</li>
                    })}
                  </ul>
                </div>
              )}
            </div>
          </section>
        )}

        {/* Activity Summary */}
        <section>
          <h2 className="text-xl font-semibold text-foreground mb-4">Activity Summary</h2>
          <div className="rounded-lg border border-border overflow-hidden">
            <ActivitySummary activity={activity} />
          </div>
        </section>
      </div>
    </div>
  )
}
