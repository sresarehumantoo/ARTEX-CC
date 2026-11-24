import { useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { MessageSquare, Square } from 'lucide-react'
import { Composer, MODELS } from '../components/chat/Composer'
import { ConversationList, RunningDot } from '../components/chat/ConversationList'
import { UsageBar } from '../components/chat/UsageBar'
import { Transcript } from '../components/chat/Transcript'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Select } from '../components/ui/select'
import { useChatComposer } from '../hooks/useChatComposer'
import { useConversations } from '../hooks/useConversations'
import { useConversationStream } from '../hooks/useConversationStream'
import type { ChatEvent } from '../lib/chat-types'

const PANE = 'rounded-xl bg-card ring-1 ring-foreground/10'

function lastUsage(events: ChatEvent[]) {
  for (let i = events.length - 1; i >= 0; i--) {
    if (events[i].kind === 'result' && events[i].usage) return events[i].usage!
  }
  return null
}

export default function Chat() {
  const [params, setParams] = useSearchParams()
  const activeId = params.get('c')
  const [newModel, setNewModel] = useState('sonnet')
  const { conversations, create, rename, remove } = useConversations()
  const conversation = conversations.find(c => c.id === activeId) ?? null
  const stream = useConversationStream(conversation?.id ?? null)
  const composer = useChatComposer(conversation?.id ?? null)

  const status = stream.status ?? conversation?.status ?? null
  const usage = useMemo(() => lastUsage(stream.events), [stream.events])

  function select(id: string | null) {
    const next = new URLSearchParams(params)
    if (id) next.set('c', id)
    else next.delete('c')
    setParams(next)
  }

  function createAndSelect(model: string) {
    create.mutate({ model }, { onSuccess: c => select(c.id) })
  }

  function handleDelete(ids: string[]) {
    remove.mutate(ids, { onSuccess: () => activeId && ids.includes(activeId) && select(null) })
  }

  return (
    <div className="grid h-[calc(100svh-3rem)] grid-rows-[minmax(0,14rem)_minmax(0,1fr)] gap-3 p-3 md:grid-cols-[18rem_1fr] md:grid-rows-1">
      <ConversationList
        conversations={conversations}
        activeId={activeId}
        onSelect={select}
        onNew={() => createAndSelect(newModel)}
        onRename={(id, title) => rename.mutate({ id, title })}
        onDelete={handleDelete}
        creating={create.isPending}
      />

      {conversation ? (
        <div className={`${PANE} flex min-h-0 min-w-0 flex-col`}>
          <div className="flex items-center gap-2 border-b border-border px-3 py-2">
            <Badge className="px-1.5 py-0 text-[10px]">{conversation.model}</Badge>
            <h2 className="min-w-0 flex-1 truncate text-sm font-medium">{conversation.title}</h2>
            <UsageBar usage={usage} />
            {status === 'running' && (
              <>
                <span className="flex items-center gap-1.5 text-xs text-sky-600 dark:text-sky-400">
                  <RunningDot />
                  Live
                </span>
                <Button size="xs" variant="destructive" onClick={() => composer.stop.mutate()} aria-label="Stop">
                  <Square className="fill-current" />
                  Stop
                </Button>
              </>
            )}
          </div>
          {stream.error && <div className="px-3 py-1 text-xs text-muted-foreground">{stream.error}</div>}
          <Transcript key={conversation.id} events={stream.events} />
          <Composer
            text={composer.text}
            onTextChange={composer.setText}
            onSubmit={composer.submit}
            onStop={() => composer.stop.mutate()}
            model={conversation.model ?? 'sonnet'}
            onModelChange={m => composer.setModel.mutate(m)}
            status={status}
            sending={composer.send.isPending}
          />
        </div>
      ) : (
        <div className={`${PANE} flex min-h-0 flex-col items-center justify-center gap-3 p-6 text-center`}>
          <div className="flex size-12 items-center justify-center rounded-full bg-primary/10 text-primary">
            <MessageSquare className="size-6" />
          </div>
          <h2 className="text-base font-medium">Start a new conversation</h2>
          <p className="max-w-sm text-xs text-muted-foreground">
            Pick a model and begin, or select an existing conversation from the list.
          </p>
          <div className="flex items-center gap-2">
            <Select size="sm" value={newModel} onChange={e => setNewModel(e.target.value)} aria-label="Model">
              {MODELS.map(m => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </Select>
            <Button size="sm" onClick={() => createAndSelect(newModel)} disabled={create.isPending}>
              New conversation
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
