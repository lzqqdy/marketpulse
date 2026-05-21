import type { MarketSnapshot, Quote } from '@/types/market'

export interface SnapshotMessage {
  type: 'snapshot'
  data: MarketSnapshot
}

export interface QuotesMessage {
  type: 'quotes'
  version: number
  ts: number
  data: Quote[]
}

export interface RatesMessage {
  type: 'rates'
  version: number
  ts: number
  data: MarketSnapshot['rates']
}

export interface IndicesMessage {
  type: 'indices'
  version: number
  ts: number
  data: MarketSnapshot['indices']
}

export interface MacroMessage {
  type: 'macro'
  version: number
  ts: number
  data: MarketSnapshot['macro']
}

export interface AlphaMessage {
  type: 'alpha'
  version: number
  ts: number
  data: MarketSnapshot['alpha']
}

export type MarketWsMessage =
  | SnapshotMessage
  | QuotesMessage
  | RatesMessage
  | IndicesMessage
  | MacroMessage
  | AlphaMessage
  | { type: 'pong'; ts: number }

export interface MarketWsCallbacks {
  onSnapshot: (msg: SnapshotMessage) => void
  onQuotes: (msg: QuotesMessage) => void
  onRates?: (msg: RatesMessage) => void
  onIndices?: (msg: IndicesMessage) => void
  onMacro?: (msg: MacroMessage) => void
  onAlpha?: (msg: AlphaMessage) => void
  onOpen?: () => void
  onClose?: () => void
  onError?: () => void
}

function wsBase(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}`
}

/** 首页行情 WebSocket，无轮询 */
export function connectMarketStream(
  channels: string[],
  callbacks: MarketWsCallbacks,
): () => void {
  const url = `${wsBase()}/ws/v1/stream?channels=${encodeURIComponent(channels.join(','))}`
  const ws = new WebSocket(url)

  ws.onopen = () => callbacks.onOpen?.()

  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data as string) as MarketWsMessage
      if (msg.type === 'snapshot' && msg.data) callbacks.onSnapshot(msg)
      else if (msg.type === 'quotes') callbacks.onQuotes(msg)
      else if (msg.type === 'rates') callbacks.onRates?.(msg)
      else if (msg.type === 'indices') callbacks.onIndices?.(msg)
      else if (msg.type === 'macro') callbacks.onMacro?.(msg)
      else if (msg.type === 'alpha') callbacks.onAlpha?.(msg)
    } catch {
      callbacks.onError?.()
    }
  }

  ws.onerror = () => callbacks.onError?.()
  ws.onclose = () => callbacks.onClose?.()

  const pingTimer = setInterval(() => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ op: 'ping' }))
    }
  }, 25000)

  return () => {
    clearInterval(pingTimer)
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close()
    }
  }
}
