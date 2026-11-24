import { useEffect, useRef } from 'react'
import { setConnected, setLastEvent } from './useSSEStatus'

export interface SSEEvent {
  type: string
  data: unknown
}

const INITIAL_DELAY = 1000
const MAX_DELAY = 30000

export function useSSE(url: string, onEvent: (event: SSEEvent) => void) {
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  const esRef = useRef<EventSource | null>(null)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const delayRef = useRef(INITIAL_DELAY)

  useEffect(() => {
    let disposed = false

    function connect() {
      const es = new EventSource(url)
      esRef.current = es

      es.onopen = () => {
        setConnected(true)
        delayRef.current = INITIAL_DELAY
      }

      es.onmessage = (e) => {
        setLastEvent(new Date())
        delayRef.current = INITIAL_DELAY
        try {
          const data = JSON.parse(e.data) as SSEEvent
          onEventRef.current(data)
        } catch {
          // skip malformed events
        }
      }

      es.onerror = () => {
        setConnected(false)
        es.close()
        if (disposed) return
        timeoutRef.current = setTimeout(() => {
          timeoutRef.current = null
          if (!disposed) connect()
        }, delayRef.current)
        delayRef.current = Math.min(delayRef.current * 2, MAX_DELAY)
      }
    }

    connect()

    return () => {
      disposed = true
      setConnected(false)
      esRef.current?.close()
      esRef.current = null
      if (timeoutRef.current !== null) clearTimeout(timeoutRef.current)
    }
  }, [url])
}
