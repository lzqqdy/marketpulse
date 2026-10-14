import type { AiEvidence } from '../types'

export function currentEvidence(record: AiEvidence, now = Date.now()): AiEvidence {
  const expires = Date.parse(record.expiresAt)
  return record.status !== 'unavailable' && (!Number.isFinite(expires) || expires <= now)
    ? { ...record, status: 'stale' } : record
}

export function safeEvidenceUrl(value?: string): string | undefined {
  if (!value) return undefined
  try {
    const url = new URL(value)
    if ((url.protocol === 'http:' || url.protocol === 'https:') && !url.username && !url.password) return url.href
  } catch { /* Invalid source URLs are not links. */ }
  return undefined
}

export function evidenceTime(value?: string): string {
  const time = value ? Date.parse(value) : NaN
  if (!Number.isFinite(time) || time <= 0) return '时点未知'
  return new Date(time).toLocaleString('zh-CN', { hour12: false })
}

export const TIME_BASIS: Record<string, string> = {
  data_time: '数据时点', publication_time: '新闻发布时间', candle_open: 'K线开盘时点',
  collection_time: '采集时点（非逐项数据时间）', unknown: '时点未知',
}
export const CHANGE_BASIS: Record<string, string> = {
  trading_session: '交易日涨跌', rolling_24h: '滚动24小时', candle_range: 'K线区间', portfolio_valuation: '资产估值与本金/快照之差',
}
export const KIND_LABELS: Record<string, string> = {
  quote: '报价', klines: 'K线', news: '新闻', get_portfolio_overview: '本人资产',
  get_market_events: '市场事件', market_event: '市场事件', get_snapshot_summary: '盘面摘要',
  get_macro_metrics: '币圈指标', get_market_breadth: '行情中心',
}
export const FACT_LABELS: Record<string, string> = {
  price: '价格', priceUsdt: '价格（USDT）', changePct: '涨跌幅（%）', change24hPct: '24h涨跌（%）',
  volume: '成交量', markPrice: '标记价格', indexPrice: '指数价格', fundingRate: '资金费率',
  interval: '周期', pair: '交易对', candleCount: 'K线根数', open: '开盘', close: '收盘', high: '最高', low: '最低',
  rangeChangePct: '区间涨跌（%）', sma7: 'SMA7', sma30: 'SMA30', lastCandleClosed: '末根已收盘',
  title: '标题', body: '内容', publishTime: '发布时间', matchedBy: '匹配依据', sessionNote: '交易时段',
  overview: '资产概览', holdings: '持仓估值', holdingCount: '持仓数量', omittedHoldings: '未展示持仓数量',
  severity: '严重程度', eventStatus: '事件状态', symbols: '标的', startTime: '事件开始', signals: '检测信号',
}

export function factValue(value: unknown): string {
  if (value === null || value === undefined) return '暂无数据'
  if (typeof value === 'number') return Number.isFinite(value) ? value.toLocaleString('zh-CN', { maximumFractionDigits: 8 }) : '无效数值'
  if (typeof value === 'boolean') return value ? '是' : '否'
  const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2)
  return text.length > 8000 ? `${text.slice(0, 8000)}…（显示已截断）` : text
}
