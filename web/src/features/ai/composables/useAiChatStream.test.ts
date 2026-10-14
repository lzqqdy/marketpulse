import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'
import { listConversations, listMessages, streamChat } from '../api'
import { useAiChatStream } from './useAiChatStream'
import type { AiStreamEvent } from '../types'

vi.mock('../api', () => ({ streamChat: vi.fn(), listConversations: vi.fn(), listMessages: vi.fn(), deleteConversation: vi.fn(), patchConversationTitle: vi.fn() }))
let scope = effectScope()
beforeEach(() => {
  scope = effectScope()
  vi.resetAllMocks()
  const storage = new Map<string, string>()
  vi.stubGlobal('localStorage', { getItem: (key: string) => storage.get(key) ?? null, setItem: (key: string, value: string) => storage.set(key, value), removeItem: (key: string) => storage.delete(key) })
  vi.mocked(listConversations).mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 })
})
afterEach(() => { scope.stop(); vi.unstubAllGlobals() })
function chat() { return scope.run(() => useAiChatStream())! }
function deferred() { let resolve!: () => void; const promise = new Promise<void>((done) => { resolve = done }); return { promise, resolve } }

describe('AI conversation stream lifecycle', () => {
  it('keeps per-answer progress and forwards template without losing token fragments', async () => {
    vi.mocked(streamChat).mockImplementation(async function* () {
      yield { event: 'tool_start', data: { name: 'get_quote', toolCallId: 'a' } }
      yield { event: 'tool_result', data: { name: 'get_quote', toolCallId: 'a', status: 'partial', ok: false } }
      yield { event: 'token', data: { text: '第一段' } }
      yield { event: 'token', data: { text: '第二段' } }
      yield { event: 'done', data: { conversationId: 'conv' } }
    })
    const state = chat()
    await state.send('token', '报价', {}, 'symbol_review')
    const answer = state.messages.value[1]
    expect(answer.content).toBe('第一段第二段')
    expect(answer.toolSteps?.[0].phase).toBe('partial')
    expect(answer.interrupted).toBe(false)
    expect(state.toolStatus.value).toBeNull()
    expect(vi.mocked(streamChat).mock.calls[0][1].templateId).toBe('symbol_review')
  })
  it('preserves partial content and marks unfinished tools interrupted on connection failure', async () => {
    vi.mocked(streamChat).mockImplementation(async function* () {
      yield { event: 'tool_start', data: { name: 'get_quote', toolCallId: 'a' } }
      yield { event: 'token', data: { text: '部分回答' } }
      throw new Error('connection reset')
    })
    const state = chat()
    await state.send('token', '报价')
    expect(state.messages.value[1].content).toBe('部分回答')
    expect(state.messages.value[1].interrupted).toBe(true)
    expect(state.messages.value[1].toolSteps?.[0].phase).toBe('interrupted')
    expect(state.streaming.value).toBe(false)
  })
  it('reset prevents late events and errors from changing a newer conversation', async () => {
    const entered = deferred(), oldGate = deferred(), newGate = deferred()
    let calls = 0
    vi.mocked(streamChat).mockImplementation(async function* (): AsyncGenerator<AiStreamEvent> {
      if (++calls === 1) { entered.resolve(); await oldGate.promise; throw new Error('old failure') }
      yield { event: 'token', data: { text: '新回答' } }
      await newGate.promise
      yield { event: 'done', data: { conversationId: 'new-conv' } }
    })
    const state = chat()
    const old = state.send('token', '旧问题')
    await entered.promise
    state.reset()
    const current = state.send('token', '新问题')
    oldGate.resolve()
    await old
    expect(state.error.value).toBe('')
    expect(state.streaming.value).toBe(true)
    newGate.resolve()
    await current
    expect(state.messages.value.map((item) => item.content)).toEqual(['新问题', '新回答'])
    expect(state.conversationId.value).toBe('new-conv')
  })
  it('restores interrupted metadata and revalidates old evidence', async () => {
    vi.mocked(listMessages).mockResolvedValue({ conversationId: 'old', messages: [{ id: 1, role: 'assistant', content: '部分回答', createdAt: '', metadata: { incomplete: true, toolResults: [{ name: 'get_quote', ok: true, status: 'ok', evidence: [{ id: 'ev_old', kind: 'quote', facts: { price: 10 }, status: 'ok', expiresAt: '2020-01-01T00:00:00Z' }] }] } }] })
    const state = chat()
    await state.loadConversation('token', 'old')
    expect(state.messages.value[0].interrupted).toBe(true)
    expect(state.messages.value[0].toolSteps?.[0].phase).toBe('stale')
    expect(state.messages.value[0].evidence?.[0].status).toBe('stale')
  })
})
