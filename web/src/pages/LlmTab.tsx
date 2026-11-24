import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { toast } from 'sonner'
import { FALLBACK_MODELS, getLLMSettings, getLLMStatus, updateLLMSettings, llmSettingsKey, llmStatusKey } from '../lib/api/llm'
import type { LLMSettings } from '../lib/api/llm'
import { Button } from '../components/ui/button'
import { Badge } from '../components/ui/badge'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../components/ui/card'
import { SelectRoot, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../components/ui/select'
import { ModelOptions } from '../components/chat/ModelOptions'
import { RadioGroup, RadioGroupItem } from '../components/ui/radio-group'
import { Slider } from '../components/ui/slider'
import { Textarea } from '../components/ui/textarea'
import { Label } from '../components/ui/label'

const THINKING_LEVELS = ['default', 'off', 'low', 'medium', 'high']

const STRATEGIES = [
  { id: 'parallel', label: 'Parallel', description: 'Run independent tasks concurrently' },
  { id: 'sequential', label: 'Sequential', description: 'Run tasks one at a time' },
  { id: 'per_agent', label: 'Per-agent', description: "Use each agent preset's configured model" },
]

function toSelect(value: string | undefined) {
  return value ? value : 'default'
}

function fromSelect(value: string) {
  return value === 'default' ? '' : value
}

function SliderField({
  label,
  value,
  min,
  max,
  disabled,
  onChange,
}: {
  label: string
  value: number
  min: number
  max: number
  disabled?: boolean
  onChange: (value: number) => void
}) {
  return (
    <div className="space-y-3">
      <div className="flex items-baseline justify-between gap-2">
        <Label>{label}</Label>
        <span className="font-mono text-sm text-muted-foreground">{value}</span>
      </div>
      <Slider
        aria-label={label}
        min={min}
        max={max}
        step={1}
        value={[value]}
        disabled={disabled}
        onValueChange={v => onChange(v[0])}
      />
    </div>
  )
}

function InfoRow({ label, value }: { label: string; value?: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 sm:flex-row sm:items-baseline sm:justify-between sm:gap-6">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className="break-all font-mono text-sm text-foreground sm:text-right">{value || 'Unknown'}</dd>
    </div>
  )
}

export default function LlmTab() {
  const queryClient = useQueryClient()
  const { data: settings } = useQuery({ queryKey: llmSettingsKey, queryFn: getLLMSettings })
  const { data: status } = useQuery({ queryKey: llmStatusKey, queryFn: getLLMStatus })
  const [draft, setDraft] = useState<LLMSettings | null>(null)
  const [detailsOpen, setDetailsOpen] = useState(false)

  useEffect(() => {
    if (settings) setDraft(settings)
  }, [settings])

  const save = useMutation({
    mutationFn: (next: LLMSettings) => updateLLMSettings(next),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: llmSettingsKey })
      queryClient.invalidateQueries({ queryKey: llmStatusKey })
      toast.success('LLM settings saved')
    },
    onError: (err: Error) => toast.error(`Failed to save: ${err.message}`),
  })

  if (!draft || !settings) {
    return <p className="text-sm text-muted-foreground">Loading LLM settings...</p>
  }

  const dirty = JSON.stringify(draft) !== JSON.stringify(settings)
  const patch = (changes: Partial<LLMSettings>) => setDraft({ ...draft, ...changes })
  const claude = status?.backends.find(b => b.id === 'claude')
  const models = status?.models ?? FALLBACK_MODELS

  const modelSelect = (label: string, hint: string, value: string, onChange: (v: string) => void) => (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      <SelectRoot value={toSelect(value)} onValueChange={v => onChange(fromSelect(v))}>
        <SelectTrigger aria-label={label} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="default">(CLI default)</SelectItem>
          <ModelOptions models={models} />
        </SelectContent>
      </SelectRoot>
      <p className="text-xs text-muted-foreground">{hint}</p>
    </div>
  )

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Backend</CardTitle>
          <CardDescription>AI backend for running conversations.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <RadioGroup value={draft.backend} onValueChange={v => patch({ backend: v })}>
            <div className="flex items-start gap-3">
              <RadioGroupItem value="claude" id="backend-claude" className="mt-0.5" />
              <div className="flex flex-wrap items-center gap-2">
                <Label htmlFor="backend-claude">Claude Code CLI</Label>
                {claude?.available ? (
                  <Badge variant="get" className="gap-1.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
                    {claude.version || 'Available'}
                  </Badge>
                ) : (
                  <Badge variant="delete" className="gap-1.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-rose-400" />
                    {claude?.error || 'Unavailable'}
                  </Badge>
                )}
              </div>
            </div>
            <div className="flex items-start gap-3 opacity-70">
              <RadioGroupItem value="opencode" id="backend-opencode" disabled className="mt-0.5" />
              <div className="flex flex-wrap items-center gap-2">
                <Label htmlFor="backend-opencode">OpenCode CLI</Label>
                <Badge variant="secondary">Coming soon</Badge>
              </div>
            </div>
          </RadioGroup>
          <p className="text-xs text-muted-foreground">Authentication is managed by your CLI login. No API keys are stored.</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Models</CardTitle>
          <CardDescription>Default models for new conversations and agent roles.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            {modelSelect('Default model', 'Used for new conversations.', draft.default_model, v => patch({ default_model: v }))}
            {modelSelect('Master model', 'Used by the orchestrator.', draft.master_model, v => patch({ master_model: v }))}
            {modelSelect('Subagent model', 'Used by subagents.', draft.subagent_model, v => patch({ subagent_model: v }))}
          </div>
          <p className="text-xs text-muted-foreground">Agent presets can override models individually.</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Subagent dispatch</CardTitle>
          <CardDescription>How subagent tasks are dispatched. These are advisory instructions.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <RadioGroup value={draft.subagent_strategy} onValueChange={v => patch({ subagent_strategy: v })}>
            {STRATEGIES.map(s => (
              <div key={s.id} className="flex items-start gap-3">
                <RadioGroupItem value={s.id} id={`strategy-${s.id}`} className="mt-0.5" />
                <div className="space-y-0.5">
                  <Label htmlFor={`strategy-${s.id}`}>{s.label}</Label>
                  <p className="text-xs text-muted-foreground">{s.description}</p>
                </div>
              </div>
            ))}
          </RadioGroup>
          <div className="max-w-sm">
            <SliderField
              label="Max parallel subagents"
              value={draft.max_parallel_subagents}
              min={1}
              max={10}
              disabled={draft.subagent_strategy === 'sequential'}
              onChange={v => patch({ max_parallel_subagents: v })}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Limits</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
            <SliderField
              label="Max turns per conversation"
              value={draft.max_turns}
              min={5}
              max={200}
              onChange={v => patch({ max_turns: v })}
            />
            <SliderField
              label="Max concurrent conversations"
              value={draft.max_conversations}
              min={1}
              max={16}
              onChange={v => patch({ max_conversations: v })}
            />
            <div className="space-y-3">
              <Label>Thinking level</Label>
              <SelectRoot value={toSelect(draft.thinking)} onValueChange={v => patch({ thinking: fromSelect(v) })}>
                <SelectTrigger aria-label="Thinking level" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {THINKING_LEVELS.map(level => (
                    <SelectItem key={level} value={level}>
                      {level.charAt(0).toUpperCase() + level.slice(1)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </SelectRoot>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Global system prompt</CardTitle>
          <CardDescription>Appended to every conversation's system prompt.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          <Textarea
            aria-label="Global system prompt"
            rows={4}
            className="font-mono"
            value={draft.global_system_prompt}
            onChange={e => patch({ global_system_prompt: e.target.value })}
          />
          <p className="text-xs text-muted-foreground">{draft.global_system_prompt.length} characters</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <button
            type="button"
            onClick={() => setDetailsOpen(o => !o)}
            aria-expanded={detailsOpen}
            className="flex w-full items-center justify-between text-left"
          >
            <CardTitle>Runner details</CardTitle>
            {detailsOpen ? <ChevronUp className="h-4 w-4 text-muted-foreground" /> : <ChevronDown className="h-4 w-4 text-muted-foreground" />}
          </button>
        </CardHeader>
        {detailsOpen && (
          <CardContent>
            <dl className="space-y-3">
              <InfoRow label="Claude binary path" value={claude?.path} />
              <InfoRow label="Claude version" value={claude?.version} />
              <InfoRow
                label="Backend status"
                value={claude ? (claude.available ? 'Available' : claude.error || 'Unavailable') : undefined}
              />
            </dl>
          </CardContent>
        )}
      </Card>

      {dirty && (
        <div className="sticky bottom-0 flex flex-col gap-3 rounded-lg border bg-background/95 p-3 backdrop-blur sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-muted-foreground">Changes apply to new conversations, not running ones.</p>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={save.isPending} onClick={() => setDraft(settings)}>
              Reset
            </Button>
            <Button size="sm" disabled={save.isPending} onClick={() => save.mutate(draft)}>
              {save.isPending ? 'Saving...' : 'Save'}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
