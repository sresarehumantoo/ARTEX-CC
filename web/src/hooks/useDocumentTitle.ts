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
  '/settings': 'Settings',
}

function setTitle(pathname: string, findingsCount: number) {
  const pageTitle = PAGE_TITLES[pathname] ?? 'cc-artex'
  document.title =
    findingsCount > 0
      ? `(${findingsCount}) ${pageTitle} · cc-artex`
      : `${pageTitle} · cc-artex`
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
