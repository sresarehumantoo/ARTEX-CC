import type { ChatEvent } from '../../../lib/chat-types'
import { ArtexToolCard } from './ArtexToolCard'
import { BashCard } from './BashCard'
import { FileCard } from './FileCard'
import { GenericCard } from './GenericCard'

export { ArtexToolCard, BashCard, FileCard, GenericCard }
export { Expandable, ResultBlock, RESULT_PREVIEW, keyArg } from './GenericCard'

const FILE_TOOLS = new Set(['Read', 'Edit', 'MultiEdit', 'Write'])

export function ToolRenderer({ event, result }: { event: ChatEvent; result?: ChatEvent }) {
  const name = event.tool_name ?? ''
  if (name === 'Bash' || name === 'BashOutput') return <BashCard event={event} result={result} />
  if (FILE_TOOLS.has(name)) return <FileCard event={event} result={result} />
  if (name.startsWith('mcp__artex__')) return <ArtexToolCard event={event} result={result} />
  return <GenericCard event={event} result={result} />
}
