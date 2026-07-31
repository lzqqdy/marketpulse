/** Chinese labels for event UI (codes remain in API). */

const SIGNAL_LABELS: Record<string, string> = {
  PRICE_DROP: '价格急跌',
  PRICE_SPIKE: '价格急涨',
  VOLUME_SPIKE: '成交量放大',
  VOLATILITY_SPIKE: '波动放大',
  LIQUIDATION_SPIKE: '爆仓放大',
}

const STATUS_LABELS: Record<string, string> = {
  DETECTED: '已发现',
  ACTIVE: '进行中',
  DEESCALATING: '缓和中',
  RESOLVED: '已结束',
}

const SEVERITY_LABELS: Record<string, string> = {
  NORMAL: '正常',
  LOW: '较低',
  MEDIUM: '中等',
  HIGH: '较高',
  EXTREME: '极端',
}

const SUBTYPE_LABELS: Record<string, string> = {
  FLASH_SELLOFF: '闪崩抛售',
  VOLUME_MOVE: '放量异动',
  DROP: '下跌异动',
  SPIKE: '上涨异动',
}

export function signalLabel(code: string): string {
  return SIGNAL_LABELS[code] ?? code
}

export function statusLabel(code: string): string {
  return STATUS_LABELS[String(code).toUpperCase()] ?? code
}

export function severityLabel(code: string): string {
  return SEVERITY_LABELS[String(code).toUpperCase()] ?? code
}

export function subtypeLabel(code: string): string {
  return SUBTYPE_LABELS[String(code).toUpperCase()] ?? code
}

export function formatSignalValue(s: {
  signalType: string
  changePct?: number
  ratio?: number
}): string {
  if (s.signalType === 'PRICE_DROP' || s.signalType === 'PRICE_SPIKE') {
    const v = s.changePct ?? 0
    const sign = v > 0 ? '+' : ''
    return `${sign}${v.toFixed(2)}%`
  }
  if (s.ratio && s.ratio > 0) {
    return `${s.ratio.toFixed(1)} 倍`
  }
  return ''
}

export function symbolDisplay(symbol: string): string {
  if (symbol === 'MARKET') return '全市场'
  return symbol
}
