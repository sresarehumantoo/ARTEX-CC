import { useEffect, useRef, useState } from 'react'
import { ArrowUp, Square } from 'lucide-react'
import type { ConversationStatus } from '../../lib/chat-types'
import { Button } from '../ui/button'
import { Select } from '../ui/select'
import { Textarea } from '../ui/textarea'
import { MentionPopover, useMentionResults, type Mention } from './MentionPopover'

const MENTION_RE = /(^|\s)@([\w.-]*)$/

export const MODELS = ['opus', 'sonnet', 'haiku']

interface ComposerProps {
  text: string
  onTextChange: (text: string) => void
  onSubmit: () => void
  onStop: () => void
  model: string
  onModelChange: (model: string) => void
  status: ConversationStatus | null
  disabled?: boolean
  sending?: boolean
}

export function Composer({
  text,
  onTextChange,
  onSubmit,
  onStop,
  model,
  onModelChange,
  status,
  disabled,
  sending,
}: ComposerProps) {
  const ref = useRef<HTMLTextAreaElement>(null)
  const running = status === 'running'
  const [caret, setCaret] = useState(0)
  const [active, setActive] = useState(0)
  const [dismissed, setDismissed] = useState(false)
  const match = MENTION_RE.exec(text.slice(0, caret))
  const mentionQuery = match && !dismissed ? match[2] : null
  const results = useMentionResults(mentionQuery)
  const open = results.length > 0

  useEffect(() => {
    setActive(0)
    setDismissed(false)
  }, [match?.[2]])

  function insert(m: Mention) {
    if (!match) return
    const start = caret - match[2].length - 1
    const token = `@${m.type}:${m.id} `
    onTextChange(text.slice(0, start) + token + text.slice(caret))
    const pos = start + token.length
    requestAnimationFrame(() => {
      ref.current?.focus()
      ref.current?.setSelectionRange(pos, pos)
      setCaret(pos)
    })
  }

  useEffect(() => {
    const el = ref.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`
  }, [text])

  return (
    <div className="border-t border-border p-3">
      <div className="relative flex items-end gap-2">
        {open && <MentionPopover results={results} active={active} onSelect={insert} onHover={setActive} />}
        <Textarea
          ref={ref}
          rows={1}
          value={text}
          disabled={disabled}
          placeholder={disabled ? 'Select a conversation' : 'Message Claude (Enter to send, Shift+Enter for newline)'}
          className="max-h-[200px] min-h-9 flex-1 resize-none text-sm"
          onChange={e => {
            onTextChange(e.target.value)
            setCaret(e.target.selectionStart)
          }}
          onSelect={e => setCaret(e.currentTarget.selectionStart)}
          onKeyDown={e => {
            if (open) {
              if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                e.preventDefault()
                setActive(i => (i + (e.key === 'ArrowDown' ? 1 : results.length - 1)) % results.length)
                return
              }
              if ((e.key === 'Enter' || e.key === 'Tab') && !e.nativeEvent.isComposing) {
                e.preventDefault()
                insert(results[active])
                return
              }
              if (e.key === 'Escape') {
                e.preventDefault()
                setDismissed(true)
                return
              }
            }
            if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault()
              if (!running) onSubmit()
            }
          }}
        />
        {running ? (
          <Button size="icon" variant="destructive" onClick={onStop} aria-label="Stop">
            <Square className="fill-current" />
          </Button>
        ) : (
          <Button size="icon" onClick={onSubmit} disabled={disabled || sending || !text.trim()} aria-label="Send">
            <ArrowUp />
          </Button>
        )}
      </div>
      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
        <Select size="sm" value={model} disabled={disabled} onChange={e => onModelChange(e.target.value)} aria-label="Model">
          {MODELS.map(m => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </Select>
        <span>{text.length > 0 ? `~${Math.ceil(text.length / 4)} tokens` : ''}</span>
      </div>
    </div>
  )
}
