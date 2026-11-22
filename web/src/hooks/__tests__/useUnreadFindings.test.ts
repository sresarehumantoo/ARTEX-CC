import { describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useUnreadFindings, incrementUnread, clearUnread } from '../useUnreadFindings'

beforeEach(() => {
  clearUnread()
})

describe('useUnreadFindings', () => {
  it('starts at zero', () => {
    const { result } = renderHook(() => useUnreadFindings())
    expect(result.current).toBe(0)
  })

  it('increments the count', () => {
    const { result } = renderHook(() => useUnreadFindings())

    act(() => { incrementUnread() })
    expect(result.current).toBe(1)

    act(() => { incrementUnread() })
    expect(result.current).toBe(2)
  })

  it('clears the count back to zero', () => {
    const { result } = renderHook(() => useUnreadFindings())

    act(() => { incrementUnread() })
    act(() => { incrementUnread() })
    expect(result.current).toBe(2)

    act(() => { clearUnread() })
    expect(result.current).toBe(0)
  })

  it('clearUnread is a no-op when already zero', () => {
    const { result } = renderHook(() => useUnreadFindings())
    act(() => { clearUnread() })
    expect(result.current).toBe(0)
  })

  it('notifies multiple subscribers', () => {
    const { result: r1 } = renderHook(() => useUnreadFindings())
    const { result: r2 } = renderHook(() => useUnreadFindings())

    act(() => { incrementUnread() })
    expect(r1.current).toBe(1)
    expect(r2.current).toBe(1)
  })
})
