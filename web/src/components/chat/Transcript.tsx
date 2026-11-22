import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { ArrowDown, Bot, Check, ChevronRight, X } from 'lucide-react'
import type { ChatEvent } from '../../lib/chat-types'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { ScrollArea } from '../ui/scroll-area'
import { MarkdownBody } from './MarkdownBody'

const KEY_ARGS = ['command', 'file_path', 'path', 'pattern', 'url', 'query', 'value', 'description']
const RESULT_PREVIEW = 160

function keyArg(input: Record<string, unknown>): string {
  for (const k of KEY_ARGS) {
    const v = input[k]
    if (typeof v === 'string' && v) return v
  }
  const first = Object.values(input).find(v => typeof v === 'string') as string | undefined
  return first ?? ''
}

function Expandable({
  header,
  children,
  className,
}: {
  header: React.ReactNode
  children: React.ReactNode
  className?: string
}) {
  const [open, setOpen] = useState(false)
  return (
    <div className={cn('min-w-0 rounded-lg ring-1 ring-foreground/10 bg-muted/40', className)}>
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        aria-expanded={open}
        className="flex w-full min-w-0 items-center gap-1.5 px-2 py-1 text-left text-xs"
      >
        <ChevronRight className={cn('size-3 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
        {header}
      </button>
      {open && <div className="border-t border-border px-2 py-1.5">{children}</div>}
    </div>
  )
}

function ToolCard({ event, result }: { event: ChatEvent; result?: ChatEvent }) {
  const tool = event.tool!
  const res = result?.tool_result
  const arg = keyArg(tool.input)
  const preview = res ? res.content.replace(/\s+/g, ' ').slice(0, RESULT_PREVIEW) : ''

  return (
    <Expandable
      header={
        <>
          <span className="shrink-0 font-mono text-sky-600 dark:text-sky-400">{tool.name}</span>
          <span className="min-w-0 flex-1 truncate font-mono text-muted-foreground">{arg}</span>
          {res &&
            (res.is_error ? (
              <X className="size-3.5 shrink-0 text-destructive" aria-label="Error" />
            ) : (
              <Check className="size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" aria-label="Success" />
            ))}
        </>
      }
    >
      <div className="space-y-2">
        <pre className="max-h-64 overflow-auto font-mono text-xs whitespace-pre-wrap break-all">
          {JSON.stringify(tool.input, null, 2)}
        </pre>
        {res && (
          <ResultBlock content={res.content} isError={res.is_error} preview={preview} />
        )}
      </div>
    </Expandable>
  )
}

function ResultBlock({ content, isError, preview }: { content: string; isError: boolean; preview: string }) {
  const [open, setOpen] = useState(false)
  const truncated = content.length > preview.length
  return (
    <div className={cn('rounded-md border px-2 py-1', isError ? 'border-destructive/30 bg-destructive/5' : 'border-border bg-background/60')}>
      <div className="flex items-center gap-1 text-xs text-muted-foreground">
        {isError ? <X className="size-3 text-destructive" /> : <Check className="size-3 text-emerald-600 dark:text-emerald-400" />}
        <span>{isError ? 'Error' : 'Result'}</span>
        {truncated && (
          <button type="button" className="ml-auto underline underline-offset-2" onClick={() => setOpen(v => !v)}>
            {open ? 'Collapse' : 'Expand'}
          </button>
        )}
      </div>
      <pre className={cn('mt-1 overflow-auto font-mono text-xs whitespace-pre-wrap break-all', open ? 'max-h-96' : 'max-h-20')}>
        {open || !truncated ? content : `${preview}...`}
      </pre>
    </div>
  )
}

function OrphanResult({ event }: { event: ChatEvent }) {
  const res = event.tool_result!
  return (
    <ResultBlock content={res.content} isError={res.is_error} preview={res.content.slice(0, RESULT_PREVIEW)} />
  )
}

function Thinking({ text }: { text: string }) {
  return (
    <Expandable header={<span className="italic text-muted-foreground">Thinking...</span>} className="bg-transparent ring-0">
      <p className="text-xs whitespace-pre-wrap italic text-muted-foreground">{text}</p>
    </Expandable>
  )
}

function EventRow({ event, results }: { event: ChatEvent; results: Map<string, ChatEvent> }) {
  if (event.kind === 'text' && event.role === 'user') {
    return (
      <div className="flex justify-end">
        <div className="max-w-[85%] rounded-2xl bg-primary px-3 py-2 text-sm whitespace-pre-wrap break-words text-primary-foreground">
          {event.text}
        </div>
      </div>
    )
  }
  if (event.kind === 'text') {
    return (
      <div className="flex gap-2">
        <div className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
          <Bot className="size-3.5" />
        </div>
        <MarkdownBody text={event.text ?? ''} className="flex-1" />
      </div>
    )
  }
  if (event.kind === 'thinking') return <Thinking text={event.text ?? ''} />
  if (event.kind === 'tool_use' && event.tool) return <ToolCard event={event} result={results.get(event.tool.id)} />
  if (event.kind === 'error') {
    return (
      <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {event.text}
      </div>
    )
  }
  return null
}

export function Transcript({ events }: { events: ChatEvent[] }) {
  const viewportRef = useRef<HTMLDivElement | null>(null)
  const rootRef = useRef<HTMLDivElement | null>(null)
  const stuckRef = useRef(true)
  const [stuck, setStuck] = useState(true)

  const { rows, results } = useMemo(() => {
    const results = new Map<string, ChatEvent>()
    const toolIds = new Set<string>()
    for (const e of events) {
      if (e.kind === 'tool_use' && e.tool) toolIds.add(e.tool.id)
    }
    for (const e of events) {
      if (e.kind === 'tool_result' && e.tool_result) results.set(e.tool_result.tool_use_id, e)
    }
    const rows = events.filter(e => {
      if (e.kind === 'result' || e.kind === 'status') return false
      if (e.kind === 'tool_result') return !e.tool_result || !toolIds.has(e.tool_result.tool_use_id)
      return true
    })
    return { rows, results }
  }, [events])

  useEffect(() => {
    const viewport = rootRef.current?.querySelector<HTMLDivElement>('[data-slot="scroll-area-viewport"]') ?? null
    viewportRef.current = viewport
    if (!viewport) return
    const onScroll = () => {
      const atBottom = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight < 48
      stuckRef.current = atBottom
      setStuck(atBottom)
    }
    viewport.addEventListener('scroll', onScroll, { passive: true })
    return () => viewport.removeEventListener('scroll', onScroll)
  }, [])

  const scrollToBottom = useCallback(() => {
    const v = viewportRef.current
    if (v) v.scrollTop = v.scrollHeight
  }, [])

  useLayoutEffect(() => {
    if (stuckRef.current) scrollToBottom()
  }, [rows, scrollToBottom])

  return (
    <div ref={rootRef} className="relative min-h-0 flex-1">
      <ScrollArea className="h-full">
        <div className="space-y-2 p-3">
          {rows.map(e => (
            <div key={e.seq}>
              {e.kind === 'tool_result' ? <OrphanResult event={e} /> : <EventRow event={e} results={results} />}
            </div>
          ))}
        </div>
      </ScrollArea>
      {!stuck && (
        <Button
          size="xs"
          variant="secondary"
          className="absolute bottom-3 left-1/2 -translate-x-1/2 shadow-md"
          onClick={() => {
            stuckRef.current = true
            setStuck(true)
            scrollToBottom()
          }}
        >
          <ArrowDown />
          New messages
        </Button>
      )}
    </div>
  )
}
