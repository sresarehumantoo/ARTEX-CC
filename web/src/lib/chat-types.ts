export type ConversationStatus = 'idle' | 'running' | 'error'

export interface Conversation {
  id: string
  title: string
  model?: string
  agent?: string
  system_prompt?: string
  session_id?: string
  status: ConversationStatus
  created_at: string
  updated_at: string
}

export type ChatEventKind =
  | 'user'
  | 'text'
  | 'thinking'
  | 'tool_use'
  | 'tool_result'
  | 'result'
  | 'error'

export interface ChatUsage {
  input_tokens: number
  output_tokens: number
  cache_read_tokens?: number
  cache_creation_tokens?: number
  cost_usd?: number
  duration_ms?: number
  num_turns?: number
}

export interface ChatEvent {
  seq: number
  kind: ChatEventKind
  text?: string
  tool_name?: string
  tool_id?: string
  input?: Record<string, unknown>
  is_error?: boolean
  usage?: ChatUsage
  time: string
}

export interface SendMessageRequest {
  message: string
}

export interface CreateConversationRequest {
  title?: string
  model?: string
  agent?: string
  system_prompt?: string
}

export interface UpdateConversationRequest {
  title?: string
  model?: string
  agent?: string
}