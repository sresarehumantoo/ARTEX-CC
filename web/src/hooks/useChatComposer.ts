import { useCallback, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { sendMessage, stopConversation, updateConversation } from '../lib/api/chat'
import { conversationsKey } from './useConversations'

export function useChatComposer(conversationId: string | null) {
  const qc = useQueryClient()
  const [text, setText] = useState('')
  const invalidate = () => qc.invalidateQueries({ queryKey: conversationsKey })

  const send = useMutation({
    mutationFn: (value: string) => sendMessage(conversationId!, { message: value }),
    onSuccess: invalidate,
  })

  const stop = useMutation({
    mutationFn: () => stopConversation(conversationId!),
    onSuccess: invalidate,
  })

  const setModel = useMutation({
    mutationFn: (model: string) => updateConversation(conversationId!, { model }),
    onSuccess: invalidate,
  })

  const submit = useCallback(() => {
    const value = text.trim()
    if (!conversationId || !value || send.isPending) return
    setText('')
    send.mutate(value, {
      onError: (err) => {
        setText(value)
        toast.error('Failed to send message', {
          description: err instanceof Error ? err.message : 'Unknown error',
        })
      },
    })
  }, [conversationId, text, send])

  return { text, setText, submit, send, stop, setModel }
}
