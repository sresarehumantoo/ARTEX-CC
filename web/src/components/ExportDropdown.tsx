import { Download } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from './ui/button'
import { cn } from '../lib/utils'

export interface ExportOption {
  label: string
  onClick: () => void
}

export function ExportDropdown({ options }: { options: ExportOption[] }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const close = useCallback(() => setOpen(false), [])

  useEffect(() => {
    if (!open) return
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) close()
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open, close])

  return (
    <div ref={ref} className="relative">
      <Button
        variant="outline"
        size="sm"
        onClick={() => setOpen(o => !o)}
        aria-expanded={open}
      >
        <Download className="mr-1.5 h-3.5 w-3.5" />
        Export
      </Button>
      {open && (
        <div className={cn(
          'absolute right-0 top-full z-50 mt-1 min-w-[160px] rounded-md border border-border',
          'bg-background py-1 shadow-lg',
        )}>
          {options.map(opt => (
            <button
              key={opt.label}
              type="button"
              className="w-full px-3 py-1.5 text-left text-sm text-foreground hover:bg-accent transition-colors"
              onClick={() => {
                opt.onClick()
                close()
              }}
            >
              {opt.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
