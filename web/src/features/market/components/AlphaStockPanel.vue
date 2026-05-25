<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMarketStore } from '@/features/market/stores/market'
import { useChartStore } from '@/features/market/stores/chart'
import { useTrendClass } from '@/features/market/composables/useTrendClass'
import { formatPct, formatPriceUsdt } from '@/utils/format'
import type { AlphaQuote } from '@/features/market/types/market'

const store = useMarketStore()
const chartStore = useChartStore()
const { priceClass, badgeClass } = useTrendClass()

const mode = ref<'indices' | 'stocks'>('indices')

const rows = computed(() =>
  (mode.value === 'indices' ? store.alpha.indices ?? [] : store.alpha.stocks ?? [])
    .filter((item) => Number.isFinite(item.price) && item.price > 0),
)

const alphaMetaText = computed(() => {
  const rowTimes = [...(store.alpha.indices ?? []), ...(store.alpha.stocks ?? [])]
    .map((item) => new Date(item.updatedAt).getTime())
    .filter((time) => Number.isFinite(time))
  const snapshotTime = store.alpha.updatedAt ? new Date(store.alpha.updatedAt).getTime() : NaN
  const latestTime = Number.isFinite(snapshotTime)
    ? snapshotTime
    : rowTimes.length > 0
      ? Math.max(...rowTimes)
      : NaN
  const updatedText = Number.isFinite(latestTime)
    ? new Date(latestTime).toLocaleString('zh-CN', { hour12: false })
    : '--'
  const source = store.alpha.source || 'binance-alpha'
  return `更新于 ${updatedText} · ${source}`
})

function openAlpha(item: AlphaQuote) {
  chartStore.openAlpha(item)
}

function formatAlphaPrice(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return formatPriceUsdt(value)
}
</script>

<template>
  <section v-if="store.alpha.indices.length || store.alpha.stocks.length" class="alpha-panel">
    <header class="alpha-header">
      <div>
        <h2>Alpha 美股参考</h2>
        <p>{{ alphaMetaText }}</p>
      </div>
      <div class="switch" role="tablist" aria-label="Alpha 美股参考分类">
        <button
          type="button"
          :class="{ active: mode === 'indices' }"
          @click="mode = 'indices'"
        >
          指数ETF
        </button>
        <button
          type="button"
          :class="{ active: mode === 'stocks' }"
          @click="mode = 'stocks'"
        >
          科技热门
        </button>
      </div>
    </header>

    <div class="alpha-grid" :class="{ compact: mode === 'stocks' }">
      <button
        v-for="item in rows"
        :key="item.id"
        type="button"
        class="alpha-card"
        @click="openAlpha(item)"
      >
        <span class="meta-row">
          <span class="name">{{ item.name }}</span>
          <span class="symbol">{{ item.symbol }}</span>
        </span>
        <span class="quote-row">
          <strong :class="priceClass(item.change24hPct)">
            {{ formatAlphaPrice(item.price) }}
          </strong>
          <span class="badge" :class="badgeClass(item.change24hPct)">
            {{ formatPct(item.change24hPct) }}
          </span>
        </span>
      </button>
    </div>
  </section>
</template>

<style scoped>
.alpha-panel {
  width: 100%;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
  padding-top: 12px;
}

.alpha-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
  text-align: left;
}

.alpha-header > div:first-child {
  min-width: 0;
  flex: 1;
}

.alpha-header h2 {
  margin: 0;
  font-size: 16px;
  color: var(--text-strong);
}

.alpha-header p {
  margin: 3px 0 0;
  font-size: 11px;
  color: var(--muted);
}

.switch {
  display: inline-flex;
  flex-shrink: 0;
  align-self: flex-start;
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 2px;
}

.switch button {
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 11px;
  padding: 5px 7px;
  border-radius: 6px;
  cursor: pointer;
}

.switch button.active {
  background: rgba(240, 185, 11, 0.14);
  color: var(--warning);
}

.alpha-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.alpha-grid.compact {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.alpha-card {
  min-width: 0;
  border: 1px solid var(--line);
  background: var(--card-soft);
  border-radius: 8px;
  padding: 8px;
  text-align: left;
  color: var(--text);
  cursor: pointer;
}

.meta-row,
.quote-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}

.name {
  min-width: 0;
  color: var(--text-strong);
  font-size: 13px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.symbol {
  flex-shrink: 0;
  color: var(--muted);
  font-size: 10px;
  text-align: right;
}

.alpha-card strong {
  min-width: 0;
  margin-top: 5px;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.alpha-card strong.up {
  color: var(--up);
}

.alpha-card strong.down {
  color: var(--down);
}

.alpha-card strong.flat {
  color: var(--muted);
}

.badge {
  flex-shrink: 0;
  margin-top: 5px;
  padding: 2px 5px;
  border-radius: 5px;
  font-size: 11px;
  font-weight: 700;
  color: #fff;
}

.badge-up {
  background-color: rgb(248, 73, 96);
}

.badge-down {
  background-color: rgb(2, 192, 118);
}

.badge-flat {
  background-color: var(--badge-flat);
}

@media (max-width: 420px) {
  .alpha-header {
    align-items: flex-start;
  }

  .alpha-grid,
  .alpha-grid.compact {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
