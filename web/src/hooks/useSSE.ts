import { useEffect } from 'react'

export interface SSEEvent {
  type: string
  data: unknown
}

export function useSSE(url: string, onEvent: (event: SSEEvent) => void) {
  useEffect(() => {
    const es = new EventSource(url)
    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data) as SSEEvent
        onEvent(data)
      } catch {
        // skip malformed events
      }
    }
    return () => es.close()
  }, [url, onEvent])
}
