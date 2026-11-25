import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { GitBranch, Search, Server, ShieldAlert } from 'lucide-react'
import { useSearch, type SearchResult } from '../hooks/useSearch'
import { navGroups } from '../lib/nav'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from './ui/command'

const TYPE_ICONS = {
  finding: ShieldAlert,
  asset: Server,
  node: GitBranch,
} as const

const TYPE_LABELS = {
  finding: 'Findings',
  asset: 'Assets',
  node: 'Nodes',
} as const

function groupResults(results: SearchResult[]) {
  const groups: { type: SearchResult['type']; items: SearchResult[] }[] = []
  for (const r of results) {
    const group = groups.find(g => g.type === r.type)
    if (group) group.items.push(r)
    else groups.push({ type: r.type, items: [r] })
  }
  return groups
}

export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')

  const searching = debouncedQuery.length >= 2
  const { data } = useSearch(debouncedQuery)
  const groups = groupResults(data?.results ?? [])

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 200)
    return () => clearTimeout(timer)
  }, [query])

  useEffect(() => {
    if (open) {
      setQuery('')
      setDebouncedQuery('')
    }
  }, [open])

  const handleDialogKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      onClose()
    }
  }, [onClose])

  if (!open) return null

  function go(path: string) {
    navigate(path)
    onClose()
  }

  const q = query.toLowerCase()

  return (
    <div
      role="dialog"
      aria-label="Command palette"
      className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh] bg-black/50 backdrop-blur-sm"
      onKeyDownCapture={handleDialogKeyDown}
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
    >
      <div className="mx-4 w-full max-w-lg overflow-hidden rounded-xl border border-border bg-popover shadow-2xl">
        <Command shouldFilter={false} loop>
          <CommandInput
            value={query}
            onValueChange={setQuery}
            placeholder="Search findings, assets, nodes..."
            aria-label="Search commands and content"
            autoFocus
          />
          <CommandList>
            {searching ? (
              <>
                {groups.length === 0 && <CommandEmpty>No results found</CommandEmpty>}
                {groups.map(group => {
                  const Icon = TYPE_ICONS[group.type]
                  return (
                    <CommandGroup key={group.type} heading={TYPE_LABELS[group.type]}>
                      {group.items.map(item => (
                        <CommandItem
                          key={`${item.type}-${item.id}`}
                          value={`${item.type}-${item.id}`}
                          onSelect={() => go(item.url)}
                        >
                          <Icon />
                          <div className="min-w-0 flex-1">
                            <div className="truncate">{item.title}</div>
                            <div className="truncate text-xs text-muted-foreground">{item.subtitle}</div>
                          </div>
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  )
                })}
                {groups.length > 0 && (
                  <CommandGroup>
                    <CommandItem value="view-all" onSelect={() => go(`/search?q=${encodeURIComponent(query)}`)}>
                      <Search />
                      <span>View all results for &ldquo;{query}&rdquo;</span>
                    </CommandItem>
                  </CommandGroup>
                )}
              </>
            ) : (
              <>
                {navGroups.map(group => {
                  const items = group.items.filter(i => i.label.toLowerCase().includes(q))
                  if (items.length === 0) return null
                  return (
                    <CommandGroup key={group.label} heading={group.label}>
                      {items.map(({ to, icon: Icon, label }) => (
                        <CommandItem key={to} value={to} onSelect={() => go(to)}>
                          <Icon />
                          <span>{label}</span>
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  )
                })}
                {query.length > 0 && !navGroups.some(g => g.items.some(i => i.label.toLowerCase().includes(q))) && (
                  <CommandEmpty>No matching actions</CommandEmpty>
                )}
              </>
            )}
          </CommandList>
        </Command>
        <div className="flex items-center gap-4 border-t border-border px-4 py-2 text-xs text-muted-foreground">
          <span><kbd className="font-mono">↑↓</kbd> navigate</span>
          <span><kbd className="font-mono">↵</kbd> select</span>
          <span><kbd className="font-mono">esc</kbd> close</span>
        </div>
      </div>
    </div>
  )
}
