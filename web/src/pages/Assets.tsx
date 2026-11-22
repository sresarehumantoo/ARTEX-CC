import { useState, useEffect, useCallback, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, GitBranch, List, Search, ShieldAlert, X, ChevronLeft, ChevronRight, Server } from 'lucide-react'
import { AssetGraph } from '../components/AssetGraph'
import { Spinner } from '../components/ui/spinner'
import { TableSkeleton } from '../components/skeletons/TableSkeleton'
import { exportAssetCSV } from '../lib/export'
import { useToast } from '../hooks/useToast'
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
  company: 'bg-slate-500/15 text-slate-400 border-slate-500/25',
}

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-rose-500/15 text-rose-400 border-rose-500/25',
  high: 'bg-orange-500/15 text-orange-400 border-orange-500/25',
  medium: 'bg-amber-500/15 text-amber-400 border-amber-500/25',
  low: 'bg-sky-500/15 text-sky-400 border-sky-500/25',
  info: 'bg-slate-500/15 text-slate-400 border-slate-500/25',
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
    OPTIONS: 'bg-slate-500/15 text-slate-400',
    HEAD: 'bg-slate-500/15 text-slate-400',
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
      <td className="px-3 py-2"><TypeBadge type={asset.type} /></td>
      <td className="px-3 py-2 text-slate-800 dark:text-slate-200 truncate max-w-sm">{asset.label}</td>
      <td className="px-3 py-2 text-slate-500 font-mono text-xs">{asset.id}</td>
    </>
  )
}

function DomainColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-slate-800 dark:text-slate-200">{asset.label}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{dataStr(asset.data.record_type)}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm truncate max-w-xs">{dataStr(asset.data.record_value)}</td>
    </>
  )
}

function IPColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-slate-800 dark:text-slate-200 font-mono text-sm">{asset.label}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm truncate max-w-xs">{dataStr(asset.data.domains)}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{dataStr(asset.data.open_ports)}</td>
    </>
  )
}

function SubdomainColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-slate-800 dark:text-slate-200">{asset.label}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{dataStr(asset.data.root_domain)}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{dataStr(asset.data.record_type)}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm truncate max-w-xs">{dataStr(asset.data.record_value)}</td>
    </>
  )
}

function ServiceColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-slate-800 dark:text-slate-200 text-sm truncate max-w-xs">{asset.label}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{dataStr(asset.data.status_code)}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm truncate max-w-[200px]">{dataStr(asset.data.technologies)}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm truncate max-w-xs">{dataStr(asset.data.title)}</td>
    </>
  )
}

function EndpointColumns({ asset }: { asset: Asset }) {
  const method = typeof asset.data.method === 'string' ? asset.data.method : 'GET'
  const params = Array.isArray(asset.data.params) ? asset.data.params.length : 0
  return (
    <>
      <td className="px-3 py-2"><MethodBadge method={method} /></td>
      <td className="px-3 py-2 text-slate-800 dark:text-slate-200 text-sm truncate max-w-md font-mono">{asset.label}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{params}</td>
    </>
  )
}

function AppColumns({ asset }: { asset: Asset }) {
  return (
    <>
      <td className="px-3 py-2 text-slate-800 dark:text-slate-200">{asset.label}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm font-mono">{dataStr(asset.data.bundle_id)}</td>
      <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{dataStr(asset.data.category)}</td>
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
      className="border-b border-slate-200 dark:border-slate-800 hover:bg-slate-100/50 dark:hover:bg-slate-800/50 cursor-pointer transition-colors"
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
  const panelRef = useRef<HTMLDivElement>(null)

  const { data: findings, isLoading: findingsLoading } = useQuery({
    queryKey: ['asset-findings', asset.id],
    queryFn: () => getAssetFindings(asset.id),
  })

  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        ref={panelRef}
        className="relative w-full max-w-md bg-white dark:bg-slate-900 border-l border-slate-200 dark:border-slate-700 shadow-xl overflow-y-auto animate-slide-in"
      >
        <div className="sticky top-0 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <TypeBadge type={asset.type} />
            <span className="text-sm text-slate-500 font-mono">#{asset.id}</span>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 hover:text-slate-800 dark:hover:text-slate-200 transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="p-4">
          <h2 className="text-lg font-medium text-slate-900 dark:text-slate-100 mb-4 break-all">{asset.label}</h2>
          <div className="space-y-2">
            {Object.entries(asset.data).map(([key, val]) => (
              <div key={key} className="flex flex-col gap-0.5">
                <span className="text-xs text-slate-500 uppercase tracking-wider">{key.replace(/_/g, ' ')}</span>
                <span className="text-sm text-slate-700 dark:text-slate-300 break-all">
                  {typeof val === 'object' && val !== null ? (
                    <pre className="text-xs bg-slate-100 dark:bg-slate-800 rounded p-2 overflow-x-auto whitespace-pre-wrap">
                      {JSON.stringify(val, null, 2)}
                    </pre>
                  ) : (
                    dataStr(val)
                  )}
                </span>
              </div>
            ))}
            {Object.keys(asset.data).length === 0 && (
              <p className="text-sm text-slate-500 italic">No additional data</p>
            )}
          </div>

          <div className="mt-6 border-t border-slate-200 dark:border-slate-800 pt-4">
            <h3 className="text-sm font-medium text-slate-900 dark:text-slate-100 flex items-center gap-1.5 mb-3">
              <ShieldAlert className="w-4 h-4 text-slate-500" />
              Linked Findings
            </h3>
            {findingsLoading ? (
              <div className="flex items-center gap-2 py-3">
                <Spinner className="h-4 w-4" />
                <span className="text-sm text-slate-500">Loading findings...</span>
              </div>
            ) : !findings || findings.length === 0 ? (
              <p className="text-sm text-slate-500 italic">No linked findings</p>
            ) : (
              <div className="space-y-2">
                {findings.map((f) => (
                  <a
                    key={f.id}
                    href={`/findings?id=${f.id}`}
                    className="block rounded-md border border-slate-200 dark:border-slate-800 p-3 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors"
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <SeverityBadge severity={f.severity} />
                      <span className="text-sm font-medium text-slate-800 dark:text-slate-200 truncate">{f.vulnclass}</span>
                    </div>
                    <p className="text-xs text-slate-500 dark:text-slate-400 line-clamp-2">{f.summary}</p>
                  </a>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default function Assets() {
  const { toast } = useToast()
  const [typeFilter, setTypeFilter] = useState('')
  const [dslInput, setDslInput] = useState('')
  const [page, setPage] = useState(0)
  const [selectedAsset, setSelectedAsset] = useState<Asset | null>(null)
  const [viewMode, setViewMode] = useState<ViewMode>('table')

  const debouncedDsl = useDebounce(dslInput, 300)

  useEffect(() => {
    setPage(0)
  }, [typeFilter, debouncedDsl])

  const { data, isLoading, error } = useQuery({
    queryKey: ['assets', typeFilter, debouncedDsl, page],
    queryFn: () =>
      fetchAssets({
        type: typeFilter || undefined,
        dsl: debouncedDsl || undefined,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
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

  const handleClose = useCallback(() => setSelectedAsset(null), [])

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 md:px-4 py-2 md:py-3">
        <div className="flex items-center gap-2 overflow-x-auto pb-1">
          {ASSET_TYPES.map((t) => (
            <button
              key={t.key}
              onClick={() => setTypeFilter(t.key)}
              className={cn(
                'px-3 py-1.5 rounded-md text-sm font-medium whitespace-nowrap transition-colors',
                typeFilter === t.key
                  ? 'bg-slate-200 dark:bg-slate-700 text-slate-900 dark:text-slate-100'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800',
              )}
            >
              {t.label}
            </button>
          ))}

          <div className="ml-auto flex items-center gap-2">
            <div className="flex rounded-md border border-slate-200 dark:border-slate-700">
              <button
                onClick={() => setViewMode('table')}
                className={cn('p-1.5 rounded-l-md', viewMode === 'table' ? 'bg-slate-200 dark:bg-slate-700 text-slate-900 dark:text-white' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300')}
              >
                <List className="h-4 w-4" />
              </button>
              <button
                onClick={() => setViewMode('graph')}
                className={cn('p-1.5 rounded-r-md', viewMode === 'graph' ? 'bg-slate-200 dark:bg-slate-700 text-slate-900 dark:text-white' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300')}
              >
                <GitBranch className="h-4 w-4" />
              </button>
            </div>
            <button
              onClick={() => { exportAssetCSV(assets, 'assets.csv'); toast('Assets exported as CSV', { variant: 'success' }) }}
              disabled={assets.length === 0}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium whitespace-nowrap transition-colors border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed"
            >
              <Download className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">Export</span>
            </button>
          </div>
        </div>
      </div>

      <div className="border-b border-slate-200 dark:border-slate-800 px-3 md:px-4 py-2">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
          <input
            type="text"
            value={dslInput}
            onChange={(e) => setDslInput(e.target.value)}
            placeholder='Search assets (e.g., domain=example.com AND type=service)'
            className="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-md py-2 pl-9 pr-9 text-sm text-slate-800 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-600 font-mono focus:outline-none focus:ring-1 focus:ring-slate-500 focus:border-slate-500"
          />
          {dslInput && (
            <button
              onClick={() => setDslInput('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        {error ? (
          <div className="flex items-center justify-center h-48 text-sm text-rose-400">
            Failed to load assets: {(error as Error).message}
          </div>
        ) : isLoading ? (
          <div className="p-3 md:p-4"><TableSkeleton columns={4} rows={10} /></div>
        ) : viewMode === 'graph' ? (
          graphLoading ? (
            <div className="flex items-center justify-center h-64"><Spinner className="h-8 w-8" /></div>
          ) : (
            <AssetGraph assets={graphData?.assets ?? []} onSelectAsset={setSelectedAsset} />
          )
        ) : assets.length === 0 ? (
          <div className="flex h-64 flex-col items-center justify-center gap-3 text-slate-500">
            <Server className="h-12 w-12 text-slate-300 dark:text-slate-700" />
            <div className="text-center">
              <p className="text-lg font-medium text-slate-600 dark:text-slate-400">No assets discovered</p>
              <p className="mt-1 text-sm text-slate-500 dark:text-slate-500">
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
                <tr className="border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
                  {headers.map((h) => (
                    <th
                      key={h}
                      className="px-3 py-2 text-xs font-medium text-slate-500 uppercase tracking-wider"
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
        <div className="border-t border-slate-200 dark:border-slate-800 px-3 md:px-4 py-2 flex items-center justify-between text-sm text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-950">
          <span>
            Showing {start}-{end} of {total}
          </span>
          <div className="flex items-center gap-1">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <button
              onClick={() => setPage((p) => p + 1)}
              disabled={end >= total}
              className="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
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
