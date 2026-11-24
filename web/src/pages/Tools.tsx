import { Fragment, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Search } from 'lucide-react'
import { getTools } from '../lib/api/tools'
import { Badge } from '../components/ui/badge'
import { Input } from '../components/ui/input'
import { Skeleton } from '../components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import Skills from './Skills'

function Catalog() {
  const { data, isLoading, error } = useQuery({ queryKey: ['tools'], queryFn: getTools })
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState('all')
  const [expanded, setExpanded] = useState<string | null>(null)

  const categories = useMemo(() => {
    const counts = new Map<string, number>()
    for (const t of data ?? []) counts.set(t.category, (counts.get(t.category) ?? 0) + 1)
    return [...counts.entries()].sort((a, b) => a[0].localeCompare(b[0]))
  }, [data])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return (data ?? []).filter(
      t =>
        (category === 'all' || t.category === category) &&
        (!q || t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q)),
    )
  }, [data, search, category])

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search tools by name or description"
            className="pl-8"
          />
        </div>
        <span className="text-xs tabular-nums text-muted-foreground">
          {filtered.length} of {data?.length ?? 0} tools
        </span>
      </div>

      <div className="flex flex-wrap gap-1.5">
        {[['all', data?.length ?? 0] as const, ...categories].map(([c, n]) => (
          <button
            key={c}
            type="button"
            onClick={() => setCategory(c)}
            className={`rounded-md px-2 py-1 text-xs tabular-nums transition-colors ${
              category === c
                ? 'bg-foreground/10 text-foreground'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground'
            }`}
          >
            {c} <span className="opacity-60">{n}</span>
          </button>
        ))}
      </div>

      {error && <p className="text-sm text-destructive">Failed to load tools: {error.message}</p>}

      {isLoading ? (
        <Skeleton className="h-64 rounded-xl" />
      ) : (
        <div className="rounded-xl bg-card ring-1 ring-foreground/10 overflow-hidden">
          <Table className="text-xs tabular-nums">
            <TableHeader>
              <TableRow>
                <TableHead className="w-6" />
                <TableHead>Name</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Category</TableHead>
                <TableHead className="w-24">Access</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map(t => {
                const open = expanded === t.name
                return (
                  <Fragment key={t.name}>
                    <TableRow className="cursor-pointer" onClick={() => setExpanded(open ? null : t.name)}>
                      <TableCell className="text-muted-foreground">
                        {open ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                      </TableCell>
                      <TableCell className="font-mono whitespace-nowrap">{t.name}</TableCell>
                      <TableCell className="max-w-xl truncate text-muted-foreground">{t.description}</TableCell>
                      <TableCell>
                        <Badge variant="outline">{t.category}</Badge>
                      </TableCell>
                      <TableCell>
                        {t.read_only ? <Badge variant="get">read-only</Badge> : <Badge variant="medium">write</Badge>}
                      </TableCell>
                    </TableRow>
                    {open && (
                      <TableRow className="hover:bg-transparent">
                        <TableCell />
                        <TableCell colSpan={4} className="whitespace-normal">
                          <p className="mb-2 text-muted-foreground">{t.description}</p>
                          <pre className="max-h-96 overflow-auto rounded-lg bg-muted/50 p-3 font-mono text-[11px] leading-relaxed">
                            {JSON.stringify(t.schema ?? {}, null, 2)}
                          </pre>
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                )
              })}
              {filtered.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className="py-6 text-center text-muted-foreground italic">
                    No tools match.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

export default function Tools() {
  return (
    <div className="p-4 md:p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-foreground">Tools</h1>
        <p className="text-sm text-muted-foreground">MCP tool catalog and skill files.</p>
      </div>
      <Tabs defaultValue="catalog">
        <TabsList>
          <TabsTrigger value="catalog">Catalog</TabsTrigger>
          <TabsTrigger value="skills">Skills</TabsTrigger>
        </TabsList>
        <TabsContent value="catalog" className="pt-4">
          <Catalog />
        </TabsContent>
        <TabsContent value="skills" className="pt-4">
          <Skills />
        </TabsContent>
      </Tabs>
    </div>
  )
}
