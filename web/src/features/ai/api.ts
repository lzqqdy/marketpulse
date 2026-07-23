import type {
  AiStreamEvent,
  ChatMessage,
  ChatRequest,
  ConversationItem,
  ConversationListResult,
  MessagesResult,
  PageContext,
} from './types'
import { formatAiError } from './utils/formatAiError'

async function parseError(res: Response): Promise<string> {
  const body = await res.json().catch(() => ({}))
  return formatAiError(body?.error?.code, body?.error?.message ?? `HTTP ${res.status}`)
}

function authHeaders(token: string, json = false): HeadersInit {
  const h: Record<string, string> = { Authorization: `Bearer ${token}` }
  if (json) h['Content-Type'] = 'application/json'
  return h
}

export async function* streamChat(
  token: string,
  req: ChatRequest,
  signal?: AbortSignal,
): AsyncGenerator<AiStreamEvent> {
  const res = await fetch('/api/v1/ai/chat', {
    method: 'POST',
    headers: {
      ...authHeaders(token, true),
      Accept: 'text/event-stream',
    },
    body: JSON.stringify(req),
    signal,
  })

  if (!res.ok) {
    throw new Error(await parseError(res))
  }
  if (!res.body) {
    throw new Error('浏览器不支持流式响应')
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let eventName = 'message'

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split('\n')
    buffer = parts.pop() ?? ''
    for (const rawLine of parts) {
      const line = rawLine.replace(/\r$/, '')
      if (line.startsWith('event:')) {
        eventName = line.slice(6).trim()
        continue
      }
      if (line.startsWith('data:')) {
        const dataStr = line.slice(5).trim()
        try {
          const data = JSON.parse(dataStr)
          yield { event: eventName, data } as AiStreamEvent
        } catch {
          /* ignore */
        }
        eventName = 'message'
        continue
      }
      if (line === '') eventName = 'message'
    }
  }
}

export async function listConversations(
  token: string,
  page = 1,
  pageSize = 20,
): Promise<ConversationListResult> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  const res = await fetch(`/api/v1/ai/conversations?${params}`, { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function listMessages(token: string, conversationId: string): Promise<MessagesResult> {
  const res = await fetch(`/api/v1/ai/conversations/${encodeURIComponent(conversationId)}/messages`, {
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function deleteConversation(token: string, conversationId: string): Promise<void> {
  const res = await fetch(`/api/v1/ai/conversations/${encodeURIComponent(conversationId)}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok && res.status !== 204) throw new Error(await parseError(res))
}

export async function patchConversationTitle(
  token: string,
  conversationId: string,
  title: string,
): Promise<ConversationItem> {
  const res = await fetch(`/api/v1/ai/conversations/${encodeURIComponent(conversationId)}`, {
    method: 'PATCH',
    headers: authHeaders(token, true),
    body: JSON.stringify({ title }),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export type { ChatMessage, PageContext }
