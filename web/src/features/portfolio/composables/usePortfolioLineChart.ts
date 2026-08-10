import { onUnmounted, shallowRef, ref, watch, type Ref } from 'vue'
import {
  createChart,
  ColorType,
  CrosshairMode,
  LineType,
  type IChartApi,
  type ISeriesApi,
  type LineData,
  type HistogramData,
  type AreaData,
  type Time,
  type MouseEventParams,
  type SeriesMarker,
  type LogicalRange,
} from 'lightweight-charts'

export type PortfolioChartKind = 'area' | 'line' | 'histogram'
export type PortfolioValueKind = 'money' | 'percent' | 'raw'

export interface PortfolioChartPoint {
  date: string
  value: number
}

export interface PortfolioChartHover {
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

function formatTickDate(time: Time): string {
  const raw = String(time)
  // YYYY-MM-DD
  if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) {
    return raw.slice(5) // MM-DD
  }
  return raw
}

function priceFormatFor(kind: PortfolioValueKind) {
  if (kind === 'percent') {
    return {
      type: 'custom' as const,
      formatter: (price: number) => `${price.toFixed(2)}%`,
      minMove: 0.01,
    }
  }
  if (kind === 'money') {
    return {
      type: 'price' as const,
      precision: 2,
      minMove: 0.01,
    }
  }
  return {
    type: 'price' as const,
    precision: 2,
    minMove: 0.01,
  }
}

function formatExtremumValue(value: number, kind: PortfolioValueKind): string {
  if (kind === 'percent') {
    const body = Math.abs(value).toFixed(2)
    if (value > 0) return `+${body}%`
    if (value < 0) return `-${body}%`
    return `${body}%`
  }
  // money / raw: keep natural sign, no forced "+"
  return value.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

function visibleSlice(
  pts: PortfolioChartPoint[],
  range: LogicalRange | null,
): PortfolioChartPoint[] {
  if (pts.length === 0) return pts
  if (!range) return pts
  const from = Math.max(0, Math.ceil(range.from))
  const to = Math.min(pts.length - 1, Math.floor(range.to))
  if (from > to) return pts
  return pts.slice(from, to + 1)
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
    valueKind?: PortfolioValueKind
    /** Show per-point markers on sparse daily series (default: auto for line/area). */
    showPoints?: boolean
  },
) {
  const chartRef = shallowRef<IChartApi | null>(null)
  const hover = ref<PortfolioChartHover | null>(null)
  let series: ISeriesApi<'Area'> | ISeriesApi<'Line'> | ISeriesApi<'Histogram'> | null = null
  let ro: ResizeObserver | null = null
  const valueKind = opts.valueKind ?? 'money'

  function destroy() {
    ro?.disconnect()
    ro = null
    chartRef.value?.remove()
    chartRef.value = null
    series = null
    hover.value = null
  }

  function shouldShowPoints() {
    if (opts.kind === 'histogram') return false
    if (opts.showPoints != null) return opts.showPoints
    const n = points.value.length
    return n > 0 && n <= 120
  }

  function updateExtremumMarkers() {
    if (!series || !chartRef.value) return
    const pts = points.value
    if (pts.length === 0) {
      series.setMarkers([])
      return
    }

    const visible = visibleSlice(pts, chartRef.value.timeScale().getVisibleLogicalRange())
    if (visible.length === 0) {
      series.setMarkers([])
      return
    }

    let hi = 0
    let lo = 0
    for (let i = 1; i < visible.length; i++) {
      if (visible[i].value > visible[hi].value) hi = i
      if (visible[i].value < visible[lo].value) lo = i
    }

    const labelColor = cssVar('--muted', '#848e9c')
    const markers: SeriesMarker<Time>[] = []

    const high = visible[hi]
    const low = visible[lo]
    const same = high.date === low.date || high.value === low.value

    markers.push({
      time: toTime(high.date),
      position: 'aboveBar',
      shape: 'circle',
      color: labelColor,
      id: 'extremum-high',
      text: formatExtremumValue(high.value, valueKind),
      size: 0,
    })

    if (!same) {
      markers.push({
        time: toTime(low.date),
        position: 'belowBar',
        shape: 'circle',
        color: labelColor,
        id: 'extremum-low',
        text: formatExtremumValue(low.value, valueKind),
        size: 0,
      })
    }

    markers.sort((a, b) => String(a.time).localeCompare(String(b.time)))
    series.setMarkers(markers)
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
      ;(series as ISeriesApi<'Area'>).applyOptions({
        pointMarkersVisible: shouldShowPoints(),
        pointMarkersRadius: 3,
      })
    } else {
      const data: LineData[] = points.value.map((p) => ({
        time: toTime(p.date),
        value: p.value,
      }))
      ;(series as ISeriesApi<'Line'>).setData(data)
      ;(series as ISeriesApi<'Line'>).applyOptions({
        pointMarkersVisible: shouldShowPoints(),
        pointMarkersRadius: 3,
      })
    }
    chartRef.value?.timeScale().fitContent()
    // fitContent updates range asynchronously; refresh extrema on next frame
    requestAnimationFrame(() => updateExtremumMarkers())
  }

  function onCrosshair(param: MouseEventParams) {
    if (!series || !param.time || !param.seriesData) {
      hover.value = null
      return
    }
    const row = param.seriesData.get(series) as { value?: number } | undefined
    if (!row || row.value == null || !Number.isFinite(row.value)) {
      hover.value = null
      return
    }
    hover.value = {
      date: String(param.time),
      value: row.value,
    }
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
    // Canvas does not support CSS color-mix; soften grid with alpha hex.
    const gridColor = line.length === 7 ? `${line}59` : 'rgba(43, 49, 57, 0.35)'

    const chart = createChart(el, {
      layout: {
        background: { type: ColorType.Solid, color: bg },
        textColor: text,
        fontSize: 11,
        fontFamily: "'PingFang SC', 'Microsoft YaHei', sans-serif",
      },
      grid: {
        vertLines: { color: gridColor },
        horzLines: { color: gridColor },
      },
      crosshair: {
        mode: CrosshairMode.Magnet,
        vertLine: {
          width: 1,
          color: muted,
          style: 2,
          labelBackgroundColor: line,
        },
        horzLine: {
          width: 1,
          color: muted,
          style: 2,
          labelBackgroundColor: line,
        },
      },
      rightPriceScale: {
        borderVisible: false,
        // Extra top/bottom room for high/low value labels
        scaleMargins: { top: 0.18, bottom: 0.16 },
        entireTextOnly: true,
      },
      timeScale: {
        borderVisible: false,
        timeVisible: false,
        rightOffset: 2,
        barSpacing: 10,
        minBarSpacing: 4,
        fixLeftEdge: true,
        fixRightEdge: true,
        tickMarkFormatter: (time: Time) => formatTickDate(time),
      },
      handleScroll: { mouseWheel: true, pressedMouseMove: true, horzTouchDrag: true },
      handleScale: { axisPressedMouseMove: true, mouseWheel: true, pinch: true },
      width: el.clientWidth,
      height: el.clientHeight || 280,
    })

    const markers = shouldShowPoints()
    const sharedLine = {
      lineWidth: 2 as const,
      lineType: LineType.Simple,
      priceLineVisible: false,
      lastValueVisible: true,
      priceFormat: priceFormatFor(valueKind),
      crosshairMarkerVisible: true,
      crosshairMarkerRadius: 5,
      crosshairMarkerBorderWidth: 2,
      crosshairMarkerBorderColor: bg,
      crosshairMarkerBackgroundColor: accent,
      pointMarkersVisible: markers,
      pointMarkersRadius: 3,
    }

    if (opts.kind === 'area') {
      series = chart.addAreaSeries({
        ...sharedLine,
        lineColor: accent,
        topColor: opts.areaTop || `${accent}4D`,
        bottomColor: opts.areaBottom || `${accent}08`,
      })
    } else if (opts.kind === 'histogram') {
      series = chart.addHistogramSeries({
        color: up,
        priceLineVisible: false,
        lastValueVisible: true,
        base: 0,
        priceFormat: priceFormatFor(valueKind),
      })
    } else {
      series = chart.addLineSeries({
        ...sharedLine,
        color: accent,
      })
    }

    chart.subscribeCrosshairMove(onCrosshair)
    chart.timeScale().subscribeVisibleLogicalRangeChange(() => updateExtremumMarkers())
    chartRef.value = chart
    applyData()

    ro = new ResizeObserver(() => {
      if (!container.value || !chartRef.value) return
      chartRef.value.applyOptions({
        width: container.value.clientWidth,
        height: container.value.clientHeight || 280,
      })
      requestAnimationFrame(() => updateExtremumMarkers())
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

  return { rebuild: build, hover }
}
