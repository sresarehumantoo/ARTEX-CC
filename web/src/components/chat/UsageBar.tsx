import type { ChatUsage } from '../../lib/chat-types'

function tokens(n: number) {
  return n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n)
}

function duration(ms: number) {
  if (ms < 1000) return `${Math.round(ms)}ms`
  const s = Math.round(ms / 1000)
  return s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${s % 60}s`
}

export function UsageBar({ usage }: { usage: ChatUsage | null }) {
  if (!usage) return null
  const cached = (usage.cache_read_tokens ?? 0) + (usage.cache_creation_tokens ?? 0)
  const turns = usage.num_turns ?? 0
  const io = [
    usage.input_tokens > 0 && `${tokens(usage.input_tokens)} in`,
    usage.output_tokens > 0 && `${tokens(usage.output_tokens)} out`,
  ].filter(Boolean)
  const parts = [
    io.join(' / '),
    cached > 0 && `${tokens(cached)} cached`,
    usage.cost_usd ? `$${usage.cost_usd.toFixed(2)}` : '',
    usage.duration_ms ? duration(usage.duration_ms) : '',
    turns > 0 && `${turns} ${turns === 1 ? 'turn' : 'turns'}`,
  ].filter(Boolean)
  if (parts.length === 0) return null
  return <span className="hidden truncate text-xs text-muted-foreground sm:inline">{parts.join(' · ')}</span>
}
