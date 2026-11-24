import { ChevronRight, FileText, Pencil } from 'lucide-react'
import type { ChatEvent } from '../../../lib/chat-types'
import { Expandable, ResultFor, StatusIcon } from './GenericCard'

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}

function Breadcrumb({ path }: { path: string }) {
  const parts = path.split('/').filter(Boolean)
  return (
    <span className="flex min-w-0 flex-1 items-center gap-0.5 overflow-hidden font-mono text-muted-foreground" title={path}>
      {parts.map((p, i) => (
        <span key={i} className="flex shrink-0 items-center gap-0.5 last:min-w-0 last:shrink last:truncate last:text-foreground">
          {i > 0 && <ChevronRight className="size-2.5" />}
          {p}
        </span>
      ))}
    </span>
  )
}

function Diff({ oldText, newText }: { oldText: string; newText: string }) {
  return (
    <pre className="max-h-80 overflow-auto rounded-md bg-background/60 py-1 font-mono text-xs">
      {oldText.split('\n').map((l, i) => (
        <div key={`o${i}`} className="bg-red-500/10 px-2 whitespace-pre-wrap break-all text-red-700 dark:text-red-400">
          {'- '}
          {l}
        </div>
      ))}
      {newText.split('\n').map((l, i) => (
        <div key={`n${i}`} className="bg-emerald-500/10 px-2 whitespace-pre-wrap break-all text-emerald-700 dark:text-emerald-400">
          {'+ '}
          {l}
        </div>
      ))}
    </pre>
  )
}

export function FileCard({ event, result }: { event: ChatEvent; result?: ChatEvent }) {
  const input = event.input ?? {}
  const path = str(input.file_path) || str(input.path)
  const isEdit = event.tool_name === 'Edit' || event.tool_name === 'MultiEdit'
  const Icon = isEdit || event.tool_name === 'Write' ? Pencil : FileText
  const edits = Array.isArray(input.edits)
    ? (input.edits as Record<string, unknown>[]).map(e => ({ o: str(e.old_string), n: str(e.new_string) }))
    : [{ o: str(input.old_string), n: str(input.new_string) }]

  return (
    <Expandable
      header={
        <>
          <Icon className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="shrink-0 font-mono text-sky-600 dark:text-sky-400">{event.tool_name}</span>
          <Breadcrumb path={path} />
          <StatusIcon result={result} />
        </>
      }
    >
      <div className="space-y-2">
        {isEdit && edits.map((e, i) => <Diff key={i} oldText={e.o} newText={e.n} />)}
        {event.tool_name === 'Write' && (
          <pre className="max-h-64 overflow-auto font-mono text-xs whitespace-pre-wrap break-all">{str(input.content)}</pre>
        )}
        <ResultFor result={result} />
      </div>
    </Expandable>
  )
}
