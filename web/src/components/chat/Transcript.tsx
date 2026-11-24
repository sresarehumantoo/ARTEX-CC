import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { ArrowDown, Bot } from 'lucide-react'
import type { ChatEvent, ConversationStatus } from '../../lib/chat-types'
import { Button } from '../ui/button'
import { ScrollArea } from '../ui/scroll-area'
import { MarkdownBody } from './MarkdownBody'
import { Expandable, RESULT_PREVIEW, ResultBlock, ToolRenderer } from './tools'

function OrphanResult({ event }: { event: ChatEvent }) {
  const content = event.text ?? ''
  return (
    <ResultBlock content={content} isError={event.is_error ?? false} preview={content.slice(0, RESULT_PREVIEW)} />
  )
}

function Thinking({ text }: { text: string }) {
  return (
    <Expandable header={<span className="italic text-muted-foreground">Thinking...</span>} className="bg-transparent ring-0">
      <p className="text-xs whitespace-pre-wrap italic text-muted-foreground">{text}</p>
    </Expandable>
  )
}

function WorkingIndicator({ events }: { events: ChatEvent[] }) {
  const last = events.findLast(e => e.kind !== 'result')
  if (last?.kind === 'text' || last?.kind === 'error') return null
  let label = 'Thinking...'
  if (last?.kind === 'tool_use') label = `Running ${last.tool_name ?? 'tool'}...`
  else if (last?.kind === 'tool_result') label = 'Analyzing...'
  return (
    <div className="flex items-center gap-2 px-1 py-1 text-sm text-muted-foreground" role="status">
      <span className="flex items-center gap-1" aria-hidden>
        {[0, 150, 300].map(d => (
          <span key={d} className="size-1.5 animate-pulse rounded-full bg-current" style={{ animationDelay: `${d}ms` }} />
        ))}
      </span>
      <span>{label}</span>
    </div>
  )
}

function EventRow({ event, results }: { event: ChatEvent; results: Map<string, ChatEvent> }) {
  if (event.kind === 'user') {
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
  if (event.kind === 'tool_use' && event.tool_id) return <ToolRenderer event={event} result={results.get(event.tool_id)} />
  if (event.kind === 'error') {
    return (
      <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {event.text}
      </div>
    )
  }
  return null
}

export function Transcript({ events, status }: { events: ChatEvent[]; status?: ConversationStatus | null }) {
  const viewportRef = useRef<HTMLDivElement | null>(null)
  const rootRef = useRef<HTMLDivElement | null>(null)
  const stuckRef = useRef(true)
  const [stuck, setStuck] = useState(true)

  const { rows, results } = useMemo(() => {
    const results = new Map<string, ChatEvent>()
    const toolIds = new Set<string>()
    for (const e of events) {
      if (e.kind === 'tool_use' && e.tool_id) toolIds.add(e.tool_id)
    }
    for (const e of events) {
      if (e.kind === 'tool_result' && e.tool_id) results.set(e.tool_id, e)
    }
    const rows = events.filter(e => {
      if (e.kind === 'result') return false
      if (e.kind === 'tool_result') return !e.tool_id || !toolIds.has(e.tool_id)
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
  }, [rows, status, scrollToBottom])

  return (
    <div ref={rootRef} className="relative min-h-0 flex-1">
      <ScrollArea className="h-full">
        <div className="space-y-2 p-3">
          {rows.map(e => (
            <div key={e.seq}>
              {e.kind === 'tool_result' ? <OrphanResult event={e} /> : <EventRow event={e} results={results} />}
            </div>
          ))}
          {status === 'running' && <WorkingIndicator events={events} />}
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
