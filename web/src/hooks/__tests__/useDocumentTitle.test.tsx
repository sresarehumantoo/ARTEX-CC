import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useDocumentTitle } from '../useDocumentTitle'

let mockPathname = '/'
let mockQueryData: Record<string, unknown> = {}
let cacheSubscribers: Array<(event: unknown) => void> = []

vi.mock('react-router-dom', () => ({
  useLocation: () => ({ pathname: mockPathname }),
}))

vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({
    getQueryData: (key: string[]) => mockQueryData[key[0]],
    getQueryCache: () => ({
      subscribe: (cb: (event: unknown) => void) => {
        cacheSubscribers.push(cb)
        return () => {
          cacheSubscribers = cacheSubscribers.filter(s => s !== cb)
        }
      },
    }),
  }),
}))

describe('useDocumentTitle', () => {
  beforeEach(() => {
    mockPathname = '/'
    mockQueryData = {}
    cacheSubscribers = []
    document.title = ''
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sets title to "Dashboard . ARTEX-CC" for root path', () => {
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('Dashboard · ARTEX-CC')
  })

  it('sets title based on pathname', () => {
    mockPathname = '/findings'
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('Findings · ARTEX-CC')
  })

  it('falls back to "ARTEX-CC" for unknown paths', () => {
    mockPathname = '/unknown'
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('ARTEX-CC')
  })

  it('includes findings count when > 0', () => {
    mockQueryData = { engagement: { findings_total: 5 } }
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('(5) Dashboard · ARTEX-CC')
  })

  it('updates title when engagement data changes via cache subscription', () => {
    mockQueryData = { engagement: { findings_total: 0 } }
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('Dashboard · ARTEX-CC')

    mockQueryData = { engagement: { findings_total: 3 } }
    for (const cb of cacheSubscribers) {
      cb({ query: { queryKey: ['engagement'] }, type: 'updated' })
    }
    expect(document.title).toBe('(3) Dashboard · ARTEX-CC')
  })

  it('ignores non-engagement cache updates', () => {
    mockQueryData = { engagement: { findings_total: 0 } }
    renderHook(() => useDocumentTitle())

    for (const cb of cacheSubscribers) {
      cb({ query: { queryKey: ['findings'] }, type: 'updated' })
    }
    expect(document.title).toBe('Dashboard · ARTEX-CC')
  })

  it('handles null engagement data', () => {
    mockQueryData = {}
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('Dashboard · ARTEX-CC')
  })

  it('sets correct titles for all known pages', () => {
    const pages: Record<string, string> = {
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

    for (const [path, title] of Object.entries(pages)) {
      mockPathname = path
      renderHook(() => useDocumentTitle())
      expect(document.title).toBe(`${title} · ARTEX-CC`)
    }
  })
})
