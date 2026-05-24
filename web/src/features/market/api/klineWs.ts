import type { Candle, KlineInterval } from '@/features/market/types/chart'

export interface KlineSnapshotMessage {
  type: 'kline_snapshot'
  symbol: string
  interval: string
  candles: Candle[]
  source: string
}

export interface KlineUpdateMessage {
  type: 'kline_update'
  symbol: string
  interval: string
  candle: Candle
}

export interface KlineErrorMessage {
  type: 'error'
  code: string
  message: string
}

export type KlineWsMessage = KlineSnapshotMessage | KlineUpdateMessage | KlineErrorMessage

export interface KlineWsCallbacks {
  onSnapshot: (msg: KlineSnapshotMessage) => void
  onUpdate: (msg: KlineUpdateMessage) => void
  onError: (msg: KlineErrorMessage) => void
  onOpen?: () => void
  onClose?: () => void
}

function wsBase(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}`
}

/** 连接 K 线 WebSocket；返回 disconnect 函数。无轮询。 */
export function connectKlineWs(
  symbol: string,
  interval: KlineInterval,
  callbacks: KlineWsCallbacks,
): () => void {
  const url = `${wsBase()}/ws/v1/market/kline?symbol=${encodeURIComponent(symbol)}&interval=${encodeURIComponent(interval)}`
  const ws = new WebSocket(url)

  ws.onopen = () => callbacks.onOpen?.()

  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data as string) as KlineWsMessage
      if (msg.type === 'kline_snapshot') callbacks.onSnapshot(msg)
      else if (msg.type === 'kline_update') callbacks.onUpdate(msg)
      else if (msg.type === 'error') callbacks.onError(msg)
    } catch {
      callbacks.onError({ type: 'error', code: 'PARSE_ERROR', message: 'invalid message' })
    }
  }

  ws.onerror = () => {
    callbacks.onError({ type: 'error', code: 'WS_ERROR', message: 'websocket error' })
  }

  ws.onclose = () => callbacks.onClose?.()

  return () => {
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close()
    }
  }
}
