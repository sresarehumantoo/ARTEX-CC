import { useState, useEffect } from 'react'
import { NavLink } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { LayoutDashboard, GitBranch, Shield, Globe, Clock, Server, Target, FileText, KeyRound, Settings as SettingsIcon, Sun, Moon, ChevronsLeft, ChevronsRight } from 'lucide-react'
import { cn } from '../lib/utils'
import { useTheme } from '../hooks/useTheme'
import { getEngagement } from '../lib/api'
import { useSSEStatus } from '../hooks/useSSEStatus'

const links = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/graph', icon: GitBranch, label: 'Graph' },
  { to: '/findings', icon: Shield, label: 'Findings', badgeKey: 'findings' as const },
  { to: '/traffic', icon: Globe, label: 'Traffic' },
  { to: '/activity', icon: Clock, label: 'Activity' },
  { to: '/assets', icon: Server, label: 'Assets', badgeKey: 'assets' as const },
  { to: '/coverage', icon: Target, label: 'Coverage' },
  { to: '/credentials', icon: KeyRound, label: 'Credentials' },
  { to: '/report', icon: FileText, label: 'Report' },
  { to: '/settings', icon: SettingsIcon, label: 'Settings' },
]

function getInitialCollapsed(): boolean {
  try {
    return localStorage.getItem('sidebar-collapsed') === 'true'
  } catch {
    return false
  }
}

function formatRelativeTime(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000)
  if (seconds < 5) return 'just now'
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ago`
}

function hasHighSeverity(findingList: Record<string, unknown>[]): boolean {
  return findingList.some((f) => {
    const sev = f.severity as string
    return sev === 'critical' || sev === 'high'
  })
}

export function Sidebar() {
  const { theme, toggleTheme } = useTheme()
  const [collapsed, setCollapsed] = useState(getInitialCollapsed)
  const sseStatus = useSSEStatus()
  const [, tick] = useState(0)

  useEffect(() => {
    if (!sseStatus.lastEvent) return
    const id = setInterval(() => tick((n) => n + 1), 15_000)
    return () => clearInterval(id)
  }, [sseStatus.lastEvent])

  const { data: engagement } = useQuery({
    queryKey: ['engagement'],
    queryFn: getEngagement,
    refetchInterval: 30_000,
  })

  function toggleCollapsed() {
    setCollapsed(prev => {
      const next = !prev
      try { localStorage.setItem('sidebar-collapsed', String(next)) } catch {}
      return next
    })
  }

  const showLabels = collapsed ? 'hidden' : 'hidden md:inline'
  const showBlock = collapsed ? 'hidden' : 'hidden md:block'

  const badges: Record<string, { count: number; urgent: boolean }> = {}
  if (engagement) {
    if (engagement.findings_total > 0) {
      badges.findings = {
        count: engagement.findings_total,
        urgent: hasHighSeverity(engagement.finding_list),
      }
    }
    if (engagement.facts > 0) {
      badges.assets = { count: engagement.facts, urgent: false }
    }
  }

  return (
    <aside className={cn(
      'flex h-screen flex-col bg-slate-50 dark:bg-slate-950 border-r border-slate-200 dark:border-slate-800 transition-all duration-200',
      collapsed ? 'w-14' : 'w-14 md:w-56',
    )}>
      <div className={cn('flex items-center gap-2 px-3 py-5', !collapsed && 'md:px-5')}>
        <Shield className="h-6 w-6 shrink-0 text-emerald-500" />
        <span className={cn(showBlock, 'text-lg font-semibold tracking-tight text-slate-900 dark:text-slate-100')}>
          cc-artex
        </span>
      </div>
      <nav className={cn('flex-1 space-y-1 px-2 py-2', !collapsed && 'md:px-3')}>
        {links.map(({ to, icon: Icon, label, badgeKey }) => {
          const badge = badgeKey ? badges[badgeKey] : undefined
          return (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-slate-200 dark:bg-slate-800 text-slate-900 dark:text-slate-100'
                    : 'text-slate-500 dark:text-slate-400 hover:bg-slate-200/50 dark:hover:bg-slate-800/50 hover:text-slate-800 dark:hover:text-slate-200',
                )
              }
            >
              <span className="relative shrink-0">
                <Icon className="h-4 w-4" />
                {badge && (
                  <span
                    className={cn(
                      'absolute -top-1 -right-1.5 block h-2 w-2 rounded-full md:hidden',
                      badge.urgent
                        ? 'bg-rose-500'
                        : 'bg-slate-400 dark:bg-slate-500',
                    )}
                  />
                )}
              </span>
              <span className={showLabels}>{label}</span>
              {badge && (
                <span
                  className={cn(
                    'ml-auto items-center justify-center rounded-full px-1.5 py-0.5 text-[10px] font-semibold leading-none',
                    collapsed ? 'hidden' : 'hidden md:inline-flex',
                    badge.urgent
                      ? 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300'
                      : 'bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-300',
                  )}
                >
                  {badge.count}
                </span>
              )}
            </NavLink>
          )
        })}
      </nav>
      <div className={cn('px-2 py-2', !collapsed && 'md:px-3')}>
        <div className="flex items-center gap-3 px-3 py-1.5">
          <span
            className={cn(
              'block h-2 w-2 shrink-0 rounded-full',
              sseStatus.connected ? 'bg-emerald-500' : 'bg-rose-500',
            )}
            title={sseStatus.connected ? 'Connected' : 'Disconnected'}
          />
          <span className={cn(showLabels, 'text-xs text-slate-500 dark:text-slate-400')}>
            {sseStatus.connected ? 'Connected' : 'Disconnected'}
          </span>
        </div>
        {sseStatus.lastEvent && (
          <div className={cn('px-3 py-0.5', collapsed ? 'hidden' : 'hidden md:block')}>
            <span className="text-[10px] text-slate-400 dark:text-slate-500">
              Updated {formatRelativeTime(sseStatus.lastEvent)}
            </span>
          </div>
        )}
      </div>
      <div className={cn('px-2 py-3', !collapsed && 'md:px-3')}>
        <button
          type="button"
          onClick={toggleCollapsed}
          className="hidden md:flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-slate-500 dark:text-slate-400 transition-colors hover:bg-slate-200/50 dark:hover:bg-slate-800/50 hover:text-slate-800 dark:hover:text-slate-200"
        >
          {collapsed ? <ChevronsRight className="h-4 w-4 shrink-0" /> : <ChevronsLeft className="h-4 w-4 shrink-0" />}
          <span className={showLabels}>{collapsed ? 'Expand' : 'Collapse'}</span>
        </button>
        <button
          type="button"
          onClick={toggleTheme}
          className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-slate-500 dark:text-slate-400 transition-colors hover:bg-slate-200/50 dark:hover:bg-slate-800/50 hover:text-slate-800 dark:hover:text-slate-200"
        >
          {theme === 'dark' ? (
            <Sun className="h-4 w-4 shrink-0" />
          ) : (
            <Moon className="h-4 w-4 shrink-0" />
          )}
          <span className={showLabels}>{theme === 'dark' ? 'Light' : 'Dark'}</span>
        </button>
      </div>
    </aside>
  )
}
