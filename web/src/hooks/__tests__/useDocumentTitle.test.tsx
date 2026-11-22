import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import React from 'react'
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

  it('sets title to "Dashboard . cc-artex" for root path', () => {
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('Dashboard · cc-artex')
  })

  it('sets title based on pathname', () => {
    mockPathname = '/findings'
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('Findings · cc-artex')
  })

  it('falls back to "cc-artex" for unknown paths', () => {
    mockPathname = '/unknown'
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('cc-artex · cc-artex')
  })

  it('includes findings count when > 0', () => {
    mockQueryData = { engagement: { findings_total: 5 } }
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('(5) Dashboard · cc-artex')
  })

  it('updates title when engagement data changes via cache subscription', () => {
    mockQueryData = { engagement: { findings_total: 0 } }
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('Dashboard · cc-artex')

    mockQueryData = { engagement: { findings_total: 3 } }
    for (const cb of cacheSubscribers) {
      cb({ query: { queryKey: ['engagement'] }, type: 'updated' })
    }
    expect(document.title).toBe('(3) Dashboard · cc-artex')
  })

  it('ignores non-engagement cache updates', () => {
    mockQueryData = { engagement: { findings_total: 0 } }
    renderHook(() => useDocumentTitle())

    for (const cb of cacheSubscribers) {
      cb({ query: { queryKey: ['findings'] }, type: 'updated' })
    }
    expect(document.title).toBe('Dashboard · cc-artex')
  })

  it('handles null engagement data', () => {
    mockQueryData = {}
    renderHook(() => useDocumentTitle())
    expect(document.title).toBe('Dashboard · cc-artex')
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
      '/settings': 'Settings',
    }

    for (const [path, title] of Object.entries(pages)) {
      mockPathname = path
      renderHook(() => useDocumentTitle())
      expect(document.title).toBe(`${title} · cc-artex`)
    }
  })
})
