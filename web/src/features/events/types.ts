export type EventStatus = 'DETECTED' | 'ACTIVE' | 'DEESCALATING' | 'RESOLVED'
export type EventSeverity = 'NORMAL' | 'LOW' | 'MEDIUM' | 'HIGH' | 'EXTREME'

export interface MarketEventSummary {
  id: string
  type: string
  subType: string
  title: string
  description?: string
  severity: EventSeverity | string
  score: number
  peakScore: number
  status: EventStatus | string
  symbols: string[]
  markets: string[]
  startTime: number
  endTime: number | null
  createdAt: number
  updatedAt: number
}

export interface EventSignal {
  id: string
  signalType: string
  symbol: string
  market: string
  value: number
  baseline: number
  ratio: number
  changePct: number
  window: string
  direction: string
  timestamp: number
  metadata?: Record<string, unknown>
}

export interface MarketEventDetail extends MarketEventSummary {
  signals: EventSignal[]
  context?: Record<string, unknown>
}

export interface EventListResponse {
  items: MarketEventSummary[]
  nextCursor?: string
  limit: number
}

export interface EventTimelineItem {
  ts: number
  kind: 'signal' | 'status' | string
  label: string
  signalType?: string
  symbol?: string
  status?: string
  payload?: Record<string, unknown>
}

export interface EventTimelineResponse {
  eventId: string
  items: EventTimelineItem[]
}

export interface EventWsPayload {
  id: string
  type?: string
  subType?: string
  title?: string
  severity?: string
  score?: number
  status?: string
  symbols?: string[]
  markets?: string[]
  startTime?: number
  endTime?: number
  updatedAt?: number
}

export type EventWsMessage =
  | { type: 'event.created'; event: EventWsPayload }
  | { type: 'event.updated'; event: EventWsPayload }
  | { type: 'event.resolved'; event: EventWsPayload }
  | { type: 'pong' }
