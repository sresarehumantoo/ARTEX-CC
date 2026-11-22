import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { ChevronRight, Pause, Play, Trash2 } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Select } from '../components/ui/select'
import { cn } from '../lib/utils'

interface LogEntry {
  seq: number
  level: 'debug' | 'info' | 'warn' | 'error'
  time: string
  message: string
  attrs: Record<string, unknown> | null
}

const LEVELS = ['debug', 'info', 'warn', 'error'] as const
const MAX_ENTRIES = 2000
const INITIAL_DELAY = 1000
const MAX_DELAY = 30000
const STICK_THRESHOLD = 48

const LEVEL_STYLES: Record<string, string> = {
  debug: 'bg-[oklch(0.65_0.01_260/0.15)] text-[oklch(0.72_0.01_260)]',
  info: 'bg-[oklch(0.65_0.15_250/0.15)] text-[oklch(0.75_0.13_250)]',
  warn: 'bg-[oklch(0.8_0.15_85/0.15)] text-[oklch(0.85_0.14_85)]',
  error: 'bg-[oklch(0.63_0.22_25/0.15)] text-[oklch(0.75_0.18_25)]',
}

function rank(level: string) {
  return Math.max(0, LEVELS.indexOf(level as (typeof LEVELS)[number]))
}

function formatTime(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const p = (n: number, w = 2) => String(n).padStart(w, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}.${p(d.getMilliseconds(), 3)}`
}

function formatValue(v: unknown) {
  return typeof v === 'string' ? v : JSON.stringify(v)
}

function LogLine({ entry }: { entry: LogEntry }) {
  const [open, setOpen] = useState(false)
  const attrs = entry.attrs ? Object.entries(entry.attrs) : []
  const hasAttrs = attrs.length > 0

  return (
    <div className="px-3 py-0.5 hover:bg-[oklch(1_0_0/0.04)]">
      <div className="flex items-start gap-2">
        <button
          type="button"
          disabled={!hasAttrs}
          aria-label={open ? 'Collapse attributes' : 'Expand attributes'}
          aria-expanded={open}
          onClick={() => setOpen(v => !v)}
          className={cn('mt-0.5 h-4 w-4 shrink-0 text-[oklch(0.6_0.01_260)]', !hasAttrs && 'invisible')}
        >
          <ChevronRight className={cn('h-4 w-4 transition-transform', open && 'rotate-90')} />
        </button>
        <span className="shrink-0 text-[oklch(0.6_0.01_260)]">{formatTime(entry.time)}</span>
        <span className={cn('w-12 shrink-0 rounded px-1.5 text-center text-xs uppercase leading-5', LEVEL_STYLES[entry.level])}>
          {entry.level}
        </span>
        <span className="min-w-0 flex-1 whitespace-pre-wrap break-words text-[oklch(0.9_0.01_260)]">
          {entry.message}
          {!open && hasAttrs && (
            <span className="ml-2 text-[oklch(0.6_0.01_260)]">
              {attrs.map(([k, v]) => `${k}=${formatValue(v)}`).join(' ')}
            </span>
          )}
        </span>
      </div>
      {open && hasAttrs && (
        <div className="ml-[7.5rem] mt-1 mb-1 grid grid-cols-[auto_1fr] gap-x-3 text-xs">
          {attrs.map(([k, v]) => (
            <div key={k} className="contents">
              <span className="text-[oklch(0.75_0.13_250)]">{k}</span>
              <span className="whitespace-pre-wrap break-all text-[oklch(0.85_0.01_260)]">{formatValue(v)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default function Logs() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [level, setLevel] = useState<string>('debug')
  const [search, setSearch] = useState('')
  const [paused, setPaused] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const lastSeq = useRef(0)
  const scroller = useRef<HTMLDivElement>(null)
  const stick = useRef(true)

  const append = useCallback((incoming: LogEntry[]) => {
    if (incoming.length === 0) return
    setEntries(prev => {
      const seen = new Set(prev.map(e => e.seq))
      const fresh = incoming.filter(e => !seen.has(e.seq))
      if (fresh.length === 0) return prev
      return [...prev, ...fresh].slice(-MAX_ENTRIES)
    })
  }, [])

  useEffect(() => {
    if (paused) return

    let disposed = false
    let es: EventSource | null = null
    let timer: ReturnType<typeof setTimeout> | null = null
    let delay = INITIAL_DELAY

    function connect() {
      es = new EventSource(`/api/logs/stream?after=${lastSeq.current}`)
      es.onopen = () => {
        delay = INITIAL_DELAY
        setError(null)
      }
      es.onmessage = (e: MessageEvent<string>) => {
        try {
          const entry = JSON.parse(e.data) as LogEntry
          if (entry.seq > lastSeq.current) lastSeq.current = entry.seq
          append([entry])
        } catch {
          return
        }
      }
      es.onerror = () => {
        es?.close()
        if (disposed) return
        setError('Connection lost, reconnecting')
        timer = setTimeout(connect, delay)
        delay = Math.min(delay * 2, MAX_DELAY)
      }
    }

    connect()
    return () => {
      disposed = true
      es?.close()
      if (timer !== null) clearTimeout(timer)
    }
  }, [paused, append])

  const visible = useMemo(() => {
    const min = rank(level)
    const q = search.trim().toLowerCase()
    return entries.filter(e => {
      if (rank(e.level) < min) return false
      if (!q) return true
      return e.message.toLowerCase().includes(q) || JSON.stringify(e.attrs ?? {}).toLowerCase().includes(q)
    })
  }, [entries, level, search])

  useLayoutEffect(() => {
    const el = scroller.current
    if (el && stick.current && !paused) el.scrollTop = el.scrollHeight
  }, [visible, paused])

  const onScroll = () => {
    const el = scroller.current
    if (!el) return
    stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < STICK_THRESHOLD
  }

  const clear = () => {
    setEntries([])
  }

  return (
    <div className="flex h-full flex-col">
      <div className="border-b border-border bg-muted px-3 py-3 md:px-4">
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="mr-2 text-lg font-semibold text-foreground">Logs</h1>
          <Select
            value={level}
            onChange={e => setLevel(e.target.value)}
            aria-label="Minimum level"
          >
            <option value="debug">All levels</option>
            <option value="info">Info and above</option>
            <option value="warn">Warn and above</option>
            <option value="error">Error only</option>
          </Select>
          <Input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search logs"
            aria-label="Search logs"
            className="w-full sm:w-64"
          />
          <div className="ml-auto flex items-center gap-2">
            {error && !paused && <span className="text-xs text-muted-foreground">{error}</span>}
            <span className="text-xs text-muted-foreground tabular-nums">{visible.length} lines</span>
            <Button variant="outline" size="sm" onClick={() => setPaused(v => !v)}>
              {paused ? <Play className="mr-1 h-4 w-4" /> : <Pause className="mr-1 h-4 w-4" />}
              {paused ? 'Resume' : 'Pause'}
            </Button>
            <Button variant="outline" size="sm" onClick={clear} disabled={entries.length === 0}>
              <Trash2 className="mr-1 h-4 w-4" />
              Clear
            </Button>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 p-3 md:p-4">
        <div
          ref={scroller}
          onScroll={onScroll}
          className="h-full overflow-y-auto rounded-xl bg-[oklch(0.17_0.01_260)] py-2 font-mono text-[13px] leading-5 ring-1 ring-foreground/10"
        >
          {visible.length === 0 ? (
            <p className="px-3 py-2 text-[oklch(0.6_0.01_260)]">{paused ? 'Paused' : 'Waiting for log output'}</p>
          ) : (
            visible.map(e => <LogLine key={e.seq} entry={e} />)
          )}
        </div>
      </div>
    </div>
  )
}
