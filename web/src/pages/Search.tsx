import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Search as SearchIcon, Shield, Server, GitBranch, Globe } from 'lucide-react'

interface SearchResult {
  type: string
  id: number | string
  title: string
  subtitle: string
  url: string
}

const TYPE_ICONS: Record<string, typeof SearchIcon> = {
  finding: Shield,
  asset: Server,
  node: GitBranch,
  traffic: Globe,
}

const TYPE_COLORS: Record<string, string> = {
  finding: 'text-rose-500',
  asset: 'text-amber-500',
  node: 'text-blue-500',
  traffic: 'text-emerald-500',
}

async function searchAPI(q: string): Promise<SearchResult[]> {
  const res = await fetch(`/api/search?q=${encodeURIComponent(q)}`)
  if (!res.ok) throw new Error(`Search failed: ${res.status}`)
  const data = await res.json()
  return data.results ?? []
}

export default function Search() {
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''

  const { data: results, isLoading, error } = useQuery({
    queryKey: ['search', query],
    queryFn: () => searchAPI(query),
    enabled: query.length >= 2,
  })

  const grouped = useMemo(() => {
    if (!results) return new Map<string, SearchResult[]>()
    const groups = new Map<string, SearchResult[]>()
    for (const r of results) {
      const list = groups.get(r.type) ?? []
      list.push(r)
      groups.set(r.type, list)
    }
    return groups
  }, [results])

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border bg-muted px-3 md:px-4 py-3">
        <div className="flex items-center gap-3">
          <SearchIcon className="h-5 w-5 text-muted-foreground" />
          <div className="relative flex-1 max-w-lg">
            <input
              type="text"
              value={query}
              onChange={(e) => setSearchParams({ q: e.target.value }, { replace: true })}
              placeholder="Search findings, assets, nodes..."
              className="w-full bg-background border border-border rounded-md py-2 px-3 text-sm text-foreground placeholder:text-slate-400 dark:placeholder:text-slate-600 focus:outline-none focus:ring-1 focus:ring-slate-500"
              autoFocus
            />
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-3 md:p-6">
        {query.length < 2 ? (
          <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
            <SearchIcon className="h-12 w-12 text-slate-300 dark:text-slate-700" />
            <div className="text-center">
              <p className="text-lg font-medium text-muted-foreground">Search the engagement</p>
              <p className="mt-1 text-sm">Type at least 2 characters to search findings, assets, and nodes</p>
            </div>
          </div>
        ) : isLoading ? (
          <div className="space-y-3">
            {[1, 2, 3].map(i => (
              <div key={i} className="animate-pulse flex items-center gap-3 p-3 rounded-lg bg-muted">
                <div className="h-8 w-8 rounded bg-muted" />
                <div className="flex-1 space-y-2">
                  <div className="h-4 w-48 rounded bg-muted" />
                  <div className="h-3 w-32 rounded bg-muted" />
                </div>
              </div>
            ))}
          </div>
        ) : error ? (
          <div className="text-sm text-rose-400">Search failed: {(error as Error).message}</div>
        ) : results && results.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
            <SearchIcon className="h-12 w-12 text-slate-300 dark:text-slate-700" />
            <div className="text-center">
              <p className="text-lg font-medium text-muted-foreground">No results</p>
              <p className="mt-1 text-sm">No matches found for &ldquo;{query}&rdquo;</p>
            </div>
          </div>
        ) : (
          <div className="space-y-6">
            {[...grouped.entries()].map(([type, items]) => {
              const Icon = TYPE_ICONS[type] ?? SearchIcon
              const color = TYPE_COLORS[type] ?? 'text-muted-foreground'
              return (
                <div key={type}>
                  <h2 className="flex items-center gap-2 text-sm font-medium text-foreground mb-2">
                    <Icon className={`h-4 w-4 ${color}`} />
                    <span className="capitalize">{type}s</span>
                    <span className="text-muted-foreground font-normal">({items.length})</span>
                  </h2>
                  <div className="space-y-1">
                    {items.map((r) => (
                      <Link
                        key={`${r.type}-${r.id}`}
                        to={r.url}
                        className="flex items-center gap-3 rounded-lg px-3 py-2.5 hover:bg-accent transition-colors group"
                      >
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium text-foreground group-hover:text-emerald-600 dark:group-hover:text-emerald-400 truncate">
                            {r.title}
                          </p>
                          {r.subtitle && (
                            <p className="text-xs text-muted-foreground truncate mt-0.5">{r.subtitle}</p>
                          )}
                        </div>
                        <span className="text-xs text-muted-foreground shrink-0">#{r.id}</span>
                      </Link>
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
