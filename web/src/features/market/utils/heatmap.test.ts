import { computed, ref } from 'vue'
import { describe, expect, it } from 'vitest'
import type { HeatmapItem } from '../types/marketCenter'
import { sortHeatmapByChange } from './heatmap'

function item(code: string, pxChangeRate: number): HeatmapItem {
  return { code, name: code, market: 'ab', metricValue: '100', pxChangeRate }
}

const items = [item('a', -2), item('b', 4), item('c', 0), item('d', -5), item('e', 4)]
const codes = (rows: HeatmapItem[]) => rows.map((row) => row.code)

describe('heatmap change order', () => {
  it('keeps provider order by default and never mutates the source array', () => {
    const original = Object.freeze([...items])
    const result = sortHeatmapByChange(original, 'default')
    expect(result).not.toBe(original)
    expect(result).toEqual(items)
    sortHeatmapByChange(original, 'desc')
    expect(original).toEqual(items)
    expect(sortHeatmapByChange(original, 'default')).toEqual(items)
  })

  it('sorts numerically in both directions and keeps ties in provider order', () => {
    expect(codes(sortHeatmapByChange(items, 'desc'))).toEqual(['b', 'e', 'c', 'a', 'd'])
    expect(codes(sortHeatmapByChange(items, 'asc'))).toEqual(['d', 'a', 'c', 'b', 'e'])
  })

  it('places invalid change rates last in both directions', () => {
    const rows = [item('missing', NaN), item('up', 2), item('invalid', Infinity), item('down', -1)]
    expect(codes(sortHeatmapByChange(rows, 'desc'))).toEqual(['up', 'down', 'missing', 'invalid'])
    expect(codes(sortHeatmapByChange(rows, 'asc'))).toEqual(['down', 'up', 'missing', 'invalid'])
    expect(sortHeatmapByChange([], 'desc')).toEqual([])
  })

  it('sorts the full list before collapsing it to nine tiles', () => {
    const rows = Array.from({ length: 12 }, (_, i) => item(String(i), i))
    const visible = sortHeatmapByChange(rows, 'desc').slice(0, 9)
    expect(codes(visible)).toEqual(['11', '10', '9', '8', '7', '6', '5', '4', '3'])
  })

  it('recalculates the selected order when data refreshes', () => {
    const source = ref([...items])
    const sorted = computed(() => sortHeatmapByChange(source.value, 'desc'))
    expect(sorted.value[0].code).toBe('b')
    source.value = [item('new', 9), ...items]
    expect(sorted.value[0].code).toBe('new')
    source.value[1].pxChangeRate = 20
    expect(sorted.value[0].code).toBe('a')
  })
})
