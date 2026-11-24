import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Coins, DatabaseZap, MessagesSquare, Repeat, Search, Sigma } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { getUsage, getUsageTotals } from '../lib/api/usage'
import { cn } from '../lib/utils'

const HIGH_COST_USD = 5

function formatTokens(n: number) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}m`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`
  return String(n)
}

function formatCost(n: number) {
  return `$${n.toFixed(2)}`
}

function formatDuration(ms: number) {
  const s = Math.round(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m ${s % 60}s`
}

function formatTime(iso: string) {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

function costClass(n: number) {
  return n >= HIGH_COST_USD
    ? 'text-amber-600 dark:text-amber-400'
    : 'text-emerald-600 dark:text-emerald-400'
}

function StatCard({ icon: Icon, title, value, valueClass, children }: { icon: typeof Coins; title: string; value: string | number; valueClass?: string; children?: React.ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4 text-muted-foreground" />
          <CardTitle>{title}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className={cn('text-3xl font-bold tabular-nums text-foreground', valueClass)}>{value}</div>
        {children && <div className="mt-2 text-sm text-muted-foreground">{children}</div>}
      </CardContent>
    </Card>
  )
}

export default function Usage() {
  const [search, setSearch] = useState('')
  const totals = useQuery({ queryKey: ['usage', 'totals'], queryFn: getUsageTotals, refetchInterval: 15_000 })
  const list = useQuery({ queryKey: ['usage', 'list'], queryFn: () => getUsage(), refetchInterval: 15_000 })

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    const rows = list.data ?? []
    if (!q) return rows
    return rows.filter(r => (r.conversation_title || r.conversation_id).toLowerCase().includes(q))
  }, [list.data, search])

  const t = totals.data
  const cacheDenominator = t ? t.total_cache_read_tokens + t.total_input_tokens : 0
  const cacheRate = t && cacheDenominator > 0 ? (t.total_cache_read_tokens / cacheDenominator) * 100 : 0
  const models = useMemo(
    () => Object.entries(t?.by_model ?? {}).sort((a, b) => b[1].cost_usd - a[1].cost_usd),
    [t],
  )
  const maxModelTokens = Math.max(1, ...models.map(([, m]) => m.input_tokens + m.output_tokens))

  return (
    <div className="space-y-4 p-3 md:p-6">
      <h1 className="text-lg md:text-2xl font-semibold text-foreground">Token Usage</h1>

      <div className="grid gap-3 grid-cols-2 lg:grid-cols-5">
        <StatCard icon={Coins} title="Total cost" value={formatCost(t?.total_cost_usd ?? 0)} valueClass={costClass(t?.total_cost_usd ?? 0)} />
        <StatCard icon={Sigma} title="Total tokens" value={formatTokens((t?.total_input_tokens ?? 0) + (t?.total_output_tokens ?? 0))}>
          <span className="text-xs tabular-nums">
            {formatTokens(t?.total_input_tokens ?? 0)} in / {formatTokens(t?.total_output_tokens ?? 0)} out
          </span>
        </StatCard>
        <StatCard icon={MessagesSquare} title="Conversations" value={t?.conversation_count ?? 0} />
        <StatCard icon={Repeat} title="Turns" value={(t?.total_turns ?? 0).toLocaleString()} />
        <StatCard icon={DatabaseZap} title="Cache hit rate" value={`${cacheRate.toFixed(1)}%`}>
          <span className="text-xs tabular-nums">{formatTokens(t?.total_cache_read_tokens ?? 0)} cached reads</span>
        </StatCard>
      </div>

      {models.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>By model</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {models.map(([name, m]) => {
              const tokens = m.input_tokens + m.output_tokens
              return (
                <div key={name} className="grid grid-cols-[6rem_1fr_auto] items-center gap-3 text-xs tabular-nums">
                  <span className="truncate font-mono">{name}</span>
                  <div className="h-2 rounded-full bg-muted overflow-hidden">
                    <div className="h-full rounded-full bg-primary" style={{ width: `${(tokens / maxModelTokens) * 100}%` }} />
                  </div>
                  <span className="text-muted-foreground whitespace-nowrap">
                    {formatTokens(m.input_tokens)} in / {formatTokens(m.output_tokens)} out ·{' '}
                    <span className={costClass(m.cost_usd)}>{formatCost(m.cost_usd)}</span>
                  </span>
                </div>
              )
            })}
          </CardContent>
        </Card>
      )}

      <div className="relative w-full sm:max-w-sm">
        <Search className="absolute left-2.5 top-2 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder="Search conversations..."
          value={search}
          onChange={e => setSearch(e.target.value)}
          className="pl-9"
        />
      </div>

      {list.isError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load usage: {(list.error as Error).message}
        </div>
      )}

      {list.data && (
        <div className="rounded-lg border border-border bg-background overflow-hidden">
          {filtered.length === 0 ? (
            <div className="flex h-48 flex-col items-center justify-center gap-2 text-muted-foreground">
              <Coins className="h-10 w-10" />
              <p className="text-sm">{search ? 'No conversations match your search' : 'No usage recorded yet'}</p>
            </div>
          ) : (
            <Table className="text-xs tabular-nums">
              <TableHeader>
                <TableRow>
                  <TableHead>Conversation</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead className="text-right">Input</TableHead>
                  <TableHead className="text-right">Output</TableHead>
                  <TableHead className="text-right">Cache read</TableHead>
                  <TableHead className="text-right">Cost</TableHead>
                  <TableHead className="text-right">Duration</TableHead>
                  <TableHead className="text-right">Turns</TableHead>
                  <TableHead>Last active</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map(r => (
                  <TableRow key={r.conversation_id}>
                    <TableCell className="max-w-64 truncate">
                      <Link to={`/chat?c=${encodeURIComponent(r.conversation_id)}`} className="hover:underline underline-offset-2">
                        {r.conversation_title || r.conversation_id}
                      </Link>
                    </TableCell>
                    <TableCell>{r.model && <Badge variant="outline">{r.model}</Badge>}</TableCell>
                    <TableCell className="text-right">{formatTokens(r.total_input_tokens)}</TableCell>
                    <TableCell className="text-right">{formatTokens(r.total_output_tokens)}</TableCell>
                    <TableCell className="text-right">{formatTokens(r.total_cache_read_tokens)}</TableCell>
                    <TableCell className={cn('text-right', costClass(r.total_cost_usd))}>{formatCost(r.total_cost_usd)}</TableCell>
                    <TableCell className="text-right whitespace-nowrap">{formatDuration(r.total_duration_ms)}</TableCell>
                    <TableCell className="text-right">{r.total_turns}</TableCell>
                    <TableCell className="whitespace-nowrap text-muted-foreground">{formatTime(r.last_time)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      )}
    </div>
  )
}
