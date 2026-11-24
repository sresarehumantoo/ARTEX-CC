import { useCallback, useEffect, useSyncExternalStore } from 'react'

export type ThemeMode = 'light' | 'dark' | 'system'
export type ContentLayout = 'full-width' | 'centered'

interface Preferences {
  theme_mode: ThemeMode
  sidebar_collapsed: boolean
  content_layout: ContentLayout
}

const KEYS = {
  theme_mode: 'artex-theme',
  sidebar_collapsed: 'sidebar-collapsed',
  content_layout: 'artex-content-layout',
} as const

function read(): Preferences {
  let theme: string | null = null
  let collapsed: string | null = null
  let layout: string | null = null
  try {
    theme = localStorage.getItem(KEYS.theme_mode)
    collapsed = localStorage.getItem(KEYS.sidebar_collapsed)
    layout = localStorage.getItem(KEYS.content_layout)
  } catch {}
  return {
    theme_mode: theme === 'light' || theme === 'dark' || theme === 'system' ? theme : 'system',
    sidebar_collapsed: collapsed === 'true',
    content_layout: layout === 'centered' ? 'centered' : 'full-width',
  }
}

let state = read()
const listeners = new Set<() => void>()

function set<K extends keyof Preferences>(key: K, value: Preferences[K]) {
  if (state[key] === value) return
  state = { ...state, [key]: value }
  try { localStorage.setItem(KEYS[key], String(value)) } catch {}
  listeners.forEach(l => l())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

function getSnapshot() {
  return state
}

export function applyTheme(mode: ThemeMode) {
  const dark = mode === 'system' ? window.matchMedia('(prefers-color-scheme: dark)').matches : mode === 'dark'
  document.documentElement.classList.toggle('dark', dark)
}

export function usePreferences() {
  const prefs = useSyncExternalStore(subscribe, getSnapshot)

  useEffect(() => {
    applyTheme(prefs.theme_mode)
    if (prefs.theme_mode !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => applyTheme('system')
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [prefs.theme_mode])

  const setThemeMode = useCallback((v: ThemeMode) => set('theme_mode', v), [])
  const setSidebarCollapsed = useCallback((v: boolean) => set('sidebar_collapsed', v), [])
  const setContentLayout = useCallback((v: ContentLayout) => set('content_layout', v), [])

  return { ...prefs, setThemeMode, setSidebarCollapsed, setContentLayout }
}
