import { ChevronDown, ChevronUp, Copy, ExternalLink, GitBranch, Search, Shield, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ExportDropdown } from '../components/ExportDropdown'
import FindingsTimeline from '../components/FindingsTimeline'
import { StatusBadge } from '../components/StatusBadge'
import { Badge, type BadgeVariant } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { ScrollArea } from '../components/ui/scroll-area'
import { Select } from '../components/ui/select'
import { Skeleton } from '../components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { TableSkeleton } from '../components/skeletons/TableSkeleton'
import { useFindingDetail, useFindings } from '../hooks/useFindings'
import { toast } from 'sonner'
import { exportCSV, exportJSON } from '../lib/export'
import { generateReport } from '../lib/report'
import type { Finding, Severity } from '../lib/types'
import { timeAgo } from '../lib/format'
import { cn } from '../lib/utils'

const SEVERITY_ORDER: Record<Severity, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
  info: 4,
}

const SEVERITY_VARIANT: Record<Severity, BadgeVariant> = {
  critical: 'critical',
  high: 'high',
  medium: 'medium',
  low: 'low',
  info: 'info',
}

const SEVERITY_BAR_CONFIG: Record<string, { color: string; bg: string; label: string; order: number }> = {
  critical: { color: 'bg-rose-600', bg: 'bg-rose-500/10', label: 'Critical', order: 0 },
  high:     { color: 'bg-orange-500', bg: 'bg-orange-500/10', label: 'High', order: 1 },
  medium:   { color: 'bg-amber-500', bg: 'bg-amber-500/10', label: 'Medium', order: 2 },
  low:      { color: 'bg-blue-500', bg: 'bg-blue-500/10', label: 'Low', order: 3 },
  info:     { color: 'bg-slate-400', bg: 'bg-slate-400/10', label: 'Info', order: 4 },
}

function SeverityBar({ findings }: { findings: Finding[] }) {
  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const f of findings) {
      c[f.severity] = (c[f.severity] ?? 0) + 1
    }
    return c
  }, [findings])

  const total = findings.length
  if (total === 0) return null

  const segments = Object.entries(SEVERITY_BAR_CONFIG)
    .filter(([sev]) => (counts[sev] ?? 0) > 0)
    .sort(([, a], [, b]) => a.order - b.order)
    .map(([sev, config]) => ({
      severity: sev,
      count: counts[sev] ?? 0,
      pct: ((counts[sev] ?? 0) / total) * 100,
      ...config,
    }))

  return (
    <div className="border-b border-border px-3 md:px-4 py-2">
      <div className="flex h-2 rounded-full overflow-hidden bg-muted mb-2">
        {segments.map(s => (
          <div
            key={s.severity}
            className={cn(s.color, 'transition-all duration-300')}
            style={{ width: `${s.pct}%` }}
            title={`${s.label}: ${s.count}`}
          />
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
        <span className="text-muted-foreground font-medium">{total} total</span>
        <span className="text-slate-300 dark:text-slate-700">|</span>
        {segments.map(s => (
          <span key={s.severity} className="flex items-center gap-1.5">
            <span className={cn('h-2 w-2 rounded-full', s.color)} />
            <span className="text-muted-foreground">{s.count} {s.label.toLowerCase()}</span>
          </span>
        ))}
      </div>
    </div>
  )
}

type SortField = 'severity' | 'id'
type SortDir = 'asc' | 'desc'

export default function Findings() {
  const { data: findings = [], isLoading, error, refetch } = useFindings()
  const [searchParams, setSearchParams] = useSearchParams()
  const [severityFilter, setSeverityFilter] = useState<Severity | null>(() => {
    const param = searchParams.get('severity')
    if (param && param in SEVERITY_ORDER) return param as Severity
    return null
  })
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')
  const [sortField, setSortField] = useState<SortField>(() => {
    const param = searchParams.get('sort')
    if (param === 'severity' || param === 'id') return param
    return 'severity'
  })
  const [sortDir, setSortDir] = useState<SortDir>(() => {
    const param = searchParams.get('dir')
    if (param === 'asc' || param === 'desc') return param
    return 'asc'
  })
  const [selectedId, setSelectedId] = useState<number | null>(() => {
    const idParam = searchParams.get('id')
    if (idParam) {
      const parsed = Number(idParam)
      if (Number.isFinite(parsed)) return parsed
    }
    return null
  })
  const [showTimeline, setShowTimeline] = useState(true)
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())

  useEffect(() => {
    setSelectedIds(new Set())
  }, [severityFilter, statusFilter, search])

  useEffect(() => {
    const timer = setTimeout(() => {
      const next = new URLSearchParams()
      if (selectedId !== null) next.set('id', String(selectedId))
      if (sortField !== 'severity') next.set('sort', sortField)
      if (sortDir !== 'asc') next.set('dir', sortDir)
      if (severityFilter) next.set('severity', severityFilter)
      setSearchParams(next, { replace: true })
    }, 300)
    return () => clearTimeout(timer)
  }, [selectedId, sortField, sortDir, severityFilter, setSearchParams])

  const { data: detail, isLoading: detailLoading } = useFindingDetail(selectedId)

  const counts = useMemo(() => {
    const c = { total: 0, critical: 0, high: 0, medium: 0, low: 0 }
    for (const f of findings) {
      c.total++
      if (f.severity in c) (c as Record<string, number>)[f.severity]++
    }
    return c
  }, [findings])

  const filtered = useMemo(() => {
    let list = findings
    if (severityFilter) list = list.filter(f => f.severity === severityFilter)
    if (statusFilter) {
      const s = statusFilter.toLowerCase()
      list = list.filter(f => {
        if (s === 'dismissed') return f.dismissed
        return f.status.toLowerCase() === s
      })
    }
    if (search) {
      const q = search.toLowerCase()
      list = list.filter(
        f => f.vulnclass.toLowerCase().includes(q) || f.summary.toLowerCase().includes(q),
      )
    }
    const dir = sortDir === 'asc' ? 1 : -1
    return [...list].sort((a, b) => {
      if (sortField === 'severity') return dir * (SEVERITY_ORDER[a.severity] - SEVERITY_ORDER[b.severity])
      return dir * (a.id - b.id)
    })
  }, [findings, severityFilter, statusFilter, search, sortField, sortDir])

  const toggleSort = useCallback(
    (field: SortField) => {
      if (sortField === field) setSortDir(d => (d === 'asc' ? 'desc' : 'asc'))
      else {
        setSortField(field)
        setSortDir('asc')
      }
    },
    [sortField],
  )

  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement).tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return
      if (e.key === 'Escape' && selectedId !== null) {
        setSelectedId(null)
        return
      }
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        if (filtered.length === 0) return
        e.preventDefault()
        const currentIdx = selectedId !== null ? filtered.findIndex(f => f.id === selectedId) : -1
        let nextIdx: number
        if (e.key === 'ArrowDown') {
          nextIdx = currentIdx < filtered.length - 1 ? currentIdx + 1 : currentIdx
        } else {
          nextIdx = currentIdx > 0 ? currentIdx - 1 : 0
        }
        const nextId = filtered[nextIdx].id
        setSelectedId(nextId)
        document.querySelector(`[data-row-id="${nextId}"]`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
      }
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [selectedId, filtered])

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortField !== field) return null
    return sortDir === 'asc' ? (
      <ChevronUp className="ml-1 inline h-3 w-3" />
    ) : (
      <ChevronDown className="ml-1 inline h-3 w-3" />
    )
  }

  if (isLoading) {
    return (
      <div className="flex h-full flex-col gap-3 md:gap-4 p-3 md:p-6">
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="rounded-lg border border-border p-6 space-y-3">
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-9 w-12" />
            </div>
          ))}
        </div>
        <TableSkeleton columns={5} rows={8} filterBar />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-96 items-center justify-center">
        <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
          <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load findings: {(error as Error).message}</p>
          <button
            type="button"
            onClick={() => refetch()}
            className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
          >
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col gap-3 md:gap-4 p-3 md:p-6">
      {/* Summary cards */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
        <SummaryCard
          label="Total"
          count={counts.total}
          active={severityFilter === null}
          onClick={() => setSeverityFilter(null)}
        />
        <SummaryCard
          label="Critical"
          count={counts.critical}
          variant="critical"
          active={severityFilter === 'critical'}
          onClick={() => setSeverityFilter(severityFilter === 'critical' ? null : 'critical')}
        />
        <SummaryCard
          label="High"
          count={counts.high}
          variant="high"
          active={severityFilter === 'high'}
          onClick={() => setSeverityFilter(severityFilter === 'high' ? null : 'high')}
        />
        <SummaryCard
          label="Medium"
          count={counts.medium}
          variant="medium"
          active={severityFilter === 'medium'}
          onClick={() => setSeverityFilter(severityFilter === 'medium' ? null : 'medium')}
        />
        <SummaryCard
          label="Low"
          count={counts.low}
          variant="low"
          active={severityFilter === 'low'}
          onClick={() => setSeverityFilter(severityFilter === 'low' ? null : 'low')}
        />
      </div>

      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-2 md:gap-3">
        <div className="flex items-center gap-1">
          {(['critical', 'high', 'medium', 'low'] as Severity[]).map(s => (
            <Button
              key={s}
              variant={severityFilter === s ? 'outline' : 'ghost'}
              size="sm"
              className={severityFilter === s ? severityRingClass(s) : ''}
              onClick={() => setSeverityFilter(severityFilter === s ? null : s)}
            >
              {s.charAt(0).toUpperCase() + s.slice(1)}
            </Button>
          ))}
          {severityFilter && (
            <Button variant="ghost" size="sm" onClick={() => setSeverityFilter(null)}>
              All
            </Button>
          )}
        </div>

        <Select
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value)}
          className="w-36"
        >
          <option value="">All statuses</option>
          <option value="confirmed">Confirmed</option>
          <option value="pending">Pending</option>
          <option value="dismissed">Dismissed</option>
        </Select>

        <div className="relative w-full md:w-64 md:ml-auto">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search vulnclass or summary..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="pl-8"
          />
          {search && (
            <button
              type="button"
              onClick={() => setSearch('')}
              className="absolute right-2.5 top-2.5 text-muted-foreground hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        <ExportDropdown options={[
          { label: 'Export CSV', onClick: () => { exportCSV(filtered, 'findings.csv'); toast.success('Exported as CSV') } },
          { label: 'Export JSON', onClick: () => { exportJSON(filtered, 'findings.json'); toast.success('Exported as JSON') } },
          {
            label: 'Generate Report',
            onClick: () => {
              const md = generateReport(filtered)
              const blob = new Blob([md], { type: 'text/markdown' })
              const url = URL.createObjectURL(blob)
              const a = document.createElement('a')
              a.href = url
              a.download = 'findings-report.md'
              a.click()
              URL.revokeObjectURL(url)
            },
          },
        ]} />
      </div>

      <SeverityBar findings={findings} />

      {filtered.length >= 2 && (
        <div>
          <button
            type="button"
            onClick={() => setShowTimeline(v => !v)}
            className="flex items-center gap-1.5 text-sm font-medium text-slate-600 hover:text-foreground  dark:hover:text-slate-200"
          >
            <ChevronDown className={cn('h-4 w-4 transition-transform', !showTimeline && '-rotate-90')} />
            <span>Timeline</span>
          </button>
          {showTimeline && <FindingsTimeline findings={filtered} className="mt-2 h-28" />}
        </div>
      )}

      {/* Main content area */}
      <div className="flex min-h-0 flex-1 gap-4">
        {/* Table */}
        <div className="min-h-0 flex-1">
          {isLoading ? (
            <TableSkeleton rows={8} columns={4} />
          ) : filtered.length === 0 ? (
            <div className="flex h-64 flex-col items-center justify-center gap-3 text-muted-foreground">
              <Shield className="h-12 w-12 text-slate-300 dark:text-slate-700" />
              <div className="text-center">
                <p className="text-lg font-medium text-muted-foreground">No findings discovered</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  {findings.length > 0
                    ? 'Try adjusting your filters'
                    : 'Findings will appear as vulnerabilities are discovered'}
                </p>
              </div>
            </div>
          ) : (
            <>
            {selectedIds.size > 0 && (
              <div className="flex items-center gap-3 rounded-lg border border-border bg-background px-3 py-2 mb-2">
                <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
                  {selectedIds.size} selected
                </span>
                <button
                  type="button"
                  onClick={() => {
                    const ids = [...selectedIds].sort((a, b) => a - b).join(', ')
                    navigator.clipboard.writeText(ids)
                    toast.success('Finding IDs copied')
                  }}
                  className="text-xs text-muted-foreground hover:text-foreground font-medium"
                >
                  Copy IDs
                </button>
                <button
                  type="button"
                  onClick={() => {
                    const csv = filtered.filter(f => selectedIds.has(f.id))
                    exportCSV(csv, 'selected-findings.csv')
                    toast.success('Exported selected findings')
                  }}
                  className="text-xs text-muted-foreground hover:text-foreground font-medium"
                >
                  Export selected
                </button>
                <button
                  type="button"
                  onClick={() => setSelectedIds(new Set())}
                  className="ml-auto text-xs text-muted-foreground hover:text-foreground"
                >
                  Clear
                </button>
              </div>
            )}
            <ScrollArea className="h-full">
              <div className="overflow-x-auto">
                <Table className="min-w-[600px]">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-8 px-3 py-2">
                        <input
                          type="checkbox"
                          checked={filtered.length > 0 && selectedIds.size === filtered.length}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setSelectedIds(new Set(filtered.map(f => f.id)))
                            } else {
                              setSelectedIds(new Set())
                            }
                          }}
                          className="rounded border-border"
                        />
                      </TableHead>
                      <TableHead className="w-24 cursor-pointer select-none" onClick={() => toggleSort('severity')}>
                        Severity <SortIcon field="severity" />
                      </TableHead>
                      <TableHead>Vuln Class</TableHead>
                      <TableHead className="max-w-md">Summary</TableHead>
                      <TableHead className="w-24">Status</TableHead>
                      <TableHead className="w-16 cursor-pointer select-none text-right" onClick={() => toggleSort('id')}>
                        ID <SortIcon field="id" />
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filtered.map((f, i) => (
                      <TableRow
                        key={f.id}
                        data-row-id={f.id}
                        className={cn(
                          'cursor-pointer',
                          selectedId === f.id && 'bg-muted',
                          i % 2 === 1 && 'bg-muted',
                        )}
                        onClick={() => setSelectedId(selectedId === f.id ? null : f.id)}
                      >
                        <TableCell className="w-8 px-3 py-2" onClick={e => e.stopPropagation()}>
                          <input
                            type="checkbox"
                            checked={selectedIds.has(f.id)}
                            onChange={() => {
                              setSelectedIds(prev => {
                                const next = new Set(prev)
                                if (next.has(f.id)) next.delete(f.id)
                                else next.add(f.id)
                                return next
                              })
                            }}
                            className="rounded border-border"
                          />
                        </TableCell>
                        <TableCell>
                          <Badge variant={SEVERITY_VARIANT[f.severity]}>{f.severity}</Badge>
                        </TableCell>
                        <TableCell className="max-w-[150px] truncate text-xs font-medium text-foreground">{f.vulnclass}</TableCell>
                        <TableCell className="max-w-md truncate text-xs text-muted-foreground">{f.summary}</TableCell>
                        <TableCell>
                          <StatusText finding={f} />
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs tabular-nums text-muted-foreground">{f.id}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </ScrollArea>
            </>
          )}
        </div>

        {/* Detail panel: full-screen overlay on mobile, side panel on md+ */}
        {selectedId !== null && (
          <div className={cn(
            'fixed inset-0 z-40 overflow-y-auto bg-background border-l-4',
            'md:static md:inset-auto md:z-auto md:w-96 md:shrink-0 md:overflow-hidden md:rounded-lg md:border md:border-l-4 md:border-slate-200 md:dark:border-slate-800',
            detail ? severityRingClass(detail.severity) : 'border-slate-300',
          )}>
            <DetailPanel
              detail={detail ?? null}
              loading={detailLoading}
              onClose={() => setSelectedId(null)}
            />
          </div>
        )}
      </div>
    </div>
  )
}

function SummaryCard({
  label,
  count,
  variant,
  active,
  onClick,
}: {
  label: string
  count: number
  variant?: BadgeVariant
  active: boolean
  onClick: () => void
}) {
  const colorText: Record<string, string> = {
    critical: 'text-rose-500',
    high: 'text-orange-500',
    medium: 'text-amber-500',
    low: 'text-blue-500',
  }
  return (
    <Card
      className={cn(
        'cursor-pointer transition-colors hover:border-slate-700',
        active && 'border-slate-400 dark:border-slate-600 bg-muted',
      )}
      onClick={onClick}
    >
      <CardHeader>
        <CardTitle>{label}</CardTitle>
      </CardHeader>
      <CardContent>
        <span className={cn('text-3xl font-bold', variant ? colorText[variant] : 'text-foreground')}>
          {count}
        </span>
      </CardContent>
    </Card>
  )
}

function StatusText({ finding }: { finding: Finding }) {
  if (finding.dismissed) return <StatusBadge domain="finding" value="ignored" />
  return <StatusBadge domain="finding" value={(finding.status || finding.state || '').toLowerCase()} />
}

function DetailPanel({
  detail,
  loading,
  onClose,
}: {
  detail: import('../lib/types').FindingDetail | null
  loading: boolean
  onClose: () => void
}) {

  if (loading || !detail) {
    return (
      <div className="p-4 space-y-4 animate-pulse">
        <div className="flex items-center gap-2">
          <div className="h-5 w-16 rounded-full bg-muted" />
          <div className="h-5 w-24 rounded bg-muted" />
        </div>
        <div className="h-4 w-3/4 rounded bg-muted" />
        <div className="h-4 w-full rounded bg-muted" />
        <div className="h-4 w-5/6 rounded bg-muted" />
        <div className="mt-4 h-20 rounded bg-muted" />
        <div className="h-4 w-1/2 rounded bg-muted" />
      </div>
    )
  }

  return (
    <ScrollArea className="h-full">
      <div className="flex flex-col gap-4 p-4">
        <div className="flex items-start justify-between">
          <Badge variant={SEVERITY_VARIANT[detail.severity]} className="text-sm">
            {detail.severity}
          </Badge>
          <button
            type="button"
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div>
          <h2 className="text-lg font-semibold text-foreground">{detail.vulnclass}</h2>
          <div className="mt-1 flex items-center gap-1.5">
            <span className="text-xs text-muted-foreground">Finding #{detail.id}</span>
            <button
              type="button"
              onClick={() => {
                navigator.clipboard.writeText(String(detail.id))
                toast.success('Finding ID copied')
              }}
              className="p-1 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
              title="Copy finding ID"
            >
              <Copy className="h-3 w-3" />
            </button>
          </div>
          {detail.created_at && (
            <div className="text-xs text-muted-foreground">
              Discovered {timeAgo(detail.created_at)}
              <span className="mx-1">&middot;</span>
              {new Date(detail.created_at).toLocaleString()}
            </div>
          )}
        </div>

        <div>
          <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">Summary</h3>
          <p className="text-sm leading-relaxed text-foreground">{detail.summary}</p>
        </div>

        <div className="flex gap-4">
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">Status</h3>
            <StatusText finding={detail} />
          </div>
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">State</h3>
            <span className="text-sm text-foreground">{detail.state}</span>
          </div>
          {detail.node_id && (
            <div>
              <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">Node</h3>
              <div className="flex items-center gap-1.5">
                <span className="font-mono text-sm text-foreground">{detail.node_id}</span>
              </div>
              <Link
                to={`/graph?focus=${detail.node_id}`}
                className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-emerald-500 transition-colors mt-1"
              >
                <GitBranch className="h-3 w-3" />
                View in graph
              </Link>
            </div>
          )}
        </div>

        {detail.dismissed && (
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">Dismiss reason</h3>
            <p className="text-sm text-muted-foreground">{detail.dismiss_reason || 'No reason given'}</p>
          </div>
        )}

        {detail.merged_into > 0 && (
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">Merged into</h3>
            <span className="font-mono text-sm text-foreground">#{detail.merged_into}</span>
          </div>
        )}

        {detail.traffic_bindings && detail.traffic_bindings.length > 0 && (
          <div>
            <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Traffic evidence ({detail.traffic_bindings.length})
            </h3>
            <div className="flex flex-col gap-2">
              {detail.traffic_bindings.map(tb => (
                <Link
                  key={tb.id}
                  to={`/traffic?selected=${tb.snapshot_id}`}
                  className="flex items-center gap-2 rounded-md border border-border bg-muted px-3 py-2 text-sm transition-colors hover:border-slate-300 dark:hover:border-slate-600"
                >
                  <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <span className="font-mono text-foreground">{tb.snapshot_id}</span>
                    {tb.role && (
                      <Badge variant="default" className="ml-2">
                        {tb.role}
                      </Badge>
                    )}
                    {tb.note && <p className="mt-0.5 truncate text-xs text-muted-foreground">{tb.note}</p>}
                  </div>
                </Link>
              ))}
            </div>
          </div>
        )}

        <div className="mt-2 border-t border-border pt-3 text-center text-xs text-muted-foreground">
          <kbd className="font-sans">↑↓</kbd> navigate · <kbd className="font-sans">Esc</kbd> close
        </div>
      </div>
    </ScrollArea>
  )
}

function severityRingClass(severity: Severity): string {
  const m: Record<Severity, string> = {
    critical: 'border-rose-500/50',
    high: 'border-orange-500/50',
    medium: 'border-amber-500/50',
    low: 'border-blue-500/50',
    info: 'border-slate-500/50',
  }
  return m[severity]
}
