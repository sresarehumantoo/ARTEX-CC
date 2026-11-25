import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import type { SSEEvent } from '../useSSE'

let handler: ((e: SSEEvent) => void) | undefined

vi.mock('../useSSE', () => ({
  useSSE: (_url: string, onEvent: (e: SSEEvent) => void) => {
    handler = onEvent
  },
}))

vi.mock('sonner', () => ({ toast: { warning: vi.fn(), info: vi.fn(), success: vi.fn() } }))

const { useSSERefresh } = await import('../useSSERefresh')
const { useUnreadFindings, clearUnread } = await import('../useUnreadFindings')

function setup() {
  const client = new QueryClient()
  const spy = vi.spyOn(client, 'invalidateQueries')
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  renderHook(() => useSSERefresh(), { wrapper })
  const unread = renderHook(() => useUnreadFindings())
  const send = (type: string, data: unknown = null) => act(() => handler!({ type, data }))
  const invalidated = () => spy.mock.calls.map((c) => (c[0]!.queryKey as string[])[0])
  return { spy, send, invalidated, unread }
}

beforeEach(() => {
  handler = undefined
  clearUnread()
})

describe('useSSERefresh', () => {
  it('refreshes broadly on unknown tool changes', () => {
    const { send, invalidated } = setup()
    send('refresh', { count: 1 })
    expect(invalidated()).toEqual(
      expect.arrayContaining(['assets', 'findings', 'graph', 'coverage', 'engagement']),
    )
  })

  it('refreshes the graph on graph changes', () => {
    const { send, invalidated } = setup()
    send('graph_changed', { count: 4 })
    expect(invalidated()).toContain('graph')
    expect(invalidated()).not.toContain('findings')
  })

  it('counts every finding in a coalesced event', () => {
    const { send, invalidated, unread } = setup()
    send('finding_added', { count: 3 })
    expect(invalidated()).toContain('findings')
    expect(unread.result.current).toBe(3)
  })

  it('does not count findings for other finding changes', () => {
    const { send, invalidated, unread } = setup()
    send('findings_changed', { count: 2 })
    expect(invalidated()).toContain('findings')
    expect(unread.result.current).toBe(0)
  })

  it('ignores unknown event types', () => {
    const { send, spy } = setup()
    send('something_else')
    expect(spy).not.toHaveBeenCalled()
  })
})
