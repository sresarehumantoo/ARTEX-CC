import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createConversation,
  deleteConversations,
  getClaudeStatus,
  getConversations,
  updateConversation,
} from '../lib/api/chat'
import type { CreateConversationRequest } from '../lib/chat-types'

export const conversationsKey = ['conversations'] as const

export function useConversations() {
  const qc = useQueryClient()
  const invalidate = () => qc.invalidateQueries({ queryKey: conversationsKey })

  const query = useQuery({
    queryKey: conversationsKey,
    queryFn: getConversations,
    refetchInterval: 10_000,
  })

  const create = useMutation({
    mutationFn: (req: CreateConversationRequest) => createConversation(req),
    onSuccess: invalidate,
  })

  const rename = useMutation({
    mutationFn: ({ id, title }: { id: string; title: string }) => updateConversation(id, { title }),
    onSuccess: invalidate,
  })

  const remove = useMutation({
    mutationFn: (ids: string[]) => deleteConversations(ids),
    onSuccess: invalidate,
  })

  return { ...query, conversations: query.data ?? [], create, rename, remove }
}

export function useClaudeStatus() {
  return useQuery({ queryKey: ['claude-status'], queryFn: getClaudeStatus, staleTime: 60_000 })
}
