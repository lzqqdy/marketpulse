<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useMarketStore } from '@/features/market/stores/market'
import { useChartStore } from '@/features/market/stores/chart'
import { useTrendClass } from '@/features/market/composables/useTrendClass'
import { formatPct, formatPriceUsdt } from '@/utils/format'
import type { AlphaQuote } from '@/features/market/types/market'

const store = useMarketStore()
const chartStore = useChartStore()
const { priceClass, badgeClass } = useTrendClass()
const expanded = ref(false)
const gridColumns = ref(3)
const COLLAPSED_ROWS = 3
let mediaQuery: MediaQueryList | null = null

const allRows = computed(() =>
  [...(store.alpha.indices ?? []), ...(store.alpha.stocks ?? [])]
    .filter((item) => Number.isFinite(item.price) && item.price > 0),
)

const collapsedCount = computed(() => gridColumns.value * COLLAPSED_ROWS)
const rows = computed(() =>
  expanded.value ? allRows.value : allRows.value.slice(0, collapsedCount.value),
)
const canToggle = computed(() => allRows.value.length > collapsedCount.value)

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
  const source = store.alpha.source || 'bitget'
  return `更新于 ${updatedText} · ${source}`
})

function updateGridColumns() {
  gridColumns.value = mediaQuery?.matches ? 2 : 3
}

onMounted(() => {
  mediaQuery = window.matchMedia('(max-width: 420px)')
  updateGridColumns()
  mediaQuery.addEventListener('change', updateGridColumns)
})

onUnmounted(() => {
  mediaQuery?.removeEventListener('change', updateGridColumns)
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
        <h2>美股参考</h2>
        <p>{{ alphaMetaText }}</p>
      </div>
    </header>

    <div class="alpha-grid">
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
    <button
      v-if="canToggle"
      type="button"
      class="alpha-toggle"
      :aria-label="expanded ? '收起美股参考列表' : '展开美股参考列表'"
      @click="expanded = !expanded"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true" :class="{ expanded }">
        <path d="m6 9 6 6 6-6" />
      </svg>
    </button>
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

.alpha-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
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

.alpha-toggle {
  display: grid;
  place-items: center;
  width: 28px;
  height: 18px;
  margin: 6px auto 0;
  border: 0;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.alpha-toggle:hover {
  color: var(--warning);
}

.alpha-toggle svg {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2.2;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: transform 0.18s ease;
}

.alpha-toggle svg.expanded {
  transform: rotate(180deg);
}

@media (max-width: 420px) {
  .alpha-header {
    align-items: flex-start;
  }

  .alpha-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
