import { useMemo, useState } from 'react'
import { CheckSquare, MoreHorizontal, Pencil, Plus, Trash2, X } from 'lucide-react'
import type { Conversation } from '../../lib/chat-types'
import { timeAgo } from '../../lib/format'
import { cn } from '../../lib/utils'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../ui/alert-dialog'
import { modelLabel } from '../../lib/api/llm'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Checkbox } from '../ui/checkbox'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../ui/dropdown-menu'
import { Input } from '../ui/input'
import { ScrollArea } from '../ui/scroll-area'

export function RunningDot({ className }: { className?: string }) {
  return (
    <span className={cn('relative flex size-2 shrink-0', className)} aria-label="Running">
      <span className="absolute inline-flex size-full animate-ping rounded-full bg-sky-500 opacity-75" />
      <span className="relative inline-flex size-2 rounded-full bg-sky-500" />
    </span>
  )
}

function sortConversations(list: Conversation[]) {
  return [...list].sort((a, b) => {
    const ra = a.status === 'running' ? 0 : 1
    const rb = b.status === 'running' ? 0 : 1
    if (ra !== rb) return ra - rb
    return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
  })
}

interface ConversationListProps {
  conversations: Conversation[]
  activeId: string | null
  onSelect: (id: string) => void
  onNew: () => void
  onRename: (id: string, title: string) => void
  onDelete: (ids: string[]) => void
  creating?: boolean
}

export function ConversationList({
  conversations,
  activeId,
  onSelect,
  onNew,
  onRename,
  onDelete,
  creating,
}: ConversationListProps) {
  const [selectMode, setSelectMode] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [editingId, setEditingId] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [pendingDelete, setPendingDelete] = useState<string[] | null>(null)

  const sorted = useMemo(() => sortConversations(conversations), [conversations])

  function startEdit(c: Conversation) {
    setEditingId(c.id)
    setDraft(c.title)
  }

  function commitEdit() {
    if (editingId && draft.trim()) onRename(editingId, draft.trim())
    setEditingId(null)
  }

  function toggle(id: string) {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function exitSelectMode() {
    setSelectMode(false)
    setSelected(new Set())
  }

  const allSelected = sorted.length > 0 && selected.size === sorted.length

  return (
    <div className="flex min-h-0 flex-col rounded-xl bg-card ring-1 ring-foreground/10">
      <div className="flex items-center gap-2 border-b border-border p-2">
        {selectMode ? (
          <>
            <Checkbox
              checked={allSelected}
              onCheckedChange={v => setSelected(v ? new Set(sorted.map(c => c.id)) : new Set())}
              aria-label="Select all"
            />
            <span className="flex-1 text-xs text-muted-foreground">{selected.size} selected</span>
            <Button
              size="xs"
              variant="destructive"
              disabled={selected.size === 0}
              onClick={() => setPendingDelete([...selected])}
            >
              <Trash2 />
              Delete
            </Button>
            <Button size="icon-xs" variant="ghost" onClick={exitSelectMode} aria-label="Exit select mode">
              <X />
            </Button>
          </>
        ) : (
          <>
            <Button size="sm" className="flex-1" onClick={onNew} disabled={creating}>
              <Plus />
              New conversation
            </Button>
            <Button
              size="icon-sm"
              variant="ghost"
              onClick={() => setSelectMode(true)}
              disabled={conversations.length === 0}
              aria-label="Select conversations"
            >
              <CheckSquare />
            </Button>
          </>
        )}
      </div>

      <ScrollArea className="min-h-0 flex-1">
        <ul className="space-y-0.5 p-1.5">
          {sorted.length === 0 && <li className="p-3 text-center text-xs text-muted-foreground">No conversations yet</li>}
          {sorted.map(c => (
            <li key={c.id}>
              <div
                role="button"
                tabIndex={0}
                onClick={() => (selectMode ? toggle(c.id) : onSelect(c.id))}
                onKeyDown={e => {
                  if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
                    e.preventDefault()
                    if (selectMode) toggle(c.id)
                    else onSelect(c.id)
                  }
                }}
                onContextMenu={e => {
                  e.preventDefault()
                  startEdit(c)
                }}
                className={cn(
                  'group flex cursor-pointer items-start gap-2 rounded-lg px-2 py-1.5 outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring/50',
                  c.id === activeId && 'bg-accent',
                )}
              >
                {selectMode && (
                  <Checkbox
                    className="mt-0.5"
                    checked={selected.has(c.id)}
                    onClick={e => e.stopPropagation()}
                    onCheckedChange={() => toggle(c.id)}
                    aria-label={`Select ${c.title}`}
                  />
                )}
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex items-center gap-1.5">
                    {c.status === 'running' && <RunningDot />}
                    {editingId === c.id ? (
                      <Input
                        autoFocus
                        value={draft}
                        className="h-6 px-1.5 text-sm"
                        onClick={e => e.stopPropagation()}
                        onChange={e => setDraft(e.target.value)}
                        onBlur={commitEdit}
                        onKeyDown={e => {
                          e.stopPropagation()
                          if (e.key === 'Enter') commitEdit()
                          if (e.key === 'Escape') setEditingId(null)
                        }}
                      />
                    ) : (
                      <span
                        className="truncate text-sm font-medium"
                        onDoubleClick={e => {
                          e.stopPropagation()
                          startEdit(c)
                        }}
                      >
                        {c.title}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <Badge className="max-w-24 truncate px-1.5 py-0 text-[10px]">{modelLabel(c.model ?? '')}</Badge>
                    {c.status === 'error' && <Badge variant="critical" className="px-1.5 py-0 text-[10px]">error</Badge>}
                    <span>{timeAgo(c.updated_at)}</span>
                  </div>
                </div>
                {!selectMode && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        size="icon-xs"
                        variant="ghost"
                        className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100 data-[state=open]:opacity-100"
                        onClick={e => e.stopPropagation()}
                        aria-label="Conversation actions"
                      >
                        <MoreHorizontal />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" onClick={e => e.stopPropagation()}>
                      <DropdownMenuItem onSelect={() => startEdit(c)}>
                        <Pencil />
                        Rename
                      </DropdownMenuItem>
                      <DropdownMenuItem variant="destructive" onSelect={() => setPendingDelete([c.id])}>
                        <Trash2 />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
            </li>
          ))}
        </ul>
      </ScrollArea>

      <AlertDialog open={pendingDelete !== null} onOpenChange={open => !open && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Delete {pendingDelete?.length === 1 ? 'conversation' : `${pendingDelete?.length ?? 0} conversations`}?
            </AlertDialogTitle>
            <AlertDialogDescription>This removes the transcript permanently.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (pendingDelete) onDelete(pendingDelete)
                setPendingDelete(null)
                exitSelectMode()
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
