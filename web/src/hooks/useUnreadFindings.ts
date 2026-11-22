import { useSyncExternalStore } from 'react'

let unreadCount = 0
const listeners = new Set<() => void>()

function notify() {
  for (const listener of listeners) listener()
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

function getSnapshot(): number {
  return unreadCount
}

export function incrementUnread() {
  unreadCount++
  notify()
}

export function clearUnread() {
  if (unreadCount === 0) return
  unreadCount = 0
  notify()
}

export function useUnreadFindings(): number {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
}
