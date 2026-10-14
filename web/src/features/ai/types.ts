export type AiAssetClass = 'crypto' | 'index' | 'us_equity' | 'us_contract' | 'alpha'
export type ToolResultStatus = 'ok' | 'partial' | 'unavailable' | 'stale'

export interface AiEvidence {
  id: string
  kind: string
  instrumentId?: string
  symbol?: string
  assetClass?: AiAssetClass
  instrumentType?: string
  source?: string
  asOf: string
  asOfBasis?: 'data_time' | 'publication_time' | 'candle_open' | 'collection_time' | 'unknown'
  fetchedAt: string
  expiresAt: string
  status: ToolResultStatus
  changeBasis?: string
  currency?: string
  facts: Record<string, unknown>
  url?: string
  limitations?: string[]
}

export interface AiToolResult {
  name: string
  toolCallId?: string
  arguments?: string
  ok: boolean
  status?: ToolResultStatus
  summary?: string
  evidence?: AiEvidence[]
  durationMs?: number
}

export interface PageContext {
  focusSymbol?: string
  assetClass?: AiAssetClass
  page?: string
  visibleSymbols?: string[]
  fearGreedValue?: number
  fearGreedLabel?: string
  btcDominancePct?: number
  usdtCny?: number
}

export interface ChatRequest {
  conversationId?: string
  message: string
  context?: PageContext
  templateId?: AnalysisTemplateId
}

export type AnalysisTemplateId = 'market_brief' | 'symbol_review' | 'comparison' | 'portfolio_review' | 'event_review'

export type AiStreamEvent =
  | { event: 'meta'; data: { conversationId: string; messageId?: number } }
  | { event: 'token'; data: { text: string } }
  | { event: 'tool_start'; data: { name: string; arguments?: string; toolCallId?: string } }
  | { event: 'tool_result'; data: AiToolResult }
  | { event: 'done'; data: { finishReason?: string; conversationId?: string } }
  | { event: 'error'; data: { code?: string; message?: string } }

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  pending?: boolean
  evidence?: AiEvidence[]
  toolResults?: AiToolResult[]
  toolSteps?: ToolStep[]
  interrupted?: boolean
}

export interface ToolStep extends ToolStatus {
  id: string
  durationMs?: number
  note?: string
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
  phase: 'running' | 'done' | 'error' | 'partial' | 'stale' | 'interrupted'
}
