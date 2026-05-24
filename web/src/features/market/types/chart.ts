export interface Candle {
  time: number
  open: number
  high: number
  low: number
  close: number
  volume: number
}

export interface KlineResponse {
  symbol: string
  pair: string
  interval: string
  candles: Candle[]
  source: string
}

export type KlineInterval = '1m' | '5m' | '15m' | '1h' | '4h' | '1d' | '1w'

/** Bars fetched from API / WS (full history for pan/zoom). */
export const KLINE_FETCH_LIMIT = 300

/** Bars shown on first open before user pans or zooms. */
export const KLINE_INITIAL_VISIBLE_BARS = 60

export const KLINE_INTERVALS: { value: KlineInterval; label: string }[] = [
  { value: '1m', label: '1分' },
  { value: '15m', label: '15分' },
  { value: '1h', label: '1时' },
  { value: '4h', label: '4时' },
  { value: '1d', label: '日线' },
  { value: '1w', label: '周线' },
]
