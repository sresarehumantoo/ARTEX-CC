import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import type { EngagementData } from '../lib/types'

const PAGE_TITLES: Record<string, string> = {
  '/': 'Dashboard',
  '/graph': 'Graph',
  '/findings': 'Findings',
  '/traffic': 'Traffic',
  '/activity': 'Activity',
  '/assets': 'Assets',
  '/coverage': 'Coverage',
  '/credentials': 'Credentials',
  '/report': 'Report',
  '/tasks': 'Tasks',
  '/chat': 'Chat',
  '/agents': 'Agents',
  '/mcp-servers': 'MCP Servers',
  '/tools': 'Tools',
  '/intercept': 'Intercept',
  '/commands': 'Commands',
  '/usage': 'Usage',
  '/workspace': 'Workspace',
  '/logs': 'Logs',
  '/search': 'Search',
  '/settings': 'Settings',
}

function setTitle(pathname: string, findingsCount: number) {
  const pageTitle = PAGE_TITLES[pathname]
  if (!pageTitle) {
    document.title = findingsCount > 0 ? `(${findingsCount}) ARTEX-CC` : 'ARTEX-CC'
    return
  }
  document.title =
    findingsCount > 0
      ? `(${findingsCount}) ${pageTitle} · ARTEX-CC`
      : `${pageTitle} · ARTEX-CC`
}

export function useDocumentTitle() {
  const location = useLocation()
  const queryClient = useQueryClient()

  useEffect(() => {
    const engagement = queryClient.getQueryData<EngagementData>(['engagement'])
    setTitle(location.pathname, engagement?.findings_total ?? 0)
  }, [location.pathname, queryClient])

  useEffect(() => {
    const unsubscribe = queryClient.getQueryCache().subscribe((event) => {
      if (event?.query?.queryKey?.[0] === 'engagement' && event.type === 'updated') {
        const engagement = queryClient.getQueryData<EngagementData>(['engagement'])
        setTitle(location.pathname, engagement?.findings_total ?? 0)
      }
    })
    return () => unsubscribe()
  }, [location.pathname, queryClient])
}
