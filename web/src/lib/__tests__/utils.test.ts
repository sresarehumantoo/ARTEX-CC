import { describe, it, expect, vi, afterEach } from 'vitest'
import { cn, relativeTime, formatBytes, truncate } from '../utils'

describe('cn', () => {
  it('merges class names', () => {
    expect(cn('foo', 'bar')).toBe('foo bar')
  })

  it('handles conditional classes', () => {
    expect(cn('base', false && 'hidden', 'visible')).toBe('base visible')
  })

  it('deduplicates conflicting tailwind classes', () => {
    expect(cn('px-2', 'px-4')).toBe('px-4')
  })

  it('returns empty string with no inputs', () => {
    expect(cn()).toBe('')
  })

  it('handles undefined and null', () => {
    expect(cn('a', undefined, null, 'b')).toBe('a b')
  })
})

describe('relativeTime', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns seconds for recent timestamps', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000030000)
    expect(relativeTime(1700000000)).toBe('30s ago')
  })

  it('returns minutes', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000000000)
    expect(relativeTime(1700000000 - 300)).toBe('5m ago')
  })

  it('returns hours', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000000000)
    expect(relativeTime(1700000000 - 7200)).toBe('2h ago')
  })

  it('returns days', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000000000)
    expect(relativeTime(1700000000 - 172800)).toBe('2d ago')
  })

  it('returns "just now" for future timestamps', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000000000)
    expect(relativeTime(1700000010)).toBe('just now')
  })

  it('handles millisecond timestamps', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000030000)
    expect(relativeTime(1700000000000)).toBe('30s ago')
  })
})

describe('formatBytes', () => {
  it('returns "0 B" for zero', () => {
    expect(formatBytes(0)).toBe('0 B')
  })

  it('returns bytes for small values', () => {
    expect(formatBytes(500)).toBe('500 B')
  })

  it('returns KB', () => {
    expect(formatBytes(1024)).toBe('1.0 KB')
  })

  it('returns MB', () => {
    expect(formatBytes(1048576)).toBe('1.0 MB')
  })

  it('formats fractional values with one decimal when < 10', () => {
    expect(formatBytes(1536)).toBe('1.5 KB')
  })

  it('rounds values >= 10', () => {
    expect(formatBytes(10240)).toBe('10 KB')
  })
})

describe('truncate', () => {
  it('returns unchanged when shorter than max', () => {
    expect(truncate('hi', 10)).toBe('hi')
  })

  it('truncates with ellipsis', () => {
    expect(truncate('hello world', 5)).toBe('hello...')
  })

  it('returns unchanged at exact length', () => {
    expect(truncate('exact', 5)).toBe('exact')
  })
})
