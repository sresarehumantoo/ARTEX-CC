export type ConversationStatus = 'idle' | 'running' | 'error'

export interface Conversation {
  id: string
  title: string
  model: string
  agent: string
  status: ConversationStatus
  session_id: string
  created_at: string
  updated_at: string
}

export type ChatEventKind =
  | 'text'
  | 'thinking'
  | 'tool_use'
  | 'tool_result'
  | 'result'
  | 'error'
  | 'status'

export interface ChatEvent {
  seq: number
  ts: string
  conversation_id: string
  role: 'user' | 'assistant' | 'system'
  kind: ChatEventKind

  text?: string

  tool?: {
    id: string
    name: string
    input: Record<string, unknown>
  }

  tool_result?: {
    tool_use_id: string
    content: string
    is_error: boolean
  }

  usage?: TokenUsage

  status?: ConversationStatus
}

export interface TokenUsage {
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  cost_usd?: number
}

export interface ChatAttachment {
  name: string
  path: string
  size: number
}

export interface ChatMention {
  type: 'finding' | 'asset' | 'node'
  id: number
  label: string
}

export interface SendMessageRequest {
  text: string
  attachments?: ChatAttachment[]
  mentions?: ChatMention[]
}

export interface CreateConversationRequest {
  title?: string
  model?: string
  agent?: string
}

export interface ClaudeStatus {
  binary_found: boolean
  version: string
  logged_in: boolean
}

export interface SSEEvent {
  seq: number
  type: 'message' | 'delta' | 'status' | 'usage' | 'error'
  data: ChatEvent
}

// Claude Code stream-json event types (what the runner parses from stdout).
// These are internal to the backend; the frontend only sees ChatEvent.

export interface ClaudeStreamInit {
  type: 'system'
  subtype: 'init'
  session_id: string
  model: string
  tools: string[]
  mcp_servers: Array<{ name: string; status: string; source: string }>
}

export interface ClaudeStreamAssistant {
  type: 'assistant'
  message: {
    id: string
    role: 'assistant'
    content: Array<
      | { type: 'text'; text: string }
      | { type: 'tool_use'; id: string; name: string; input: Record<string, unknown> }
      | { type: 'thinking'; thinking: string }
    >
    stop_reason: string | null
    usage: {
      input_tokens: number
      output_tokens: number
      cache_read_input_tokens: number
      cache_creation_input_tokens: number
    }
  }
  session_id: string
}

export interface ClaudeStreamResult {
  type: 'result'
  subtype: 'success' | 'error'
  result: string
  duration_ms: number
  num_turns: number
  total_cost_usd: number
  usage: {
    input_tokens: number
    output_tokens: number
    cache_read_input_tokens: number
    cache_creation_input_tokens: number
    output_tokens_details: { thinking_tokens: number }
  }
  session_id: string
}

export type ClaudeStreamEvent =
  | ClaudeStreamInit
  | ClaudeStreamAssistant
  | ClaudeStreamResult
  | { type: 'system'; subtype: string; session_id: string; [key: string]: unknown }
  | { type: 'rate_limit_event'; session_id: string; [key: string]: unknown }
