import { ref, watch } from 'vue'
import {
  deleteConversation,
  listConversations,
  listMessages,
  patchConversationTitle,
  streamChat,
} from '../api'
import type { ChatMessage, ConversationItem, PageContext, ToolStatus } from '../types'
import { formatAiError } from '../utils/formatAiError'

const CONV_KEY = 'marketpulse-ai-conversation-id'

const TOOL_LABELS: Record<string, string> = {
  get_quote: '报价',
  get_snapshot_summary: '盘面摘要',
  get_klines_summary: 'K线摘要',
  get_express_news: '快讯',
  get_market_breadth: '涨跌广度',
}

function toolLabel(name: string) {
  return TOOL_LABELS[name] || name
}

export function useAiChatStream() {
  const messages = ref<ChatMessage[]>([])
  const conversationId = ref(localStorage.getItem(CONV_KEY) ?? '')
  const conversations = ref<ConversationItem[]>([])
  const streaming = ref(false)
  const error = ref('')
  const loadingList = ref(false)
  const toolStatus = ref<ToolStatus | null>(null)
  let abort: AbortController | null = null

  watch(conversationId, (id) => {
    if (id) localStorage.setItem(CONV_KEY, id)
    else localStorage.removeItem(CONV_KEY)
  })

  function reset() {
    abort?.abort()
    abort = null
    messages.value = []
    conversationId.value = ''
    error.value = ''
    streaming.value = false
    toolStatus.value = null
  }

  async function refreshConversations(token: string) {
    loadingList.value = true
    try {
      const res = await listConversations(token)
      conversations.value = res.items
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载会话失败'
    } finally {
      loadingList.value = false
    }
  }

  async function loadConversation(token: string, id: string) {
    error.value = ''
    toolStatus.value = null
    abort?.abort()
    const res = await listMessages(token, id)
    conversationId.value = res.conversationId
    messages.value = res.messages
      .filter((m) => m.role === 'user' || m.role === 'assistant')
      .map((m) => ({
        id: String(m.id),
        role: m.role as 'user' | 'assistant',
        content: m.content,
      }))
  }

  async function removeConversation(token: string, id: string) {
    await deleteConversation(token, id)
    if (conversationId.value === id) reset()
    await refreshConversations(token)
  }

  async function send(token: string, text: string, context?: PageContext) {
    const content = text.trim()
    if (!content || streaming.value) return
    error.value = ''
    toolStatus.value = null
    streaming.value = true
    const userId = `u-${Date.now()}`
    messages.value.push({ id: userId, role: 'user', content })
    const assistantId = `a-${Date.now()}`
    messages.value.push({ id: assistantId, role: 'assistant', content: '', pending: true })

    abort = new AbortController()
    try {
      for await (const ev of streamChat(
        token,
        {
          conversationId: conversationId.value || undefined,
          message: content,
          context,
        },
        abort.signal,
      )) {
        if (ev.event === 'meta' && ev.data.conversationId) {
          conversationId.value = ev.data.conversationId
        } else if (ev.event === 'token') {
          const last = messages.value.find((m) => m.id === assistantId)
          if (last) last.content += ev.data.text ?? ''
        } else if (ev.event === 'tool_start') {
          toolStatus.value = {
            name: ev.data.name,
            label: toolLabel(ev.data.name),
            phase: 'running',
          }
        } else if (ev.event === 'tool_result') {
          toolStatus.value = {
            name: ev.data.name,
            label: toolLabel(ev.data.name),
            phase: ev.data.ok ? 'done' : 'error',
          }
        } else if (ev.event === 'error') {
          error.value = formatAiError(ev.data.code, ev.data.message || 'AI 出错')
          toolStatus.value = null
        } else if (ev.event === 'done') {
          if (ev.data.conversationId) conversationId.value = ev.data.conversationId
          toolStatus.value = null
        }
      }
      void refreshConversations(token)
    } catch (e) {
      if ((e as Error).name === 'AbortError') {
        error.value = '已停止生成'
      } else {
        error.value = formatAiError(undefined, e instanceof Error ? e.message : '请求失败')
      }
      toolStatus.value = null
    } finally {
      const last = messages.value.find((m) => m.id === assistantId)
      if (last) last.pending = false
      streaming.value = false
      abort = null
    }
  }

  function stop() {
    abort?.abort()
    toolStatus.value = null
  }

  async function renameConversation(token: string, id: string, title: string) {
    const trimmed = title.trim().slice(0, 64)
    if (!trimmed) return
    const item = await patchConversationTitle(token, id, trimmed)
    const idx = conversations.value.findIndex((c) => c.conversationId === id)
    if (idx >= 0) conversations.value[idx] = { ...conversations.value[idx], ...item, title: trimmed }
    else await refreshConversations(token)
  }

  return {
    messages,
    conversationId,
    conversations,
    streaming,
    error,
    loadingList,
    toolStatus,
    send,
    stop,
    reset,
    refreshConversations,
    loadConversation,
    removeConversation,
    renameConversation,
  }
}
