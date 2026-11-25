import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Clock, Download, Search, X } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { StatusBadge } from '../components/StatusBadge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Select } from '../components/ui/select'
import { ActivitySkeleton } from '../components/skeletons/ActivitySkeleton'
import { getActivity } from '../lib/api'
import { downloadFile } from '../lib/export'
import { toast } from 'sonner'
import type { Activity as ActivityType } from '../lib/types'

function exportActivityCSV(items: ActivityType[]) {
  const headers = ['Timestamp', 'Kind', 'Tool', 'Summary', 'Detail', 'Node ID', 'Worker', 'Error']
  const rows = items.map(a => [
    a.created_at ? new Date(a.created_at).toISOString() : '',
    a.kind,
    a.tool ?? '',
    a.summary ?? '',
    a.detail ?? '',
    a.node_id != null ? String(a.node_id) : '',
    a.worker ?? '',
    a.is_error ? 'true' : 'false',
  ])
  const csv = [headers, ...rows].map(r => r.map(v => `"${v.replace(/"/g, '""')}"`).join(',')).join('\n')
  downloadFile(csv, `activity-${new Date().toISOString().slice(0, 10)}.csv`, 'text/csv')
}

function relativeTime(iso: string): string {
  const seconds = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

const PAGE_SIZE = 50

function ActivityItem({ item }: { item: ActivityType }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border-b border-border last:border-0">
      <button
        type="button"
        onClick={() => setExpanded(v => !v)}
        className="flex w-full items-start gap-2 md:gap-3 px-3 md:px-4 py-3 text-left hover:bg-accent transition-colors"
      >
        <div className="mt-1.5 shrink-0">
          <span
            className={`block h-2 w-2 rounded-full ${item.is_error ? 'bg-rose-500' : 'bg-emerald-500'}`}
          />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-medium text-foreground truncate max-w-[150px] sm:max-w-none">
              {item.tool}
            </span>
            <StatusBadge domain="activity" value={item.kind} />
            {item.is_error && <Badge variant="critical">error</Badge>}
            {item.worker && (
              <span className="text-xs text-muted-foreground">
                {item.worker}
              </span>
            )}
          </div>
          <p className="mt-0.5 text-sm text-muted-foreground line-clamp-2">
            {item.summary}
          </p>
        </div>
        <div className="shrink-0 flex items-center gap-2">
          {item.created_at && (
            <span className="text-xs text-muted-foreground whitespace-nowrap">
              {relativeTime(item.created_at)}
            </span>
          )}
          {item.detail && (
            <span className="text-muted-foreground">
              {expanded ? (
                <ChevronDown className="h-4 w-4" />
              ) : (
                <ChevronRight className="h-4 w-4" />
              )}
            </span>
          )}
        </div>
      </button>
      {expanded && item.detail && (
        <div className="px-3 md:px-4 pb-3 pl-7 md:pl-9">
          <pre className="whitespace-pre-wrap break-words rounded-md bg-muted p-2 md:p-3 text-xs text-foreground max-h-96 overflow-y-auto overflow-x-auto">
            {item.detail}
          </pre>
        </div>
      )}
    </div>
  )
}

function NewEventsBar({ count, onClick }: { count: number; onClick: () => void }) {
  if (count <= 0) return null
  return (
    <button
      type="button"
      onClick={onClick}
      className="sticky top-0 z-10 flex w-full items-center justify-center gap-2 bg-emerald-500/10 border-b border-emerald-500/20 py-1.5 text-xs font-medium text-emerald-600 dark:text-emerald-400 hover:bg-emerald-500/20 transition-colors"
    >
      <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
      {count} new event{count !== 1 ? 's' : ''}
    </button>
  )
}

export default function Activity() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [limit, setLimit] = useState(PAGE_SIZE)
  const [kindFilter, setKindFilter] = useState(() => searchParams.get('kind') ?? '')
  const [errorsOnly, setErrorsOnly] = useState(() => searchParams.get('errors') === 'true')
  const [toolFilter, setToolFilter] = useState(() => searchParams.get('tool') ?? '')
  const [searchText, setSearchText] = useState(() => searchParams.get('q') ?? '')
  const [seenCount, setSeenCount] = useState(0)
  const scrollRef = useRef<HTMLDivElement>(null)

  const { data, isLoading, isError, error, dataUpdatedAt, refetch } = useQuery({
    queryKey: ['activity', limit, kindFilter],
    queryFn: () =>
      getActivity({
        limit: String(limit),
        kind: kindFilter || undefined,
      }),
    refetchInterval: 10_000,
  })

  useEffect(() => {
    setSeenCount(0)
  }, [kindFilter, limit])

  useEffect(() => {
    if (data && seenCount === 0) {
      setSeenCount(data.length)
    }
  }, [data, seenCount])

  useEffect(() => {
    const timer = setTimeout(() => {
      const next = new URLSearchParams()
      if (kindFilter) next.set('kind', kindFilter)
      if (errorsOnly) next.set('errors', 'true')
      if (toolFilter) next.set('tool', toolFilter)
      if (searchText) next.set('q', searchText)
      setSearchParams(next, { replace: true })
    }, 300)
    return () => clearTimeout(timer)
  }, [kindFilter, errorsOnly, toolFilter, searchText, setSearchParams])

  const newCount = (data?.length ?? 0) - seenCount

  function handleNewEventsClick() {
    setSeenCount(data?.length ?? 0)
    scrollRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const clearFilters = useCallback(() => {
    setKindFilter('')
    setErrorsOnly(false)
    setToolFilter('')
    setSearchText('')
    setSeenCount(data?.length ?? 0)
    setSearchParams({}, { replace: true })
  }, [data, setSearchParams])

  const uniqueKinds = useMemo(() => {
    if (!data) return []
    const kinds = new Set(data.map(a => a.kind))
    return Array.from(kinds).sort()
  }, [data])

  const uniqueTools = useMemo(() => {
    if (!data) return []
    const tools = new Set(data.map(a => a.tool).filter(Boolean))
    return Array.from(tools).sort()
  }, [data])

  const kindCounts = useMemo(() => {
    if (!data) return {} as Record<string, number>
    const counts: Record<string, number> = {}
    for (const a of data) {
      counts[a.kind] = (counts[a.kind] ?? 0) + 1
    }
    return counts
  }, [data])

  const filtered = useMemo(() => {
    if (!data) return []
    let items = data
    if (errorsOnly) {
      items = items.filter(a => a.is_error)
    }
    if (toolFilter) {
      items = items.filter(a => a.tool === toolFilter)
    }
    if (searchText) {
      const q = searchText.toLowerCase()
      items = items.filter(
        a =>
          (a.summary ?? '').toLowerCase().includes(q) ||
          (a.tool ?? '').toLowerCase().includes(q) ||
          (a.worker ?? '').toLowerCase().includes(q),
      )
    }
    return items
  }, [data, errorsOnly, toolFilter, searchText])

  const hasFilters = kindFilter || errorsOnly || toolFilter || searchText
  const isLive = dataUpdatedAt > 0

  return (
    <div className="space-y-4 p-3 md:p-6">
      <div className="flex items-center gap-3">
        <h1 className="text-lg md:text-2xl font-semibold text-foreground">
          Activity Log
        </h1>
        {isLive && (
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
            </span>
            Live
          </span>
        )}
        <div className="ml-auto">
          <Button
            variant="outline"
            size="sm"
            disabled={!filtered.length}
            onClick={() => {
              exportActivityCSV(filtered)
              toast.success('Activity exported')
            }}
          >
            <Download className="h-4 w-4 mr-1" />
            Download
          </Button>
        </div>
      </div>

      <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-2">
        <div className="relative min-w-0 w-full sm:w-auto sm:flex-1 sm:min-w-[200px] sm:max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search summary, tool, worker..."
            value={searchText}
            onChange={e => setSearchText(e.target.value)}
            className="pl-9"
          />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Select
            value={kindFilter}
            onChange={e => setKindFilter(e.target.value)}
          >
            <option value="">All kinds</option>
            {uniqueKinds.map(k => (
              <option key={k} value={k}>
                {k}
              </option>
            ))}
          </Select>
          <Select
            value={toolFilter}
            onChange={e => setToolFilter(e.target.value)}
          >
            <option value="">All tools</option>
            {uniqueTools.map(t => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </Select>
          <Button
            variant={errorsOnly ? 'default' : 'outline'}
            size="sm"
            onClick={() => setErrorsOnly(v => !v)}
          >
            Errors only
          </Button>
          {hasFilters && (
            <Button variant="ghost" size="sm" onClick={clearFilters}>
              <X className="h-4 w-4 mr-1" />
              Clear
            </Button>
          )}
        </div>
      </div>

      {Object.keys(kindCounts).length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 px-3 md:px-4 py-1.5 border-b border-border text-xs">
          {Object.entries(kindCounts).map(([kind, count]) => (
            <span key={kind} className="inline-flex items-center gap-1">
              <StatusBadge domain="activity" value={kind} />
              <span className="tabular-nums text-muted-foreground">{count}</span>
            </span>
          ))}
        </div>
      )}

      {isLoading && <ActivitySkeleton />}

      {isError && (
        <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
          <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load activity: {(error as Error).message}</p>
          <button
            type="button"
            onClick={() => refetch()}
            className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
          >
            Try again
          </button>
        </div>
      )}

      {data && (
        <>
          <div className="text-xs text-muted-foreground">
            Showing {filtered.length} of {data.length} entries
          </div>
          <div ref={scrollRef} className="rounded-lg border border-border bg-background overflow-hidden">
            <NewEventsBar count={newCount} onClick={handleNewEventsClick} />
            {filtered.length === 0 ? (
              <div className="flex h-64 flex-col items-center justify-center gap-3 text-muted-foreground">
                <Clock className="h-12 w-12 text-slate-300 dark:text-slate-700" />
                <div className="text-center">
                  <p className="text-lg font-medium text-muted-foreground">No activity recorded</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {hasFilters
                      ? 'Try adjusting your filters'
                      : 'Activity events will appear as the engagement runs'}
                  </p>
                </div>
              </div>
            ) : (
              filtered.map(item => (
                <ActivityItem key={item.id} item={item} />
              ))
            )}
          </div>
          {data.length >= limit && (
            <div className="text-center">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setLimit(prev => prev + PAGE_SIZE)}
              >
                Load more
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
