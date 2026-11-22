import { useCallback, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Search, X } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Select } from '../components/ui/select'
import { ActivitySkeleton } from '../components/skeletons/ActivitySkeleton'
import { getActivity } from '../lib/api'
import type { Activity as ActivityType } from '../lib/types'

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
    <div className="border-b border-slate-100 dark:border-slate-800 last:border-0">
      <button
        type="button"
        onClick={() => setExpanded(v => !v)}
        className="flex w-full items-start gap-2 md:gap-3 px-3 md:px-4 py-3 text-left hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors"
      >
        <div className="mt-1.5 shrink-0">
          <span
            className={`block h-2 w-2 rounded-full ${item.is_error ? 'bg-rose-500' : 'bg-emerald-500'}`}
          />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-medium text-slate-900 dark:text-slate-100 truncate max-w-[150px] sm:max-w-none">
              {item.tool}
            </span>
            <Badge variant="default">{item.kind}</Badge>
            {item.is_error && <Badge variant="critical">error</Badge>}
            {item.worker && (
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {item.worker}
              </span>
            )}
          </div>
          <p className="mt-0.5 text-sm text-slate-600 dark:text-slate-400 line-clamp-2">
            {item.summary}
          </p>
        </div>
        <div className="shrink-0 flex items-center gap-2">
          {item.created_at && (
            <span className="text-xs text-slate-400 dark:text-slate-500 whitespace-nowrap">
              {relativeTime(item.created_at)}
            </span>
          )}
          {item.detail && (
            <span className="text-slate-400 dark:text-slate-500">
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
          <pre className="whitespace-pre-wrap break-words rounded-md bg-slate-100 dark:bg-slate-800 p-2 md:p-3 text-xs text-slate-700 dark:text-slate-300 max-h-96 overflow-y-auto overflow-x-auto">
            {item.detail}
          </pre>
        </div>
      )}
    </div>
  )
}

export default function Activity() {
  const [limit, setLimit] = useState(PAGE_SIZE)
  const [kindFilter, setKindFilter] = useState('')
  const [errorsOnly, setErrorsOnly] = useState(false)
  const [toolFilter, setToolFilter] = useState('')
  const [searchText, setSearchText] = useState('')

  const { data, isLoading, isError, error, dataUpdatedAt } = useQuery({
    queryKey: ['activity', limit, kindFilter],
    queryFn: () =>
      getActivity({
        limit: String(limit),
        kind: kindFilter || undefined,
      }),
    refetchInterval: 10_000,
  })

  const clearFilters = useCallback(() => {
    setKindFilter('')
    setErrorsOnly(false)
    setToolFilter('')
    setSearchText('')
  }, [])

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
          a.summary.toLowerCase().includes(q) ||
          a.tool.toLowerCase().includes(q) ||
          a.worker.toLowerCase().includes(q),
      )
    }
    return items
  }, [data, errorsOnly, toolFilter, searchText])

  const hasFilters = kindFilter || errorsOnly || toolFilter || searchText
  const isLive = dataUpdatedAt > 0

  return (
    <div className="space-y-4 p-3 md:p-6">
      <div className="flex items-center gap-3">
        <h1 className="text-lg md:text-2xl font-semibold text-slate-900 dark:text-slate-100">
          Activity Log
        </h1>
        {isLive && (
          <span className="flex items-center gap-1.5 text-xs text-slate-400 dark:text-slate-500">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
            </span>
            Live
          </span>
        )}
      </div>

      <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-2">
        <div className="relative min-w-0 w-full sm:w-auto sm:flex-1 sm:min-w-[200px] sm:max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-slate-400" />
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

      {isLoading && <ActivitySkeleton />}

      {isError && (
        <p className="text-sm text-rose-400">
          Failed to load activity: {(error as Error).message}
        </p>
      )}

      {data && (
        <>
          <div className="text-xs text-slate-400 dark:text-slate-500">
            Showing {filtered.length} of {data.length} entries
          </div>
          <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden">
            {filtered.length === 0 ? (
              <p className="px-4 py-8 text-center text-sm text-slate-500">
                No matching activity.
              </p>
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
