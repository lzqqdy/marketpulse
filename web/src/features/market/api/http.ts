import type { MarketSnapshot } from '@/features/market/types/market'

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
