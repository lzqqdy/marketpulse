import { KLINE_FETCH_LIMIT, type KlineInterval, type KlineResponse } from '@/features/market/types/chart'
import { generateMockKlines } from '@/features/market/mock/kline'

export async function fetchKlines(
  symbol: string,
  interval: KlineInterval,
  limit = KLINE_FETCH_LIMIT,
): Promise<KlineResponse> {
  const q = new URLSearchParams({ symbol, interval, limit: String(limit) })
  try {
    const res = await fetch(`/api/v1/market/klines?${q}`)
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    return (await res.json()) as KlineResponse
  } catch {
    return {
      symbol,
      pair: `${symbol}USDT`,
      interval,
      candles: generateMockKlines(symbol, interval, limit),
      source: 'mock',
    }
  }
}

export async function fetchIndexKlines(
  id: string,
  interval: KlineInterval,
  limit = KLINE_FETCH_LIMIT,
): Promise<KlineResponse> {
  const q = new URLSearchParams({ id, interval, limit: String(limit) })
  const res = await fetch(`/api/v1/market/index-klines?${q}`)
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}`)
  }
  return (await res.json()) as KlineResponse
}
