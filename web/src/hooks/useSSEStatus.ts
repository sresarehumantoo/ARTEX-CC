import { useSyncExternalStore } from 'react'

interface SSEStatus {
  connected: boolean
  lastEvent: Date | null
}

let status: SSEStatus = { connected: false, lastEvent: null }
const listeners = new Set<() => void>()

function notify() {
  for (const listener of listeners) listener()
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

function getSnapshot(): SSEStatus {
  return status
}

export function setConnected(connected: boolean) {
  if (status.connected === connected) return
  status = { ...status, connected }
  notify()
}

export function setLastEvent(lastEvent: Date) {
  status = { ...status, lastEvent }
  notify()
}

export function useSSEStatus(): SSEStatus {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
}
