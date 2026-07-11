import type { ExpressNewsResponse, ExpressNewsTag } from '@/features/market/types/expressNews'

export interface FetchExpressNewsParams {
  tag?: ExpressNewsTag
  pn?: number
  rn?: number
}

export async function fetchExpressNews(
  params: FetchExpressNewsParams = {},
): Promise<ExpressNewsResponse> {
  const q = new URLSearchParams()
  if (params.tag) q.set('tag', params.tag)
  q.set('pn', String(params.pn ?? 0))
  q.set('rn', String(params.rn ?? 20))

  const res = await fetch(`/api/v1/market/expressnews?${q}`)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.error?.message ?? `express news HTTP ${res.status}`)
  }
  return res.json()
}
