import { WifiOff } from 'lucide-react'
import { useSSEStatus } from '../hooks/useSSEStatus'

export function ConnectionBanner() {
  const { connected } = useSSEStatus()

  if (connected) return null

  return (
    <div role="alert" className="no-print flex items-center justify-center gap-2 bg-amber-500/10 border-b border-amber-500/20 px-4 py-1.5 text-xs text-amber-600 dark:text-amber-400">
      <WifiOff className="h-3.5 w-3.5" />
      <span>Connection lost. Reconnecting...</span>
      <span className="ml-1 inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-amber-500" />
    </div>
  )
}
