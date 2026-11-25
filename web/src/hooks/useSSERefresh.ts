import { useQueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import { useSSE, type SSEEvent } from './useSSE'
import { toast } from 'sonner'
import { incrementUnread } from './useUnreadFindings'
import { conversationsKey } from './useConversations'

function eventLabel(data: unknown): string | undefined {
  if (data && typeof data === 'object') {
    const d = data as Record<string, unknown>
    if (typeof d.name === 'string' && d.name) return d.name
    if (typeof d.label === 'string' && d.label) return d.label
    if (typeof d.title === 'string' && d.title) return d.title
  }
  return undefined
}

// The server coalesces bursts of changes into one event per type and reports
// how many changes it stands for.
function eventCount(data: unknown): number {
  if (data && typeof data === 'object') {
    const n = (data as Record<string, unknown>).count
    if (typeof n === 'number' && n > 0) return n
  }
  return 1
}

const findingKeys = ['findings', 'asset-findings', 'engagement', 'activity', 'coverage', 'search']
const assetKeys = ['assets', 'assets-distribution', 'assets-graph', 'engagement', 'activity', 'search']
const graphKeys = ['graph', 'constraints', 'engagement', 'activity', 'coverage', 'coverage-gaps']
const storeKeys = ['engagement', 'activity', 'coverage', 'coverage-gaps', 'credentials']

const conversationKeys = [conversationsKey[0]]

export const invalidations: Record<string, readonly string[]> = {
  finding_added: findingKeys,
  findings_changed: findingKeys,
  asset_added: assetKeys,
  graph_changed: graphKeys,
  node_added: graphKeys,
  traffic_changed: ['traffic'],
  store_changed: storeKeys,
  refresh: [...new Set([...findingKeys, ...assetKeys, ...graphKeys, ...storeKeys])],
  conversation_created: conversationKeys,
  conversation_updated: conversationKeys,
  conversation_deleted: conversationKeys,
  conversation_status: conversationKeys,
}

export function useSSERefresh() {
  const queryClient = useQueryClient()

  const handleEvent = useCallback(
    (event: SSEEvent) => {
      for (const key of invalidations[event.type] ?? []) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }

      const label = eventLabel(event.data)
      const count = eventCount(event.data)
      switch (event.type) {
        case 'finding_added':
          for (let i = 0; i < count; i++) incrementUnread()
          if (count > 1) toast.warning(`${count} new findings`)
          else toast.warning(label ? `New finding: ${label}` : 'New finding discovered')
          break
        case 'asset_added':
          toast.info(label ? `New asset: ${label}` : 'New assets identified')
          break
        case 'node_added':
          toast.info(label ? `Graph updated: ${label}` : 'Graph updated')
          break
      }
    },
    [queryClient],
  )

  useSSE('/api/events', handleEvent)
}
