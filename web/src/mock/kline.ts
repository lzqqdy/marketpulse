import type { Candle, KlineInterval } from '@/types/chart'

/** 后端不可用时的演示 K 线 */
export function generateMockKlines(symbol: string, interval: KlineInterval, count = 200): Candle[] {
  const intervalSec: Record<KlineInterval, number> = {
    '1m': 60,
    '5m': 300,
    '15m': 900,
    '1h': 3600,
    '4h': 14400,
    '1d': 86400,
    '1w': 604800,
  }
  const step = intervalSec[interval]
  const seed = symbol.split('').reduce((a, c) => a + c.charCodeAt(0), 0)
  let price = 1000 + (seed % 50000)
  const now = Math.floor(Date.now() / 1000)
  const start = now - step * count
  const out: Candle[] = []

  for (let i = 0; i < count; i++) {
    const t = start + i * step
    const drift = (Math.sin(i / 12 + seed) + (Math.random() - 0.5) * 0.8) * price * 0.008
    const open = price
    const close = Math.max(0.01, price + drift)
    const high = Math.max(open, close) * (1 + Math.random() * 0.004)
    const low = Math.min(open, close) * (1 - Math.random() * 0.004)
    const volume = Math.abs(drift) * 800 + Math.random() * 50000
    out.push({
      time: t,
      open,
      high,
      low,
      close,
      volume,
    })
    price = close
  }
  return out
}
