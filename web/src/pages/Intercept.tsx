import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import {
  createInterceptRule,
  decideApproval,
  deleteInterceptRule,
  getApprovals,
  getInterceptRules,
  updateInterceptRule,
  type Approval,
  type InterceptRule,
  type RuleAction,
  type RuleScope,
} from '../lib/api/intercept'
import { toneClasses, type Tone } from '../lib/status'
import { timeAgo, truncate } from '../lib/format'
import { cn } from '../lib/utils'
import { StatusBadge } from '../components/StatusBadge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { SelectRoot, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../components/ui/select'
import { Switch } from '../components/ui/switch'
import { Textarea } from '../components/ui/textarea'
import { Skeleton } from '../components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
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

const actionTone: Record<RuleAction, Tone> = { allow: 'green', ask: 'amber', deny: 'red' }

function ActionBadge({ action }: { action: RuleAction }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        toneClasses[actionTone[action]],
      )}
    >
      {action}
    </span>
  )
}

function inputPreview(input: unknown) {
  const text = typeof input === 'string' ? input : JSON.stringify(input ?? {})
  return truncate(text, 100)
}

function RuleDialog({
  open,
  rule,
  onOpenChange,
}: {
  open: boolean
  rule: InterceptRule | null
  onOpenChange: (open: boolean) => void
}) {
  const queryClient = useQueryClient()
  const [pattern, setPattern] = useState('')
  const [scope, setScope] = useState<RuleScope>('tool_name')
  const [action, setAction] = useState<RuleAction>('ask')
  const [description, setDescription] = useState('')

  useEffect(() => {
    if (!open) return
    setPattern(rule?.pattern ?? '')
    setScope(rule?.scope ?? 'tool_name')
    setAction(rule?.action ?? 'ask')
    setDescription(rule?.description ?? '')
  }, [open, rule])

  const save = useMutation({
    mutationFn: () => {
      const input = { pattern: pattern.trim(), scope, action, description }
      return rule ? updateInterceptRule(rule.id, input) : createInterceptRule(input)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['intercept-rules'] })
      toast.success(rule ? 'Rule saved' : 'Rule created')
      onOpenChange(false)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{rule ? 'Edit rule' : 'New rule'}</DialogTitle>
          <DialogDescription>
            Rules are evaluated top to bottom. The first matching rule decides the outcome.
          </DialogDescription>
        </DialogHeader>
        <form
          id="rule-form"
          className="space-y-4"
          onSubmit={e => {
            e.preventDefault()
            save.mutate()
          }}
        >
          <div className="space-y-1.5">
            <Label htmlFor="rule-pattern">Pattern</Label>
            <Input
              id="rule-pattern"
              value={pattern}
              onChange={e => setPattern(e.target.value)}
              className="font-mono"
              placeholder="Bash or mcp__artex__*"
              required
            />
            <p className="text-xs text-muted-foreground">Exact name or glob.</p>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="rule-scope">Scope</Label>
              <SelectRoot value={scope} onValueChange={v => setScope(v as RuleScope)}>
                <SelectTrigger id="rule-scope">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="tool_name">tool_name</SelectItem>
                  <SelectItem value="input">input</SelectItem>
                </SelectContent>
              </SelectRoot>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="rule-action">Action</Label>
              <SelectRoot value={action} onValueChange={v => setAction(v as RuleAction)}>
                <SelectTrigger id="rule-action">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="allow">allow</SelectItem>
                  <SelectItem value="ask">ask</SelectItem>
                  <SelectItem value="deny">deny</SelectItem>
                </SelectContent>
              </SelectRoot>
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="rule-description">Description</Label>
            <Textarea
              id="rule-description"
              value={description}
              onChange={e => setDescription(e.target.value)}
              rows={2}
            />
          </div>
        </form>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" form="rule-form" disabled={save.isPending || !pattern.trim()}>
            {save.isPending ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function RulesTab() {
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useQuery({ queryKey: ['intercept-rules'], queryFn: getInterceptRules })
  const [editing, setEditing] = useState<InterceptRule | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleting, setDeleting] = useState<InterceptRule | null>(null)

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['intercept-rules'] })
  const onError = (err: Error) => toast.error(err.message)

  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => updateInterceptRule(id, { enabled }),
    onSuccess: refresh,
    onError,
  })
  const remove = useMutation({
    mutationFn: (id: string) => deleteInterceptRule(id),
    onSuccess: () => {
      setDeleting(null)
      refresh()
      toast.success('Rule deleted')
    },
    onError,
  })

  const openRule = (rule: InterceptRule | null) => {
    setEditing(rule)
    setDialogOpen(true)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <p className="text-sm text-muted-foreground">
          Rules are evaluated top to bottom; the first enabled match wins.
        </p>
        <Button onClick={() => openRule(null)}>
          <Plus /> New Rule
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">Failed to load rules: {error.message}</p>}

      {isLoading ? (
        <Skeleton className="h-40 rounded-xl" />
      ) : data && data.length === 0 ? (
        <p className="text-sm text-muted-foreground italic">No rules yet.</p>
      ) : (
        <div className="rounded-xl border overflow-x-auto">
          <Table className="text-xs">
            <TableHeader>
              <TableRow>
                <TableHead className="w-10">#</TableHead>
                <TableHead>Pattern</TableHead>
                <TableHead>Scope</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Description</TableHead>
                <TableHead className="w-20">Enabled</TableHead>
                <TableHead className="w-20" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {data?.map((rule, i) => (
                <TableRow key={rule.id} className={cn(!rule.enabled && 'opacity-60')}>
                  <TableCell className="text-muted-foreground tabular-nums">{i + 1}</TableCell>
                  <TableCell className="font-mono">{rule.pattern}</TableCell>
                  <TableCell className="text-muted-foreground">{rule.scope}</TableCell>
                  <TableCell>
                    <ActionBadge action={rule.action} />
                  </TableCell>
                  <TableCell className="text-muted-foreground max-w-xs truncate">{rule.description}</TableCell>
                  <TableCell>
                    <Switch
                      size="sm"
                      checked={rule.enabled}
                      aria-label={`Toggle ${rule.pattern}`}
                      onCheckedChange={enabled => toggle.mutate({ id: rule.id, enabled })}
                    />
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="icon-xs" aria-label="Edit rule" onClick={() => openRule(rule)}>
                        <Pencil />
                      </Button>
                      <Button variant="ghost" size="icon-xs" aria-label="Delete rule" onClick={() => setDeleting(rule)}>
                        <Trash2 />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <RuleDialog open={dialogOpen} rule={editing} onOpenChange={setDialogOpen} />

      <AlertDialog open={!!deleting} onOpenChange={open => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete rule {deleting?.pattern}?</AlertDialogTitle>
            <AlertDialogDescription>This removes the rule permanently.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => deleting && remove.mutate(deleting.id)}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

type ApprovalFilter = 'all' | 'pending' | 'decided'

function ApprovalRow({
  approval,
  onDecide,
  busy,
}: {
  approval: Approval
  onDecide?: (id: string, action: 'approve' | 'deny') => void
  busy: boolean
}) {
  const pending = approval.status === 'pending'
  return (
    <TableRow className={cn(!pending && 'opacity-70')}>
      <TableCell className="font-mono">{approval.tool_name}</TableCell>
      <TableCell>
        {approval.conversation_id ? (
          <Link
            to={`/chat?c=${encodeURIComponent(approval.conversation_id)}`}
            className="font-mono text-primary hover:underline"
          >
            {truncate(approval.conversation_id, 12)}
          </Link>
        ) : (
          <span className="text-muted-foreground">-</span>
        )}
      </TableCell>
      <TableCell className="max-w-sm truncate font-mono text-muted-foreground" title={inputPreview(approval.tool_input)}>
        {inputPreview(approval.tool_input)}
      </TableCell>
      <TableCell className="text-muted-foreground whitespace-nowrap">{timeAgo(approval.created_at)}</TableCell>
      <TableCell>
        {pending ? (
          <div className="flex justify-end gap-1.5">
            <Button
              size="xs"
              disabled={busy}
              className="bg-emerald-600 text-white hover:bg-emerald-600/90"
              onClick={() => onDecide?.(approval.id, 'approve')}
            >
              Approve
            </Button>
            <Button size="xs" variant="destructive" disabled={busy} onClick={() => onDecide?.(approval.id, 'deny')}>
              Deny
            </Button>
          </div>
        ) : (
          <div className="flex justify-end">
            <StatusBadge domain="approval" value={approval.status} />
          </div>
        )}
      </TableCell>
    </TableRow>
  )
}

function ApprovalsTab() {
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState<ApprovalFilter>('all')
  const { data, isLoading, error } = useQuery({
    queryKey: ['approvals'],
    queryFn: () => getApprovals(),
    refetchInterval: 5000,
  })

  const decide = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'approve' | 'deny' }) => decideApproval(id, action),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['approvals'] }),
    onError: (err: Error) => {
      toast.error(err.message)
      queryClient.invalidateQueries({ queryKey: ['approvals'] })
    },
  })

  const byNewest = (a: Approval, b: Approval) => b.created_at.localeCompare(a.created_at)
  const pending = (data ?? []).filter(a => a.status === 'pending').sort(byNewest)
  const decided = (data ?? []).filter(a => a.status !== 'pending').sort(byNewest)
  const rows = filter === 'pending' ? pending : filter === 'decided' ? decided : [...pending, ...decided]

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <p className="text-sm text-muted-foreground">
          {pending.length} pending, {decided.length} decided.
        </p>
        <SelectRoot value={filter} onValueChange={v => setFilter(v as ApprovalFilter)}>
          <SelectTrigger aria-label="Filter approvals" className="w-32">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="pending">Pending</SelectItem>
            <SelectItem value="decided">Decided</SelectItem>
          </SelectContent>
        </SelectRoot>
      </div>

      {error && <p className="text-sm text-destructive">Failed to load approvals: {error.message}</p>}

      {isLoading ? (
        <Skeleton className="h-40 rounded-xl" />
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted-foreground italic">No approvals.</p>
      ) : (
        <div className="rounded-xl border overflow-x-auto">
          <Table className="text-xs">
            <TableHeader>
              <TableRow>
                <TableHead>Tool</TableHead>
                <TableHead>Conversation</TableHead>
                <TableHead>Input</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="text-right">Decision</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map(a => (
                <ApprovalRow
                  key={a.id}
                  approval={a}
                  busy={decide.isPending}
                  onDecide={(id, action) => decide.mutate({ id, action })}
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

export default function Intercept() {
  const { data } = useQuery({ queryKey: ['approvals'], queryFn: () => getApprovals(), refetchInterval: 5000 })
  const pendingCount = (data ?? []).filter(a => a.status === 'pending').length

  return (
    <div className="p-4 md:p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-foreground">Intercept</h1>
        <p className="text-sm text-muted-foreground">Control which tool calls chat subprocesses may make.</p>
      </div>

      <Tabs defaultValue="rules">
        <TabsList>
          <TabsTrigger value="rules">Rules</TabsTrigger>
          <TabsTrigger value="approvals">
            Approvals{pendingCount > 0 && <span className="ml-1.5 text-amber-600 dark:text-amber-400">{pendingCount}</span>}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="rules" className="pt-2">
          <RulesTab />
        </TabsContent>
        <TabsContent value="approvals" className="pt-2">
          <ApprovalsTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
