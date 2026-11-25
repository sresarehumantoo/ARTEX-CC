import type { ChatEvent, Conversation, SendMessageRequest } from './chat-types'

export function isChatMock(): boolean {
  if (import.meta.env.VITE_MOCK === '1') return true
  return typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('mock') === '1'
}

const minutesAgo = (m: number) => new Date(Date.now() - m * 60_000).toISOString()

let conversations: Conversation[] = [
  { id: 'c-1a2b3c', title: 'Recon example.com', model: 'opus', agent: 'claude', status: 'idle', session_id: 's1', created_at: minutesAgo(90), updated_at: minutesAgo(12) },
  { id: 'c-4d5e6f', title: 'Probe login endpoint', model: 'sonnet', agent: 'claude', status: 'running', session_id: 's2', created_at: minutesAgo(30), updated_at: minutesAgo(1) },
  { id: 'c-7a8b9c', title: 'Summarize findings', model: 'haiku', agent: 'claude', status: 'error', session_id: 's3', created_at: minutesAgo(600), updated_at: minutesAgo(240) },
]

function buildEvents(scenario: 'idle' | 'running' | 'error'): ChatEvent[] {
  const base = Date.now() - 20 * 60_000
  const out: ChatEvent[] = []
  let seq = 0
  const push = (e: Omit<ChatEvent, 'seq' | 'time'>) => {
    seq++
    out.push({ seq, time: new Date(base + seq * 20_000).toISOString(), ...e })
  }
  const usage = (i: number, o: number) => ({ input_tokens: i, output_tokens: o, cache_read_tokens: 1200, cache_creation_tokens: 300, cost_usd: 0.0123 })

  push({ kind: 'user', text: 'Start with reconnaissance on example.com and record what you find.' })
  push({ kind: 'thinking', text: 'I should resolve the host first, then check which services respond before inserting assets.' })
  push({ kind: 'text', text: "I'll begin with DNS resolution and a quick port check." })
  push({ kind: 'tool_use', tool_name: 'Bash', tool_id: 'tu1', input: { command: 'dig +short example.com', description: 'Resolve example.com' } })
  push({ kind: 'tool_result', tool_id: 'tu1', text: '203.0.113.10\n' })
  push({ kind: 'tool_use', tool_name: 'Bash', tool_id: 'tu2', input: { command: 'nmap -sT -p 80,443,8080 203.0.113.10' } })
  push({ kind: 'tool_result', tool_id: 'tu2', text: 'PORT     STATE  SERVICE\n80/tcp   open   http\n443/tcp  open   https\n8080/tcp closed http-proxy\n' })
  push({ kind: 'tool_use', tool_name: 'mcp__artex__insert_asset', tool_id: 'tu3', input: { type: 'host', value: '203.0.113.10', label: 'example.com' } })
  push({ kind: 'tool_result', tool_id: 'tu3', text: '{"id": 41, "created": true}' })
  push({ kind: 'tool_use', tool_name: 'Read', tool_id: 'tu4', input: { file_path: '/home/operator/pentest/notes/scope.md' } })
  push({ kind: 'tool_result', tool_id: 'tu4', text: '# Scope\n\n- example.com\n- *.example.com\n\nOut of scope: payments.example.com' })
  push({ kind: 'text', text: '## Summary\n\nThe host resolves to `203.0.113.10` with these services:\n\n| Port | State | Service |\n|------|-------|---------|\n| 80 | open | http |\n| 443 | open | https |\n| 8080 | closed | http-proxy |\n\n```bash\ncurl -sI https://example.com\n```\n\nSee the [scope notes](https://example.com/scope) for details.' })
  push({ kind: 'result', usage: usage(4210, 880) })
  push({ kind: 'user', text: 'Now check the login page for common weaknesses.' })
  push({ kind: 'thinking', text: 'Fetch the login page, look at forms and headers.' })
  push({ kind: 'tool_use', tool_name: 'Bash', tool_id: 'tu5', input: { command: 'curl -sI https://example.com/login' } })

  if (scenario === 'error') {
    push({ kind: 'tool_result', tool_id: 'tu5', text: 'curl: (6) Could not resolve host: example.com', is_error: true })
    push({ kind: 'error', text: 'claude process exited with status 1: rate limit exceeded' })
    push({ kind: 'result', usage: usage(5100, 920) })
  } else if (scenario === 'idle') {
    push({ kind: 'tool_result', tool_id: 'tu5', text: 'HTTP/2 200\ncontent-type: text/html\nset-cookie: sid=abc; HttpOnly\n' })
    push({ kind: 'text', text: 'The login page responds with `200` and sets an `HttpOnly` cookie. No obvious weaknesses in headers.' })
    push({ kind: 'result', usage: usage(6400, 1320) })
  }
  return out
}

const eventStore: Record<string, ChatEvent[]> = {
  'c-1a2b3c': buildEvents('idle'),
  'c-4d5e6f': buildEvents('running'),
  'c-7a8b9c': buildEvents('error'),
}

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

function parseBody<T>(init?: RequestInit): T {
  return JSON.parse(String(init?.body ?? '{}')) as T
}

async function handle(url: URL, init?: RequestInit): Promise<Response | null> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const path = url.pathname

  if (path === '/api/claude/status') return json({ available: true, version: '2.1.330', path: '/usr/bin/claude' })

  if (path === '/api/conversations') {
    if (method === 'GET') return json(conversations)
    if (method === 'POST') {
      const req = parseBody<Partial<Conversation>>(init)
      const now = new Date().toISOString()
      const conv: Conversation = {
        id: `c-${Math.random().toString(16).slice(2, 8)}`,
        title: req.title || 'New conversation',
        model: req.model || 'sonnet',
        agent: req.agent || 'claude',
        status: 'idle',
        session_id: '',
        created_at: now,
        updated_at: now,
      }
      conversations = [conv, ...conversations]
      eventStore[conv.id] = []
      return json(conv, 201)
    }
    if (method === 'DELETE') {
      const { ids } = parseBody<{ ids: string[] }>(init)
      conversations = conversations.filter(c => !ids.includes(c.id))
      return new Response(null, { status: 204 })
    }
  }

  const m = path.match(/^\/api\/conversations\/([^/]+)(?:\/(messages|stop|stream))?$/)
  if (!m) return null
  const [, id, sub] = m
  const conv = conversations.find(c => c.id === id)
  if (!conv) return json({ error: 'not found' }, 404)

  if (!sub) {
    if (method === 'GET') return json(conv)
    if (method === 'PATCH') {
      Object.assign(conv, parseBody<Partial<Conversation>>(init), { updated_at: new Date().toISOString() })
      return json(conv)
    }
    if (method === 'DELETE') {
      conversations = conversations.filter(c => c.id !== id)
      return new Response(null, { status: 204 })
    }
  }
  if (sub === 'messages') {
    if (method === 'GET') {
      const after = Number(url.searchParams.get('after') ?? 0)
      return json((eventStore[id] ?? []).filter(e => e.seq > after))
    }
    if (method === 'POST') {
      const req = parseBody<SendMessageRequest>(init)
      const list = (eventStore[id] ??= [])
      list.push({ seq: list.length + 1, time: new Date().toISOString(), kind: 'user', text: req.message })
      conv.updated_at = new Date().toISOString()
      return new Response(null, { status: 204 })
    }
  }
  if (sub === 'stop' && method === 'POST') {
    conv.status = 'idle'
    return new Response(null, { status: 204 })
  }
  return null
}

class MockEventSource extends EventTarget {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSED = 2
  readonly url: string
  readyState = 0
  onopen: ((e: Event) => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  private timer: ReturnType<typeof setTimeout>

  constructor(url: string | URL) {
    super()
    this.url = String(url)
    this.timer = setTimeout(() => {
      this.readyState = 1
      this.onopen?.(new Event('open'))
    }, 50)
  }

  close() {
    this.readyState = 2
    clearTimeout(this.timer)
  }
}

let installed = false

export function installChatMock() {
  if (installed) return
  installed = true
  const realFetch = window.fetch.bind(window)
  window.fetch = async (input, init) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    const url = new URL(raw, window.location.origin)
    if (url.pathname.startsWith('/api/conversations') || url.pathname === '/api/claude/status') {
      const res = await handle(url, init)
      if (res) return res
    }
    return realFetch(input, init)
  }
  const RealEventSource = window.EventSource
  window.EventSource = new Proxy(RealEventSource, {
    construct(target, args: [string | URL, EventSourceInit?]) {
      if (String(args[0]).includes('/api/conversations/')) {
        return new MockEventSource(args[0]) as unknown as EventSource
      }
      return new target(...args)
    },
  })
}
