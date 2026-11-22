import { useQueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import { useSSE, type SSEEvent } from './useSSE'
import { useToast } from './useToast'
import { incrementUnread } from './useUnreadFindings'

function eventLabel(data: unknown): string | undefined {
  if (data && typeof data === 'object') {
    const d = data as Record<string, unknown>
    if (typeof d.name === 'string' && d.name) return d.name
    if (typeof d.label === 'string' && d.label) return d.label
    if (typeof d.title === 'string' && d.title) return d.title
  }
  return undefined
}

export function useSSERefresh() {
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const handleEvent = useCallback(
    (event: SSEEvent) => {
      const label = eventLabel(event.data)

      switch (event.type) {
        case 'finding_added':
          queryClient.invalidateQueries({ queryKey: ['findings'] })
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          queryClient.invalidateQueries({ queryKey: ['coverage'] })
          incrementUnread()
          toast(label ? `New finding: ${label}` : 'New finding discovered', { variant: 'warning' })
          break
        case 'asset_added':
          queryClient.invalidateQueries({ queryKey: ['assets'] })
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          toast(label ? `New asset: ${label}` : 'New asset identified', { variant: 'info' })
          break
        case 'full_refresh':
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          queryClient.invalidateQueries({ queryKey: ['graph'] })
          queryClient.invalidateQueries({ queryKey: ['coverage'] })
          queryClient.invalidateQueries({ queryKey: ['coverage-gaps'] })
          queryClient.invalidateQueries({ queryKey: ['credentials'] })
          toast('Data refreshed', { variant: 'success' })
          break
        case 'node_added':
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          queryClient.invalidateQueries({ queryKey: ['graph'] })
          toast(label ? `Graph updated: ${label}` : 'Graph updated', { variant: 'info' })
          break
        case 'node_updated':
        case 'edge_added':
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          queryClient.invalidateQueries({ queryKey: ['graph'] })
          break
        case 'store_changed':
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          queryClient.invalidateQueries({ queryKey: ['coverage'] })
          queryClient.invalidateQueries({ queryKey: ['coverage-gaps'] })
          queryClient.invalidateQueries({ queryKey: ['credentials'] })
          break
      }
    },
    [queryClient, toast],
  )

  useSSE('/api/events', handleEvent)
}
