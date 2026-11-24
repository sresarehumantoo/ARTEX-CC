import { Fragment, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ChevronRight, Search, Terminal, TriangleAlert, X } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Select } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { ResultBlock, keyArg } from '../components/chat/tools'
import { getCommands, getCommandStats, type CommandEntry } from '../lib/api/commands'
import { getConversations } from '../lib/api/chat'
import { cn } from '../lib/utils'

const PAGE_SIZE = 200

function shortTool(name: string) {
  return name.replace(/^mcp__artex__/, '')
}

function formatTime(iso: string) {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

function StatCard({ icon: Icon, title, value, children }: { icon: typeof Terminal; title: string; value: string | number; children?: React.ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4 text-muted-foreground" />
          <CardTitle>{title}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold tabular-nums text-foreground">{value}</div>
        {children && <div className="mt-2 text-sm text-muted-foreground">{children}</div>}
      </CardContent>
    </Card>
  )
}

function CommandRow({ entry }: { entry: CommandEntry }) {
  const [open, setOpen] = useState(false)
  const arg = keyArg(entry.input ?? {})
  return (
    <Fragment>
      <TableRow
        onClick={() => setOpen(v => !v)}
        className={cn('cursor-pointer', entry.is_error && 'bg-destructive/5 hover:bg-destructive/10')}
      >
        <TableCell className="w-6">
          <ChevronRight className={cn('size-3 text-muted-foreground transition-transform', open && 'rotate-90')} />
        </TableCell>
        <TableCell className="font-mono text-sky-600 dark:text-sky-400 whitespace-nowrap">{shortTool(entry.tool_name)}</TableCell>
        <TableCell className="max-w-0 w-full">
          <div className="truncate font-mono text-muted-foreground">{arg}</div>
        </TableCell>
        <TableCell className="whitespace-nowrap">
          <Link
            to={`/chat?c=${encodeURIComponent(entry.conversation_id)}`}
            onClick={e => e.stopPropagation()}
            className="hover:underline underline-offset-2"
          >
            {entry.conversation_title || entry.conversation_id}
          </Link>
        </TableCell>
        <TableCell>{entry.is_error && <Badge variant="critical">error</Badge>}</TableCell>
        <TableCell className="whitespace-nowrap text-muted-foreground">{formatTime(entry.time)}</TableCell>
      </TableRow>
      {open && (
        <TableRow className="hover:bg-transparent">
          <TableCell />
          <TableCell colSpan={5} className="whitespace-normal">
            <div className="space-y-2 py-1">
              <pre className="max-h-64 overflow-auto rounded-md border border-border bg-background/60 p-2 font-mono text-xs whitespace-pre-wrap break-all">
                {JSON.stringify(entry.input, null, 2)}
              </pre>
              {entry.output_preview && (
                <ResultBlock content={entry.output_preview} isError={entry.is_error} preview={entry.output_preview} />
              )}
            </div>
          </TableCell>
        </TableRow>
      )}
    </Fragment>
  )
}

export default function Commands() {
  const [limit, setLimit] = useState(PAGE_SIZE)
  const [tool, setTool] = useState('')
  const [conv, setConv] = useState('')
  const [errorsOnly, setErrorsOnly] = useState(false)
  const [search, setSearch] = useState('')

  const stats = useQuery({ queryKey: ['commands', 'stats'], queryFn: getCommandStats, refetchInterval: 15_000 })
  const conversations = useQuery({ queryKey: ['conversations'], queryFn: getConversations })
  const list = useQuery({
    queryKey: ['commands', limit, tool, conv, errorsOnly],
    queryFn: () => getCommands({ limit, tool, conv, error: errorsOnly }),
    refetchInterval: 15_000,
  })

  const titles = useMemo(() => new Map((conversations.data ?? []).map(c => [c.id, c.title])), [conversations.data])
  const toolNames = useMemo(() => Object.keys(stats.data?.by_tool ?? {}).sort(), [stats.data])
  const convIds = useMemo(() => Object.keys(stats.data?.by_conversation ?? {}), [stats.data])
  const topTools = useMemo(
    () => Object.entries(stats.data?.by_tool ?? {}).sort((a, b) => b[1] - a[1]).slice(0, 3),
    [stats.data],
  )

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return list.data ?? []
    return (list.data ?? []).filter(
      e =>
        e.tool_name.toLowerCase().includes(q) ||
        e.conversation_title.toLowerCase().includes(q) ||
        keyArg(e.input ?? {}).toLowerCase().includes(q) ||
        e.output_preview.toLowerCase().includes(q),
    )
  }, [list.data, search])

  const hasFilters = tool || conv || errorsOnly || search

  return (
    <div className="space-y-4 p-3 md:p-6">
      <h1 className="text-lg md:text-2xl font-semibold text-foreground">Commands</h1>

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard icon={Terminal} title="Total commands" value={stats.data?.total ?? 0} />
        <StatCard icon={TriangleAlert} title="Errors" value={stats.data?.error_count ?? 0} />
        <StatCard icon={Terminal} title="Top tools" value={topTools[0] ? topTools[0][1] : 0}>
          {topTools.length === 0 ? (
            'No commands yet'
          ) : (
            <ul className="space-y-0.5 text-xs">
              {topTools.map(([name, count]) => (
                <li key={name} className="flex justify-between gap-2">
                  <span className="truncate font-mono">{shortTool(name)}</span>
                  <span className="tabular-nums">{count}</span>
                </li>
              ))}
            </ul>
          )}
        </StatCard>
      </div>

      <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-2">
        <div className="relative w-full sm:w-auto sm:flex-1 sm:min-w-[200px] sm:max-w-sm">
          <Search className="absolute left-2.5 top-2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search commands, output..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select value={tool} onChange={e => setTool(e.target.value)} aria-label="Tool">
          <option value="">All tools</option>
          {toolNames.map(t => (
            <option key={t} value={t}>{shortTool(t)}</option>
          ))}
        </Select>
        <Select value={conv} onChange={e => setConv(e.target.value)} aria-label="Conversation">
          <option value="">All conversations</option>
          {convIds.map(id => (
            <option key={id} value={id}>{titles.get(id) || id}</option>
          ))}
        </Select>
        <Button variant={errorsOnly ? 'default' : 'outline'} size="sm" onClick={() => setErrorsOnly(v => !v)}>
          Errors only
        </Button>
        {hasFilters && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setTool('')
              setConv('')
              setErrorsOnly(false)
              setSearch('')
            }}
          >
            <X className="h-4 w-4 mr-1" />
            Clear
          </Button>
        )}
      </div>

      {list.isError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load commands: {(list.error as Error).message}
        </div>
      )}

      {list.data && (
        <>
          <div className="text-xs text-muted-foreground tabular-nums">
            Showing {filtered.length} of {stats.data?.total ?? list.data.length} commands
          </div>
          <div className="rounded-lg border border-border bg-background overflow-hidden">
            {filtered.length === 0 ? (
              <div className="flex h-48 flex-col items-center justify-center gap-2 text-muted-foreground">
                <Terminal className="h-10 w-10" />
                <p className="text-sm">{hasFilters ? 'No commands match your filters' : 'No commands recorded yet'}</p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-6" />
                    <TableHead>Tool</TableHead>
                    <TableHead>Argument</TableHead>
                    <TableHead>Conversation</TableHead>
                    <TableHead />
                    <TableHead>Time</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filtered.map(e => (
                    <CommandRow key={`${e.conversation_id}:${e.seq}`} entry={e} />
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
          {list.data.length >= limit && (
            <div className="text-center">
              <Button variant="outline" size="sm" onClick={() => setLimit(l => l + PAGE_SIZE)}>
                Load more
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
