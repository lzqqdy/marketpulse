<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useMarketStore } from '@/features/market/stores/market'
import { useChartStore } from '@/features/market/stores/chart'
import { useTrendClass } from '@/features/market/composables/useTrendClass'
import { formatNumber, formatPct } from '@/utils/format'
import type { IndexQuote } from '@/features/market/types/market'

type IndexRegion = '中国' | '香港' | '美国' | '日本' | '韩国' | '商品'

interface IndexMeta {
  region: IndexRegion
  flag: string
  x: number
  y: number
  size: 'sm' | 'md' | 'lg'
  shortName?: string
}

const INDEX_META: Record<string, IndexMeta> = {
  sh000001: { region: '中国', flag: '🇨🇳', x: 54, y: 49, size: 'sm', shortName: '上证' },
  sz399001: { region: '中国', flag: '🇨🇳', x: 48, y: 59, size: 'sm', shortName: '深证' },
  sz399006: { region: '中国', flag: '🇨🇳', x: 52, y: 64, size: 'sm', shortName: '创业板' },
  sh000688: { region: '中国', flag: '🇨🇳', x: 58, y: 56, size: 'sm', shortName: '科创50' },
  hsi: { region: '香港', flag: '🇭🇰', x: 38, y: 62, size: 'md', shortName: '恒生' },
  dji: { region: '美国', flag: '🇺🇸', x: 82, y: 34, size: 'sm', shortName: '道琼斯' },
  ixic: { region: '美国', flag: '🇺🇸', x: 74, y: 52, size: 'md', shortName: '纳斯达克' },
  gspc: { region: '美国', flag: '🇺🇸', x: 90, y: 62, size: 'md', shortName: '标普500' },
  n225: { region: '日本', flag: '🇯🇵', x: 55, y: 28, size: 'lg', shortName: '日经225' },
  ks11: { region: '韩国', flag: '🇰🇷', x: 66, y: 43, size: 'sm', shortName: 'KOSPI' },
  gold: { region: '商品', flag: '🥇', x: 30, y: 74, size: 'sm', shortName: '国际金' },
  silver: { region: '商品', flag: '🥈', x: 33, y: 72, size: 'sm', shortName: '白银' },
  crude: { region: '商品', flag: '🛢️', x: 27, y: 78, size: 'sm', shortName: '原油' },
  'sge-au9999': { region: '商品', flag: '🥇', x: 36, y: 68, size: 'sm', shortName: '国内金' },
}

const REGION_ORDER: IndexRegion[] = ['中国', '香港', '美国', '日本', '韩国', '商品']

const ASIA_MAP_ORDER = ['n225', 'ks11', 'sh000001', 'sz399001', 'sz399006', 'sh000688', 'hsi'] as const
const US_MAP_ORDER = ['dji', 'ixic', 'gspc'] as const
const COMMODITY_MAP_ORDER = ['gold', 'silver', 'crude', 'sge-au9999'] as const

const store = useMarketStore()
const chartStore = useChartStore()
const { priceClass } = useTrendClass()
const viewMode = ref<'map' | 'grid'>('map')
const liveStartedAt = ref(Date.now())
let tickTimer: ReturnType<typeof setInterval> | null = null
const nowMs = ref(Date.now())

onMounted(() => {
  liveStartedAt.value = Date.now()
  tickTimer = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (tickTimer) clearInterval(tickTimer)
})

const indices = computed(() =>
  (store.indices ?? [])
    .map((item) => ({ ...item, meta: INDEX_META[item.id] }))
    .filter((item) => item.meta),
)

const equityState = computed(() => store.ingestHealth?.ingest?.equity ?? '')

function indexFreshnessTime(item: IndexQuote) {
  const ts = item.fetchedAt || item.updatedAt
  const ms = new Date(ts).getTime()
  return Number.isNaN(ms) ? null : ms
}

const indicesUpdatedLabel = computed(() => {
  const times = indices.value
    .map(indexFreshnessTime)
    .filter((t): t is number => t !== null)
  if (times.length === 0) return ''
  const latest = new Date(Math.max(...times))
  return latest.toLocaleString('zh-CN', { hour12: false })
})

const indicesStale = computed(() => {
  if (indices.value.some((i) => i.stale)) return true
  const times = indices.value
    .map(indexFreshnessTime)
    .filter((t): t is number => t !== null)
  if (times.length === 0) return false
  return Date.now() - Math.max(...times) > 3 * 60 * 1000
})

const indicesLoading = computed(
  () =>
    store.live &&
    indices.value.length === 0 &&
    !['error', 'degraded'].includes(equityState.value),
)

const indicesLoadingSlow = computed(
  () => indicesLoading.value && nowMs.value - liveStartedAt.value > 45_000,
)

const indicesFailed = computed(
  () => store.live && indices.value.length === 0 && equityState.value === 'error',
)

const indicesCachedHint = computed(() => {
  if (indices.value.length === 0) return ''
  const source = [...new Set(indices.value.map((i) => i.source).filter(Boolean))].join('/')
  const sourceText = source ? ` · ${source}` : ''
  if (equityState.value === 'degraded' || indicesStale.value) {
    return `缓存数据 · 更新于 ${indicesUpdatedLabel.value}${sourceText} · 后台重试中`
  }
  if (indicesUpdatedLabel.value) {
    return `更新于 ${indicesUpdatedLabel.value}${sourceText}`
  }
  return ''
})

const mapItems = computed(() => indices.value)

function sortMapItems(items: (IndexQuote & { meta: IndexMeta })[], order: readonly string[]) {
  const rank = new Map(order.map((id, index) => [id, index]))
  return [...items].sort((a, b) => (rank.get(a.id) ?? 99) - (rank.get(b.id) ?? 99))
}

const asiaMapItems = computed(() =>
  sortMapItems(
    mapItems.value.filter((item) => ['中国', '香港', '日本', '韩国'].includes(item.meta.region)),
    ASIA_MAP_ORDER,
  ),
)

const usMapItems = computed(() =>
  sortMapItems(
    mapItems.value.filter((item) => item.meta.region === '美国'),
    US_MAP_ORDER,
  ),
)

const commodityMapItems = computed(() =>
  sortMapItems(
    mapItems.value.filter((item) => item.meta.region === '商品'),
    COMMODITY_MAP_ORDER,
  ),
)

const heatmapGroups = computed(() =>
  [
    { key: 'asia', title: '亚洲市场', items: asiaMapItems.value },
    { key: 'us', title: '美国市场', items: usMapItems.value },
    { key: 'commodity', title: '商品', items: commodityMapItems.value },
  ].filter((group) => group.items.length > 0),
)

const groupedIndices = computed(() =>
  REGION_ORDER.map((region) => ({
    region,
    rows: indices.value.filter((item) => item.meta.region === region),
  })).filter((group) => group.rows.length > 0),
)

function canOpenChart(item: IndexQuote) {
  return item.id !== 'sge-au9999'
}

function openChart(item: IndexQuote) {
  if (!canOpenChart(item)) return
  chartStore.openIndex(item)
}

function bubbleClass(item: IndexQuote & { meta: IndexMeta }) {
  return [`bubble-${item.meta.size}`, priceClass(item.changePct)]
}

</script>

<template>
  <section class="index-panel">
    <header class="panel-head">
      <div class="panel-title">
        <h2>全球速览</h2>
        <p v-if="indicesCachedHint" class="indices-meta" :class="{ stale: indicesStale || equityState === 'degraded' }">
          {{ indicesCachedHint }}
        </p>
      </div>
      <div class="view-actions" aria-label="指数视图切换">
        <button
          type="button"
          class="view-btn"
          :class="{ active: viewMode === 'map' }"
          aria-label="地图视图"
          @click="viewMode = 'map'"
        >
          <span class="view-icon view-icon-list" aria-hidden="true"></span>
        </button>
        <button
          type="button"
          class="view-btn"
          :class="{ active: viewMode === 'grid' }"
          aria-label="方块视图"
          @click="viewMode = 'grid'"
        >
          <span class="view-icon view-icon-grid" aria-hidden="true"></span>
        </button>
      </div>
    </header>

    <p v-if="!indicesCachedHint && indicesLoadingSlow" class="indices-loading">
      指数拉取较慢（数据源限流），请稍候或约 2 分钟后刷新…
    </p>
    <p v-else-if="indicesLoading" class="indices-loading">指数加载中…</p>
    <p v-else-if="indicesFailed" class="indices-loading indices-failed">
      指数暂不可用，约 2 分钟后自动重试
    </p>

    <div v-if="viewMode === 'map'" class="heatmap-preview">
      <section
        v-for="group in heatmapGroups"
        :key="group.key"
        class="heatmap-section"
        :class="`heatmap-section-${group.key}`"
      >
        <h3 class="heatmap-title">{{ group.title }}</h3>
        <div class="heatmap-bubbles">
          <button
            v-for="item in group.items"
            :key="item.id"
            type="button"
            class="map-bubble"
            :class="[bubbleClass(item), { stale: item.stale, disabled: !canOpenChart(item) }]"
            @click="openChart(item)"
          >
            <span>{{ item.meta.shortName ?? item.name }}</span>
            <strong>{{ formatPct(item.changePct) }}</strong>
          </button>
        </div>
      </section>
    </div>

    <div v-else class="index-grid">
      <article
        v-for="item in indices"
        :key="item.id"
        class="index-card"
        :class="{ clickable: canOpenChart(item), disabled: !canOpenChart(item), stale: item.stale }"
        @click="openChart(item)"
      >
        <p class="index-title">
          <span class="flag">{{ item.meta.flag }}</span>
          {{ item.name }}
        </p>
        <p class="index-price" :class="priceClass(item.changePct)">
          {{ formatNumber(item.price, 2) }}
        </p>
        <p class="index-chg" :class="priceClass(item.changePct)">
          {{ formatPct(item.changePct) }}
        </p>
      </article>
    </div>

    <!--
    <div v-if="viewMode === 'map' && groupedIndices.length > 0" class="index-list">
      <section v-for="group in groupedIndices" :key="group.region" class="region-group">
        <h3>{{ group.region }}</h3>
        <button
          v-for="item in group.rows"
          :key="item.id"
          type="button"
          class="index-row"
          :class="{ disabled: !canOpenChart(item), stale: item.stale }"
          @click="openChart(item)"
        >
          <span class="row-name">
            <span class="flag">{{ item.meta.flag }}</span>
            {{ item.name }}
          </span>
          <span class="row-price">{{ formatNumber(item.price, 2) }}</span>
          <span class="row-change" :class="priceClass(item.changePct)">
            {{ formatPct(item.changePct) }}
          </span>
        </button>
      </section>
    </div>
    -->
  </section>
</template>

<style scoped>
.index-panel {
  width: 100%;
  margin: 0 auto;
  min-width: 0;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 0 2px 8px;
}

.panel-title {
  min-width: 0;
  text-align: left;
}

.panel-head h2 {
  margin: 0;
  font-size: 15px;
  line-height: 1.3;
  color: var(--text);
}

.indices-meta {
  margin: 4px 0 0;
  max-width: 100%;
  font-size: 11px;
  line-height: 1.35;
  color: var(--muted);
  text-align: left;
}

.indices-meta.stale {
  color: var(--warning);
}

.indices-loading {
  margin: 0 0 8px;
  padding: 12px 10px;
  font-size: 12px;
  color: var(--muted);
  text-align: center;
  background: var(--card-soft);
  border-radius: 6px;
}

.indices-failed {
  color: var(--warning);
}

.view-actions {
  display: flex;
  gap: 6px;
}

.view-btn {
  width: 30px;
  height: 30px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--card-soft);
  color: var(--muted);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.view-btn.active {
  color: var(--warning);
  border-color: rgba(240, 185, 11, 0.7);
  background: rgba(240, 185, 11, 0.08);
}

.view-icon {
  width: 13px;
  height: 13px;
  color: currentColor;
}

.view-icon-list {
  background:
    linear-gradient(currentColor 0 0) 0 1px / 13px 2px no-repeat,
    linear-gradient(currentColor 0 0) 0 6px / 13px 2px no-repeat,
    linear-gradient(currentColor 0 0) 0 11px / 13px 2px no-repeat;
}

.view-icon-grid {
  background-image:
    linear-gradient(currentColor 1px, transparent 1px),
    linear-gradient(90deg, currentColor 1px, transparent 1px);
  background-size: 4px 4px;
  border: 1px solid currentColor;
  opacity: 0.95;
}

.heatmap-preview {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: hidden;
  border-radius: 8px;
  border: 1px solid var(--line);
  background: var(--map-panel);
  padding: 10px;
}

.heatmap-preview::after {
  content: "";
  position: relative;
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.28;
  background-image: radial-gradient(var(--map-dot) 0.6px, transparent 0.6px);
  background-size: 16px 16px;
}

.heatmap-section {
  position: relative;
  z-index: 1;
  min-width: 0;
  border-radius: 6px;
  padding: 8px;
  background: color-mix(in srgb, var(--card-soft) 78%, transparent);
  border: 1px solid color-mix(in srgb, var(--line) 68%, transparent);
}

.heatmap-title {
  position: relative;
  z-index: 1;
  margin: 0 0 8px;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.2;
  color: var(--map-title);
  letter-spacing: 0.04em;
}

.heatmap-bubbles {
  position: relative;
  z-index: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 0;
}

.heatmap-section-asia .heatmap-bubbles {
  display: grid;
  grid-template-columns: repeat(4, max-content);
  justify-content: center;
  align-items: center;
}

.map-bubble {
  position: relative;
  flex: 0 0 auto;
  border: 2px solid rgba(255, 255, 255, 0.08);
  border-radius: 999px;
  color: #fff;
  background: rgba(3, 130, 24, 0.88);
  box-shadow: 0 0 0 4px rgba(3, 130, 24, 0.16);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  padding: 4px;
  text-align: center;
  cursor: pointer;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.map-bubble.up {
  background: rgba(182, 34, 48, 0.88);
  box-shadow: 0 0 0 4px rgba(182, 34, 48, 0.16);
}

.map-bubble.flat {
  background: rgba(97, 106, 120, 0.88);
  box-shadow: 0 0 0 8px rgba(97, 106, 120, 0.18);
}

.map-bubble:hover {
  filter: brightness(1.12);
}

.map-bubble.disabled {
  cursor: default;
}

.map-bubble.disabled:hover {
  filter: none;
}

.map-bubble.stale,
.index-card.stale,
.index-row.stale {
  opacity: 0.78;
}

.map-bubble span {
  max-width: 100%;
  font-size: 10px;
  line-height: 1.1;
  white-space: normal;
}

.map-bubble strong {
  font-size: 11px;
  line-height: 1.1;
}

.bubble-sm {
  width: 50px;
  height: 50px;
}

.bubble-md {
  width: 58px;
  height: 58px;
}

.bubble-lg {
  width: 66px;
  height: 66px;
}

.bubble-lg span {
  font-size: 12px;
}

.bubble-lg strong {
  font-size: 13px;
}

.index-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 4px;
}

.index-card {
  background: var(--card);
  border-radius: 4px;
  padding: 8px 4px;
  text-align: center;
  min-height: 62px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-width: 0;
}

.index-card.clickable,
.index-row:not(.disabled) {
  cursor: pointer;
}

.index-card.clickable:hover,
.index-row:not(.disabled):hover {
  background: var(--hover-strong);
}

.index-card.clickable:active,
.index-row:not(.disabled):active {
  background: var(--hover);
}

.index-card.disabled,
.index-row.disabled {
  cursor: default;
}

.index-title {
  margin: 0;
  font-size: 12px;
  font-weight: bold;
  color: var(--text);
  line-height: 1.3;
}

.flag {
  margin-right: 5px;
  font-size: 14px;
}

.index-price {
  margin: 4px 0 0;
  font-size: 12px;
  font-weight: bold;
  line-height: 1.3;
}

.index-chg {
  margin: 2px 0 0;
  font-size: 12px;
  line-height: 1.3;
}

.index-list {
  margin-top: 8px;
  border-radius: 6px;
  overflow: hidden;
  background: var(--card-soft);
}

.region-group + .region-group {
  border-top: 1px solid var(--line);
}

.region-group h3 {
  margin: 0;
  padding: 9px 10px 5px;
  text-align: left;
  font-size: 12px;
  line-height: 1.3;
  color: var(--muted);
}

.index-row {
  width: 100%;
  display: grid;
  grid-template-columns: minmax(84px, 1.1fr) minmax(86px, 0.9fr) minmax(64px, 0.7fr);
  align-items: center;
  gap: 6px;
  min-height: 42px;
  border: none;
  border-top: 1px solid var(--line);
  background: transparent;
  color: var(--text);
  padding: 7px 10px;
  font: inherit;
  text-align: left;
}

.row-name {
  min-width: 0;
  font-size: 13px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row-price,
.row-change {
  font-size: 13px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.index-price.up,
.index-chg.up,
.row-change.up {
  color: var(--up);
}

.index-price.down,
.index-chg.down,
.row-change.down {
  color: var(--down);
}

.index-price.flat,
.index-chg.flat,
.row-change.flat {
  color: var(--muted);
}

@media (max-width: 430px) {
  .heatmap-preview {
    padding: 8px;
    gap: 7px;
  }

  .heatmap-section {
    padding: 7px;
  }

  .heatmap-bubbles {
    gap: 8px;
  }

  .heatmap-section-asia .heatmap-bubbles {
    grid-template-columns: repeat(4, max-content);
  }

  .map-bubble {
    border-width: 2px;
    box-shadow: 0 0 0 3px rgba(3, 130, 24, 0.14);
  }

  .bubble-sm {
    width: 44px;
    height: 44px;
  }

  .bubble-md {
    width: 52px;
    height: 52px;
  }

  .bubble-lg {
    width: 58px;
    height: 58px;
  }

  .map-bubble span,
  .map-bubble strong {
    font-size: 10px;
  }

  .bubble-lg span,
  .bubble-lg strong {
    font-size: 12px;
  }

}

@media (min-width: 760px) {
  .panel-head h2 {
    font-size: 16px;
  }

  .index-grid {
    grid-template-columns: repeat(auto-fit, minmax(118px, 1fr));
    gap: 6px;
  }

  .index-card {
    min-height: 70px;
    padding: 10px 6px;
  }

  .index-title,
  .index-price,
  .index-chg {
    font-size: 13px;
  }
}
</style>
