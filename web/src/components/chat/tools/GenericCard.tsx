import { useState } from 'react'
import { Check, ChevronRight, Loader2, X } from 'lucide-react'
import type { ChatEvent } from '../../../lib/chat-types'
import { cn } from '../../../lib/utils'

export const RESULT_PREVIEW = 160

const KEY_ARGS = ['command', 'file_path', 'path', 'pattern', 'url', 'query', 'value', 'description']

export function keyArg(input: Record<string, unknown>): string {
  for (const k of KEY_ARGS) {
    const v = input[k]
    if (typeof v === 'string' && v) return v
  }
  const first = Object.values(input).find(v => typeof v === 'string') as string | undefined
  return first ?? ''
}

export function Expandable({
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

export function ResultBlock({ content, isError, preview }: { content: string; isError: boolean; preview: string }) {
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

export function StatusIcon({ result }: { result?: ChatEvent }) {
  if (!result) return <Loader2 className="size-3.5 shrink-0 animate-spin text-muted-foreground" aria-label="Running" />
  if (result.is_error) return <X className="size-3.5 shrink-0 text-destructive" aria-label="Error" />
  return <Check className="size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" aria-label="Success" />
}

export function ResultFor({ result }: { result?: ChatEvent }) {
  if (!result) return null
  const text = result.text ?? ''
  return <ResultBlock content={text} isError={result.is_error ?? false} preview={text.replace(/\s+/g, ' ').slice(0, RESULT_PREVIEW)} />
}

export function GenericCard({ event, result }: { event: ChatEvent; result?: ChatEvent }) {
  const arg = event.input ? keyArg(event.input) : ''
  return (
    <Expandable
      header={
        <>
          <span className="shrink-0 font-mono text-sky-600 dark:text-sky-400">{event.tool_name}</span>
          <span className="min-w-0 flex-1 truncate font-mono text-muted-foreground">{arg}</span>
          <StatusIcon result={result} />
        </>
      }
    >
      <div className="space-y-2">
        <pre className="max-h-64 overflow-auto font-mono text-xs whitespace-pre-wrap break-all">
          {JSON.stringify(event.input, null, 2)}
        </pre>
        <ResultFor result={result} />
      </div>
    </Expandable>
  )
}
