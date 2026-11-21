import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '../../lib/utils'
import { ToastContext, type ToastItem, type ToastOptions, type ToastVariant } from '../../hooks/useToast'

const MAX_VISIBLE = 5
const DEFAULT_DURATION = 4000

const variantStyles: Record<ToastVariant, string> = {
  info: 'border-blue-300 bg-blue-50 text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-400',
  success: 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-400',
  warning: 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400',
  error: 'border-rose-300 bg-rose-50 text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-400',
}

function ToastEntry({ item, onDismiss }: { item: ToastItem; onDismiss: (id: number) => void }) {
  useEffect(() => {
    const timer = setTimeout(() => onDismiss(item.id), item.duration)
    return () => clearTimeout(timer)
  }, [item.id, item.duration, onDismiss])

  return (
    <div
      className={cn(
        'pointer-events-auto rounded-md border px-3 py-2 text-xs shadow-lg backdrop-blur-sm',
        'animate-[toast-in_200ms_ease-out]',
        item.removing && 'animate-[toast-out_150ms_ease-in_forwards]',
        variantStyles[item.variant],
      )}
    >
      {item.message}
    </div>
  )
}

function ToastContainer({ toasts, onDismiss }: { toasts: ToastItem[]; onDismiss: (id: number) => void }) {
  return createPortal(
    <div className="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-2 pointer-events-none">
      {toasts.map((t) => (
        <ToastEntry key={t.id} item={t} onDismiss={onDismiss} />
      ))}
    </div>,
    document.body,
  )
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const nextId = useRef(0)

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => {
      const idx = prev.findIndex((t) => t.id === id)
      if (idx === -1) return prev
      if (!prev[idx].removing) {
        const next = [...prev]
        next[idx] = { ...next[idx], removing: true }
        setTimeout(() => setToasts((p) => p.filter((t) => t.id !== id)), 150)
        return next
      }
      return prev
    })
  }, [])

  const toast = useCallback((message: string, options?: ToastOptions) => {
    const id = nextId.current++
    const item: ToastItem = {
      id,
      message,
      variant: options?.variant ?? 'info',
      duration: options?.duration ?? DEFAULT_DURATION,
    }
    setToasts((prev) => {
      const next = [item, ...prev]
      if (next.length > MAX_VISIBLE) {
        return next.slice(0, MAX_VISIBLE)
      }
      return next
    })
  }, [])

  return (
    <ToastContext value={{ toast }}>
      {children}
      <ToastContainer toasts={toasts} onDismiss={dismiss} />
    </ToastContext>
  )
}
