import { ref, shallowRef, watch, onUnmounted, type Ref } from 'vue'
import {
  createChart,
  ColorType,
  CrosshairMode,
  type IChartApi,
  type ISeriesApi,
  type CandlestickData,
  type HistogramData,
  type LineData,
  type UTCTimestamp,
} from 'lightweight-charts'
import type { Candle } from '@/types/chart'
import { KLINE_INITIAL_VISIBLE_BARS } from '@/types/chart'

const MA_LINES = [
  { period: 5, color: '#f0b90b' },
  { period: 10, color: '#22d3ee' },
  { period: 20, color: '#a78bfa' },
  { period: 60, color: '#f97316' },
] as const

function ma(values: number[], period: number): (number | null)[] {
  const out: (number | null)[] = []
  for (let i = 0; i < values.length; i++) {
    if (i < period - 1) {
      out.push(null)
      continue
    }
    let sum = 0
    for (let j = 0; j < period; j++) sum += values[i - j]
    out.push(sum / period)
  }
  return out
}

export function useKlineChart(container: Ref<HTMLElement | null>, candles: Ref<Candle[]>) {
  const chartRef = shallowRef<IChartApi | null>(null)
  const candleSeries = shallowRef<ISeriesApi<'Candlestick'> | null>(null)
  const volumeSeries = shallowRef<ISeriesApi<'Histogram'> | null>(null)
  const maSeries = shallowRef<ISeriesApi<'Line'>[]>([])
  const crosshairPrice = ref<number | null>(null)
  const crosshairTime = ref<string>('')

  let ro: ResizeObserver | null = null
  let themeObserver: MutationObserver | null = null

  function cssVar(name: string, fallback: string) {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
    return value || fallback
  }

  function buildChart() {
    const el = container.value
    if (!el) return

    if (chartRef.value) {
      chartRef.value.remove()
      chartRef.value = null
    }
    ro?.disconnect()
    ro = null

    const bg = cssVar('--panel', '#0b0e11')
    const text = cssVar('--muted', '#848e9c')
    const line = cssVar('--line', '#2b3139')
    const muted = cssVar('--muted-2', '#5f5f5f')
    const up = cssVar('--up', '#f6465d')
    const down = cssVar('--down', '#0ecb81')

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
      rightPriceScale: {
        borderColor: line,
        scaleMargins: { top: 0.08, bottom: 0.22 },
      },
      timeScale: {
        borderColor: line,
        timeVisible: true,
        secondsVisible: false,
        rightOffset: 6,
      },
      width: el.clientWidth,
      height: el.clientHeight,
    })

    const candles_s = chart.addCandlestickSeries({
      upColor: up,
      downColor: down,
      borderUpColor: up,
      borderDownColor: down,
      wickUpColor: up,
      wickDownColor: down,
    })

    const vol = chart.addHistogramSeries({
      color: '#26a69a',
      priceFormat: { type: 'volume' },
      priceScaleId: 'volume',
    })
    chart.priceScale('volume').applyOptions({
      scaleMargins: { top: 0.82, bottom: 0 },
    })

    const lines = MA_LINES.map((line) =>
      chart.addLineSeries({
        color: line.color,
        lineWidth: 1,
        priceLineVisible: false,
        lastValueVisible: false,
        crosshairMarkerVisible: false,
      }),
    )

    chart.subscribeCrosshairMove((param) => {
      if (!param.time || !param.seriesData.size) {
        crosshairPrice.value = null
        crosshairTime.value = ''
        return
      }
      const d = param.seriesData.get(candles_s) as CandlestickData | undefined
      if (d && 'close' in d) {
        crosshairPrice.value = d.close
      }
      const t = param.time as number
      crosshairTime.value = new Date(t * 1000).toLocaleString('zh-CN', { hour12: false })
    })

    chartRef.value = chart
    candleSeries.value = candles_s
    volumeSeries.value = vol
    maSeries.value = lines

    ro = new ResizeObserver(() => {
      if (container.value && chartRef.value) {
        chartRef.value.applyOptions({
          width: container.value.clientWidth,
          height: container.value.clientHeight,
        })
      }
    })
    ro.observe(el)
  }

  function setData(data: Candle[]) {
    if (!candleSeries.value || !volumeSeries.value || maSeries.value.length !== MA_LINES.length) return

    const candleData: CandlestickData[] = data.map((c) => ({
      time: c.time as UTCTimestamp,
      open: c.open,
      high: c.high,
      low: c.low,
      close: c.close,
    }))

    const volData: HistogramData[] = data.map((c) => ({
      time: c.time as UTCTimestamp,
      value: c.volume,
      color: c.close >= c.open ? 'rgba(246, 70, 93, 0.45)' : 'rgba(14, 203, 129, 0.45)',
    }))

    candleSeries.value.setData(candleData)
    volumeSeries.value.setData(volData)
    setMAData(data)

    const chart = chartRef.value
    if (!chart || data.length === 0) return

    const visible = Math.min(KLINE_INITIAL_VISIBLE_BARS, data.length)
    chart.timeScale().setVisibleLogicalRange({
      from: data.length - visible,
      to: data.length,
    })
  }

  function setMAData(data: Candle[]) {
    const closes = data.map((c) => c.close)
    MA_LINES.forEach((line, lineIndex) => {
      const values = ma(closes, line.period)
      const lineData: LineData[] = []
      data.forEach((c, i) => {
        const value = values[i]
        if (value != null) {
          lineData.push({ time: c.time as UTCTimestamp, value })
        }
      })
      maSeries.value[lineIndex]?.setData(lineData)
    })
  }

  watch(
    () => container.value,
    (el) => {
      if (el) {
        buildChart()
        if (candles.value.length) setData(candles.value)
      }
    },
    { flush: 'post' },
  )

  watch(
    candles,
    (data, prev) => {
      if (!data.length || !candleSeries.value) return
      const canPatch =
        prev &&
        prev.length === data.length &&
        prev.length > 0 &&
        prev[prev.length - 1].time === data[data.length - 1].time
      if (canPatch) {
        const c = data[data.length - 1]
        candleSeries.value!.update({
          time: c.time as UTCTimestamp,
          open: c.open,
          high: c.high,
          low: c.low,
          close: c.close,
        })
        volumeSeries.value!.update({
          time: c.time as UTCTimestamp,
          value: c.volume,
          color: c.close >= c.open ? 'rgba(246, 70, 93, 0.45)' : 'rgba(14, 203, 129, 0.45)',
        })
        setMAData(data)
        return
      }
      setData(data)
    },
    { deep: true },
  )

  themeObserver = new MutationObserver(() => {
    if (!container.value || !chartRef.value) return
    buildChart()
    if (candles.value.length) setData(candles.value)
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme'],
  })

  onUnmounted(() => {
    ro?.disconnect()
    themeObserver?.disconnect()
    chartRef.value?.remove()
  })

  return { crosshairPrice, crosshairTime }
}
