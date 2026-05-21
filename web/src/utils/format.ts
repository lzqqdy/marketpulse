export function formatNumber(value: number, precision = 2): string {
  if (!Number.isFinite(value)) return '--'
  const abs = Math.abs(value)
  if (abs >= 1e12) return `${(value / 1e12).toFixed(2)}万亿`
  if (abs >= 1e8) return `${(value / 1e8).toFixed(2)}亿`
  if (abs >= 1e4) return value.toLocaleString('en-US', {
    minimumFractionDigits: precision,
    maximumFractionDigits: precision,
  })
  if (abs >= 1) return value.toFixed(precision)
  return value.toFixed(Math.max(precision, 4))
}

export function formatPriceUsdt(value: number): string {
  if (value >= 1000) return formatNumber(value, 2)
  if (value >= 1) return value.toFixed(2)
  return value.toFixed(4)
}

export function formatPct(value: number, signed = true): string {
  const n = Math.abs(value).toFixed(2)
  if (!signed) return `${n}%`
  if (value > 0) return `+${n}%`
  if (value < 0) return `-${n}%`
  return `${n}%`
}

export function formatRank(rank?: number): string {
  if (!rank) return '--'
  return rank < 10 ? `0${rank}` : String(rank)
}

export function formatTime(ts: number | string): string {
  const d = typeof ts === 'number' ? new Date(ts) : new Date(ts)
  if (Number.isNaN(d.getTime())) return '--'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
