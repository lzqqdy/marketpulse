import type {
  MarketBreadth,
  MarketInternals,
  MarketSnapshot,
  MarketWind,
  SectorQuote,
} from '@/features/market/types/market'

export interface HealthzResponse {
  status: string
  uptimeSec: number
  symbolCount: number
  storeVersion: number
  appMode: string
  ingest: Record<string, string>
}

export async function fetchSnapshot(): Promise<MarketSnapshot> {
  const res = await fetch('/api/v1/market/snapshot')
  if (!res.ok) {
    throw new Error(`snapshot HTTP ${res.status}`)
  }
  return res.json() as Promise<MarketSnapshot>
}

export async function fetchHealthz(): Promise<HealthzResponse> {
  const res = await fetch('/healthz')
  if (!res.ok) {
    throw new Error(`healthz HTTP ${res.status}`)
  }
  return res.json() as Promise<HealthzResponse>
}

export async function fetchInternals(market = 'cn'): Promise<MarketInternals> {
  const res = await fetch(`/api/v1/market/internals?market=${encodeURIComponent(market)}`)
  if (!res.ok) throw new Error(`internals HTTP ${res.status}`)
  return res.json() as Promise<MarketInternals>
}

export async function fetchBreadth(market = 'cn'): Promise<MarketBreadth> {
  const res = await fetch(`/api/v1/market/breadth?market=${encodeURIComponent(market)}`)
  if (!res.ok) throw new Error(`breadth HTTP ${res.status}`)
  return res.json() as Promise<MarketBreadth>
}

export async function fetchSectors(type: 'industry' | 'concept', market = 'cn'): Promise<{ type: string; sectors: SectorQuote[]; updatedAt: string }> {
  const q = new URLSearchParams({ market, type })
  const res = await fetch(`/api/v1/market/sectors?${q}`)
  if (!res.ok) throw new Error(`sectors HTTP ${res.status}`)
  return res.json() as Promise<{ type: string; sectors: SectorQuote[]; updatedAt: string }>
}

export async function fetchMarketWind(market = 'cn'): Promise<MarketWind> {
  const res = await fetch(`/api/v1/market/wind?market=${encodeURIComponent(market)}`)
  if (!res.ok) throw new Error(`wind HTTP ${res.status}`)
  return res.json() as Promise<MarketWind>
}
