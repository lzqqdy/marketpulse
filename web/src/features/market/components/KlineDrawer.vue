<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useChartStore } from '@/features/market/stores/chart'
import { useKlineChart } from '@/features/market/composables/useKlineChart'
import { KLINE_INTERVALS, type KlineInterval } from '@/features/market/types/chart'
import { formatPct, formatPriceUsdt } from '@/utils/format'
import { useTrendClass } from '@/features/market/composables/useTrendClass'

const chartStore = useChartStore()
const { priceClass } = useTrendClass()

const chartEl = ref<HTMLElement | null>(null)
const candlesRef = computed(() => chartStore.candles)
const { crosshairPrice, crosshairTime } = useKlineChart(chartEl, candlesRef)

const displayPrice = computed(() => {
  if (crosshairPrice.value != null) return crosshairPrice.value
  if (chartStore.kind === 'crypto') return chartStore.quote?.priceUsdt ?? chartStore.candles.at(-1)?.close
  if (chartStore.kind === 'alpha') return chartStore.alphaQuote?.price ?? chartStore.candles.at(-1)?.close
  return chartStore.indexQuote?.price ?? chartStore.candles.at(-1)?.close
})

const priceChange = computed(() => chartStore.changePct)

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

function setInterval(iv: KlineInterval) {
  chartStore.setInterval(iv)
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
                  @click="setInterval(iv.value)"
                >
                  {{ iv.label }}
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

            <p v-if="crosshairTime" class="crosshair-hint">{{ crosshairTime }}</p>

            <div class="chart-wrap">
              <div v-if="chartStore.loading" class="chart-state">加载 K 线…</div>
              <div v-else-if="chartStore.error" class="chart-state error">
                {{ chartStore.error }}
                <button type="button" class="link-btn" @click="chartStore.reload()">重试</button>
              </div>
              <div v-else ref="chartEl" class="chart-container" />
            </div>

            <footer class="panel-footer">
              <span>拖动 · 滚轮缩放</span>
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

.crosshair-hint {
  margin: 0;
  padding: 0 16px 4px;
  font-size: 11px;
  color: var(--muted);
  text-align: left;
  min-height: 16px;
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

.link-btn {
  background: none;
  border: none;
  color: var(--warning);
  cursor: pointer;
  text-decoration: underline;
}

.panel-footer {
  display: flex;
  justify-content: space-between;
  padding: 8px 16px 16px;
  font-size: 11px;
  color: var(--muted-2);
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
