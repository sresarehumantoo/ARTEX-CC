import { Terminal } from 'lucide-react'
import type { ChatEvent } from '../../../lib/chat-types'
import { cn } from '../../../lib/utils'
import { Expandable, StatusIcon } from './GenericCard'

export function BashCard({ event, result }: { event: ChatEvent; result?: ChatEvent }) {
  const input = event.input ?? {}
  const command = typeof input.command === 'string' ? input.command : ''
  const description = typeof input.description === 'string' ? input.description : ''
  const output = result?.text ?? ''

  return (
    <Expandable
      header={
        <>
          <Terminal className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="shrink-0 font-mono text-sky-600 dark:text-sky-400">{event.tool_name}</span>
          <span className="min-w-0 flex-1 truncate font-mono text-muted-foreground">{description || command}</span>
          <StatusIcon result={result} />
        </>
      }
    >
      <div className="space-y-2">
        <pre className="max-h-40 overflow-auto rounded-md bg-background/60 px-2 py-1 font-mono text-xs whitespace-pre-wrap break-all">
          <span className="select-none text-muted-foreground">$ </span>
          {command}
        </pre>
        {result && (
          <pre
            className={cn(
              'max-h-80 overflow-auto rounded-md border-l-2 bg-background/60 px-2 py-1 font-mono text-xs whitespace-pre-wrap break-all',
              result.is_error ? 'border-destructive' : 'border-emerald-500',
            )}
          >
            {output || '(no output)'}
          </pre>
        )}
      </div>
    </Expandable>
  )
}
