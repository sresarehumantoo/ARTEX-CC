import { useEffect, useReducer } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { getMessages } from '../lib/api/chat'
import type { ChatEvent, ConversationStatus } from '../lib/chat-types'
import { conversationsKey } from './useConversations'

export interface StreamState {
  events: ChatEvent[]
  status: ConversationStatus | null
  error: string | null
}

export type StreamAction =
  | { type: 'reset' }
  | { type: 'merge'; events: ChatEvent[] }
  | { type: 'status'; status: ConversationStatus }
  | { type: 'error'; error: string | null }

export const initialStreamState: StreamState = { events: [], status: null, error: null }

export function streamReducer(state: StreamState, action: StreamAction): StreamState {
  switch (action.type) {
    case 'reset':
      return initialStreamState
    case 'merge': {
      if (action.events.length === 0) return state
      const bySeq = new Map(state.events.map(e => [e.seq, e]))
      for (const e of action.events) bySeq.set(e.seq, e)
      return { ...state, events: [...bySeq.values()].sort((a, b) => a.seq - b.seq) }
    }
    case 'status':
      return { ...state, status: action.status }
    case 'error':
      return { ...state, error: action.error }
  }
}

const INITIAL_DELAY = 1000
const MAX_DELAY = 30000

export function useConversationStream(id: string | null) {
  const qc = useQueryClient()
  const [state, dispatch] = useReducer(streamReducer, initialStreamState)

  useEffect(() => {
    dispatch({ type: 'reset' })
    if (!id) return

    let disposed = false
    let es: EventSource | null = null
    let timer: ReturnType<typeof setTimeout> | null = null
    let delay = INITIAL_DELAY
    let lastSeq = 0

    const refreshList = () => qc.invalidateQueries({ queryKey: conversationsKey })

    function connect() {
      const url = `/api/conversations/${id}/stream?last_event_id=${lastSeq}`
      es = new EventSource(url)

      es.onopen = () => {
        delay = INITIAL_DELAY
        dispatch({ type: 'error', error: null })
      }

      es.onmessage = (e: MessageEvent<string>) => {
        try {
          const ev = JSON.parse(e.data) as ChatEvent
          if (ev.seq > lastSeq) lastSeq = ev.seq
          dispatch({ type: 'merge', events: [ev] })
          if (ev.kind === 'error') refreshList()
          if (ev.kind === 'result') refreshList()
        } catch {
          return
        }
      }

      es.onerror = () => {
        es?.close()
        if (disposed) return
        dispatch({ type: 'error', error: 'Connection lost, reconnecting' })
        timer = setTimeout(() => {
          timer = null
          if (!disposed) void backfill().then(connect)
        }, delay)
        delay = Math.min(delay * 2, MAX_DELAY)
      }
    }

    async function backfill() {
      try {
        const events = await getMessages(id!, lastSeq || undefined)
        if (disposed) return
        if (events.length > 0) lastSeq = Math.max(lastSeq, ...events.map(e => e.seq))
        dispatch({ type: 'merge', events })
      } catch (err) {
        if (!disposed) dispatch({ type: 'error', error: err instanceof Error ? err.message : 'Failed to load messages' })
      }
    }

    void backfill().then(() => {
      if (!disposed) connect()
    })

    return () => {
      disposed = true
      es?.close()
      if (timer !== null) clearTimeout(timer)
    }
  }, [id, qc])

  return state
}
