export type PortfolioAssetType = 'crypto' | 'alpha'

export interface HoldingView {
  assetType: PortfolioAssetType
  symbol: string
  quantity: number
  priceUsdt: number
  valueUsdt: number
  valueCny: number
  changeCny: number
  missing?: boolean
}

export interface HoldingsResult {
  holdings: HoldingView[]
  principalCny: number
  usdtCny: number
  usdtPremiumPct: number
  rateFallback?: boolean
  missingSymbols?: string[]
}

export interface HoldingInput {
  assetType: PortfolioAssetType
  symbol: string
  quantity: number
}

export interface PortfolioSettings {
  principalCny: number
}

export interface PnLWindow {
  pnlCny: number
  pnlPct: number | null
}

export interface PortfolioOverview {
  totalUsdt: number
  totalCny: number
  usdtCny: number
  usdtPremiumPct: number
  rateFallback?: boolean
  today: PnLWindow | null
  d7: PnLWindow | null
  d30: PnLWindow | null
  allTime: PnLWindow | null
  missingSymbols: string[]
}

export interface PortfolioSnapshot {
  date: string
  totalValue: number
  totalValueCny: number
  dailyProfit: number
  dailyProfitRate: number
  totalProfit: number
  totalProfitRate: number
}

export interface SnapshotsResult {
  total: number
  page: number
  pageSize: number
  items: PortfolioSnapshot[]
}

export interface EligibleSymbol {
  symbol: string
  name?: string
  assetType: PortfolioAssetType
}

export interface EligibleSymbolsResult {
  crypto: EligibleSymbol[]
  alpha: EligibleSymbol[]
}

export interface ListSnapshotsQuery {
  page?: number
  pageSize?: number
  from?: string
  to?: string
  sort?: string
  order?: 'asc' | 'desc'
}

export type ReportRange = '7d' | '30d' | '90d' | '180d' | '1y' | 'all'

export interface ReportSeriesPoint {
  date: string
  totalValue: number
  totalValueCny: number
  dailyProfit: number
  dailyProfitRate: number
  totalProfit: number
  totalProfitRate: number
}

export interface ReportSeriesSummary {
  startCny: number
  endCny: number
  pnlCny: number
  pnlPct: number | null
}

export interface ReportSeriesResult {
  range: ReportRange
  from: string
  to: string
  summary: ReportSeriesSummary
  points: ReportSeriesPoint[]
}

export interface AllocationItem {
  assetType: PortfolioAssetType
  symbol: string
  valueCny: number
  valueUsdt: number
  weightPct: number
}

export interface AllocationResult {
  totalCny: number
  totalUsdt: number
  items: AllocationItem[]
  missingSymbols: string[]
  rateFallback?: boolean
}
