import type { MarketSnapshot, Quote } from '@/features/market/types/market'

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

const PING_INTERVAL_MS = 25_000
const WATCHDOG_INTERVAL_MS = 10_000
const STALE_CONNECTION_MS = 60_000
const RECONNECT_BASE_MS = 1_000
const RECONNECT_MAX_MS = 15_000

function wsBase(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}`
}

/** 首页行情 WebSocket，无轮询 */
export function connectMarketStream(
  channels: string[],
  callbacks: MarketWsCallbacks,
): () => void {
  const url = `${wsBase()}/ws/v1/market/stream?channels=${encodeURIComponent(channels.join(','))}`
  let ws: WebSocket | null = null
  let stopped = false
  let reconnectAttempts = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let watchdogTimer: ReturnType<typeof setInterval> | null = null
  let lastMessageAt = Date.now()

  function clearConnectionTimers() {
    if (pingTimer) {
      clearInterval(pingTimer)
      pingTimer = null
    }
    if (watchdogTimer) {
      clearInterval(watchdogTimer)
      watchdogTimer = null
    }
  }

  function scheduleReconnect() {
    if (stopped || reconnectTimer) return
    const delay = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempts, RECONNECT_MAX_MS)
    reconnectAttempts += 1
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, delay)
  }

  function closeStaleConnection() {
    const current = ws
    if (!current || current.readyState === WebSocket.CLOSED) return
    current.close()
  }

  function connect() {
    clearConnectionTimers()
    lastMessageAt = Date.now()
    ws = new WebSocket(url)

    ws.onopen = () => {
      reconnectAttempts = 0
      lastMessageAt = Date.now()
      callbacks.onOpen?.()
    }

    ws.onmessage = (ev) => {
      lastMessageAt = Date.now()
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

    ws.onerror = () => {
      if (!stopped) callbacks.onError?.()
    }
    ws.onclose = () => {
      clearConnectionTimers()
      if (stopped) return
      callbacks.onClose?.()
      scheduleReconnect()
    }

    pingTimer = setInterval(() => {
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ op: 'ping' }))
      }
    }, PING_INTERVAL_MS)

    watchdogTimer = setInterval(() => {
      if (Date.now() - lastMessageAt > STALE_CONNECTION_MS) {
        closeStaleConnection()
      }
    }, WATCHDOG_INTERVAL_MS)
  }

  connect()

  return () => {
    stopped = true
    clearConnectionTimers()
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (ws) {
      ws.onopen = null
      ws.onmessage = null
      ws.onerror = null
      ws.onclose = null
    }
    if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) {
      ws.close()
    }
    ws = null
  }
}
