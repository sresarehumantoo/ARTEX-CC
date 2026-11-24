import { useState, useEffect } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronsLeft, ChevronsRight, Code } from 'lucide-react'
import { BrandMark } from './BrandMark'
import { cn } from '../lib/utils'
import { navGroups } from '../lib/nav'
import { getEngagement } from '../lib/api'
import { SOURCE_URL } from '../lib/source'
import { useSSEStatus } from '../hooks/useSSEStatus'
import { useUnreadFindings, clearUnread } from '../hooks/useUnreadFindings'
import { usePreferences } from '../stores/preferences'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from './ui/collapsible'

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

interface SidebarProps {
  mobileOpen?: boolean
  onMobileClose?: () => void
}

export function Sidebar({ mobileOpen, onMobileClose }: SidebarProps) {
  const { sidebar_collapsed: collapsed, setSidebarCollapsed } = usePreferences()
  const sseStatus = useSSEStatus()
  const unread = useUnreadFindings()
  const location = useLocation()
  const [, tick] = useState(0)
  const [closedGroups, setClosedGroups] = useState<Record<string, boolean>>({})

  useEffect(() => {
    if (location.pathname === '/findings') clearUnread()
  }, [location.pathname])

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

  const showLabels = mobileOpen ? 'inline' : (collapsed ? 'hidden' : 'hidden md:inline')
  const showBlock = mobileOpen ? 'block' : (collapsed ? 'hidden' : 'hidden md:block')

  const badges: Record<string, { count: number; urgent: boolean }> = {}
  if (engagement) {
    if (engagement.findings_total > 0) {
      badges.findings = {
        count: engagement.findings_total,
        urgent: hasHighSeverity(engagement.finding_list),
      }
    }
  }

  return (
    <>
    {mobileOpen && (
      <div
        className="fixed inset-0 z-40 bg-black/50 md:hidden"
        onClick={onMobileClose}
        aria-hidden="true"
      />
    )}
    <aside className={cn(
      'flex flex-col bg-card text-card-foreground border-r border-border transition-all duration-200',
      mobileOpen
        ? 'fixed inset-y-0 left-0 z-50 w-56 shadow-xl'
        : cn('h-full', collapsed ? 'w-14' : 'w-14 md:w-56'),
    )}>
      <div className={cn('flex items-center gap-2 px-3 py-4', !collapsed && 'md:px-5')}>
        <BrandMark className="size-7 shrink-0" />
        <span className={cn(showBlock, 'text-lg font-bold tracking-tight')}>
          ARTEX<span className="text-sky-600 dark:text-sky-400">-CC</span>
        </span>
      </div>
      <nav className={cn('flex-1 space-y-2 overflow-y-auto px-2 py-2', !collapsed && 'md:px-3')}>
        {navGroups.map((group) => {
          const open = !closedGroups[group.label]
          return (
            <Collapsible
              key={group.label}
              open={open}
              onOpenChange={(v) => setClosedGroups((prev) => ({ ...prev, [group.label]: !v }))}
            >
              <CollapsibleTrigger
                className={cn(
                  'items-center justify-between w-full px-3 py-1 text-xs font-medium uppercase tracking-wider text-muted-foreground hover:text-foreground',
                  mobileOpen ? 'flex' : (collapsed ? 'hidden' : 'hidden md:flex'),
                )}
              >
                {group.label}
                <ChevronDown className={cn('h-3 w-3 transition-transform', !open && '-rotate-90')} />
              </CollapsibleTrigger>
              <CollapsibleContent
                forceMount
                className={cn(
                  'space-y-1 pt-1',
                  !mobileOpen && (collapsed ? 'data-[state=closed]:block' : 'data-[state=closed]:max-md:block'),
                )}
              >
                {group.items.map(({ to, icon: Icon, label, badgeKey }) => {
                  const badge = badgeKey ? badges[badgeKey] : undefined
                  const isActive = location.pathname === to
                  const showUnreadDot = to === '/findings' && unread > 0 && !isActive
                  return (
                    <NavLink
                      key={to}
                      to={to}
                      end={to === '/'}
                      onClick={() => { if (mobileOpen && onMobileClose) onMobileClose() }}
                      className={({ isActive: active }) =>
                        cn(
                          'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                          active
                            ? 'bg-accent text-accent-foreground'
                            : 'text-muted-foreground hover:bg-accent/50 hover:text-accent-foreground',
                        )
                      }
                    >
                      <span className="relative shrink-0">
                        <Icon className="h-4 w-4" />
                        {badge && !mobileOpen && (
                          <span
                            className={cn(
                              'absolute -top-1 -right-1.5 block h-2 w-2 rounded-full md:hidden',
                              badge.urgent ? 'bg-destructive' : 'bg-muted-foreground',
                            )}
                          />
                        )}
                        {showUnreadDot && (
                          <span className="absolute -top-0.5 -right-0.5 block h-2 w-2 rounded-full bg-primary animate-pulse" />
                        )}
                      </span>
                      <span className={showLabels}>{label}</span>
                      {badge && (
                        <span
                          className={cn(
                            'ml-auto items-center justify-center rounded-full px-1.5 py-0.5 text-[10px] font-semibold leading-none',
                            mobileOpen ? 'inline-flex' : (collapsed ? 'hidden' : 'hidden md:inline-flex'),
                            badge.urgent
                              ? 'bg-destructive/15 text-destructive'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {badge.count}
                        </span>
                      )}
                    </NavLink>
                  )
                })}
              </CollapsibleContent>
            </Collapsible>
          )
        })}
      </nav>
      <div className={cn('px-2 py-2', !collapsed && 'md:px-3')}>
        <div className="flex items-center gap-3 px-3 py-1.5">
          <span
            className={cn(
              'block h-2 w-2 shrink-0 rounded-full',
              sseStatus.connected ? 'bg-primary' : 'bg-destructive',
            )}
            role="status"
            aria-label={sseStatus.connected ? 'Connected to server' : 'Disconnected from server'}
            title={sseStatus.connected ? 'Connected' : 'Disconnected'}
          />
          <span className={cn(showLabels, 'text-xs text-muted-foreground')}>
            {sseStatus.connected ? 'Connected' : 'Disconnected'}
          </span>
        </div>
        {sseStatus.lastEvent && (
          <div className={cn('px-3 py-0.5', showBlock)}>
            <span className="text-[10px] text-muted-foreground">
              Updated {formatRelativeTime(sseStatus.lastEvent)}
            </span>
          </div>
        )}
        <a
          href={SOURCE_URL}
          target="_blank"
          rel="noreferrer"
          title="Source code (AGPL-3.0)"
          className="flex items-center gap-3 rounded-md px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <Code className="h-3.5 w-3.5 shrink-0" />
          <span className={showLabels}>Source (AGPL-3.0)</span>
        </a>
      </div>
      <div className={cn('px-2 py-3', !collapsed && 'md:px-3')}>
        <button
          type="button"
          onClick={() => setSidebarCollapsed(!collapsed)}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          className="hidden md:flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent/50 hover:text-accent-foreground"
        >
          {collapsed ? <ChevronsRight className="h-4 w-4 shrink-0" /> : <ChevronsLeft className="h-4 w-4 shrink-0" />}
          <span className={showLabels}>{collapsed ? 'Expand' : 'Collapse'}</span>
        </button>
      </div>
    </aside>
    </>
  )
}
