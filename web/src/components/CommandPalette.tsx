import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Clock,
  FileText,
  GitBranch,
  Globe,
  KeyRound,
  LayoutDashboard,
  Moon,
  Search,
  Server,
  ShieldAlert,
  Sun,
  Target,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useSearch, type SearchResult } from '../hooks/useSearch'
import { useTheme } from '../hooks/useTheme'

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

interface QuickAction {
  id: string
  label: string
  subtitle: string
  icon: LucideIcon
  action: () => void
}

function groupResults(results: SearchResult[]) {
  const groups: { type: SearchResult['type']; label: string; items: SearchResult[] }[] = []
  const seen = new Set<string>()
  for (const r of results) {
    if (!seen.has(r.type)) {
      seen.add(r.type)
      groups.push({ type: r.type, label: TYPE_LABELS[r.type], items: [] })
    }
    groups.find(g => g.type === r.type)!.items.push(r)
  }
  return groups
}

export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate()
  const { theme, toggleTheme } = useTheme()
  const inputRef = useRef<HTMLInputElement>(null)
  const overlayRef = useRef<HTMLDivElement>(null)
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)

  const { data } = useSearch(debouncedQuery)
  const results = data?.results ?? []
  const groups = groupResults(results)

  const quickActions: QuickAction[] = useMemo(() => {
    function go(path: string) {
      return () => { navigate(path); onClose() }
    }
    return [
      { id: 'nav-dashboard',    label: 'Dashboard',    subtitle: 'Overview and stats',     icon: LayoutDashboard, action: go('/') },
      { id: 'nav-graph',        label: 'Graph',        subtitle: 'Attack graph',           icon: GitBranch,       action: go('/graph') },
      { id: 'nav-findings',     label: 'Findings',     subtitle: 'Vulnerabilities found',  icon: ShieldAlert,     action: go('/findings') },
      { id: 'nav-traffic',      label: 'Traffic',      subtitle: 'Captured requests',      icon: Globe,           action: go('/traffic') },
      { id: 'nav-activity',     label: 'Activity',     subtitle: 'Recent actions',         icon: Clock,           action: go('/activity') },
      { id: 'nav-assets',       label: 'Assets',       subtitle: 'Hosts and services',     icon: Server,          action: go('/assets') },
      { id: 'nav-coverage',     label: 'Coverage',     subtitle: 'Testing coverage',       icon: Target,          action: go('/coverage') },
      { id: 'nav-credentials',  label: 'Credentials',  subtitle: 'Stored credentials',     icon: KeyRound,        action: go('/credentials') },
      { id: 'nav-report',       label: 'Report',       subtitle: 'Generate report',        icon: FileText,        action: go('/report') },
      { id: 'toggle-theme',     label: 'Toggle theme', subtitle: `Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`, icon: theme === 'dark' ? Sun : Moon, action: () => { toggleTheme(); onClose() } },
    ]
  }, [navigate, onClose, theme, toggleTheme])

  const filteredActions = useMemo(() => {
    if (debouncedQuery.length >= 2) return []
    if (query.length === 0) return quickActions
    const q = query.toLowerCase()
    return quickActions.filter(a => a.label.toLowerCase().includes(q))
  }, [query, debouncedQuery, quickActions])

  const activeItemCount = debouncedQuery.length >= 2 ? results.length : filteredActions.length

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 200)
    return () => clearTimeout(timer)
  }, [query])

  useEffect(() => {
    setSelectedIndex(0)
  }, [debouncedQuery, query])

  useEffect(() => {
    if (open) {
      setQuery('')
      setDebouncedQuery('')
      setSelectedIndex(0)
      setTimeout(() => inputRef.current?.focus(), 0)
    }
  }, [open])

  const selectResult = useCallback((result: SearchResult) => {
    navigate(result.url)
    onClose()
  }, [navigate, onClose])

  useEffect(() => {
    if (!open) return
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.preventDefault()
        e.stopPropagation()
        onClose()
        return
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex(i => Math.min(i + 1, activeItemCount - 1))
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex(i => Math.max(i - 1, 0))
        return
      }
      if (e.key === 'Enter' && activeItemCount > 0) {
        e.preventDefault()
        if (debouncedQuery.length >= 2) {
          selectResult(results[selectedIndex])
        } else {
          filteredActions[selectedIndex]?.action()
        }
      }
    }
    document.addEventListener('keydown', handleKey, true)
    return () => document.removeEventListener('keydown', handleKey, true)
  }, [open, activeItemCount, debouncedQuery, results, filteredActions, selectedIndex, selectResult, onClose])

  if (!open) return null

  let flatIndex = -1

  return (
    <div
      ref={overlayRef}
      role="dialog"
      aria-label="Command palette"
      className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh] bg-black/50 backdrop-blur-sm"
      onClick={e => { if (e.target === overlayRef.current) onClose() }}
    >
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl shadow-2xl max-w-lg w-full mx-4 overflow-hidden">
        <div className="border-b border-slate-200 dark:border-slate-700">
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search findings, assets, nodes..."
            aria-label="Search commands and content"
            className="w-full bg-transparent px-4 py-3 text-sm text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 outline-none"
          />
        </div>

        <div className="max-h-72 overflow-y-auto">
          {debouncedQuery.length >= 2 && results.length === 0 && (
            <div className="px-4 py-8 text-center text-sm text-slate-500">
              No results found
            </div>
          )}

          {groups.map(group => (
            <div key={group.type}>
              <div className="px-4 pt-3 pb-1 text-xs font-medium uppercase tracking-wider text-slate-400 dark:text-slate-500">
                {group.label}
              </div>
              {group.items.map(item => {
                flatIndex++
                const idx = flatIndex
                const Icon = TYPE_ICONS[item.type]
                return (
                  <button
                    key={`${item.type}-${item.id}`}
                    className={`w-full flex items-center gap-3 px-4 py-2 text-left transition-colors ${
                      idx === selectedIndex
                        ? 'bg-slate-100 dark:bg-slate-800'
                        : 'hover:bg-slate-50 dark:hover:bg-slate-800/50'
                    }`}
                    onClick={() => selectResult(item)}
                    onMouseEnter={() => setSelectedIndex(idx)}
                  >
                    <Icon className="h-4 w-4 shrink-0 text-slate-400 dark:text-slate-500" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm text-slate-900 dark:text-slate-100">
                        {item.title}
                      </div>
                      <div className="truncate text-xs text-slate-500 dark:text-slate-400">
                        {item.subtitle}
                      </div>
                    </div>
                  </button>
                )
              })}
            </div>
          ))}

          {groups.length > 0 && (
            <button
              className="w-full flex items-center gap-3 px-4 py-2 text-left hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors border-t border-slate-100 dark:border-slate-800"
              onClick={() => {
                navigate(`/search?q=${encodeURIComponent(query)}`)
                onClose()
              }}
            >
              <Search className="h-4 w-4 shrink-0 text-slate-400" />
              <span className="text-sm text-slate-600 dark:text-slate-400">
                View all results for &ldquo;{query}&rdquo;
              </span>
            </button>
          )}

          {debouncedQuery.length < 2 && filteredActions.length > 0 && (
            <div>
              <div className="px-4 pt-3 pb-1 text-xs font-medium uppercase tracking-wider text-slate-400 dark:text-slate-500">
                Quick Actions
              </div>
              {filteredActions.map((action, idx) => {
                const Icon = action.icon
                return (
                  <button
                    key={action.id}
                    className={`w-full flex items-center gap-3 px-4 py-2 text-left transition-colors ${
                      idx === selectedIndex
                        ? 'bg-slate-100 dark:bg-slate-800'
                        : 'hover:bg-slate-50 dark:hover:bg-slate-800/50'
                    }`}
                    onClick={() => action.action()}
                    onMouseEnter={() => setSelectedIndex(idx)}
                  >
                    <Icon className="h-4 w-4 shrink-0 text-slate-400 dark:text-slate-500" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm text-slate-900 dark:text-slate-100">
                        {action.label}
                      </div>
                      <div className="truncate text-xs text-slate-500 dark:text-slate-400">
                        {action.subtitle}
                      </div>
                    </div>
                  </button>
                )
              })}
            </div>
          )}

          {debouncedQuery.length < 2 && filteredActions.length === 0 && query.length > 0 && (
            <div className="px-4 py-8 text-center text-sm text-slate-500">
              No matching actions
            </div>
          )}
        </div>

        <div className="border-t border-slate-200 dark:border-slate-700 px-4 py-2 flex items-center gap-4 text-xs text-slate-400 dark:text-slate-500">
          <span><kbd className="font-mono">↑↓</kbd> navigate</span>
          <span><kbd className="font-mono">↵</kbd> select</span>
          <span><kbd className="font-mono">esc</kbd> close</span>
        </div>
      </div>
    </div>
  )
}
