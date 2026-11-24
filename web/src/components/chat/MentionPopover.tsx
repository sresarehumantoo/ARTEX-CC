import { useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Bug, Server } from 'lucide-react'
import { cn } from '../../lib/utils'

export interface Mention {
  type: 'finding' | 'asset'
  id: number | string
  label: string
}

export function useMentionResults(query: string | null) {
  const { data } = useQuery({
    queryKey: ['mentions', query],
    queryFn: async (): Promise<Mention[]> => {
      const res = await fetch(`/api/search?q=${encodeURIComponent(query ?? '')}&types=finding,asset`)
      if (!res.ok) return []
      const body = (await res.json()) as { results?: { type: string; id: number | string; title: string }[] }
      return (body.results ?? [])
        .filter(r => r.type === 'finding' || r.type === 'asset')
        .slice(0, 8)
        .map(r => ({ type: r.type as Mention['type'], id: r.id, label: r.title }))
    },
    enabled: query !== null && query.length > 0,
    staleTime: 30_000,
  })
  return query ? (data ?? []) : []
}

interface MentionPopoverProps {
  results: Mention[]
  active: number
  onSelect: (m: Mention) => void
  onHover: (i: number) => void
}

export function MentionPopover({ results, active, onSelect, onHover }: MentionPopoverProps) {
  const activeRef = useRef<HTMLButtonElement>(null)
  useEffect(() => activeRef.current?.scrollIntoView({ block: 'nearest' }), [active])
  if (results.length === 0) return null

  return (
    <div
      role="listbox"
      className="absolute bottom-full left-0 z-20 mb-1 max-h-56 w-72 overflow-y-auto rounded-lg bg-popover p-1 text-popover-foreground shadow-md ring-1 ring-foreground/10"
    >
      {results.map((m, i) => {
        const Icon = m.type === 'finding' ? Bug : Server
        return (
          <button
            key={`${m.type}:${m.id}`}
            ref={i === active ? activeRef : undefined}
            type="button"
            role="option"
            aria-selected={i === active}
            className={cn('flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs', i === active && 'bg-accent text-accent-foreground')}
            onMouseDown={e => {
              e.preventDefault()
              onSelect(m)
            }}
            onMouseEnter={() => onHover(i)}
          >
            <Icon className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate">{m.label}</span>
            <span className="shrink-0 text-muted-foreground">
              {m.type}:{m.id}
            </span>
          </button>
        )
      })}
    </div>
  )
}
