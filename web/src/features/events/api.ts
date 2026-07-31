import type { EventListResponse, EventTimelineResponse, MarketEventDetail } from './types'

export async function fetchEvents(params: {
  limit?: number
  cursor?: string
  status?: string
  severity?: string
  symbol?: string
  market?: string
} = {}): Promise<EventListResponse> {
  const q = new URLSearchParams()
  if (params.limit) q.set('limit', String(params.limit))
  if (params.cursor) q.set('cursor', params.cursor)
  if (params.status) q.set('status', params.status)
  if (params.severity) q.set('severity', params.severity)
  if (params.symbol) q.set('symbol', params.symbol)
  if (params.market) q.set('market', params.market)
  const res = await fetch(`/api/v1/events?${q}`)
  if (res.status === 503) {
    const body = await res.json().catch(() => null)
    const err = new Error(body?.error?.message || 'event disabled') as Error & { code?: string }
    err.code = body?.error?.code || 'event_disabled'
    throw err
  }
  if (!res.ok) {
    throw new Error(`events HTTP ${res.status}`)
  }
  return res.json() as Promise<EventListResponse>
}

export async function fetchEvent(id: string): Promise<MarketEventDetail> {
  const res = await fetch(`/api/v1/events/${encodeURIComponent(id)}`)
  if (res.status === 404) {
    throw new Error('event_not_found')
  }
  if (!res.ok) {
    throw new Error(`event HTTP ${res.status}`)
  }
  return res.json() as Promise<MarketEventDetail>
}

export async function fetchEventTimeline(id: string): Promise<EventTimelineResponse> {
  const res = await fetch(`/api/v1/events/${encodeURIComponent(id)}/timeline`)
  if (!res.ok) {
    throw new Error(`timeline HTTP ${res.status}`)
  }
  return res.json() as Promise<EventTimelineResponse>
}
