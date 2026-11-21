import { useEffect, useRef } from 'react'

export interface SSEEvent {
  type: string
  data: unknown
}

const INITIAL_DELAY = 1000
const MAX_DELAY = 30000

export function useSSE(url: string, onEvent: (event: SSEEvent) => void) {
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const delayRef = useRef(INITIAL_DELAY)

  useEffect(() => {
    let disposed = false

    function connect() {
      const es = new EventSource(url)

      es.onmessage = (e) => {
        delayRef.current = INITIAL_DELAY
        try {
          const data = JSON.parse(e.data) as SSEEvent
          onEvent(data)
        } catch {
          // skip malformed events
        }
      }

      es.onerror = () => {
        es.close()
        if (disposed) return
        timeoutRef.current = setTimeout(() => {
          timeoutRef.current = null
          if (!disposed) connect()
        }, delayRef.current)
        delayRef.current = Math.min(delayRef.current * 2, MAX_DELAY)
      }

      return es
    }

    const es = connect()

    return () => {
      disposed = true
      es.close()
      if (timeoutRef.current !== null) clearTimeout(timeoutRef.current)
    }
  }, [url, onEvent])
}
