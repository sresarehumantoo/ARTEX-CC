import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { createAgent, deleteAgent, updateAgent, type Agent } from '../lib/api/agents'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { SelectRoot, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../components/ui/select'
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

interface AgentDetailProps {
  open: boolean
  agent: Agent | null
  onOpenChange: (open: boolean) => void
}

function parseTools(value: string) {
  return value.split(',').map(t => t.trim()).filter(Boolean)
}

export default function AgentDetail({ open, agent, onOpenChange }: AgentDetailProps) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [model, setModel] = useState('sonnet')
  const [tools, setTools] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(agent?.name ?? '')
    setDescription(agent?.description ?? '')
    setSystemPrompt(agent?.system_prompt ?? '')
    setModel(agent?.model || 'sonnet')
    setTools((agent?.allowed_tools ?? []).join(', '))
  }, [open, agent])

  const onSuccess = (message: string) => {
    queryClient.invalidateQueries({ queryKey: ['agents'] })
    toast.success(message)
    onOpenChange(false)
  }
  const onError = (err: Error) => toast.error(err.message)

  const save = useMutation({
    mutationFn: () => {
      const input = {
        name: name.trim(),
        description,
        system_prompt: systemPrompt,
        model,
        allowed_tools: parseTools(tools),
      }
      return agent ? updateAgent(agent.id, input) : createAgent(input)
    },
    onSuccess: () => onSuccess(agent ? 'Agent saved' : 'Agent created'),
    onError,
  })

  const remove = useMutation({
    mutationFn: () => deleteAgent(agent!.id),
    onSuccess: () => {
      setConfirmDelete(false)
      onSuccess('Agent deleted')
    },
    onError,
  })

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{agent ? 'Edit agent' : 'New agent'}</DialogTitle>
            <DialogDescription>
              A preset for chat conversations: system prompt, model, and allowed tools.
            </DialogDescription>
          </DialogHeader>

          <form
            id="agent-form"
            className="space-y-4"
            onSubmit={e => {
              e.preventDefault()
              save.mutate()
            }}
          >
            <div className="space-y-1.5">
              <Label htmlFor="agent-name">Name</Label>
              <Input id="agent-name" value={name} onChange={e => setName(e.target.value)} required />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="agent-description">Description</Label>
              <Textarea
                id="agent-description"
                value={description}
                onChange={e => setDescription(e.target.value)}
                rows={2}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="agent-prompt">System prompt</Label>
              <Textarea
                id="agent-prompt"
                value={systemPrompt}
                onChange={e => setSystemPrompt(e.target.value)}
                className="font-mono min-h-[200px]"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="agent-model">Model</Label>
              <SelectRoot value={model} onValueChange={setModel}>
                <SelectTrigger id="agent-model">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="opus">opus</SelectItem>
                  <SelectItem value="sonnet">sonnet</SelectItem>
                  <SelectItem value="haiku">haiku</SelectItem>
                </SelectContent>
              </SelectRoot>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="agent-tools">Allowed tools</Label>
              <Input
                id="agent-tools"
                value={tools}
                onChange={e => setTools(e.target.value)}
                className="font-mono"
                placeholder="Bash, Read, mcp__artex__graph_overview"
              />
              <p className="text-xs text-muted-foreground">Comma-separated tool names.</p>
            </div>
          </form>

          <DialogFooter className="sm:justify-between">
            {agent ? (
              <Button type="button" variant="destructive" onClick={() => setConfirmDelete(true)}>
                Delete
              </Button>
            ) : (
              <span />
            )}
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" form="agent-form" disabled={save.isPending || !name.trim()}>
                {save.isPending ? 'Saving...' : 'Save'}
              </Button>
            </div>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {agent?.name}?</AlertDialogTitle>
            <AlertDialogDescription>This removes the preset permanently.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => remove.mutate()}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
