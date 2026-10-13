import type { HeatmapItem } from '../types/marketCenter'

export type HeatmapChangeOrder = 'default' | 'desc' | 'asc'

export function sortHeatmapByChange(
  items: readonly HeatmapItem[],
  order: HeatmapChangeOrder,
): HeatmapItem[] {
  const sorted = [...items]
  if (order === 'default') return sorted
  const direction = order === 'asc' ? 1 : -1
  return sorted.sort((a, b) => {
    const aValid = Number.isFinite(a.pxChangeRate)
    const bValid = Number.isFinite(b.pxChangeRate)
    if (!aValid || !bValid) return Number(bValid) - Number(aValid)
    return direction * (a.pxChangeRate - b.pxChangeRate)
  })
}
