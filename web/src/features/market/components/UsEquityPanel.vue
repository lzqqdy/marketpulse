<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useMarketStore } from '@/features/market/stores/market'
import { useChartStore } from '@/features/market/stores/chart'
import { useTrendClass } from '@/features/market/composables/useTrendClass'
import { formatNumber, formatPct, formatPriceUsdt } from '@/utils/format'
import type { AlphaQuote, IndexQuote } from '@/features/market/types/market'

interface UsEquityMeta {
  flag: string
  shortName: string
  ticker: string
  alphaId: string
}

const US_EQUITY_META: Record<string, UsEquityMeta> = {
  'us-qqq': { flag: '🇺🇸', shortName: '纳指ETF', ticker: 'QQQ', alphaId: 'qqq' },
  'us-spy': { flag: '🇺🇸', shortName: '标普ETF', ticker: 'SPY', alphaId: 'spy' },
  'us-aapl': { flag: '🇺🇸', shortName: '苹果', ticker: 'AAPL', alphaId: 'aapl' },
  'us-msft': { flag: '🇺🇸', shortName: '微软', ticker: 'MSFT', alphaId: 'msft' },
  'us-nvda': { flag: '🇺🇸', shortName: '英伟达', ticker: 'NVDA', alphaId: 'nvda' },
  'us-amzn': { flag: '🇺🇸', shortName: '亚马逊', ticker: 'AMZN', alphaId: 'amzn' },
  'us-googl': { flag: '🇺🇸', shortName: '谷歌', ticker: 'GOOGL', alphaId: 'googl' },
  'us-meta': { flag: '🇺🇸', shortName: 'META', ticker: 'META', alphaId: 'meta' },
  'us-tsla': { flag: '🇺🇸', shortName: '特斯拉', ticker: 'TSLA', alphaId: 'tsla' },
  'us-mu': { flag: '🇺🇸', shortName: '美光', ticker: 'MU', alphaId: 'mu' },
  'us-avgo': { flag: '🇺🇸', shortName: '博通', ticker: 'AVGO', alphaId: 'avgo' },
  'us-tsm': { flag: '🇺🇸', shortName: '台积电', ticker: 'TSM', alphaId: 'tsm' },
  'us-skhy': { flag: '🇺🇸', shortName: 'SK海力士', ticker: 'SKHY', alphaId: 'skhy' },
}

const US_EQUITY_ORDER = [
  'us-qqq',
  'us-spy',
  'us-aapl',
  'us-msft',
  'us-nvda',
  'us-amzn',
  'us-googl',
  'us-meta',
  'us-tsla',
  'us-mu',
  'us-avgo',
  'us-tsm',
  'us-skhy',
] as const

interface UsEquityRow {
  id: string
  flag: string
  name: string
  ticker: string
  spot: IndexQuote | null
  alpha: AlphaQuote | null
}

const store = useMarketStore()
const chartStore = useChartStore()
const { priceClass, badgeClass } = useTrendClass()
const expanded = ref(false)
const gridColumns = ref(2)
const COLLAPSED_ROWS = 3
let mediaQueryWide: MediaQueryList | null = null

const spotById = computed(() => {
  const map = new Map<string, IndexQuote>()
  for (const item of store.indices ?? []) {
    if (US_EQUITY_META[item.id]) map.set(item.id, item)
  }
  return map
})

const alphaById = computed(() => {
  const map = new Map<string, AlphaQuote>()
  for (const item of [...(store.alpha.indices ?? []), ...(store.alpha.stocks ?? [])]) {
    map.set(item.id, item)
  }
  return map
})

const allRows = computed<UsEquityRow[]>(() =>
  US_EQUITY_ORDER.map((id) => {
    const meta = US_EQUITY_META[id]
    const spot = spotById.value.get(id) ?? null
    const alpha = alphaById.value.get(meta.alphaId) ?? null
    if (!spot && !alpha) return null
    return {
      id,
      flag: meta.flag,
      name: meta.shortName,
      ticker: meta.ticker,
      spot,
      alpha,
    }
  }).filter((row): row is UsEquityRow => row !== null),
)

const collapsedCount = computed(() => gridColumns.value * COLLAPSED_ROWS)
const rows = computed(() =>
  expanded.value ? allRows.value : allRows.value.slice(0, collapsedCount.value),
)
const canToggle = computed(() => allRows.value.length > collapsedCount.value)

const metaText = computed(() => {
  const spots = allRows.value.map((r) => r.spot).filter((x): x is IndexQuote => x != null)
  const alphas = allRows.value.map((r) => r.alpha).filter((x): x is AlphaQuote => x != null)
  const spotTimes = spots
    .map((item) => new Date(item.fetchedAt || item.updatedAt).getTime())
    .filter((t) => Number.isFinite(t))
  const alphaTimes = alphas
    .map((item) => new Date(item.updatedAt).getTime())
    .filter((t) => Number.isFinite(t))
  const latestSpot = spotTimes.length ? Math.max(...spotTimes) : NaN
  const latestAlpha = alphaTimes.length ? Math.max(...alphaTimes) : NaN
  const latest = [latestSpot, latestAlpha].filter((t) => Number.isFinite(t))
  const updatedText = latest.length
    ? new Date(Math.max(...latest)).toLocaleString('zh-CN', { hour12: false })
    : '--'
  const sources = [
    ...new Set(spots.map((s) => s.source).filter(Boolean)),
    alphas.length ? store.alpha.source || 'bitget' : '',
  ].filter(Boolean)
  const sourceText = sources.length ? ` · ${sources.join('/')}` : ''
  return `现货日涨跌 · 参考 24h${sourceText} · ${updatedText}`
})

function updateGridColumns() {
  gridColumns.value = mediaQueryWide?.matches ? 3 : 2
}

onMounted(() => {
  mediaQueryWide = window.matchMedia('(min-width: 720px)')
  updateGridColumns()
  mediaQueryWide.addEventListener('change', updateGridColumns)
})

onUnmounted(() => {
  mediaQueryWide?.removeEventListener('change', updateGridColumns)
})

function openSpot(item: IndexQuote) {
  chartStore.openIndex(item)
}

function openAlpha(item: AlphaQuote) {
  chartStore.openAlpha(item)
}

function formatSpotPrice(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return formatNumber(value, 2)
}

function formatAlphaPrice(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return formatPriceUsdt(value)
}
</script>

<template>
  <section v-if="allRows.length" id="us-equity-panel" class="us-panel">
    <header class="us-header">
      <div>
        <h2>美股</h2>
        <p>{{ metaText }}</p>
      </div>
    </header>

    <div class="us-grid" :style="{ gridTemplateColumns: `repeat(${gridColumns}, minmax(0, 1fr))` }">
      <article v-for="item in rows" :key="item.id" class="us-card">
        <div class="meta-row">
          <span class="name">
            <span class="flag">{{ item.flag }}</span>
            {{ item.name }}
          </span>
          <span class="ticker">{{ item.ticker }}</span>
        </div>

        <button
          type="button"
          class="quote-row"
          :class="{ disabled: !item.spot }"
          :disabled="!item.spot"
          @click="item.spot && openSpot(item.spot)"
        >
          <span class="leg">现货</span>
          <strong :class="item.spot ? priceClass(item.spot.changePct) : 'flat'">
            {{ item.spot ? formatSpotPrice(item.spot.price) : '--' }}
          </strong>
          <span class="badge" :class="item.spot ? badgeClass(item.spot.changePct) : 'badge-flat'">
            {{ item.spot ? formatPct(item.spot.changePct) : '--' }}
          </span>
        </button>

        <button
          type="button"
          class="quote-row"
          :class="{ disabled: !item.alpha }"
          :disabled="!item.alpha"
          @click="item.alpha && openAlpha(item.alpha)"
        >
          <span class="leg">参考</span>
          <strong :class="item.alpha ? priceClass(item.alpha.change24hPct) : 'flat'">
            {{ item.alpha ? formatAlphaPrice(item.alpha.price) : '--' }}
          </strong>
          <span class="badge" :class="item.alpha ? badgeClass(item.alpha.change24hPct) : 'badge-flat'">
            {{ item.alpha ? formatPct(item.alpha.change24hPct) : '--' }}
          </span>
        </button>
      </article>
    </div>

    <button
      v-if="canToggle"
      type="button"
      class="us-toggle"
      :aria-label="expanded ? '收起美股列表' : '展开美股列表'"
      @click="expanded = !expanded"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true" :class="{ expanded }">
        <path d="m6 9 6 6 6-6" />
      </svg>
    </button>
  </section>
</template>

<style scoped>
.us-panel {
  width: 100%;
  scroll-margin-top: 12px;
}

.us-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
  text-align: left;
}

.us-header h2 {
  margin: 0;
  font-size: 16px;
  color: var(--text-strong);
}

.us-header p {
  margin: 3px 0 0;
  font-size: 11px;
  color: var(--muted);
}

.us-grid {
  display: grid;
  gap: 8px;
}

.us-card {
  min-width: 0;
  border: 1px solid var(--line);
  background: var(--card-soft);
  border-radius: 8px;
  padding: 8px;
  color: var(--text);
}

.meta-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  margin-bottom: 6px;
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

.flag {
  margin-right: 4px;
}

.ticker {
  flex-shrink: 0;
  color: var(--muted);
  font-size: 10px;
}

.quote-row {
  display: grid;
  grid-template-columns: 2em minmax(4.5em, 1fr) auto;
  align-items: center;
  column-gap: 4px;
  width: 100%;
  margin: 0;
  padding: 3px 0;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.quote-row:hover:not(:disabled) {
  background: var(--hover-strong);
}

.quote-row.disabled,
.quote-row:disabled {
  cursor: default;
  opacity: 0.55;
}

.leg {
  font-size: 10px;
  color: var(--muted);
}

.quote-row strong {
  min-width: 0;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.quote-row strong.up {
  color: var(--up);
}

.quote-row strong.down {
  color: var(--down);
}

.quote-row strong.flat {
  color: var(--muted);
}

.badge {
  flex-shrink: 0;
  padding: 1px 4px;
  border-radius: 5px;
  font-size: 10px;
  font-weight: 700;
  color: #fff;
  white-space: nowrap;
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

.us-toggle {
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

.us-toggle:hover {
  color: var(--warning);
}

.us-toggle svg {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2.2;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: transform 0.18s ease;
}

.us-toggle svg.expanded {
  transform: rotate(180deg);
}
</style>
