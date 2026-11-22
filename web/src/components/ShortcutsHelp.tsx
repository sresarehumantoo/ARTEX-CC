import { useEffect, useRef } from 'react'

const SHORTCUTS = [
  {
    section: 'Search',
    items: [
      { key: '⌘K / Ctrl+K', label: 'Open search' },
    ],
  },
  {
    section: 'Navigation',
    items: [
      { key: '1', label: 'Dashboard' },
      { key: '2', label: 'Graph' },
      { key: '3', label: 'Findings' },
      { key: '4', label: 'Traffic' },
      { key: '5', label: 'Assets' },
      { key: '6', label: 'Activity' },
      { key: '7', label: 'Coverage' },
      { key: '8', label: 'Credentials' },
      { key: '9', label: 'Report' },
      { key: '0', label: 'Settings' },
    ],
  },
  {
    section: 'Tables',
    items: [
      { key: '↑↓', label: 'Navigate rows' },
      { key: 'Esc', label: 'Close detail panel' },
    ],
  },
]

export function ShortcutsHelp({ open, onClose }: { open: boolean; onClose: () => void }) {
  const overlayRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onClose()
      }
    }
    document.addEventListener('keydown', handleKey, true)
    return () => document.removeEventListener('keydown', handleKey, true)
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      ref={overlayRef}
      role="dialog"
      aria-label="Keyboard shortcuts"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={e => {
        if (e.target === overlayRef.current) onClose()
      }}
    >
      <div className="w-80 rounded-lg border border-border bg-background p-5 shadow-xl">
        <h2 className="mb-4 text-sm font-semibold text-foreground">Keyboard Shortcuts</h2>
        {SHORTCUTS.map(group => (
          <div key={group.section} className="mb-4">
            <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {group.section}
            </h3>
            <div className="flex flex-col gap-1.5">
              {group.items.map(item => (
                <div key={item.key} className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">{item.label}</span>
                  <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-xs text-foreground">
                    {item.key}
                  </kbd>
                </div>
              ))}
            </div>
          </div>
        ))}
        <div className="flex items-center justify-between border-t border-border pt-3">
          <span className="text-sm text-muted-foreground">Toggle this help</span>
          <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-xs text-foreground">
            ?
          </kbd>
        </div>
      </div>
    </div>
  )
}
