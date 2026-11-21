import { ArrowDown, ArrowUp, ArrowUpDown, ChevronDown, ChevronUp, Clipboard, Globe, Terminal, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Badge, type BadgeVariant } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { ScrollArea } from '../components/ui/scroll-area'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { Spinner } from '../components/ui/spinner'
import { TableSkeleton } from '../components/skeletons/TableSkeleton'
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
  const [bodySearch, setBodySearch] = useState(false)
  const [respMin, setRespMin] = useState('')
  const [respMax, setRespMax] = useState('')
  const [sortCol, setSortCol] = useState('')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc' | ''>('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [debouncedHost, setDebouncedHost] = useState('')
  const [debouncedStatus, setDebouncedStatus] = useState('')
  const [debouncedRespMin, setDebouncedRespMin] = useState('')
  const [debouncedRespMax, setDebouncedRespMax] = useState('')
  const [page, setPage] = useState(0)
  const [selectedId, setSelectedId] = useState<string | null>(
    searchParams.get('selected'),
  )
  const [detailHeight, setDetailHeight] = useState(400)

  const queryTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const hostTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const statusTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const respMinTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const respMaxTimer = useRef<ReturnType<typeof setTimeout>>(undefined)

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
    clearTimeout(respMinTimer.current)
    respMinTimer.current = setTimeout(() => setDebouncedRespMin(respMin), 300)
    return () => clearTimeout(respMinTimer.current)
  }, [respMin])

  useEffect(() => {
    clearTimeout(respMaxTimer.current)
    respMaxTimer.current = setTimeout(() => setDebouncedRespMax(respMax), 300)
    return () => clearTimeout(respMaxTimer.current)
  }, [respMax])

  useEffect(() => {
    setPage(0)
  }, [debouncedHost, method, debouncedStatus, debouncedQuery, debouncedRespMin, debouncedRespMax, bodySearch])

  const filters: TrafficFilters = useMemo(
    () => ({
      host: debouncedHost,
      method,
      status: debouncedStatus,
      query: bodySearch ? '' : debouncedQuery,
      body: bodySearch ? debouncedQuery : '',
      respMin: debouncedRespMin,
      respMax: debouncedRespMax,
      sort: sortCol,
      order: sortOrder,
      page,
      size: PAGE_SIZE,
    }),
    [debouncedHost, method, debouncedStatus, debouncedQuery, bodySearch, debouncedRespMin, debouncedRespMax, sortCol, sortOrder, page],
  )

  const { data, isLoading, error } = useTraffic(filters)
  const { data: detail, isLoading: detailLoading } = useTrafficDetail(selectedId)

  const rows = data?.rows ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / PAGE_SIZE)
  const rangeStart = page * PAGE_SIZE + 1
  const rangeEnd = Math.min((page + 1) * PAGE_SIZE, total)

  useEffect(() => {
    if (rows.length > 0 && selectedId === null) {
      setSelectedId(rows[0].id)
    }
  }, [rows, selectedId])

  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement).tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return
      if (e.key === 'Escape' && selectedId !== null) {
        setSelectedId(null)
        return
      }
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        if (rows.length === 0) return
        e.preventDefault()
        const currentIdx = selectedId !== null ? rows.findIndex(r => r.id === selectedId) : -1
        let nextIdx: number
        if (e.key === 'ArrowDown') {
          nextIdx = currentIdx < rows.length - 1 ? currentIdx + 1 : currentIdx
        } else {
          nextIdx = currentIdx > 0 ? currentIdx - 1 : 0
        }
        const nextId = rows[nextIdx].id
        setSelectedId(nextId)
        document.querySelector(`[data-row-id="${nextId}"]`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
      }
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [selectedId, rows])

  const toggleSort = useCallback((col: string) => {
    if (sortCol === col) {
      if (sortOrder === 'desc') setSortOrder('asc')
      else { setSortCol(''); setSortOrder('') }
    } else {
      setSortCol(col)
      setSortOrder('desc')
    }
  }, [sortCol, sortOrder])

  const clearFilters = useCallback(() => {
    setHost('')
    setMethod('')
    setStatusInput('')
    setQuery('')
    setBodySearch(false)
    setRespMin('')
    setRespMax('')
    setSortCol('')
    setSortOrder('')
  }, [])

  const hasFilters = host || method || statusInput || query || respMin || respMax

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
      <div className="flex flex-wrap items-center gap-2 md:gap-3 border-b border-slate-200 dark:border-slate-800 px-3 md:px-4 py-2">
        <Input
          placeholder="Host..."
          value={host}
          onChange={e => setHost(e.target.value)}
          className="w-full sm:w-44"
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
          placeholder="Min"
          value={respMin}
          onChange={e => setRespMin(e.target.value)}
          className="w-20"
          type="number"
        />
        <Input
          placeholder="Max"
          value={respMax}
          onChange={e => setRespMax(e.target.value)}
          className="w-20"
          type="number"
        />
        <div className="flex items-center gap-1 w-full sm:w-auto">
          <Input
            placeholder={bodySearch ? 'Search request/response bodies...' : 'Search...'}
            value={query}
            onChange={e => setQuery(e.target.value)}
            className="w-full sm:w-52"
          />
          <button
            type="button"
            onClick={() => setBodySearch(b => !b)}
            className={cn(
              'rounded px-2 py-1 text-[10px] font-medium whitespace-nowrap',
              bodySearch
                ? 'bg-amber-500/20 text-amber-400'
                : 'bg-slate-800 text-slate-500 hover:text-slate-700 dark:text-slate-300',
            )}
            title={bodySearch ? 'Searching request/response bodies' : 'Searching URLs'}
          >
            Body
          </button>
        </div>
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
          <div className="p-3 md:p-4"><TableSkeleton columns={7} rows={12} /></div>
        ) : rows.length === 0 ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-slate-500">
            <Globe className="h-10 w-10 text-slate-400 dark:text-slate-600" />
            <p className="text-lg">No traffic captured</p>
            <p className="text-sm">
              {hasFilters
                ? 'Try adjusting your filters'
                : 'Traffic will appear when the proxy is running'}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
          <Table className="min-w-[800px]">
            <TableHeader>
              <TableRow className="text-[11px]">
                <SortableHead col="time" label="Time" current={sortCol} order={sortOrder} onToggle={toggleSort} className="w-16" />
                <TableHead>Host</TableHead>
                <TableHead className="w-16">Method</TableHead>
                <TableHead>URL</TableHead>
                <SortableHead col="status" label="Status" current={sortCol} order={sortOrder} onToggle={toggleSort} className="w-14" />
                <TableHead>Type</TableHead>
                <SortableHead col="size" label="Size" current={sortCol} order={sortOrder} onToggle={toggleSort} className="w-16 text-right" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map(row => (
                <TableRow
                  key={row.id}
                  data-row-id={row.id}
                  className={cn(
                    'cursor-pointer text-[13px] leading-tight',
                    selectedId === row.id && 'bg-slate-800',
                  )}
                  onClick={() => setSelectedId(selectedId === row.id ? null : row.id)}
                >
                  <TableCell className="whitespace-nowrap text-xs text-slate-500">
                    {relativeTime(row.ts)}
                  </TableCell>
                  <TableCell className="max-w-[200px] truncate font-mono text-xs text-slate-700 dark:text-slate-300">
                    {row.host}
                  </TableCell>
                  <TableCell>
                    <MethodBadge method={row.method} />
                  </TableCell>
                  <TableCell className="max-w-md truncate font-mono text-xs text-slate-500 dark:text-slate-400">
                    <HighlightText text={displayUrl(row)} query={!bodySearch ? debouncedQuery : ''} />
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
          </div>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between border-t border-slate-200 dark:border-slate-800 px-4 py-2">
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
            className="h-1.5 cursor-row-resize border-t border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700"
            onMouseDown={onDragStart}
          />
          <div style={{ height: detailHeight, maxHeight: '60vh' }} className="flex flex-col bg-white dark:bg-slate-900">
            <div className="flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 px-4 py-1.5">
              <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
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
                <div className="flex flex-1 items-center justify-center"><Spinner className="h-8 w-8" /></div>
              ) : (
                <>
                  <HttpPanel label="Request" content={detail.request} showCurl />
                  <div className="w-px bg-slate-200 dark:bg-slate-800" />
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

function HttpPanel({ label, content, showCurl }: { label: string; content: string; showCurl?: boolean }) {
  const [copied, setCopied] = useState(false)
  const [copiedCurl, setCopiedCurl] = useState(false)
  const formatted = useMemo(() => formatHttp(content), [content])

  const copy = useCallback(() => {
    navigator.clipboard.writeText(content).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }, [content])

  const copyCurl = useCallback(() => {
    const cmd = buildCurl(content)
    if (!cmd) return
    navigator.clipboard.writeText(cmd).then(() => {
      setCopiedCurl(true)
      setTimeout(() => setCopiedCurl(false), 1500)
    })
  }, [content])

  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <div className="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-3 py-1">
        <span className="text-xs font-medium text-slate-500">{label}</span>
        <div className="flex items-center gap-2">
          {showCurl && (
            <button
              type="button"
              onClick={copyCurl}
              className="flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-700 dark:text-slate-300"
            >
              <Terminal className="h-3 w-3" />
              {copiedCurl ? 'Copied' : 'curl'}
            </button>
          )}
          <button
            type="button"
            onClick={copy}
            className="flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-700 dark:text-slate-300"
          >
            <Clipboard className="h-3 w-3" />
            {copied ? 'Copied' : 'Copy'}
          </button>
        </div>
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
  if (!raw) return <span className="text-slate-400 dark:text-slate-600">Empty</span>

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
          <span className="font-semibold text-slate-800 dark:text-slate-200">{line}</span>
          {i < lines.length - 1 ? '\n' : ''}
        </span>
      )
    }
    const name = line.slice(0, colonIdx)
    const value = line.slice(colonIdx)
    return (
      <span key={i}>
        <span className="text-blue-400">{name}</span>
        <span className="text-slate-500 dark:text-slate-400">{value}</span>
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
      return highlightJson(formatted)
    } catch {
      // not valid JSON
    }
  }
  if (trimmed.startsWith('<')) {
    return highlightMarkup(trimmed)
  }
  return <span className="text-slate-700 dark:text-slate-300">{body}</span>
}

function highlightJson(json: string): React.ReactNode {
  const nodes: React.ReactNode[] = []
  const re = /("(?:[^"\\]|\\.)*")(\s*:)?|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|(\btrue\b|\bfalse\b|\bnull\b)|([{}[\]:,])/g
  let m: RegExpExecArray | null
  let last = 0
  let k = 0

  while ((m = re.exec(json)) !== null) {
    if (m.index > last) nodes.push(json.slice(last, m.index))
    last = re.lastIndex

    if (m[1]) {
      if (m[2]) {
        nodes.push(<span key={k++} className="text-cyan-400">{m[1]}</span>)
        nodes.push(m[2])
      } else {
        nodes.push(<span key={k++} className="text-emerald-400">{m[1]}</span>)
      }
    } else if (m[3]) {
      nodes.push(<span key={k++} className="text-amber-400">{m[3]}</span>)
    } else if (m[4]) {
      nodes.push(<span key={k++} className="text-purple-400">{m[4]}</span>)
    } else if (m[5]) {
      nodes.push(<span key={k++} className="text-slate-500">{m[5]}</span>)
    }
  }

  if (last < json.length) nodes.push(json.slice(last))
  return <>{nodes}</>
}

function highlightMarkup(html: string): React.ReactNode {
  const nodes: React.ReactNode[] = []
  const tagRe = /<\/?[a-zA-Z][\w:.-]*(?:\s[^>]*)?\/?>/g
  let m: RegExpExecArray | null
  let last = 0
  let k = 0

  while ((m = tagRe.exec(html)) !== null) {
    if (m.index > last) nodes.push(html.slice(last, m.index))
    last = tagRe.lastIndex

    const tag = m[0]
    const tm = tag.match(/^(<\/?)([a-zA-Z][\w:.-]*)([\s\S]*?)(\/?>)$/)
    if (!tm) { nodes.push(tag); continue }

    nodes.push(<span key={k++} className="text-slate-500">{tm[1]}</span>)
    nodes.push(<span key={k++} className="text-blue-400">{tm[2]}</span>)

    if (tm[3]) {
      const attrRe = /(\s+)([\w:.-]+)(?:(=)("[^"]*"|'[^']*'|\S+))?/g
      let am: RegExpExecArray | null
      let aLast = 0
      while ((am = attrRe.exec(tm[3])) !== null) {
        if (am.index > aLast) nodes.push(tm[3].slice(aLast, am.index))
        aLast = attrRe.lastIndex
        nodes.push(am[1])
        nodes.push(<span key={k++} className="text-amber-400">{am[2]}</span>)
        if (am[3]) {
          nodes.push(am[3])
          nodes.push(<span key={k++} className="text-emerald-400">{am[4]}</span>)
        }
      }
      if (aLast < tm[3].length) nodes.push(tm[3].slice(aLast))
    }

    nodes.push(<span key={k++} className="text-slate-500">{tm[4]}</span>)
  }

  if (last < html.length) nodes.push(html.slice(last))
  return <>{nodes}</>
}

function buildCurl(request: string): string {
  const lines = request.split(/\r?\n/)
  if (lines.length === 0) return ''
  const [method, path] = lines[0].split(' ')
  if (!method || !path) return ''
  const hostLine = lines.find(l => l.toLowerCase().startsWith('host:'))
  const host = hostLine?.split(':').slice(1).join(':').trim() ?? ''
  const scheme = /(?:^|:)443$/.test(host) ? 'https' : 'http'
  const url = `${scheme}://${host}${path}`

  const esc = (s: string) => s.replace(/'/g, "'\\''")

  const headers = lines.slice(1).filter(l => l.includes(':') && !l.toLowerCase().startsWith('host:'))
  const headerFlags = headers.map(h => `-H '${esc(h)}'`).join(' ')

  const bodyStart = request.indexOf('\r\n\r\n')
  const body = bodyStart >= 0 ? request.slice(bodyStart + 4).trim() : ''

  let cmd = `curl -X ${method} '${esc(url)}'`
  if (headerFlags) cmd += ` ${headerFlags}`
  if (body) cmd += ` -d '${esc(body)}'`
  return cmd
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function HighlightText({ text, query }: { text: string; query: string }) {
  if (!query) return <>{text}</>
  const parts = text.split(new RegExp(`(${escapeRegex(query)})`, 'gi'))
  return (
    <>
      {parts.map((p, i) =>
        p.toLowerCase() === query.toLowerCase()
          ? <mark key={i} className="bg-amber-500/30 text-inherit rounded-sm px-0.5">{p}</mark>
          : p
      )}
    </>
  )
}

function SortableHead({
  col, label, current, order, onToggle, className,
}: {
  col: string
  label: string
  current: string
  order: 'asc' | 'desc' | ''
  onToggle: (col: string) => void
  className?: string
}) {
  const active = current === col
  const Icon = active ? (order === 'asc' ? ArrowUp : ArrowDown) : ArrowUpDown
  return (
    <TableHead className={cn('cursor-pointer select-none', className)} onClick={() => onToggle(col)}>
      <span className="inline-flex items-center gap-0.5">
        {label}
        <Icon className={cn('h-3 w-3', active ? 'text-slate-700 dark:text-slate-300' : 'text-slate-600')} />
      </span>
    </TableHead>
  )
}
