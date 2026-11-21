export function timeAgo(ts: number | string): string {
  const date = typeof ts === 'number' ? new Date(ts * 1000) : new Date(ts)
  const now = Date.now()
  const diff = now - date.getTime()

  if (diff < 60000) return 'just now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
  return `${Math.floor(diff / 86400000)}d ago`
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(i > 0 ? 1 : 0)} ${sizes[i]}`
}

export function truncate(s: string, max: number): string {
  if (s.length <= max) return s
  return s.slice(0, max) + '...'
}

export const severityOrder: Record<string, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
  info: 4,
}

export const severityColor: Record<string, string> = {
  critical: 'rose',
  high: 'orange',
  medium: 'amber',
  low: 'blue',
  info: 'slate',
}

export const methodColor: Record<string, string> = {
  GET: 'emerald',
  POST: 'blue',
  PUT: 'amber',
  DELETE: 'rose',
  PATCH: 'violet',
  OPTIONS: 'slate',
  HEAD: 'slate',
}

export function statusColor(status: number): string {
  if (status < 300) return 'emerald'
  if (status < 400) return 'blue'
  if (status < 500) return 'amber'
  return 'rose'
}
