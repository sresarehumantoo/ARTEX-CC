import { toast } from 'sonner'
import type { Activity, Asset, Finding, GraphData, TrafficExchange } from '../lib/types'

const READ_ONLY = 'Demo is read-only'

interface LogEntry {
  seq: number
  level: string
}

interface CommandEntry {
  conversation_id: string
  tool_name: string
  is_error: boolean
}

interface ChatEvent {
  seq: number
}

interface Fixtures {
  meta: { captured_at: string }
  routes: Record<string, unknown>
  assets: Asset[]
  traffic: TrafficExchange[]
  activity: Activity[]
  logs: LogEntry[]
  commands: CommandEntry[]
  workspace: { dirs: Record<string, unknown[]>; files: Record<string, string> }
}

function loadFixtures(): Fixtures {
  const files = import.meta.glob<unknown>('./fixtures/*.json', { eager: true, import: 'default' })
  const raw: Record<string, unknown> = {}
  for (const [path, value] of Object.entries(files)) {
    raw[path.slice('./fixtures/'.length, -'.json'.length)] = value
  }
  // Move every timestamp forward so the capture moment reads as "just now".
  const delta = Date.now() - Date.parse((raw.meta as Fixtures['meta']).captured_at)
  const shifted = JSON.stringify(raw)
    .replace(/\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z/g, m => new Date(Date.parse(m) + delta).toISOString())
    .replace(/"ts":(\d+)/g, (_, s: string) => `"ts":${Number(s) + Math.round(delta / 1000)}`)
  return JSON.parse(shifted) as Fixtures
}

const fx = loadFixtures()

function json(body: unknown, status = 200, statusText = '') {
  return new Response(JSON.stringify(body), { status, statusText, headers: { 'Content-Type': 'application/json' } })
}

function int(v: string | null, fallback: number) {
  const n = Number.parseInt(v ?? '', 10)
  return Number.isNaN(n) ? fallback : n
}

const has = (haystack: unknown, needle: string) => String(haystack ?? '').toLowerCase().includes(needle.toLowerCase())

function matchAssets(dsl: string): Asset[] {
  const terms = dsl.split(/\s+AND\s+/i).map(t => t.trim()).filter(Boolean)
  return fx.assets.filter(a =>
    terms.every(term => {
      const m = term.match(/^(\w+)\s*[=:~]\s*"?([^"]*)"?$/)
      if (!m) return has(JSON.stringify(a), term)
      const [, field, value] = m
      if (field === 'type') return a.type === value
      if (field === 'id') return String(a.id) === value
      return has(a.data[field] ?? a.label, value)
    }),
  )
}

function assetsPage(sp: URLSearchParams) {
  let rows = matchAssets(sp.get('dsl') ?? '')
  if (sp.get('type')) rows = rows.filter(a => a.type === sp.get('type'))
  const offset = int(sp.get('offset'), 0)
  return { assets: rows.slice(offset, offset + int(sp.get('limit'), 50)), total: rows.length }
}

function trafficText(id: string) {
  const d = fx.routes[`/api/traffic/${id}`] as { request: string; response: string } | undefined
  return d ? d.request + '\n' + d.response : ''
}

function statusMatches(status: number, token: string) {
  const t = token.trim().toLowerCase()
  if (/^[1-5]xx$/.test(t)) return Math.floor(status / 100) === Number(t[0])
  return String(status) === t
}

function trafficPage(sp: URLSearchParams) {
  const host = sp.get('host')?.trim()
  const method = sp.get('method')?.trim().toUpperCase()
  const path = sp.get('path')?.trim()
  const status = sp.get('status')?.trim()
  const q = sp.get('q')?.trim()
  const body = sp.get('body')?.trim()
  const respMin = sp.get('resp_min')
  const respMax = sp.get('resp_max')
  const rows = fx.traffic.filter(r => {
    if (host && !has(r.host, host)) return false
    if (method && r.method !== method) return false
    if (path && !has(r.url_template, path)) return false
    if (status && !statusMatches(r.status, status)) return false
    if (respMin && r.resp_len < Number(respMin)) return false
    if (respMax && r.resp_len > Number(respMax)) return false
    if (q && ![r.host, r.url, r.url_template, r.method, r.content_type, r.status, trafficText(r.id)].some(v => has(v, q))) return false
    if (body && body.length >= 3 && !has(trafficText(r.id), body)) return false
    return true
  })
  const key = (['ts', 'status', 'resp_len'] as const).find(k => k === sp.get('sort')) ?? 'ts'
  const dir = sp.get('order') === 'asc' ? 1 : -1
  rows.sort((a, b) => (a[key] - b[key]) * dir || b.id.localeCompare(a.id))
  let size = int(sp.get('size'), 100)
  if (size <= 0 || size > 500) size = 100
  const page = Math.max(0, int(sp.get('page'), 0))
  return { rows: rows.slice(page * size, page * size + size), total: rows.length }
}

function activity(sp: URLSearchParams) {
  const kind = sp.get('kind')
  const node = int(sp.get('node_id'), 0)
  const limit = Math.min(int(sp.get('limit'), 50) || 50, 500)
  return fx.activity.filter(a => (!kind || a.kind === kind) && (!node || a.node_id === node)).slice(0, limit)
}

const LEVEL_RANK: Record<string, number> = { info: 1, warn: 2, error: 3 }

function logs(after: number, level: string | null) {
  const min = LEVEL_RANK[level ?? ''] ?? 0
  return fx.logs.filter(e => e.seq > after && (LEVEL_RANK[e.level] ?? 0) >= min)
}

function commands(sp: URLSearchParams) {
  const tool = sp.get('tool') ?? ''
  const conv = sp.get('conv')
  const err = sp.get('error')
  const rows = fx.commands.filter(
    c =>
      c.tool_name.startsWith(tool) &&
      (!conv || c.conversation_id === conv) &&
      (err !== 'true' && err !== 'false' ? true : c.is_error === (err === 'true')),
  )
  const offset = Math.max(0, int(sp.get('offset'), 0))
  const limit = int(sp.get('limit'), 100)
  return rows.slice(offset, limit > 0 ? offset + limit : undefined)
}

function search(q: string) {
  if (!q) return { results: [] }
  const findings = (fx.routes['/api/findings'] as Finding[])
    .filter(f => has(f.vulnclass, q) || has(f.summary, q))
    .slice(0, 5)
    .map(f => ({
      type: 'finding',
      id: f.id,
      title: f.vulnclass,
      subtitle: `${f.severity} — ${f.summary.slice(0, 60)}`,
      url: `/findings?id=${f.id}`,
    }))
  const assets = matchAssets(q)
    .slice(0, 5)
    .map(a => ({ type: 'asset', id: a.id, title: a.label, subtitle: a.type, url: `/assets?id=${a.id}` }))
  const nodes = (fx.routes['/api/graph'] as GraphData).nodes
    .map(n => ({ n, text: String(n.payload?.summary ?? n.payload?.text ?? n.payload?.name ?? '') }))
    .filter(({ text }) => has(text, q))
    .slice(0, 5)
    .map(({ n, text }) => ({
      type: 'node',
      id: n.id,
      title: text,
      subtitle: n.state ? `${n.kind} — ${n.state}` : n.kind,
      url: `/graph?node=${n.id}`,
    }))
  return { results: [...findings, ...assets, ...nodes] }
}

function handleGet(url: URL): Response {
  const p = url.pathname
  const sp = url.searchParams
  switch (p) {
    case '/api/assets':
      return json(assetsPage(sp))
    case '/api/traffic':
      return json(trafficPage(sp))
    case '/api/activity':
      return json(activity(sp))
    case '/api/logs':
      return json(logs(int(sp.get('after'), 0), sp.get('level')))
    case '/api/commands':
      return json(commands(sp))
    case '/api/usage': {
      const conv = sp.get('conv')
      const rows = fx.routes['/api/usage'] as { conversation_id: string }[]
      return json(conv ? rows.filter(r => r.conversation_id === conv) : rows)
    }
    case '/api/search':
      return json(search(sp.get('q') ?? ''))
    case '/api/workspace': {
      const entries = fx.workspace.dirs[sp.get('path') ?? '']
      return entries ? json(entries) : json({ error: 'not found' }, 404, 'Not Found')
    }
    case '/api/workspace/file': {
      const text = fx.workspace.files[sp.get('path') ?? '']
      return text === undefined ? json({ error: 'not found' }, 404, 'Not Found') : new Response(text, { headers: { 'Content-Type': 'text/plain' } })
    }
  }

  if (/^\/api\/conversations\/[^/]+\/messages$/.test(p)) {
    const events = (fx.routes[p] ?? []) as ChatEvent[]
    return json(events.filter(e => e.seq > int(sp.get('after'), 0)))
  }
  if (p in fx.routes) return json(fx.routes[p])
  return json({ error: 'not available in the demo' }, 404, 'Not Found')
}

function handle(url: URL, method: string): Response {
  if (method === 'GET' || method === 'HEAD') return handleGet(url)
  if (url.pathname.startsWith('/api/auth/')) return new Response(null, { status: 204 })
  return json({ error: READ_ONLY }, 403, READ_ONLY)
}

class DemoEventSource extends EventTarget {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSED = 2
  readonly CONNECTING = 0
  readonly OPEN = 1
  readonly CLOSED = 2
  readonly url: string
  readonly withCredentials = false
  readyState = 0
  onopen: ((e: Event) => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  private timer: ReturnType<typeof setTimeout>

  constructor(url: string | URL) {
    super()
    this.url = new URL(String(url), window.location.origin).toString()
    this.timer = setTimeout(() => this.open(), 30)
  }

  private open() {
    this.readyState = 1
    this.emit(new Event('open'), this.onopen)
    const u = new URL(this.url)
    if (u.pathname === '/api/logs/stream') {
      for (const entry of logs(int(u.searchParams.get('after'), 0), u.searchParams.get('level'))) {
        this.emit(new MessageEvent('message', { data: JSON.stringify(entry), lastEventId: String(entry.seq) }), this.onmessage)
      }
    }
  }

  private emit<E extends Event>(e: E, handler: ((e: E) => void) | null) {
    if (this.readyState === 2) return
    handler?.(e)
    this.dispatchEvent(e)
  }

  close() {
    this.readyState = 2
    clearTimeout(this.timer)
  }
}

function apiURL(input: RequestInfo | URL): URL | null {
  const raw = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
  const url = new URL(raw, window.location.origin)
  return url.origin === window.location.origin && url.pathname.startsWith('/api/') ? url : null
}

export function installDemoTransport() {
  const realFetch = window.fetch.bind(window)
  window.fetch = async (input, init) => {
    const url = apiURL(input)
    if (!url) return realFetch(input, init)
    const method = (init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase()
    return handle(url, method)
  }
  window.EventSource = DemoEventSource as unknown as typeof EventSource

  // Plain links into the API (workspace downloads) would leave the static site.
  document.addEventListener(
    'click',
    e => {
      const a = (e.target as Element | null)?.closest?.('a[href]')
      if (a && apiURL(a.getAttribute('href') ?? '')) {
        e.preventDefault()
        toast(READ_ONLY)
      }
    },
    true,
  )
}
