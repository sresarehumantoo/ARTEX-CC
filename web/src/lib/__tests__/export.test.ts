import { describe, it, expect, vi, beforeEach } from 'vitest'
import { downloadFile, exportCSV, exportJSON, exportTrafficCSV, exportAssetCSV } from '../export'
import type { Finding, TrafficExchange } from '../types'

describe('downloadFile', () => {
  let clickSpy: ReturnType<typeof vi.fn>
  let createObjectURLSpy: ReturnType<typeof vi.fn>
  let revokeObjectURLSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    clickSpy = vi.fn()
    createObjectURLSpy = vi.fn().mockReturnValue('blob:test')
    revokeObjectURLSpy = vi.fn()

    vi.stubGlobal('URL', {
      createObjectURL: createObjectURLSpy,
      revokeObjectURL: revokeObjectURLSpy,
    })

    vi.spyOn(document, 'createElement').mockReturnValue({
      href: '',
      download: '',
      click: clickSpy,
    } as unknown as HTMLAnchorElement)
  })

  it('creates a blob URL, clicks, and revokes', () => {
    downloadFile('content', 'file.txt', 'text/plain')

    expect(createObjectURLSpy).toHaveBeenCalledOnce()
    expect(clickSpy).toHaveBeenCalledOnce()
    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:test')
  })

  it('sets the download attribute to the filename', () => {
    const mockAnchor = { href: '', download: '', click: vi.fn() }
    vi.spyOn(document, 'createElement').mockReturnValue(mockAnchor as unknown as HTMLAnchorElement)

    downloadFile('data', 'report.csv', 'text/csv')

    expect(mockAnchor.download).toBe('report.csv')
    expect(mockAnchor.href).toBe('blob:test')
  })
})

function makeFinding(overrides: Partial<Finding> = {}): Finding {
  return {
    id: 1,
    node_id: 10,
    vulnclass: 'sqli',
    severity: 'high' as Finding['severity'],
    summary: 'SQL injection in login',
    state: 'open',
    status: 'confirmed',
    merged_into: 0,
    dismissed: false,
    dismiss_reason: '',
    ...overrides,
  }
}

describe('exportCSV', () => {
  beforeEach(() => {
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn().mockReturnValue('blob:test'),
      revokeObjectURL: vi.fn(),
    })
    vi.spyOn(document, 'createElement').mockReturnValue({
      href: '', download: '', click: vi.fn(),
    } as unknown as HTMLAnchorElement)
  })

  it('generates CSV with headers and rows', () => {
    const findings = [makeFinding()]
    const spy = vi.fn()
    vi.stubGlobal('Blob', class {
      parts: string[]
      constructor(parts: string[]) { this.parts = parts; spy(parts[0]) }
    })

    exportCSV(findings, 'findings.csv')

    const csv = spy.mock.calls[0][0] as string
    const lines = csv.split('\n')
    expect(lines[0]).toBe('ID,Severity,Vuln Class,Summary,Status,Dismissed,Node ID,Merged Into')
    expect(lines[1]).toContain('sqli')
    expect(lines[1]).toContain('"SQL injection in login"')
  })

  it('escapes double quotes in summary', () => {
    const findings = [makeFinding({ summary: 'Contains "quotes"' })]
    const spy = vi.fn()
    vi.stubGlobal('Blob', class {
      parts: string[]
      constructor(parts: string[]) { this.parts = parts; spy(parts[0]) }
    })

    exportCSV(findings, 'test.csv')

    const csv = spy.mock.calls[0][0] as string
    expect(csv).toContain('"Contains ""quotes"""')
  })
})

describe('exportJSON', () => {
  beforeEach(() => {
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn().mockReturnValue('blob:test'),
      revokeObjectURL: vi.fn(),
    })
    vi.spyOn(document, 'createElement').mockReturnValue({
      href: '', download: '', click: vi.fn(),
    } as unknown as HTMLAnchorElement)
  })

  it('generates pretty-printed JSON', () => {
    const findings = [makeFinding()]
    const spy = vi.fn()
    vi.stubGlobal('Blob', class {
      parts: string[]
      constructor(parts: string[]) { this.parts = parts; spy(parts[0]) }
    })

    exportJSON(findings, 'findings.json')

    const json = spy.mock.calls[0][0] as string
    expect(JSON.parse(json)).toEqual(findings)
    expect(json).toContain('\n')
  })
})

describe('exportTrafficCSV', () => {
  beforeEach(() => {
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn().mockReturnValue('blob:test'),
      revokeObjectURL: vi.fn(),
    })
    vi.spyOn(document, 'createElement').mockReturnValue({
      href: '', download: '', click: vi.fn(),
    } as unknown as HTMLAnchorElement)
  })

  it('generates traffic CSV with correct headers', () => {
    const rows: TrafficExchange[] = [{
      id: 'abc',
      ts: 1700000000,
      host: 'example.com',
      method: 'GET',
      url_template: '/api/v1',
      url: '/api/v1',
      status: 200,
      content_type: 'application/json',
      resp_len: 1234,
      rewritten: false,
    }]
    const spy = vi.fn()
    vi.stubGlobal('Blob', class {
      parts: string[]
      constructor(parts: string[]) { this.parts = parts; spy(parts[0]) }
    })

    exportTrafficCSV(rows, 'traffic.csv')

    const csv = spy.mock.calls[0][0] as string
    const lines = csv.split('\n')
    expect(lines[0]).toBe('Timestamp,Host,Method,URL,Status,Content-Type,Size,Rewritten')
    expect(lines[1]).toContain('example.com')
    expect(lines[1]).toContain('GET')
  })
})

describe('exportAssetCSV', () => {
  beforeEach(() => {
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn().mockReturnValue('blob:test'),
      revokeObjectURL: vi.fn(),
    })
    vi.spyOn(document, 'createElement').mockReturnValue({
      href: '', download: '', click: vi.fn(),
    } as unknown as HTMLAnchorElement)
  })

  it('generates asset CSV with correct headers', () => {
    const assets = [{ id: 1, type: 'host', label: 'example.com', data: {} }]
    const spy = vi.fn()
    vi.stubGlobal('Blob', class {
      parts: string[]
      constructor(parts: string[]) { this.parts = parts; spy(parts[0]) }
    })

    exportAssetCSV(assets, 'assets.csv')

    const csv = spy.mock.calls[0][0] as string
    const lines = csv.split('\n')
    expect(lines[0]).toBe('ID,Type,Label')
    expect(lines[1]).toContain('example.com')
  })
})
