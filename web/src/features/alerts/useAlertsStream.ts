import type { AlertInboxItem, AlertWsMessage } from './types'

const RECONNECT_BASE_MS = 1_000
const RECONNECT_MAX_MS = 15_000
const PING_INTERVAL_MS = 25_000

function wsBase(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}`
}

export interface AlertsStreamHandlers {
  onAlert: (item: AlertInboxItem) => void
  onSnapshot?: (items: AlertInboxItem[]) => void
  onOpen?: () => void
  onClose?: () => void
}

/** 登录后连接站内告警 WS；返回 disconnect。 */
export function connectAlertsStream(token: string, handlers: AlertsStreamHandlers): () => void {
  let closed = false
  let ws: WebSocket | null = null
  let reconnectAttempt = 0
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  function clearTimers() {
    if (pingTimer) {
      clearInterval(pingTimer)
      pingTimer = null
    }
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  function scheduleReconnect() {
    if (closed) return
    const delay = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempt, RECONNECT_MAX_MS)
    reconnectAttempt += 1
    reconnectTimer = setTimeout(connect, delay)
  }

  function connect() {
    if (closed || !token) return
    clearTimers()
    const url = `${wsBase()}/ws/v1/alerts/stream?token=${encodeURIComponent(token)}`
    ws = new WebSocket(url)

    ws.onopen = () => {
      reconnectAttempt = 0
      handlers.onOpen?.()
      pingTimer = setInterval(() => {
        if (ws?.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ping' }))
        }
      }, PING_INTERVAL_MS)
    }

    ws.onmessage = (ev) => {
      let msg: AlertWsMessage
      try {
        msg = JSON.parse(ev.data as string) as AlertWsMessage
      } catch {
        return
      }
      if (msg.type === 'inbox_snapshot') {
        handlers.onSnapshot?.(msg.data.items ?? [])
        for (const item of msg.data.items ?? []) {
          handlers.onAlert(item)
        }
      } else if (msg.type === 'alert') {
        handlers.onAlert(msg.data)
      }
    }

    ws.onclose = () => {
      clearTimers()
      handlers.onClose?.()
      scheduleReconnect()
    }

    ws.onerror = () => {
      ws?.close()
    }
  }

  connect()

  return () => {
    closed = true
    clearTimers()
    if (ws) {
      ws.onclose = null
      ws.close()
      ws = null
    }
  }
}

export function sendAck(ws: WebSocket | null, deliveryIds: number[]) {
  if (!ws || ws.readyState !== WebSocket.OPEN || !deliveryIds.length) return
  ws.send(JSON.stringify({ type: 'ack', deliveryIds }))
}
