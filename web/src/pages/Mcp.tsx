import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plug, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { addMcpServer, deleteMcpServer, getMcpServers, type McpServer } from '../lib/api/tools'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { SelectRoot, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../components/ui/select'
import { Skeleton } from '../components/ui/skeleton'
import { Textarea } from '../components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../components/ui/alert-dialog'

function parseEnv(text: string) {
  const env: Record<string, string> = {}
  for (const line of text.split('\n')) {
    const i = line.indexOf('=')
    if (i <= 0) continue
    env[line.slice(0, i).trim()] = line.slice(i + 1).trim()
  }
  return env
}

function AddServerDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [type, setType] = useState<'stdio' | 'http'>('stdio')
  const [command, setCommand] = useState('')
  const [args, setArgs] = useState('')
  const [url, setUrl] = useState('')
  const [env, setEnv] = useState('')

  const add = useMutation({
    mutationFn: () =>
      addMcpServer({
        name: name.trim(),
        type,
        ...(type === 'stdio'
          ? { command: command.trim(), args: args.split(/\s+/).filter(Boolean) }
          : { url: url.trim() }),
        env: parseEnv(env),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mcp-servers'] })
      toast.success('Server added')
      setName('')
      setCommand('')
      setArgs('')
      setUrl('')
      setEnv('')
      onOpenChange(false)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Add MCP server</DialogTitle>
          <DialogDescription>An extra server made available alongside the built-in one.</DialogDescription>
        </DialogHeader>
        <form
          id="mcp-form"
          className="space-y-4"
          onSubmit={e => {
            e.preventDefault()
            add.mutate()
          }}
        >
          <div className="space-y-1.5">
            <Label htmlFor="mcp-name">Name</Label>
            <Input id="mcp-name" value={name} onChange={e => setName(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="mcp-type">Type</Label>
            <SelectRoot value={type} onValueChange={v => setType(v as 'stdio' | 'http')}>
              <SelectTrigger id="mcp-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="stdio">stdio</SelectItem>
                <SelectItem value="http">http</SelectItem>
              </SelectContent>
            </SelectRoot>
          </div>
          {type === 'stdio' ? (
            <>
              <div className="space-y-1.5">
                <Label htmlFor="mcp-command">Command</Label>
                <Input id="mcp-command" value={command} onChange={e => setCommand(e.target.value)} className="font-mono" required />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="mcp-args">Arguments</Label>
                <Input id="mcp-args" value={args} onChange={e => setArgs(e.target.value)} className="font-mono" placeholder="space separated" />
              </div>
            </>
          ) : (
            <div className="space-y-1.5">
              <Label htmlFor="mcp-url">URL</Label>
              <Input id="mcp-url" type="url" value={url} onChange={e => setUrl(e.target.value)} className="font-mono" required />
            </div>
          )}
          <div className="space-y-1.5">
            <Label htmlFor="mcp-env">Environment variables</Label>
            <Textarea
              id="mcp-env"
              value={env}
              onChange={e => setEnv(e.target.value)}
              rows={3}
              className="font-mono text-xs"
              placeholder="KEY=value, one per line"
            />
          </div>
        </form>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" form="mcp-form" disabled={add.isPending}>
            Add
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function Mcp() {
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useQuery({ queryKey: ['mcp-servers'], queryFn: getMcpServers })
  const [adding, setAdding] = useState(false)
  const [pendingDelete, setPendingDelete] = useState<McpServer | null>(null)

  const remove = useMutation({
    mutationFn: (name: string) => deleteMcpServer(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mcp-servers'] })
      toast.success('Server removed')
      setPendingDelete(null)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  return (
    <div className="p-4 md:p-6 space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-foreground">MCP Servers</h1>
          <p className="text-sm text-muted-foreground">Servers available to chat conversations.</p>
        </div>
        <Button onClick={() => setAdding(true)}>
          <Plus /> Add Server
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">Failed to load servers: {error.message}</p>}

      {isLoading ? (
        <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {[0, 1].map(i => <Skeleton key={i} className="h-32 rounded-xl" />)}
        </div>
      ) : (
        <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {data?.map(s => (
            <Card key={s.name}>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Plug className="size-4 text-muted-foreground" />
                  <span className="truncate">{s.name}</span>
                  {s.built_in ? (
                    <Badge variant="secondary" className="ml-auto">Built-in</Badge>
                  ) : (
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="ml-auto"
                      aria-label={`Delete ${s.name}`}
                      onClick={() => setPendingDelete(s)}
                    >
                      <Trash2 />
                    </Button>
                  )}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 text-xs">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="outline">{s.type}</Badge>
                  <span className="flex items-center gap-1.5 text-muted-foreground">
                    <span className={`size-1.5 rounded-full ${s.built_in ? 'bg-emerald-500' : 'bg-muted-foreground/50'}`} />
                    {s.built_in ? 'Running' : 'Configured'}
                  </span>
                </div>
                <p className="truncate font-mono text-muted-foreground" title={s.type === 'stdio' ? [s.command, ...(s.args ?? [])].join(' ') : s.url}>
                  {s.type === 'stdio' ? [s.command, ...(s.args ?? [])].join(' ') : s.url}
                </p>
                {s.env_keys.length > 0 && (
                  <p className="tabular-nums text-muted-foreground">
                    {s.env_keys.length} env {s.env_keys.length === 1 ? 'var' : 'vars'}: {s.env_keys.join(', ')}
                  </p>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <AddServerDialog open={adding} onOpenChange={setAdding} />

      <AlertDialog open={pendingDelete !== null} onOpenChange={o => !o && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {pendingDelete?.name}?</AlertDialogTitle>
            <AlertDialogDescription>This removes the server configuration.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => pendingDelete && remove.mutate(pendingDelete.name)}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
