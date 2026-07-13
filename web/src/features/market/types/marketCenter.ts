export type MarketCode = 'ab' | 'hk' | 'us'

export interface ChgDiagramBar {
  title: string
  status: string
  count: number
}

export interface ChgDiagram {
  totalTitle?: string
  totalValue?: string
  up: number
  down: number
  balance: number
  bars: ChgDiagramBar[]
}

export interface HeatmapItem {
  code: string
  name: string
  market: string
  amount?: string
  volume?: string
  marketValue?: string
  lastPx?: string
  pxChangeRate: number
  metricValue: string
  logo?: string
}

export interface Heatmap {
  sortKey: string
  typeCode: string
  items: HeatmapItem[]
}

export interface FundflowItem {
  code: string
  name: string
  mainNetTurnover: string
  netAmount: number
}

export interface FundflowGroup {
  blockType: string
  blockTypeName: string
  items: FundflowItem[]
}

export interface Fundflow {
  groups: FundflowGroup[]
}

export interface OverviewItem {
  code: string
  name: string
  price: number
  changePct: number
  changeStatus: string
  leadName?: string
  leadChangePct?: number
  trend?: number[]
}

export interface OverviewTab {
  type: string
  name: string
  items: OverviewItem[]
}

export interface Overview {
  tabs: OverviewTab[]
}

export interface MarketCenterResponse {
  market: MarketCode
  source: string
  fetchedAt: number
  marketActive?: boolean
  chgdiagram: ChgDiagram
  heatmap: Heatmap
  fundflow: Fundflow
  overview: Overview
}

export type HeatmapSortKey = 'amount' | 'volume' | 'marketValue'

export const MARKET_TABS: { value: MarketCode; label: string }[] = [
  { value: 'ab', label: 'A股' },
  { value: 'hk', label: '港股' },
  { value: 'us', label: '美股' },
]

export const HEATMAP_SORT_OPTIONS: { value: HeatmapSortKey; label: string }[] = [
  { value: 'amount', label: '成交额' },
  { value: 'volume', label: '成交量' },
  { value: 'marketValue', label: '市值' },
]
