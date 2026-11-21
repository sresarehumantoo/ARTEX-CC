import { createContext, useContext } from 'react'

export type ToastVariant = 'info' | 'success' | 'warning' | 'error'

export interface ToastOptions {
  variant?: ToastVariant
  duration?: number
}

export interface ToastItem {
  id: number
  message: string
  variant: ToastVariant
  duration: number
  removing?: boolean
}

export interface ToastContextValue {
  toast: (message: string, options?: ToastOptions) => void
}

export const ToastContext = createContext<ToastContextValue | null>(null)

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
