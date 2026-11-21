import { useQueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import { useSSE, type SSEEvent } from './useSSE'

export function useSSERefresh() {
  const queryClient = useQueryClient()

  const handleEvent = useCallback(
    (event: SSEEvent) => {
      switch (event.type) {
        case 'finding_added':
          queryClient.invalidateQueries({ queryKey: ['findings'] })
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          break
        case 'asset_added':
          queryClient.invalidateQueries({ queryKey: ['assets'] })
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          break
        case 'full_refresh':
        case 'node_added':
        case 'node_updated':
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          break
        case 'store_changed':
          queryClient.invalidateQueries({ queryKey: ['engagement'] })
          queryClient.invalidateQueries({ queryKey: ['activity'] })
          break
      }
    },
    [queryClient],
  )

  useSSE('/api/events', handleEvent)
}
