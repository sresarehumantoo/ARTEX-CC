import { ChevronDown, ChevronUp, Clipboard, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Badge, type BadgeVariant } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { ScrollArea } from '../components/ui/scroll-area'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { type TrafficFilters, useTraffic, useTrafficDetail } from '../hooks/useTraffic'
import type { TrafficExchange } from '../lib/types'
import { cn, formatBytes, relativeTime } from '../lib/utils'

const PAGE_SIZE = 50

const METHOD_VARIANT: Record<string, BadgeVariant> = {
  GET: 'get',
  POST: 'post',
  PUT: 'put',
  DELETE: 'delete',
  PATCH: 'patch',
}

function statusColor(code: number): string {
  if (code >= 200 && code < 300) return 'text-emerald-400'
  if (code >= 300 && code < 400) return 'text-blue-400'
  if (code >= 400 && code < 500) return 'text-amber-400'
  if (code >= 500) return 'text-rose-400'
  return 'text-slate-400'
}

export default function Traffic() {
  const [searchParams] = useSearchParams()
  const [host, setHost] = useState('')
  const [method, setMethod] = useState('')
  const [statusInput, setStatusInput] = useState('')
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [debouncedHost, setDebouncedHost] = useState('')
  const [debouncedStatus, setDebouncedStatus] = useState('')
  const [page, setPage] = useState(0)
  const [selectedId, setSelectedId] = useState<string | null>(
    searchParams.get('selected'),
  )
  const [detailHeight, setDetailHeight] = useState(400)

  const queryTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const hostTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const statusTimer = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => {
    clearTimeout(queryTimer.current)
    queryTimer.current = setTimeout(() => setDebouncedQuery(query), 300)
    return () => clearTimeout(queryTimer.current)
  }, [query])

  useEffect(() => {
    clearTimeout(hostTimer.current)
    hostTimer.current = setTimeout(() => setDebouncedHost(host), 300)
    return () => clearTimeout(hostTimer.current)
  }, [host])

  useEffect(() => {
    clearTimeout(statusTimer.current)
    statusTimer.current = setTimeout(() => setDebouncedStatus(statusInput), 300)
    return () => clearTimeout(statusTimer.current)
  }, [statusInput])

  useEffect(() => {
    setPage(0)
  }, [debouncedHost, method, debouncedStatus, debouncedQuery])

  const filters: TrafficFilters = useMemo(
    () => ({
      host: debouncedHost,
      method,
      status: debouncedStatus,
      query: debouncedQuery,
      page,
      size: PAGE_SIZE,
    }),
    [debouncedHost, method, debouncedStatus, debouncedQuery, page],
  )

  const { data, isLoading, error } = useTraffic(filters)
  const { data: detail, isLoading: detailLoading } = useTrafficDetail(selectedId)

  const rows = data?.rows ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / PAGE_SIZE)
  const rangeStart = page * PAGE_SIZE + 1
  const rangeEnd = Math.min((page + 1) * PAGE_SIZE, total)

  const clearFilters = useCallback(() => {
    setHost('')
    setMethod('')
    setStatusInput('')
    setQuery('')
  }, [])

  const hasFilters = host || method || statusInput || query

  // Resizable detail panel
  const dragRef = useRef<{ startY: number; startH: number } | null>(null)

  const onDragStart = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault()
      dragRef.current = { startY: e.clientY, startH: detailHeight }
      const onMove = (ev: MouseEvent) => {
        if (!dragRef.current) return
        const delta = dragRef.current.startY - ev.clientY
        setDetailHeight(Math.max(200, Math.min(800, dragRef.current.startH + delta)))
      }
      const onUp = () => {
        dragRef.current = null
        document.removeEventListener('mousemove', onMove)
        document.removeEventListener('mouseup', onUp)
      }
      document.addEventListener('mousemove', onMove)
      document.addEventListener('mouseup', onUp)
    },
    [detailHeight],
  )

  if (error) {
    return (
      <div className="flex h-96 items-center justify-center text-slate-500">
        Failed to load traffic: {(error as Error).message}
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      {/* Filter bar */}
      <div className="flex items-center gap-3 border-b border-slate-800 px-4 py-2">
        <Input
          placeholder="Host..."
          value={host}
          onChange={e => setHost(e.target.value)}
          className="w-44"
        />
        <Select value={method} onChange={e => setMethod(e.target.value)} className="w-28">
          <option value="">Method</option>
          {['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'].map(m => (
            <option key={m} value={m}>{m}</option>
          ))}
        </Select>
        <Input
          placeholder="Status (200, 4xx...)"
          value={statusInput}
          onChange={e => setStatusInput(e.target.value)}
          className="w-36"
        />
        <Input
          placeholder="Search..."
          value={query}
          onChange={e => setQuery(e.target.value)}
          className="w-52"
        />
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            <X className="mr-1 h-3 w-3" /> Clear
          </Button>
        )}
        <div className="ml-auto text-xs text-slate-500">
          {total > 0 && `${rangeStart}-${rangeEnd} of ${total}`}
        </div>
      </div>

      {/* Table */}
      <div className="min-h-0 flex-1 overflow-auto">
        {isLoading ? (
          <div className="flex h-64 items-center justify-center text-slate-500">Loading...</div>
        ) : rows.length === 0 ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-slate-500">
            <p className="text-lg">No traffic captured</p>
            <p className="text-sm">
              {hasFilters
                ? 'Try adjusting your filters'
                : 'Traffic will appear when the proxy is running'}
            </p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="text-[11px]">
                <TableHead className="w-16">Time</TableHead>
                <TableHead>Host</TableHead>
                <TableHead className="w-16">Method</TableHead>
                <TableHead>URL</TableHead>
                <TableHead className="w-14">Status</TableHead>
                <TableHead>Type</TableHead>
                <TableHead className="w-16 text-right">Size</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map(row => (
                <TableRow
                  key={row.id}
                  className={cn(
                    'cursor-pointer text-[13px] leading-tight',
                    selectedId === row.id && 'bg-slate-800',
                  )}
                  onClick={() => setSelectedId(selectedId === row.id ? null : row.id)}
                >
                  <TableCell className="whitespace-nowrap text-xs text-slate-500">
                    {relativeTime(row.ts)}
                  </TableCell>
                  <TableCell className="max-w-[200px] truncate font-mono text-xs text-slate-300">
                    {row.host}
                  </TableCell>
                  <TableCell>
                    <MethodBadge method={row.method} />
                  </TableCell>
                  <TableCell className="max-w-md truncate font-mono text-xs text-slate-400">
                    {displayUrl(row)}
                  </TableCell>
                  <TableCell>
                    <StatusCode code={row.status} />
                  </TableCell>
                  <TableCell className="max-w-[120px] truncate text-xs text-slate-500">
                    {shortContentType(row.content_type)}
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs text-slate-500">
                    {formatBytes(row.resp_len)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between border-t border-slate-800 px-4 py-2">
          <Button
            variant="ghost"
            size="sm"
            disabled={page === 0}
            onClick={() => setPage(p => p - 1)}
          >
            <ChevronUp className="mr-1 h-3 w-3 -rotate-90" /> Prev
          </Button>
          <span className="text-xs text-slate-500">
            Page {page + 1} of {totalPages}
          </span>
          <Button
            variant="ghost"
            size="sm"
            disabled={page >= totalPages - 1}
            onClick={() => setPage(p => p + 1)}
          >
            Next <ChevronDown className="ml-1 h-3 w-3 -rotate-90" />
          </Button>
        </div>
      )}

      {/* Detail panel */}
      {selectedId !== null && (
        <>
          {/* Drag handle */}
          <div
            className="h-1.5 cursor-row-resize border-t border-slate-700 bg-slate-800 hover:bg-slate-700"
            onMouseDown={onDragStart}
          />
          <div style={{ height: detailHeight }} className="flex flex-col bg-slate-900">
            <div className="flex items-center gap-2 border-b border-slate-800 px-4 py-1.5">
              <span className="text-xs font-medium text-slate-400">
                {selectedId}
              </span>
              <div className="ml-auto flex items-center gap-1">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setSelectedId(null)}
                >
                  <X className="h-3 w-3" />
                </Button>
              </div>
            </div>
            <div className="flex min-h-0 flex-1">
              {detailLoading || !detail ? (
                <div className="flex flex-1 items-center justify-center text-slate-500">Loading...</div>
              ) : (
                <>
                  <HttpPanel label="Request" content={detail.request} />
                  <div className="w-px bg-slate-800" />
                  <HttpPanel label="Response" content={detail.response} />
                </>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

function MethodBadge({ method }: { method: string }) {
  const variant = METHOD_VARIANT[method.toUpperCase()] ?? 'default'
  return (
    <Badge variant={variant} className="text-[10px] px-1.5 py-0">
      {method}
    </Badge>
  )
}

function StatusCode({ code }: { code: number }) {
  return <span className={cn('font-mono text-xs font-medium', statusColor(code))}>{code}</span>
}

function displayUrl(row: TrafficExchange): string {
  return row.url_template || row.url
}

function shortContentType(ct: string): string {
  if (!ct) return ''
  const semi = ct.indexOf(';')
  const base = semi >= 0 ? ct.slice(0, semi) : ct
  return base.replace('application/', '').replace('text/', '')
}

function HttpPanel({ label, content }: { label: string; content: string }) {
  const [copied, setCopied] = useState(false)
  const formatted = useMemo(() => formatHttp(content), [content])

  const copy = useCallback(() => {
    navigator.clipboard.writeText(content).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }, [content])

  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <div className="flex items-center justify-between border-b border-slate-800 px-3 py-1">
        <span className="text-xs font-medium text-slate-500">{label}</span>
        <button
          type="button"
          onClick={copy}
          className="flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-300"
        >
          <Clipboard className="h-3 w-3" />
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <ScrollArea className="flex-1 p-3">
        <pre className="font-mono text-xs leading-relaxed whitespace-pre-wrap break-all">
          {formatted}
        </pre>
      </ScrollArea>
    </div>
  )
}

function formatHttp(raw: string): React.ReactNode {
  if (!raw) return <span className="text-slate-600">Empty</span>

  const headerEnd = raw.indexOf('\r\n\r\n')
  const splitPos = headerEnd >= 0 ? headerEnd : raw.indexOf('\n\n')

  if (splitPos < 0) return highlightHeaders(raw)

  const headers = raw.slice(0, splitPos)
  const body = raw.slice(splitPos).replace(/^[\r\n]+/, '')

  return (
    <>
      {highlightHeaders(headers)}
      {'\n\n'}
      {formatBody(body)}
    </>
  )
}

function highlightHeaders(text: string): React.ReactNode {
  const lines = text.split(/\r?\n/)
  return lines.map((line, i) => {
    const colonIdx = line.indexOf(':')
    if (i === 0 || colonIdx < 0) {
      return (
        <span key={i}>
          <span className="font-semibold text-slate-200">{line}</span>
          {i < lines.length - 1 ? '\n' : ''}
        </span>
      )
    }
    const name = line.slice(0, colonIdx)
    const value = line.slice(colonIdx)
    return (
      <span key={i}>
        <span className="text-blue-400">{name}</span>
        <span className="text-slate-400">{value}</span>
        {i < lines.length - 1 ? '\n' : ''}
      </span>
    )
  })
}

function formatBody(body: string): React.ReactNode {
  const trimmed = body.trim()
  if (!trimmed) return null
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      const formatted = JSON.stringify(JSON.parse(trimmed), null, 2)
      return <span className="text-slate-300">{formatted}</span>
    } catch {
      // not valid JSON
    }
  }
  return <span className="text-slate-300">{body}</span>
}
