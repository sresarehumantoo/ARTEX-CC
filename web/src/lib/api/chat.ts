import type {
  ChatEvent,
  Conversation,
  CreateConversationRequest,
  SendMessageRequest,
} from '../chat-types'
import { installChatMock, isChatMock } from '../chat-mock'

const BASE = '/api'

if (isChatMock()) installChatMock()

async function request<T>(path: string, init?: RequestInit, params?: Record<string, string>): Promise<T> {
  const url = new URL(path, window.location.origin)
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v) url.searchParams.set(k, v)
    }
  }
  const res = await fetch(url.toString(), init)
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    const parsed = body ? (() => { try { return JSON.parse(body) } catch { return null } })() : null
    const detail = parsed?.error || body || res.statusText
    throw new Error(detail)
  }
  if (res.status === 204) return undefined as T
  const text = await res.text()
  return (text ? JSON.parse(text) : undefined) as T
}

function jsonInit(method: string, body?: unknown): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  }
}

export async function getConversations() {
  return (await request<Conversation[] | null>(`${BASE}/conversations`)) ?? []
}

export function createConversation(req: CreateConversationRequest) {
  return request<Conversation>(`${BASE}/conversations`, jsonInit('POST', req))
}

export function getConversation(id: string) {
  return request<Conversation>(`${BASE}/conversations/${id}`)
}

export function updateConversation(id: string, fields: Partial<Conversation>) {
  return request<Conversation>(`${BASE}/conversations/${id}`, jsonInit('PATCH', fields))
}

export function deleteConversation(id: string) {
  return request<void>(`${BASE}/conversations/${id}`, { method: 'DELETE' })
}

export function deleteConversations(ids: string[]) {
  return request<void>(`${BASE}/conversations`, jsonInit('DELETE', { ids }))
}

export async function getMessages(id: string, after?: number) {
  const events = await request<ChatEvent[] | null>(
    `${BASE}/conversations/${id}/messages`,
    undefined,
    after !== undefined ? { after: String(after) } : undefined,
  )
  return events ?? []
}

export function sendMessage(id: string, req: SendMessageRequest) {
  return request<void>(`${BASE}/conversations/${id}/messages`, jsonInit('POST', req))
}

export function stopConversation(id: string) {
  return request<void>(`${BASE}/conversations/${id}/stop`, { method: 'POST' })
}

export interface ClaudeStatus {
  available: boolean
  path?: string
  version?: string
  error?: string
}

export function getClaudeStatus() {
  return request<ClaudeStatus>(`${BASE}/claude/status`)
}
