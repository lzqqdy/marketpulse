import { ref, watch } from 'vue'
import {
  deleteConversation,
  listConversations,
  listMessages,
  patchConversationTitle,
  streamChat,
} from '../api'
import type { AnalysisTemplateId, ChatMessage, ConversationItem, PageContext, ToolStatus } from '../types'
import { formatAiError } from '../utils/formatAiError'
import { mergeEvidence, restoreToolResults } from '../utils/evidence'
import { startTool, finishTool, interruptTools, restoredSteps } from '../utils/toolProgress'

const CONV_KEY = 'marketpulse-ai-conversation-id'

export function useAiChatStream() {
  const messages = ref<ChatMessage[]>([])
  const conversationId = ref(localStorage.getItem(CONV_KEY) ?? '')
  const conversations = ref<ConversationItem[]>([])
  const streaming = ref(false)
  const error = ref('')
  const loadingList = ref(false)
  const toolStatus = ref<ToolStatus | null>(null)
  let abort: AbortController | null = null
  let generation = 0

  watch(conversationId, (id) => {
    if (id) localStorage.setItem(CONV_KEY, id)
    else localStorage.removeItem(CONV_KEY)
  })

  function reset(clearHistory = false) {
    generation++
    abort?.abort()
    abort = null
    messages.value = []
    conversationId.value = ''
    error.value = ''
    streaming.value = false
    toolStatus.value = null
    if (clearHistory) conversations.value = []
  }

  async function refreshConversations(token: string) {
    const current = generation
    loadingList.value = true
    try {
      const res = await listConversations(token)
      if (current === generation) conversations.value = res.items
    } catch (e) {
      if (current === generation) error.value = e instanceof Error ? e.message : '加载会话失败'
    } finally {
      loadingList.value = false
    }
  }

  async function loadConversation(token: string, id: string) {
    const current = ++generation
    error.value = ''
    toolStatus.value = null
    abort?.abort()
    streaming.value = false
    messages.value = []
    const res = await listMessages(token, id)
    if (current !== generation) return
    conversationId.value = res.conversationId
    messages.value = res.messages
      .filter((m) => m.role === 'user' || m.role === 'assistant')
      .map((m) => {
        const toolResults = restoreToolResults(m.metadata)
        return {
          id: String(m.id),
          role: m.role as 'user' | 'assistant',
          content: m.content,
          toolResults,
          toolSteps: restoredSteps(toolResults),
          interrupted: !!(m.metadata && typeof m.metadata === 'object' && 'incomplete' in m.metadata && m.metadata.incomplete),
          evidence: toolResults.flatMap((result) => result.evidence ?? []),
        }
      })
  }

  async function removeConversation(token: string, id: string) {
    await deleteConversation(token, id)
    if (conversationId.value === id) reset()
    await refreshConversations(token)
  }

  async function send(token: string, text: string, context?: PageContext, templateId?: AnalysisTemplateId) {
    const content = text.trim()
    if (!content || streaming.value) return
    const current = ++generation
    error.value = ''
    toolStatus.value = null
    streaming.value = true
    const userId = `u-${Date.now()}-${current}`
    messages.value.push({ id: userId, role: 'user', content })
    const assistantId = `a-${Date.now()}-${current}`
    messages.value.push({ id: assistantId, role: 'assistant', content: '', pending: true })

    const controller = new AbortController()
    abort = controller
    let completed = false
    try {
      for await (const ev of streamChat(
        token,
        {
          conversationId: conversationId.value || undefined,
          message: content,
          context,
          templateId,
        },
        controller.signal,
      )) {
        if (current !== generation) return
        if (controller.signal.aborted) throw new DOMException('Stopped', 'AbortError')
        const assistant = messages.value.find((m) => m.id === assistantId)
        if (ev.event === 'meta' && ev.data.conversationId) {
          conversationId.value = ev.data.conversationId
        } else if (ev.event === 'token') {
          const last = messages.value.find((m) => m.id === assistantId)
          if (last) last.content += ev.data.text ?? ''
        } else if (ev.event === 'tool_start') {
          if (assistant) toolStatus.value = startTool(assistant, ev.data.name, ev.data.toolCallId)
        } else if (ev.event === 'tool_result') {
          const last = messages.value.find((m) => m.id === assistantId)
          if (last) {
            toolStatus.value = finishTool(last, ev.data)
            last.evidence = mergeEvidence(last.evidence ?? [], ev.data.evidence ?? [])
          }
        } else if (ev.event === 'error') {
          error.value = formatAiError(ev.data.code, ev.data.message || 'AI 出错')
          toolStatus.value = null
        } else if (ev.event === 'done') {
          completed = true
          if (ev.data.conversationId) conversationId.value = ev.data.conversationId
          toolStatus.value = null
        }
      }
      if (current === generation) void refreshConversations(token)
    } catch (e) {
      if (current !== generation) return
      if ((e as Error).name === 'AbortError') {
        error.value = '已停止生成'
      } else {
        error.value = formatAiError(undefined, e instanceof Error ? e.message : '请求失败')
      }
      toolStatus.value = null
    } finally {
      if (current === generation) {
        const last = messages.value.find((m) => m.id === assistantId)
        if (last) {
          last.pending = false
          last.interrupted = !completed
          interruptTools(last)
        }
        toolStatus.value = null
        streaming.value = false
        abort = null
      }
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
