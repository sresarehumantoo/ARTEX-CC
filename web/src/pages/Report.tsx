import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, Printer } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Badge, severityVariant } from '../components/ui/badge'
import { Spinner } from '../components/ui/spinner'
import { getEngagement, getFindings, getAssets, getActivity } from '../lib/api'
import { generateFullReport, markdownToHtml, wrapHtmlDocument } from '../lib/report'
import { downloadFile } from '../lib/export'
import { useToast } from '../hooks/useToast'
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
          <tr className="border-b border-slate-200 dark:border-slate-700">
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Severity</th>
            <th className="text-right py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Count</th>
          </tr>
        </thead>
        <tbody>
          {SEVERITY_ORDER.map(sev => (
            <tr key={sev} className="border-b border-slate-100 dark:border-slate-800">
              <td className="py-1.5 px-3">
                <Badge variant={severityVariant(sev)}>{sev}</Badge>
              </td>
              <td className="py-1.5 px-3 text-right font-medium text-slate-900 dark:text-slate-100">
                {counts[sev]}
              </td>
            </tr>
          ))}
          <tr className="font-semibold">
            <td className="py-1.5 px-3 text-slate-900 dark:text-slate-100">Total (active)</td>
            <td className="py-1.5 px-3 text-right text-slate-900 dark:text-slate-100">{active.length}</td>
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
          <tr className="border-b border-slate-200 dark:border-slate-700">
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Metric</th>
            <th className="text-right py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Value</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(([label, value]) => (
            <tr key={label} className="border-b border-slate-100 dark:border-slate-800">
              <td className="py-1.5 px-3 text-slate-600 dark:text-slate-400">{label}</td>
              <td className="py-1.5 px-3 text-right font-medium text-slate-900 dark:text-slate-100">{value}</td>
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
    <div className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 p-4">
      <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-200 mb-3">Executive Summary</h3>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <div className="text-center">
          <div className="text-2xl font-bold text-slate-900 dark:text-slate-100">{total}</div>
          <div className="text-xs text-slate-500">Active Findings</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-bold text-rose-500">{counts.critical + counts.high}</div>
          <div className="text-xs text-slate-500">Critical + High</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-bold text-slate-900 dark:text-slate-100">{assetCount}</div>
          <div className="text-xs text-slate-500">Assets</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-bold text-slate-900 dark:text-slate-100">{findings.length - total}</div>
          <div className="text-xs text-slate-500">Dismissed</div>
        </div>
      </div>
      {total > 0 && (
        <div className="mt-3 flex h-2 rounded-full overflow-hidden bg-slate-100 dark:bg-slate-800">
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
    return <p className="text-sm text-slate-500 dark:text-slate-400">No active findings.</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-slate-200 dark:border-slate-700">
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">#</th>
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Severity</th>
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Vuln Class</th>
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Summary</th>
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Status</th>
          </tr>
        </thead>
        <tbody>
          {active.map((f, i) => (
            <tr key={f.id} className="border-b border-slate-100 dark:border-slate-800">
              <td className="py-1.5 px-3 text-slate-500 dark:text-slate-400">{i + 1}</td>
              <td className="py-1.5 px-3">
                <Badge variant={severityVariant(f.severity)}>{f.severity}</Badge>
              </td>
              <td className="py-1.5 px-3 font-medium text-slate-900 dark:text-slate-100">{f.vulnclass}</td>
              <td className="py-1.5 px-3 text-slate-600 dark:text-slate-400 max-w-md">{f.summary}</td>
              <td className="py-1.5 px-3 text-slate-600 dark:text-slate-400">{f.status}</td>
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
    return <p className="text-sm text-slate-500 dark:text-slate-400">No assets discovered.</p>
  }

  return (
    <div className="space-y-4">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 dark:border-slate-700">
              <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Type</th>
              <th className="text-right py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Count</th>
            </tr>
          </thead>
          <tbody>
            {Array.from(byType).map(([type, list]) => (
              <tr key={type} className="border-b border-slate-100 dark:border-slate-800">
                <td className="py-1.5 px-3 text-slate-900 dark:text-slate-100">{type}</td>
                <td className="py-1.5 px-3 text-right font-medium text-slate-900 dark:text-slate-100">{list.length}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {Array.from(byType).map(([type, list]) => (
        <div key={type}>
          <h4 className="text-sm font-semibold text-slate-700 dark:text-slate-300 mb-1">{type}</h4>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 dark:border-slate-700">
                  <th className="text-left py-1.5 px-3 font-semibold text-slate-700 dark:text-slate-300">ID</th>
                  <th className="text-left py-1.5 px-3 font-semibold text-slate-700 dark:text-slate-300">Label</th>
                </tr>
              </thead>
              <tbody>
                {list.map(a => (
                  <tr key={a.id} className="border-b border-slate-100 dark:border-slate-800">
                    <td className="py-1 px-3 text-slate-500 dark:text-slate-400">{a.id}</td>
                    <td className="py-1 px-3 text-slate-900 dark:text-slate-100">{a.label}</td>
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
  const significant = activity.filter(a => a.kind !== 'noop' && a.summary.trim().length > 0).slice(0, 50)

  if (significant.length === 0) {
    return <p className="text-sm text-slate-500 dark:text-slate-400">No significant activity recorded.</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-slate-200 dark:border-slate-700">
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Time</th>
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Tool</th>
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Kind</th>
            <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Summary</th>
          </tr>
        </thead>
        <tbody>
          {significant.map(a => (
            <tr key={a.id} className="border-b border-slate-100 dark:border-slate-800">
              <td className="py-1.5 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {a.created_at ? a.created_at.split('T')[0] : ''}
              </td>
              <td className="py-1.5 px-3 text-slate-900 dark:text-slate-100">{a.tool}</td>
              <td className="py-1.5 px-3">
                <Badge variant={a.is_error ? 'critical' : 'default'}>{a.kind}</Badge>
              </td>
              <td className="py-1.5 px-3 text-slate-600 dark:text-slate-400 max-w-lg truncate">{a.summary}</td>
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
      <h3 className="text-base font-semibold text-slate-900 dark:text-slate-100 mb-2">Goals</h3>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 dark:border-slate-700">
              <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Goal</th>
              <th className="text-left py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">Status</th>
            </tr>
          </thead>
          <tbody>
            {engagement.goals.map(g => {
              const label = (g.payload.label as string) ?? (g.payload.description as string) ?? `Goal #${g.id}`
              return (
                <tr key={g.id} className="border-b border-slate-100 dark:border-slate-800">
                  <td className="py-1.5 px-3 text-slate-900 dark:text-slate-100">{label}</td>
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
  const { toast } = useToast()
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

  const engagement = engagementQ.data
  const findings = findingsQ.data ?? []
  const assets = assetsQ.data?.assets ?? []
  const activity = activityQ.data ?? []
  const now = new Date().toISOString().split('T')[0]

  function handleDownloadMarkdown() {
    if (!engagement) return
    const md = generateFullReport(engagement, findings, assets, activity)
    downloadFile(md, `engagement-report-${now}.md`, 'text/markdown')
    toast('Report downloaded as Markdown', { variant: 'success' })
  }

  function handleDownloadHtml() {
    if (!engagement) return
    const md = generateFullReport(engagement, findings, assets, activity)
    const bodyHtml = markdownToHtml(md)
    const html = wrapHtmlDocument(bodyHtml, 'Engagement Report')
    downloadFile(html, `engagement-report-${now}.html`, 'text/html')
    toast('Report downloaded as HTML', { variant: 'success' })
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spinner className="h-8 w-8" />
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
        <h1 className="text-lg md:text-2xl font-semibold text-slate-900 dark:text-slate-100">
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

      <div className="report-preview rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-4 md:p-8 space-y-8">
        {/* Title Block */}
        <div className="border-b border-slate-200 dark:border-slate-700 pb-6">
          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">Engagement Report</h1>
          {engagement?.goal && (
            <p className="mt-2 text-lg text-slate-700 dark:text-slate-300">{engagement.goal}</p>
          )}
          {engagement?.description && (
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{engagement.description}</p>
          )}
          <p className="mt-2 text-xs text-slate-400 dark:text-slate-500">Generated: {now}</p>
        </div>

        {/* Executive Summary */}
        <section>
          <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100 mb-4">Executive Summary</h2>
          <div className="grid gap-6 md:grid-cols-2">
            <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
              <div className="bg-slate-50 dark:bg-slate-800 px-3 py-2">
                <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-300">Findings by Severity</h3>
              </div>
              <SeverityTable findings={findings} />
            </div>
            {engagement && (
              <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
                <div className="bg-slate-50 dark:bg-slate-800 px-3 py-2">
                  <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-300">Key Statistics</h3>
                </div>
                <StatsTable engagement={engagement} assets={assets} />
              </div>
            )}
          </div>
        </section>

        {/* Findings */}
        <section>
          <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100 mb-4">Findings</h2>
          <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
            <FindingsTable findings={findings} />
          </div>
        </section>

        {/* Asset Inventory */}
        <section>
          <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100 mb-4">Asset Inventory</h2>
          <div className="rounded-lg border border-slate-200 dark:border-slate-700 p-3 md:p-4 overflow-hidden">
            <AssetInventory assets={assets} />
          </div>
        </section>

        {/* Coverage and Methodology */}
        {engagement && (
          <section>
            <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100 mb-4">Coverage and Methodology</h2>
            <div className="rounded-lg border border-slate-200 dark:border-slate-700 p-3 md:p-4 space-y-4">
              <GoalsTable engagement={engagement} />
              {engagement.hints.length > 0 && (
                <div>
                  <h3 className="text-base font-semibold text-slate-900 dark:text-slate-100 mb-2">Hints</h3>
                  <ul className="list-disc pl-5 text-sm text-slate-600 dark:text-slate-400 space-y-1">
                    {engagement.hints.map(h => {
                      const text = (h.payload.text as string) ?? (h.payload.label as string) ?? `Hint #${h.id}`
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
          <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100 mb-4">Activity Summary</h2>
          <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
            <ActivitySummary activity={activity} />
          </div>
        </section>
      </div>
    </div>
  )
}
