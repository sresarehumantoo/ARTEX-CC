import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FileText, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { deleteSkill, getSkills, saveSkill, type Skill } from '../lib/api/tools'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Skeleton } from '../components/ui/skeleton'
import { Textarea } from '../components/ui/textarea'
import { Badge } from '../components/ui/badge'

interface Draft {
  name: string
  description: string
  content: string
  isNew: boolean
}

const emptyDraft: Draft = { name: '', description: '', content: '', isNew: true }

export default function Skills() {
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useQuery({ queryKey: ['skills'], queryFn: getSkills })
  const [draft, setDraft] = useState<Draft | null>(null)

  const select = (s: Skill) =>
    setDraft({ name: s.name, description: s.description, content: s.content, isNew: false })

  const save = useMutation({
    mutationFn: (d: Draft) => saveSkill(d.name.trim(), { description: d.description, content: d.content }),
    onSuccess: s => {
      queryClient.invalidateQueries({ queryKey: ['skills'] })
      toast.success('Skill saved')
      select(s)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const remove = useMutation({
    mutationFn: (name: string) => deleteSkill(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skills'] })
      toast.success('Skill deleted')
      setDraft(null)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  return (
    <div className="grid gap-4 md:grid-cols-[16rem_1fr]">
      <div className="space-y-2">
        <Button size="sm" className="w-full" onClick={() => setDraft(emptyDraft)}>
          <Plus /> New Skill
        </Button>
        {error && <p className="text-sm text-destructive">Failed to load skills: {error.message}</p>}
        {isLoading ? (
          <Skeleton className="h-24 rounded-xl" />
        ) : data && data.length === 0 ? (
          <p className="text-xs text-muted-foreground italic">No skills yet.</p>
        ) : (
          <ul className="rounded-xl bg-card ring-1 ring-foreground/10 overflow-hidden divide-y divide-foreground/10">
            {data?.map(s => (
              <li key={s.name}>
                <button
                  type="button"
                  onClick={() => select(s)}
                  className={`flex w-full items-start gap-2 px-3 py-2 text-left text-xs transition-colors hover:bg-muted/50 ${
                    draft?.name === s.name && !draft.isNew ? 'bg-muted/50' : ''
                  }`}
                >
                  <FileText className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                  <span className="min-w-0">
                    <span className="block truncate font-mono">{s.name}</span>
                    {s.description && <span className="block truncate text-muted-foreground">{s.description}</span>}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {draft ? (
        <form
          className="space-y-4 rounded-xl bg-card p-4 ring-1 ring-foreground/10"
          onSubmit={e => {
            e.preventDefault()
            save.mutate(draft)
          }}
        >
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-sm font-medium">{draft.isNew ? 'New skill' : 'Edit skill'}</h2>
            {!draft.isNew && <Badge variant="outline">{draft.name}</Badge>}
          </div>
          {draft.isNew && (
            <div className="space-y-1.5">
              <Label htmlFor="skill-name">Name</Label>
              <Input
                id="skill-name"
                value={draft.name}
                onChange={e => setDraft({ ...draft, name: e.target.value })}
                placeholder="web-recon"
                pattern="[a-z0-9]+(-[a-z0-9]+)*"
                title="Lowercase letters, digits, and hyphens"
                required
              />
            </div>
          )}
          <div className="space-y-1.5">
            <Label htmlFor="skill-description">Description</Label>
            <Input
              id="skill-description"
              value={draft.description}
              onChange={e => setDraft({ ...draft, description: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="skill-content">Content</Label>
            <Textarea
              id="skill-content"
              value={draft.content}
              onChange={e => setDraft({ ...draft, content: e.target.value })}
              className="min-h-[320px] font-mono text-xs"
            />
          </div>
          <div className="flex justify-between">
            {draft.isNew ? (
              <span />
            ) : (
              <Button
                type="button"
                variant="destructive"
                disabled={remove.isPending}
                onClick={() => {
                  if (window.confirm(`Delete skill "${draft.name}"?`)) remove.mutate(draft.name)
                }}
              >
                <Trash2 /> Delete
              </Button>
            )}
            <Button type="submit" disabled={save.isPending}>
              Save
            </Button>
          </div>
        </form>
      ) : (
        <p className="text-sm text-muted-foreground italic">Select or create a skill.</p>
      )}
    </div>
  )
}
