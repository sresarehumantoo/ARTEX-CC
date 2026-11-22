import { describe, it, expect, vi, afterEach } from 'vitest'
import {
  timeAgo,
  formatBytes,
  truncate,
  severityOrder,
  severityColor,
  methodColor,
  statusColor,
} from '../format'

describe('timeAgo', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns "just now" for timestamps less than 60s ago', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000060000)
    expect(timeAgo(1700000001)).toBe('just now')
  })

  it('returns minutes for timestamps less than 1h ago', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000000000)
    // 5 minutes ago (unix seconds)
    expect(timeAgo(1700000000 - 300)).toBe('5m ago')
  })

  it('returns hours for timestamps less than 24h ago', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000000000)
    expect(timeAgo(1700000000 - 7200)).toBe('2h ago')
  })

  it('returns days for timestamps >= 24h ago', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1700000000000)
    expect(timeAgo(1700000000 - 172800)).toBe('2d ago')
  })

  it('accepts ISO string timestamps', () => {
    const now = Date.now()
    vi.spyOn(Date, 'now').mockReturnValue(now)
    const fiveMinAgo = new Date(now - 300000).toISOString()
    expect(timeAgo(fiveMinAgo)).toBe('5m ago')
  })
})

describe('formatBytes', () => {
  it('returns "0 B" for zero', () => {
    expect(formatBytes(0)).toBe('0 B')
  })

  it('returns bytes for small values', () => {
    expect(formatBytes(500)).toBe('500 B')
  })

  it('returns KB for kilobyte range', () => {
    expect(formatBytes(1024)).toBe('1.0 KB')
  })

  it('returns MB for megabyte range', () => {
    expect(formatBytes(1048576)).toBe('1.0 MB')
  })

  it('returns GB for gigabyte range', () => {
    expect(formatBytes(1073741824)).toBe('1.0 GB')
  })

  it('formats fractional values', () => {
    expect(formatBytes(1536)).toBe('1.5 KB')
  })
})

describe('truncate', () => {
  it('returns the string unchanged when shorter than max', () => {
    expect(truncate('hello', 10)).toBe('hello')
  })

  it('returns the string unchanged when exactly max', () => {
    expect(truncate('hello', 5)).toBe('hello')
  })

  it('truncates and appends ellipsis when longer than max', () => {
    expect(truncate('hello world', 5)).toBe('hello...')
  })
})

describe('severityOrder', () => {
  it('ranks critical highest (0)', () => {
    expect(severityOrder['critical']).toBe(0)
  })

  it('ranks info lowest (4)', () => {
    expect(severityOrder['info']).toBe(4)
  })

  it('orders correctly: critical < high < medium < low < info', () => {
    const ordered = Object.entries(severityOrder)
      .sort(([, a], [, b]) => a - b)
      .map(([k]) => k)
    expect(ordered).toEqual(['critical', 'high', 'medium', 'low', 'info'])
  })
})

describe('severityColor', () => {
  it('maps all severity levels to colors', () => {
    expect(severityColor['critical']).toBe('rose')
    expect(severityColor['high']).toBe('orange')
    expect(severityColor['medium']).toBe('amber')
    expect(severityColor['low']).toBe('blue')
    expect(severityColor['info']).toBe('slate')
  })
})

describe('methodColor', () => {
  it('maps HTTP methods to colors', () => {
    expect(methodColor['GET']).toBe('emerald')
    expect(methodColor['POST']).toBe('blue')
    expect(methodColor['DELETE']).toBe('rose')
  })
})

describe('statusColor', () => {
  it('returns emerald for 2xx', () => {
    expect(statusColor(200)).toBe('emerald')
    expect(statusColor(201)).toBe('emerald')
    expect(statusColor(299)).toBe('emerald')
  })

  it('returns blue for 3xx', () => {
    expect(statusColor(301)).toBe('blue')
    expect(statusColor(304)).toBe('blue')
  })

  it('returns amber for 4xx', () => {
    expect(statusColor(400)).toBe('amber')
    expect(statusColor(404)).toBe('amber')
  })

  it('returns rose for 5xx', () => {
    expect(statusColor(500)).toBe('rose')
    expect(statusColor(503)).toBe('rose')
  })
})
