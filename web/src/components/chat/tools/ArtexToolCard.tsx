import { Link } from 'react-router-dom'
import { Crosshair } from 'lucide-react'
import type { ChatEvent } from '../../../lib/chat-types'
import { Expandable, ResultFor, StatusIcon, keyArg } from './GenericCard'

const PREFIX = 'mcp__artex__'

export function prettifyToolName(name: string): string {
  return name
    .replace(PREFIX, '')
    .split('_')
    .filter(Boolean)
    .map(w => w[0].toUpperCase() + w.slice(1))
    .join(' ')
}

function linkFor(key: string, value: unknown): string | null {
  if (typeof value !== 'string' && typeof value !== 'number') return null
  const id = encodeURIComponent(String(value))
  if (key === 'finding_id') return `/findings?id=${id}`
  if (key === 'asset_id') return `/assets?id=${id}`
  return null
}

export function ArtexToolCard({ event, result }: { event: ChatEvent; result?: ChatEvent }) {
  const input = event.input ?? {}
  const links = Object.entries(input).flatMap(([k, v]) => {
    const to = linkFor(k, v)
    return to ? [{ key: k, value: String(v), to }] : []
  })

  return (
    <Expandable
      header={
        <>
          <Crosshair className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="shrink-0 font-medium text-sky-600 dark:text-sky-400">{prettifyToolName(event.tool_name ?? '')}</span>
          <span className="min-w-0 flex-1 truncate font-mono text-muted-foreground">{keyArg(input)}</span>
          <StatusIcon result={result} />
        </>
      }
    >
      <div className="space-y-2">
        {links.length > 0 && (
          <div className="flex flex-wrap gap-2 text-xs">
            {links.map(l => (
              <Link key={l.key} to={l.to} className="font-mono text-primary underline underline-offset-2">
                {l.key.replace('_id', '')} {l.value}
              </Link>
            ))}
          </div>
        )}
        <pre className="max-h-64 overflow-auto font-mono text-xs whitespace-pre-wrap break-all">
          {JSON.stringify(input, null, 2)}
        </pre>
        <ResultFor result={result} />
      </div>
    </Expandable>
  )
}
