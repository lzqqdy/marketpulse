<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useChartStore } from '@/features/market/stores/chart'
import { useKlineChart } from '@/features/market/composables/useKlineChart'
import { KLINE_INTERVALS, type KlineInterval } from '@/features/market/types/chart'
import { formatNumber, formatPct, formatPriceUsdt } from '@/utils/format'
import { useTrendClass } from '@/features/market/composables/useTrendClass'

const chartStore = useChartStore()
const { priceClass } = useTrendClass()

const chartEl = ref<HTMLElement | null>(null)
const candlesRef = computed(() => chartStore.candles)
const {
  crosshairPrice,
  crosshairTime,
  detailCandle,
  detailPoint,
  detailPinned,
  clearPinned,
  scrollToLatest,
} = useKlineChart(chartEl, candlesRef)

const displayPrice = computed(() => {
  if (crosshairPrice.value != null) return crosshairPrice.value
  if (chartStore.kind === 'crypto') return chartStore.quote?.priceUsdt ?? chartStore.candles.at(-1)?.close
  if (chartStore.kind === 'alpha') return chartStore.alphaQuote?.price ?? chartStore.candles.at(-1)?.close
  return chartStore.indexQuote?.price ?? chartStore.candles.at(-1)?.close
})

const priceChange = computed(() => chartStore.changePct)
const activeCandle = computed(() => detailCandle.value ?? chartStore.candles.at(-1) ?? null)

const visibleIntervals = computed(() =>
  chartStore.kind === 'index'
    ? KLINE_INTERVALS.filter((iv) =>
        ['15m', '1h', '1d', '1w'].includes(iv.value),
      )
    : KLINE_INTERVALS,
)

const pairText = computed(() =>
  chartStore.kind === 'index' ? chartStore.pairLabel : `/ ${chartStore.pairLabel}`,
)

const subtitle = computed(() => {
  if (chartStore.kind === 'index') {
    return `${chartStore.source === 'eastmoney' || !chartStore.source ? '东方财富' : chartStore.source} 延迟行情 · MA5 / MA10 / MA20 / MA60`
  }
  if (chartStore.kind === 'alpha') {
    return `${chartStore.source || 'bitget'} · USDT-FUTURES · 仅供参考 · MA5 / MA10 / MA20 / MA60`
  }
  return `${chartStore.wsLive ? 'Binance WS 实时' : chartStore.source === 'mock' ? '演示 Mock' : chartStore.source} · MA5 / MA10 / MA20 / MA60`
})

const feedMode = computed(() => {
  if (chartStore.kind === 'index') return 'REST历史'
  if (chartStore.kind === 'alpha') {
    return ['1m', '5m', '1h'].includes(chartStore.interval) && chartStore.wsLive
      ? 'WS实时覆盖'
      : 'REST历史'
  }
  return chartStore.wsLive ? 'WS实时' : 'REST历史'
})

const displayError = computed(() => {
  if (!chartStore.error) return ''
  if (chartStore.error.includes('unsupported') || chartStore.error.includes('HTTP 400')) {
    return '当前数据源暂不支持该周期'
  }
  if (chartStore.error.includes('websocket')) return '实时连接暂不可用，点击重试'
  return chartStore.error
})

const candleTooltip = computed(() => {
  const c = detailCandle.value
  if (!c) return null
  const change = c.open > 0 ? ((c.close - c.open) / c.open) * 100 : 0
  return {
    ...c,
    change,
    timeText: new Date(c.time * 1000).toLocaleString('zh-CN', { hour12: false }),
  }
})

const tooltipStyle = computed(() => {
  const point = detailPoint.value
  const width = chartEl.value?.clientWidth ?? 0
  const height = chartEl.value?.clientHeight ?? 0
  if (!point) return {}
  const placeLeft = width > 0 && point.x > width - 180
  const placeTop = height > 0 && point.y > height - 150
  return {
    left: `${Math.max(8, point.x + (placeLeft ? -168 : 12))}px`,
    top: `${Math.max(8, point.y + (placeTop ? -132 : 12))}px`,
  }
})

function intervalIsLive(iv: KlineInterval) {
  if (chartStore.kind === 'alpha') return ['1m', '5m', '1h'].includes(iv)
  if (chartStore.kind === 'crypto') return true
  return false
}

function setKlineInterval(iv: KlineInterval) {
  chartStore.setKlineInterval(iv)
}

function goLatest() {
  scrollToLatest()
}

watch(
  () => chartStore.visible,
  async (v) => {
    if (v) {
      document.body.style.overflow = 'hidden'
      await nextTick()
    } else {
      document.body.style.overflow = ''
    }
  },
)

function onBackdrop(e: MouseEvent) {
  if ((e.target as HTMLElement).classList.contains('kline-overlay')) {
    chartStore.close()
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="chartStore.visible"
        class="kline-overlay"
        role="dialog"
        aria-modal="true"
        @click="onBackdrop"
      >
        <Transition name="slide-up">
          <section v-if="chartStore.visible" class="kline-panel" @click.stop>
            <header class="panel-header">
              <div class="header-left">
                <img
                  v-if="chartStore.quote?.iconUrl"
                  :src="chartStore.quote.iconUrl"
                  :alt="chartStore.symbol"
                  class="coin-icon"
                />
                <div>
                  <h2 class="title">
                    {{ chartStore.displayName }}
                    <span class="pair">{{ pairText }}</span>
                  </h2>
                  <p class="subtitle">
                    {{ subtitle }}
                    <span class="feed-badge" :class="{ live: feedMode.includes('WS') }">{{ feedMode }}</span>
                  </p>
                </div>
              </div>
              <div class="header-right">
                <p class="live-price" :class="priceClass(priceChange)">
                  {{ formatPriceUsdt(displayPrice ?? 0) }}
                </p>
                <p class="chg" :class="priceClass(priceChange)">
                  24h {{ formatPct(priceChange) }}
                </p>
              </div>
              <button type="button" class="btn-close" aria-label="关闭" @click="chartStore.close">
                ×
              </button>
            </header>

            <div class="toolbar">
              <div class="intervals">
                <button
                  v-for="iv in visibleIntervals"
                  :key="iv.value"
                  type="button"
                  class="iv-btn"
                  :class="{ active: chartStore.interval === iv.value }"
                  :disabled="chartStore.loading"
                  @click="setKlineInterval(iv.value)"
                >
                  {{ iv.label }}
                  <span v-if="intervalIsLive(iv.value)" class="live-dot" aria-label="支持实时"></span>
                </button>
              </div>
              <button
                type="button"
                class="btn-refresh"
                :disabled="chartStore.loading"
                @click="chartStore.reload()"
              >
                {{ chartStore.loading ? '加载中…' : '刷新' }}
              </button>
            </div>

            <div class="ohlc-bar">
              <span class="ohlc-time">{{ crosshairTime || '最新' }}</span>
              <template v-if="activeCandle">
                <span>O {{ formatPriceUsdt(activeCandle.open) }}</span>
                <span>H {{ formatPriceUsdt(activeCandle.high) }}</span>
                <span>L {{ formatPriceUsdt(activeCandle.low) }}</span>
                <span>C {{ formatPriceUsdt(activeCandle.close) }}</span>
                <span>Vol {{ formatNumber(activeCandle.volume, 2) }}</span>
              </template>
            </div>

            <div class="chart-wrap">
              <div v-if="chartStore.candles.length > 0" ref="chartEl" class="chart-container" />
              <div
                v-if="candleTooltip && detailPoint"
                class="candle-tooltip"
                :class="{ pinned: detailPinned }"
                :style="tooltipStyle"
              >
                <button
                  v-if="detailPinned"
                  type="button"
                  class="tooltip-close"
                  aria-label="取消固定"
                  @click="clearPinned"
                >
                  ×
                </button>
                <p class="tooltip-time">{{ candleTooltip.timeText }}</p>
                <div class="tooltip-grid">
                  <span>开</span><strong>{{ formatPriceUsdt(candleTooltip.open) }}</strong>
                  <span>高</span><strong>{{ formatPriceUsdt(candleTooltip.high) }}</strong>
                  <span>低</span><strong>{{ formatPriceUsdt(candleTooltip.low) }}</strong>
                  <span>收</span><strong>{{ formatPriceUsdt(candleTooltip.close) }}</strong>
                  <span>涨跌</span><strong :class="priceClass(candleTooltip.change)">{{ formatPct(candleTooltip.change) }}</strong>
                  <span>量</span><strong>{{ formatNumber(candleTooltip.volume, 2) }}</strong>
                </div>
              </div>
              <div v-if="chartStore.loading && chartStore.candles.length > 0" class="chart-loading-pill">
                更新中…
              </div>
              <div v-if="chartStore.loading && chartStore.candles.length === 0" class="chart-state">
                加载 K 线…
              </div>
              <div v-else-if="chartStore.error && chartStore.candles.length === 0" class="chart-state error">
                {{ displayError }}
                <button type="button" class="link-btn" @click="chartStore.reload()">重试</button>
              </div>
              <div v-else-if="chartStore.error" class="chart-inline-error">
                {{ displayError }}
              </div>
            </div>

            <footer class="panel-footer">
              <span>拖动 · 滚轮缩放</span>
              <button type="button" class="latest-btn" @click="goLatest">最新</button>
              <span>{{ chartStore.candles.length }} 根 K 线</span>
            </footer>
          </section>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.kline-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: var(--modal-backdrop);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.kline-panel {
  width: 100%;
  max-width: 480px;
  max-height: 92dvh;
  background: var(--panel);
  border-radius: 16px 16px 0 0;
  border: 1px solid var(--line);
  border-bottom: none;
  display: flex;
  flex-direction: column;
  box-shadow: 0 -8px 40px var(--shadow);
}

.panel-header {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 8px;
  align-items: start;
  padding: 14px 16px 8px;
  border-bottom: 1px solid var(--card);
}

.header-left {
  display: flex;
  gap: 10px;
  align-items: center;
  text-align: left;
}

.coin-icon {
  width: 36px;
  height: 36px;
  border-radius: 50%;
}

.title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--text);
}

.pair {
  font-size: 13px;
  font-weight: 400;
  color: var(--muted);
}

.subtitle {
  margin: 2px 0 0;
  font-size: 11px;
  color: var(--muted-2);
}

.feed-badge {
  display: inline-flex;
  align-items: center;
  margin-left: 6px;
  padding: 1px 5px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--line);
  color: var(--muted);
  font-size: 10px;
  line-height: 1.3;
}

.feed-badge.live {
  color: var(--down);
  border-color: color-mix(in srgb, var(--down) 60%, transparent);
  background: color-mix(in srgb, var(--down) 10%, transparent);
}

.header-right {
  text-align: right;
}

.live-price {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.chg {
  margin: 2px 0 0;
  font-size: 12px;
  font-weight: 600;
}

.live-price.up,
.chg.up {
  color: var(--up);
}

.live-price.down,
.chg.down {
  color: var(--down);
}

.btn-close {
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 8px;
  background: var(--card);
  color: var(--text);
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  overflow-x: auto;
}

.intervals {
  display: flex;
  gap: 4px;
  flex-wrap: nowrap;
}

.iv-btn {
  flex-shrink: 0;
  padding: 6px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.iv-btn.active {
  background: var(--line);
  color: var(--warning);
  border-color: var(--warning);
}

.iv-btn:disabled {
  opacity: 0.5;
}

.btn-refresh {
  flex-shrink: 0;
  padding: 6px 12px;
  border: none;
  border-radius: 6px;
  background: var(--line);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
}

.live-dot {
  width: 5px;
  height: 5px;
  border-radius: var(--radius-pill);
  background: var(--down);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--down) 16%, transparent);
}

.ohlc-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 24px;
  margin: 0 12px 4px;
  padding: 5px 8px;
  overflow-x: auto;
  border: 1px solid color-mix(in srgb, var(--line) 70%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--card-soft) 70%, transparent);
  font-size: 11px;
  color: var(--muted);
  text-align: left;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.ohlc-time {
  color: var(--text);
  font-weight: 600;
}

.chart-wrap {
  position: relative;
  flex: 1;
  min-height: 280px;
  margin: 0 8px;
}

.chart-container {
  width: 100%;
  height: min(360px, calc(100dvh - 280px));
  min-height: 280px;
}

.chart-loading-pill {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 2;
  padding: 4px 8px;
  border-radius: var(--radius-pill);
  background: color-mix(in srgb, var(--panel) 88%, transparent);
  border: 1px solid var(--line);
  color: var(--warning);
  font-size: 11px;
}

.candle-tooltip {
  position: absolute;
  z-index: 4;
  width: 156px;
  pointer-events: none;
  border: 1px solid color-mix(in srgb, var(--line) 78%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--panel) 94%, transparent);
  box-shadow: 0 12px 28px var(--shadow);
  padding: 8px;
  color: var(--text);
  backdrop-filter: blur(8px);
}

.candle-tooltip.pinned {
  pointer-events: auto;
  border-color: color-mix(in srgb, var(--warning) 55%, var(--line));
}

.tooltip-time {
  margin: 0 18px 6px 0;
  color: var(--muted);
  font-size: 10px;
  line-height: 1.2;
}

.tooltip-grid {
  display: grid;
  grid-template-columns: 34px 1fr;
  gap: 4px 8px;
  align-items: baseline;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.tooltip-grid span {
  color: var(--muted);
}

.tooltip-grid strong {
  text-align: right;
  font-weight: 700;
}

.tooltip-grid .up {
  color: var(--up);
}

.tooltip-grid .down {
  color: var(--down);
}

.tooltip-grid .flat {
  color: var(--muted);
}

.tooltip-close {
  position: absolute;
  top: 4px;
  right: 5px;
  width: 18px;
  height: 18px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  font-size: 14px;
  line-height: 18px;
}

.chart-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: min(360px, calc(100dvh - 280px));
  min-height: 280px;
  color: var(--muted);
  font-size: 14px;
  flex-direction: column;
  gap: 8px;
}

.chart-state.error {
  color: var(--up);
}

.chart-inline-error {
  position: absolute;
  left: 50%;
  bottom: 12px;
  transform: translateX(-50%);
  max-width: calc(100% - 24px);
  padding: 5px 9px;
  border-radius: var(--radius-pill);
  background: color-mix(in srgb, var(--panel) 88%, transparent);
  border: 1px solid color-mix(in srgb, var(--up) 40%, transparent);
  color: var(--up);
  font-size: 11px;
  white-space: nowrap;
}

.link-btn {
  background: none;
  border: none;
  color: var(--warning);
  cursor: pointer;
  text-decoration: underline;
}

.panel-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 16px 16px;
  font-size: 11px;
  color: var(--muted-2);
}

.latest-btn {
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: transparent;
  color: var(--warning);
  font-size: 11px;
  padding: 3px 10px;
  cursor: pointer;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slide-up-enter-active,
.slide-up-leave-active {
  transition: transform 0.28s cubic-bezier(0.32, 0.72, 0, 1);
}

.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
}

@media (min-width: 481px) {
  .kline-overlay {
    align-items: center;
    padding: 16px;
  }

  .kline-panel {
    border-radius: 16px;
    border-bottom: 1px solid var(--line);
    max-height: 88vh;
  }

  .chart-container,
  .chart-state {
    height: min(480px, calc(88vh - 220px));
    min-height: 340px;
  }

  .slide-up-enter-from,
  .slide-up-leave-to {
    transform: translateY(24px);
    opacity: 0;
  }
}

@media (min-width: 900px) {
  .kline-overlay {
    padding: 24px;
  }

  .kline-panel {
    max-width: 920px;
  }

  .panel-header {
    padding: 18px 20px 12px;
  }

  .toolbar {
    padding: 10px 16px;
  }

  .chart-wrap {
    margin: 0 12px;
  }

  .chart-container,
  .chart-state {
    height: min(560px, calc(88vh - 220px));
    min-height: 420px;
  }
}

@media (max-width: 380px) {
  .panel-header {
    grid-template-columns: 1fr auto;
  }

  .header-right {
    grid-column: 1 / 2;
    text-align: left;
  }

  .btn-close {
    grid-column: 2 / 3;
    grid-row: 1 / 2;
  }
}
</style>
