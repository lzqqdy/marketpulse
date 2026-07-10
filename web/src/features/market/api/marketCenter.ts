import type { Heatmap, HeatmapSortKey, MarketCenterResponse, MarketCode } from '@/features/market/types/marketCenter'

export async function fetchMarketCenter(market: MarketCode): Promise<MarketCenterResponse> {
  const q = new URLSearchParams({ market })
  const res = await fetch(`/api/v1/market/center?${q}`)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.error?.message ?? `market center HTTP ${res.status}`)
  }
  return res.json()
}

export async function fetchMarketCenterHeatmap(
  market: MarketCode,
  sortKey: HeatmapSortKey,
): Promise<Heatmap> {
  const q = new URLSearchParams({ market, sortKey })
  const res = await fetch(`/api/v1/market/center/heatmap?${q}`)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.error?.message ?? `heatmap HTTP ${res.status}`)
  }
  return res.json()
}
