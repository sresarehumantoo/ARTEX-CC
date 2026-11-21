import { ChevronDown, ChevronUp, ExternalLink, Search, Shield, X } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Badge, type BadgeVariant } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { ScrollArea } from '../components/ui/scroll-area'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { Spinner } from '../components/ui/spinner'
import { useFindingDetail, useFindings } from '../hooks/useFindings'
import type { Finding, Severity } from '../lib/types'
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

type SortField = 'severity' | 'id'
type SortDir = 'asc' | 'desc'

export default function Findings() {
  const { data: findings = [], isLoading, error } = useFindings()
  const [severityFilter, setSeverityFilter] = useState<Severity | null>(null)
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')
  const [sortField, setSortField] = useState<SortField>('severity')
  const [sortDir, setSortDir] = useState<SortDir>('asc')
  const [selectedId, setSelectedId] = useState<number | null>(null)

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

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortField !== field) return null
    return sortDir === 'asc' ? (
      <ChevronUp className="ml-1 inline h-3 w-3" />
    ) : (
      <ChevronDown className="ml-1 inline h-3 w-3" />
    )
  }

  if (error) {
    return (
      <div className="flex h-96 items-center justify-center text-slate-500">
        Failed to load findings: {(error as Error).message}
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col gap-4 p-6">
      {/* Summary cards */}
      <div className="grid grid-cols-5 gap-3">
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
      <div className="flex items-center gap-3">
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

        <div className="relative ml-auto w-64">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-slate-500" />
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
              className="absolute right-2.5 top-2.5 text-slate-500 hover:text-slate-300"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      {/* Main content area */}
      <div className="flex min-h-0 flex-1 gap-4">
        {/* Table */}
        <div className="min-h-0 flex-1">
          {isLoading ? (
            <div className="flex h-64 items-center justify-center"><Spinner className="h-8 w-8" /></div>
          ) : filtered.length === 0 ? (
            <div className="flex h-64 flex-col items-center justify-center gap-2 text-slate-500">
              <Shield className="h-10 w-10 text-slate-600" />
              <p className="text-lg">No findings</p>
              <p className="text-sm">
                {findings.length > 0
                  ? 'Try adjusting your filters'
                  : 'Findings will appear here as the engagement progresses'}
              </p>
            </div>
          ) : (
            <ScrollArea className="h-full">
              <Table>
                <TableHeader>
                  <TableRow>
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
                      className={cn(
                        'cursor-pointer',
                        selectedId === f.id && 'bg-slate-800',
                        i % 2 === 1 && 'bg-slate-900/50',
                      )}
                      onClick={() => setSelectedId(selectedId === f.id ? null : f.id)}
                    >
                      <TableCell>
                        <Badge variant={SEVERITY_VARIANT[f.severity]}>{f.severity}</Badge>
                      </TableCell>
                      <TableCell className="font-medium text-slate-200">{f.vulnclass}</TableCell>
                      <TableCell className="max-w-md truncate text-slate-400">{f.summary}</TableCell>
                      <TableCell>
                        <StatusText finding={f} />
                      </TableCell>
                      <TableCell className="text-right font-mono text-slate-500">{f.id}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </ScrollArea>
          )}
        </div>

        {/* Detail panel */}
        {selectedId !== null && (
          <div className="w-96 shrink-0 overflow-hidden rounded-lg border border-slate-800 bg-slate-900">
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
        active && 'border-slate-600 bg-slate-800',
      )}
      onClick={onClick}
    >
      <CardHeader>
        <CardTitle>{label}</CardTitle>
      </CardHeader>
      <CardContent>
        <span className={cn('text-3xl font-bold', variant ? colorText[variant] : 'text-slate-100')}>
          {count}
        </span>
      </CardContent>
    </Card>
  )
}

function StatusText({ finding }: { finding: Finding }) {
  if (finding.dismissed) return <span className="text-slate-500">Dismissed</span>
  const color: Record<string, string> = {
    confirmed: 'text-emerald-400',
    pending: 'text-amber-400',
  }
  return (
    <span className={color[finding.status.toLowerCase()] ?? 'text-slate-400'}>
      {finding.status || finding.state}
    </span>
  )
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
      <div className="flex h-64 items-center justify-center"><Spinner className="h-8 w-8" /></div>
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
            className="text-slate-500 hover:text-slate-300"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div>
          <h2 className="text-lg font-semibold text-slate-100">{detail.vulnclass}</h2>
          <p className="mt-1 text-xs text-slate-500">Finding #{detail.id}</p>
        </div>

        <div>
          <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-slate-500">Summary</h3>
          <p className="text-sm leading-relaxed text-slate-300">{detail.summary}</p>
        </div>

        <div className="flex gap-4">
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-slate-500">Status</h3>
            <StatusText finding={detail} />
          </div>
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-slate-500">State</h3>
            <span className="text-sm text-slate-300">{detail.state}</span>
          </div>
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-slate-500">Node</h3>
            <span className="font-mono text-sm text-slate-300">{detail.node_id}</span>
          </div>
        </div>

        {detail.dismissed && (
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-slate-500">Dismiss reason</h3>
            <p className="text-sm text-slate-400">{detail.dismiss_reason || 'No reason given'}</p>
          </div>
        )}

        {detail.merged_into > 0 && (
          <div>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wider text-slate-500">Merged into</h3>
            <span className="font-mono text-sm text-slate-300">#{detail.merged_into}</span>
          </div>
        )}

        {detail.traffic_bindings && detail.traffic_bindings.length > 0 && (
          <div>
            <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-slate-500">
              Traffic evidence ({detail.traffic_bindings.length})
            </h3>
            <div className="flex flex-col gap-2">
              {detail.traffic_bindings.map(tb => (
                <Link
                  key={tb.id}
                  to={`/traffic?selected=${tb.snapshot_id}`}
                  className="flex items-center gap-2 rounded-md border border-slate-700 bg-slate-800 px-3 py-2 text-sm transition-colors hover:border-slate-600"
                >
                  <ExternalLink className="h-3.5 w-3.5 shrink-0 text-slate-500" />
                  <div className="min-w-0 flex-1">
                    <span className="font-mono text-slate-300">{tb.snapshot_id}</span>
                    {tb.role && (
                      <Badge variant="default" className="ml-2">
                        {tb.role}
                      </Badge>
                    )}
                    {tb.note && <p className="mt-0.5 truncate text-xs text-slate-500">{tb.note}</p>}
                  </div>
                </Link>
              ))}
            </div>
          </div>
        )}
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
