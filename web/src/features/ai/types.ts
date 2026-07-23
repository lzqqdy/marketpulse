export interface PageContext {
  focusSymbol?: string
  assetClass?: string
  page?: string
  visibleSymbols?: string[]
}

export interface ChatRequest {
  conversationId?: string
  message: string
  context?: PageContext
}

export type AiStreamEvent =
  | { event: 'meta'; data: { conversationId: string; messageId?: number } }
  | { event: 'token'; data: { text: string } }
  | { event: 'tool_start'; data: { name: string; arguments?: string } }
  | { event: 'tool_result'; data: { name: string; ok: boolean; summary?: string } }
  | { event: 'done'; data: { finishReason?: string; conversationId?: string } }
  | { event: 'error'; data: { code?: string; message?: string } }

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  pending?: boolean
}

export interface ConversationItem {
  conversationId: string
  title: string
  updatedAt: string
  createdAt: string
  status?: string
}

export interface ConversationListResult {
  total: number
  page: number
  pageSize: number
  items: ConversationItem[]
}

export interface StoredMessage {
  id: number
  role: string
  content: string
  createdAt: string
  metadata?: unknown
}

export interface MessagesResult {
  conversationId: string
  messages: StoredMessage[]
}

export interface ToolStatus {
  name: string
  label: string
  phase: 'running' | 'done' | 'error'
}
