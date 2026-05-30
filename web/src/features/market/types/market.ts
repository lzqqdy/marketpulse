/** 与 docs/RFC-002-api-contract.md 保持同步 */

export interface Quote {
  symbol: string
  priceUsdt: number
  priceCny: number
  changeDayPct: number
  change24hPct: number
  rank?: number
  iconUrl?: string
  marketCapUsd?: number
  volume24hUsd?: number
  updatedAt: string
}

export interface MarketSnapshot {
  version: number
  ts: number
  quotes: Quote[]
  rates: { usdtCny: number; usdCny: number; updatedAt: string }
  indices: IndexQuote[]
  alpha: AlphaSnapshot
  macro: MacroSnapshot
}

export interface IndexQuote {
  id: string
  name: string
  price: number
  changePct: number
  source?: string
  stale?: boolean
  fetchedAt?: string
  updatedAt: string
}

export interface AlphaQuote {
  id: string
  name: string
  symbol: string
  price: number
  change24hPct: number
  changeDayPct: number
  volume: number
  markPrice?: number
  indexPrice?: number
  fundingRate?: number
  updatedAt: string
  source: string
  category: 'index' | 'stock'
}

export interface AlphaSnapshot {
  indices: AlphaQuote[]
  stocks: AlphaQuote[]
  updatedAt?: string
  source?: string
}

export interface MacroSnapshot {
  totalMarketCapUsd: number
  totalVolume24hUsd: number
  totalMarketCapChange24hPct: number
  fearGreed: { value: number; label: string }
  btcDominancePct: number
  ethDominancePct: number
  stablecoinMarketCapUsd?: number
  stablecoinMarketCapChange24hPct?: number
  longShort?: {
    symbol: string
    ratio: number
    longAccountPct: number
    shortAccountPct: number
    updatedAt: string
  }
  topLongShort?: {
    symbol: string
    ratio: number
    longAccountPct: number
    shortAccountPct: number
    updatedAt: string
  }
  funding?: {
    symbol: string
    rate: number
    markPrice?: number
    indexPrice?: number
    premiumPct?: number
    nextFundingTime: string
    updatedAt: string
  }
  openInterest?: {
    symbol: string
    valueUsd: number
    changePct: number
    updatedAt: string
  }
  takerBuySell?: {
    symbol: string
    ratio: number
    buyVol: number
    sellVol: number
    updatedAt: string
  }
  liquidations?: {
    window: string
    longUsd: number
    shortUsd: number
    totalUsd: number
    updatedAt: string
  }
}
