import type { EventWsMessage } from './types'

const RECONNECT_BASE_MS = 1_000
const RECONNECT_MAX_MS = 15_000
const PING_INTERVAL_MS = 25_000

function wsBase(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}`
}

export interface EventsStreamHandlers {
  onMessage: (msg: EventWsMessage) => void
  onOpen?: () => void
  onClose?: () => void
}

/** Public event stream — no auth token. */
export function connectEventsStream(handlers: EventsStreamHandlers): () => void {
  let closed = false
  let ws: WebSocket | null = null
  let reconnectAttempt = 0
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  function clearTimers() {
    if (pingTimer) clearInterval(pingTimer)
    pingTimer = null
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = null
  }

  function scheduleReconnect() {
    if (closed) return
    const delay = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempt, RECONNECT_MAX_MS)
    reconnectAttempt += 1
    reconnectTimer = setTimeout(connect, delay)
  }

  function connect() {
    if (closed) return
    clearTimers()
    ws = new WebSocket(`${wsBase()}/ws/v1/events`)
    ws.onopen = () => {
      reconnectAttempt = 0
      handlers.onOpen?.()
      pingTimer = setInterval(() => {
        if (ws?.readyState === WebSocket.OPEN) {
          ws.send('ping')
        }
      }, PING_INTERVAL_MS)
    }
    ws.onmessage = (ev) => {
      let msg: EventWsMessage
      try {
        msg = JSON.parse(ev.data as string) as EventWsMessage
      } catch {
        return
      }
      handlers.onMessage(msg)
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
    ws?.close()
    ws = null
  }
}
