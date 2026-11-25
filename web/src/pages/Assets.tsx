import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, GitBranch, List, Search, ShieldAlert, X, ChevronLeft, ChevronRight, Server } from 'lucide-react'
import { AssetGraph } from '../components/AssetGraph'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '../components/ui/sheet'

import { TableSkeleton } from '../components/skeletons/TableSkeleton'
import { exportAssetCSV } from '../lib/export'
import { Link, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { cn } from '../lib/utils'
import { fetchAssets, getAssetFindings, type Asset } from '../lib/api'

type ViewMode = 'table' | 'graph'

const ASSET_TYPES = [
  { key: '', label: 'All' },
  { key: 'root_domain', label: 'Domains' },
  { key: 'ip', label: 'IPs' },
  { key: 'subdomain', label: 'Subdomains' },
  { key: 'service', label: 'Services' },
  { key: 'endpoint', label: 'Endpoints' },
  { key: 'app', label: 'Apps' },
] as const

const TYPE_COLORS: Record<string, string> = {
  root_domain: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/25',
  subdomain: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/25',
  ip: 'bg-violet-500/15 text-violet-400 border-violet-500/25',
  service: 'bg-blue-500/15 text-blue-400 border-blue-500/25',
  endpoint: 'bg-amber-500/15 text-amber-400 border-amber-500/25',
  app: 'bg-rose-500/15 text-rose-400 border-rose-500/25',
  company: 'bg-slate-500/15 text-muted-foreground border-slate-500/25',
}

const TYPE_BAR_COLORS: Record<string, string> = {
  root_domain: 'bg-emerald-500',
  subdomain: 'bg-cyan-500',
  ip: 'bg-violet-500',
  service: 'bg-blue-500',
  endpoint: 'bg-amber-500',
  app: 'bg-rose-500',
  company: 'bg-slate-500',
}

function AssetTypeBar({ assets }: { assets: Asset[] }) {
  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const a of assets) {
      c[a.type] = (c[a.type] ?? 0) + 1
    }
    return c
  }, [assets])

  const total = assets.length
  if (total === 0) return null

  const segments = Object.entries(counts)
    .sort(([, a], [, b]) => b - a)
    .map(([type, count]) => ({
      type,
      count,
      pct: (count / total) * 100,
      color: TYPE_BAR_COLORS[type] ?? 'bg-slate-400',
    }))

  return (
    <div className="border-b border-border px-3 md:px-4 py-2">
      <div className="flex h-2 rounded-full overflow-hidden bg-muted mb-2">
        {segments.map(s => (
          <div
            key={s.type}
            className={cn(s.color, 'transition-all duration-300')}
            style={{ width: `${s.pct}%` }}
            title={`${s.type}: ${s.count}`}
          />
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
        <span className="text-muted-foreground font-medium">{total} total</span>
        <span className="text-slate-300 dark:text-slate-700">|</span>
        {segments.map(s => (
          <span key={s.type} className="flex items-center gap-1.5">
            <span className={cn('h-2 w-2 rounded-full', s.color)} />
            <span className="text-muted-foreground">{s.count} {s.type}</span>
          </span>
        ))}
      </div>
    </div>
  )
}

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-rose-500/15 text-rose-400 border-rose-500/25',
  high: 'bg-orange-500/15 text-orange-400 border-orange-500/25',
  medium: 'bg-amber-500/15 text-amber-400 border-amber-500/25',
  low: 'bg-sky-500/15 text-sky-400 border-sky-500/25',
  info: 'bg-slate-500/15 text-muted-foreground border-slate-500/25',
}

const PAGE_SIZE = 50

function useDebounce<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), ms)
    return () => clearTimeout(timer)
  }, [value, ms])
  return debounced
}

function TypeBadge({ type }: { type: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium border',
        TYPE_COLORS[type] ?? TYPE_COLORS.company,
      )}
    >
      {type}
    </span>
  )
}

function MethodBadge({ method }: { method: string }) {
  const colors: Record<string, string> = {
    GET: 'bg-emerald-500/15 text-emerald-400',
    POST: 'bg-blue-500/15 text-blue-400',
    PUT: 'bg-amber-500/15 text-amber-400',
    DELETE: 'bg-rose-500/15 text-rose-400',
    PATCH: 'bg-violet-500/15 text-violet-400',
    OPTIONS: 'bg-slate-500/15 text-muted-foreground',
    HEAD: 'bg-slate-500/15 text-muted-foreground',
  }
  return (
    <span
      className={cn(
        'inline-flex items-center rounded px-1.5 py-0.5 text-xs font-mono font-medium',
        colors[method] ?? colors.GET,
      )}
    >
      {method}
    </span>
  )
}

function dataStr(val: unknown): string {
  if (val == null) return ''
  if (typeof val === 'string') return val
  if (typeof val === 'number' || typeof val === 'boolean') return String(val)
  if (Array.isArray(val)) return val.map(dataStr).join(', ')
  return JSON.stringify(val)
}

function AllColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-xs"><TypeBadge type={asset.type} /></td>
      <td className="px-3 py-2 text-xs text-foreground truncate max-w-sm">{asset.label}</td>
      <td className="px-3 py-2 text-muted-foreground font-mono text-xs tabular-nums">{asset.id}</td>
    </>
  )
}

function DomainColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-xs text-foreground">{asset.label}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs">{dataStr(asset.data.record_type)}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs truncate max-w-xs">{dataStr(asset.data.record_value)}</td>
    </>
  )
}

function IPColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-foreground font-mono text-xs">{asset.label}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs truncate max-w-xs">{dataStr(asset.data.domains)}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs tabular-nums">{dataStr(asset.data.open_ports)}</td>
    </>
  )
}

function SubdomainColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-xs text-foreground">{asset.label}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs">{dataStr(asset.data.root_domain)}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs">{dataStr(asset.data.record_type)}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs truncate max-w-xs">{dataStr(asset.data.record_value)}</td>
    </>
  )
}

function ServiceColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-foreground text-xs truncate max-w-xs">{asset.label}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs tabular-nums">{dataStr(asset.data.status_code)}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs truncate max-w-[200px]">{dataStr(asset.data.technologies)}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs truncate max-w-xs">{dataStr(asset.data.title)}</td>
    </>
  )
}

function EndpointColumns({ asset }: { asset: Asset }) {
  const method = typeof asset.data.method === 'string' ? asset.data.method : 'GET'
  const params = Array.isArray(asset.data.params) ? asset.data.params.length : 0
  return (
    <>
      <td className="px-3 py-2 text-xs"><MethodBadge method={method} /></td>
      <td className="px-3 py-2 text-foreground text-xs truncate max-w-md font-mono">{asset.label}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs">{params}</td>
    </>
  )
}

function AppColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-xs text-foreground">{asset.label}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs font-mono">{dataStr(asset.data.bundle_id)}</td>
      <td className="px-3 py-2 text-muted-foreground text-xs">{dataStr(asset.data.category)}</td>
    </>
  )
}

const COLUMN_HEADERS: Record<string, string[]> = {
  '': ['Type', 'Label', 'ID'],
  root_domain: ['Domain', 'Record Type', 'Record Value'],
  ip: ['IP Address', 'Bound Domains', 'Open Ports'],
  subdomain: ['Subdomain', 'Root Domain', 'Record Type', 'Record Value'],
  service: ['URL', 'Status Code', 'Technologies', 'Page Title'],
  endpoint: ['Method', 'URL', 'Params'],
  app: ['Name', 'Bundle ID', 'Category'],
}

function AssetRow({ asset, typeFilter, onClick }: { asset: Asset; typeFilter: string; onClick: () => void }) {
  const key = typeFilter || ''
  return (
    <tr
      onClick={onClick}
      className="border-b border-border hover:bg-accent cursor-pointer transition-colors"
    >
      {key === '' && <AllColumns asset={asset} />}
      {key === 'root_domain' && <DomainColumns asset={asset} />}
      {key === 'ip' && <IPColumns asset={asset} />}
      {key === 'subdomain' && <SubdomainColumns asset={asset} />}
      {key === 'service' && <ServiceColumns asset={asset} />}
      {key === 'endpoint' && <EndpointColumns asset={asset} />}
      {key === 'app' && <AppColumns asset={asset} />}
      {key === 'company' && <AllColumns asset={asset} />}
    </tr>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium border',
        SEVERITY_COLORS[severity] ?? SEVERITY_COLORS.info,
      )}
    >
      {severity}
    </span>
  )
}

function DetailPanel({ asset, onClose }: { asset: Asset; onClose: () => void }) {
  const { data: findings, isLoading: findingsLoading } = useQuery({
    queryKey: ['asset-findings', asset.id],
    queryFn: () => getAssetFindings(asset.id),
  })

  return (
    <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
      <SheetContent className="w-full max-w-md overflow-y-auto sm:max-w-md">
        <SheetHeader className="border-b border-border">
          <SheetTitle className="flex items-center gap-2">
            <TypeBadge type={asset.type} />
            <span className="text-sm text-muted-foreground font-mono">#{asset.id}</span>
          </SheetTitle>
          <SheetDescription className="sr-only">Asset details</SheetDescription>
        </SheetHeader>
        <div className="p-4">
          <h2 className="text-lg font-medium text-foreground mb-4 break-all">{asset.label}</h2>
          <div className="space-y-2">
            {Object.entries(asset.data).map(([key, val]) => (
              <div key={key} className="flex flex-col gap-0.5">
                <span className="text-xs text-muted-foreground uppercase tracking-wider">{key.replace(/_/g, ' ')}</span>
                <span className="text-sm text-foreground break-all">
                  {typeof val === 'object' && val !== null ? (
                    <pre className="text-xs bg-muted rounded p-2 overflow-x-auto whitespace-pre-wrap">
                      {JSON.stringify(val, null, 2)}
                    </pre>
                  ) : (
                    dataStr(val)
                  )}
                </span>
              </div>
            ))}
            {Object.keys(asset.data).length === 0 && (
              <p className="text-sm text-muted-foreground italic">No additional data</p>
            )}
          </div>

          <div className="mt-6 border-t border-border pt-4">
            <h3 className="text-sm font-medium text-foreground flex items-center gap-1.5 mb-3">
              <ShieldAlert className="w-4 h-4 text-muted-foreground" />
              Linked Findings
            </h3>
            {findingsLoading ? (
              <div className="flex items-center gap-2 py-3">
                <div className="h-4 w-4 rounded-full bg-slate-300 dark:bg-slate-600 animate-pulse" />
                <span className="text-sm text-muted-foreground">Loading findings...</span>
              </div>
            ) : !findings || findings.length === 0 ? (
              <p className="text-sm text-muted-foreground italic">No linked findings</p>
            ) : (
              <div className="space-y-2">
                {findings.map((f) => (
                  <Link
                    key={f.id}
                    to={`/findings?id=${f.id}`}
                    className="block rounded-md border border-border p-3 hover:bg-accent transition-colors"
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <SeverityBadge severity={f.severity} />
                      <span className="text-sm font-medium text-foreground truncate">{f.vulnclass}</span>
                    </div>
                    <p className="text-xs text-muted-foreground line-clamp-2">{f.summary}</p>
                  </Link>
                ))}
              </div>
            )}
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}

export default function Assets() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [typeFilter, setTypeFilter] = useState(searchParams.get('type') ?? '')
  const [dslInput, setDslInput] = useState('')
  const [page, setPage] = useState(0)
  const [selectedAsset, setSelectedAsset] = useState<Asset | null>(null)
  const [viewMode, setViewMode] = useState<ViewMode>('table')
  const pendingAssetId = useRef<number | null>(
    searchParams.get('id') ? Number(searchParams.get('id')) : null
  )

  const debouncedDsl = useDebounce(dslInput, 300)

  useEffect(() => {
    setPage(0)
  }, [typeFilter, debouncedDsl])

  const selectedId = selectedAsset?.id ?? null

  useEffect(() => {
    const timer = setTimeout(() => {
      const params = new URLSearchParams()
      if (typeFilter) params.set('type', typeFilter)
      if (selectedId != null) params.set('id', String(selectedId))
      setSearchParams(params, { replace: true })
    }, 300)
    return () => clearTimeout(timer)
  }, [typeFilter, selectedId, setSearchParams])

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['assets', typeFilter, debouncedDsl, page],
    queryFn: () =>
      fetchAssets({
        type: typeFilter || undefined,
        dsl: debouncedDsl || undefined,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
  })

  const { data: allData } = useQuery({
    queryKey: ['assets-distribution'],
    queryFn: () => fetchAssets({ limit: 1000, offset: 0 }),
  })

  const { data: graphData, isLoading: graphLoading } = useQuery({
    queryKey: ['assets-graph', typeFilter, debouncedDsl],
    queryFn: () =>
      fetchAssets({
        type: typeFilter || undefined,
        dsl: debouncedDsl || undefined,
        limit: 500,
        offset: 0,
      }),
    enabled: viewMode === 'graph',
  })

  const total = data?.total ?? 0
  const assets = data?.assets ?? []
  const start = page * PAGE_SIZE + 1
  const end = Math.min((page + 1) * PAGE_SIZE, total)
  const headers = COLUMN_HEADERS[typeFilter] ?? COLUMN_HEADERS['']

  useEffect(() => {
    if (pendingAssetId.current != null && assets.length > 0) {
      const match = assets.find(a => a.id === pendingAssetId.current)
      if (match) setSelectedAsset(match)
      pendingAssetId.current = null
    }
  }, [assets])

  const handleClose = useCallback(() => setSelectedAsset(null), [])

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border bg-muted px-3 md:px-4 py-2 md:py-3">
        <div className="flex items-center gap-2 overflow-x-auto pb-1">
          {ASSET_TYPES.map((t) => (
            <button
              key={t.key}
              onClick={() => setTypeFilter(t.key)}
              className={cn(
                'px-3 py-1.5 rounded-md text-sm font-medium whitespace-nowrap transition-colors',
                typeFilter === t.key
                  ? 'bg-muted text-foreground'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent',
              )}
            >
              {t.label}
            </button>
          ))}

          <div className="ml-auto flex items-center gap-2">
            <div className="flex rounded-md border border-border">
              <button
                onClick={() => setViewMode('table')}
                className={cn('p-1.5 rounded-l-md', viewMode === 'table' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground')}
              >
                <List className="h-4 w-4" />
              </button>
              <button
                onClick={() => setViewMode('graph')}
                className={cn('p-1.5 rounded-r-md', viewMode === 'graph' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground')}
              >
                <GitBranch className="h-4 w-4" />
              </button>
            </div>
            <button
              onClick={() => { exportAssetCSV(assets, 'assets.csv'); toast.success('Assets exported as CSV') }}
              disabled={assets.length === 0}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium whitespace-nowrap transition-colors border border-border text-muted-foreground hover:bg-accent disabled:opacity-30 disabled:cursor-not-allowed"
            >
              <Download className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">Export</span>
            </button>
          </div>
        </div>
      </div>

      <div className="border-b border-border px-3 md:px-4 py-2">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            value={dslInput}
            onChange={(e) => setDslInput(e.target.value)}
            placeholder='Search assets (e.g., domain=example.com AND type=service)'
            className="w-full bg-background border border-border rounded-md py-2 pl-9 pr-9 text-sm text-foreground placeholder:text-slate-400 dark:placeholder:text-slate-600 font-mono focus:outline-none focus:ring-1 focus:ring-slate-500 focus:border-slate-500"
          />
          {dslInput && (
            <button
              onClick={() => setDslInput('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      <AssetTypeBar assets={allData?.assets ?? []} />

      <div className="flex-1 overflow-auto">
        {error ? (
          <div className="flex items-center justify-center h-48">
            <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
              <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load assets: {(error as Error).message}</p>
              <button
                type="button"
                onClick={() => refetch()}
                className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
              >
                Try again
              </button>
            </div>
          </div>
        ) : isLoading ? (
          <div className="p-3 md:p-4"><TableSkeleton columns={4} rows={10} /></div>
        ) : viewMode === 'graph' ? (
          graphLoading ? (
            <TableSkeleton rows={8} columns={5} />
          ) : (
            <AssetGraph assets={graphData?.assets ?? []} onSelectAsset={setSelectedAsset} />
          )
        ) : assets.length === 0 ? (
          <div className="flex h-64 flex-col items-center justify-center gap-3 text-muted-foreground">
            <Server className="h-12 w-12 text-slate-300 dark:text-slate-700" />
            <div className="text-center">
              <p className="text-lg font-medium text-muted-foreground">No assets discovered</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {(dslInput || typeFilter)
                  ? 'Try adjusting your filters'
                  : 'Assets will appear as the engagement discovers hosts, services, and endpoints'}
              </p>
            </div>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left min-w-[600px]">
              <thead>
                <tr className="border-b border-border bg-muted">
                  {headers.map((h) => (
                    <th
                      key={h}
                      className="px-3 py-2 text-xs font-medium text-muted-foreground uppercase tracking-wider"
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {assets.map((asset) => (
                  <AssetRow
                    key={asset.id}
                    asset={asset}
                    typeFilter={typeFilter}
                    onClick={() => setSelectedAsset(asset)}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {total > 0 && (
        <div className="border-t border-border px-3 md:px-4 py-2 flex items-center justify-between text-sm text-muted-foreground bg-muted">
          <span>
            Showing {start}-{end} of {total}
          </span>
          <div className="flex items-center gap-1">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className="p-1 rounded hover:bg-accent disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <button
              onClick={() => setPage((p) => p + 1)}
              disabled={end >= total}
              className="p-1 rounded hover:bg-accent disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {selectedAsset && <DetailPanel asset={selectedAsset} onClose={handleClose} />}
    </div>
  )
}
