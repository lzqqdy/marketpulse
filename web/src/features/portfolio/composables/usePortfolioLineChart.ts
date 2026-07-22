import { onUnmounted, shallowRef, watch, type Ref } from 'vue'
import {
  createChart,
  ColorType,
  CrosshairMode,
  type IChartApi,
  type ISeriesApi,
  type LineData,
  type HistogramData,
  type AreaData,
  type Time,
} from 'lightweight-charts'

export type PortfolioChartKind = 'area' | 'line' | 'histogram'

export interface PortfolioChartPoint {
  date: string
  value: number
}

function cssVar(name: string, fallback: string) {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value || fallback
}

function toTime(date: string): Time {
  return date as Time
}

export function usePortfolioLineChart(
  container: Ref<HTMLElement | null>,
  points: Ref<PortfolioChartPoint[]>,
  opts: {
    kind: PortfolioChartKind
    /** For histogram: color by sign using --up/--down */
    signedBars?: boolean
    lineColor?: string
    colorToken?: '--accent' | '--chart-2' | '--chart-3' | '--coin' | '--warning'
    areaTop?: string
    areaBottom?: string
  },
) {
  const chartRef = shallowRef<IChartApi | null>(null)
  let series: ISeriesApi<'Area'> | ISeriesApi<'Line'> | ISeriesApi<'Histogram'> | null = null
  let ro: ResizeObserver | null = null

  function destroy() {
    ro?.disconnect()
    ro = null
    chartRef.value?.remove()
    chartRef.value = null
    series = null
  }

  function applyData() {
    if (!series) return
    if (opts.kind === 'histogram' && opts.signedBars) {
      const up = cssVar('--up', '#f6465d')
      const down = cssVar('--down', '#0ecb81')
      const data: HistogramData[] = points.value.map((p) => ({
        time: toTime(p.date),
        value: p.value,
        color: p.value >= 0 ? up : down,
      }))
      ;(series as ISeriesApi<'Histogram'>).setData(data)
    } else if (opts.kind === 'area') {
      const data: AreaData[] = points.value.map((p) => ({
        time: toTime(p.date),
        value: p.value,
      }))
      ;(series as ISeriesApi<'Area'>).setData(data)
    } else {
      const data: LineData[] = points.value.map((p) => ({
        time: toTime(p.date),
        value: p.value,
      }))
      ;(series as ISeriesApi<'Line'>).setData(data)
    }
    chartRef.value?.timeScale().fitContent()
  }

  function build() {
    const el = container.value
    if (!el) return
    destroy()

    const bg = cssVar('--panel', '#0b0e11')
    const text = cssVar('--muted', '#848e9c')
    const line = cssVar('--line', '#2b3139')
    const muted = cssVar('--muted-2', '#5f5f5f')
    const token = opts.colorToken || '--accent'
    const accent = opts.lineColor || cssVar(token, cssVar('--accent', '#dbaa6a'))
    const up = cssVar('--up', '#f6465d')

    const chart = createChart(el, {
      layout: {
        background: { type: ColorType.Solid, color: bg },
        textColor: text,
        fontFamily: "'PingFang SC', 'Microsoft YaHei', sans-serif",
      },
      grid: {
        vertLines: { color: line },
        horzLines: { color: line },
      },
      crosshair: {
        mode: CrosshairMode.Normal,
        vertLine: { color: muted, labelBackgroundColor: line },
        horzLine: { color: muted, labelBackgroundColor: line },
      },
      rightPriceScale: { borderColor: line },
      timeScale: {
        borderColor: line,
        timeVisible: false,
      },
      width: el.clientWidth,
      height: el.clientHeight || 240,
    })

    if (opts.kind === 'area') {
      series = chart.addAreaSeries({
        lineColor: accent,
        topColor: opts.areaTop || `${accent}59`,
        bottomColor: opts.areaBottom || `${accent}05`,
        lineWidth: 2,
        priceLineVisible: false,
      })
    } else if (opts.kind === 'histogram') {
      series = chart.addHistogramSeries({
        color: up,
        priceLineVisible: false,
        base: 0,
      })
    } else {
      series = chart.addLineSeries({
        color: accent,
        lineWidth: 2,
        priceLineVisible: false,
      })
    }

    chartRef.value = chart
    applyData()

    ro = new ResizeObserver(() => {
      if (!container.value || !chartRef.value) return
      chartRef.value.applyOptions({
        width: container.value.clientWidth,
        height: container.value.clientHeight || 240,
      })
    })
    ro.observe(el)
  }

  watch(
    container,
    (el) => {
      if (el) build()
      else destroy()
    },
    { immediate: true },
  )

  watch(points, () => applyData(), { deep: true })

  onUnmounted(destroy)

  return { rebuild: build }
}
