<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useMarketStore } from '@/stores/market'
import { useChartStore } from '@/stores/chart'
import { useTrendClass } from '@/composables/useTrendClass'
import { formatNumber, formatPct } from '@/utils/format'
import type { IndexQuote } from '@/types/market'

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

const MAP_FOCUS_REGIONS = new Set<IndexRegion>(['中国', '香港', '美国', '日本', '韩国'])

const ASIA_MAP_ORDER = ['n225', 'ks11', 'sh000001', 'sz399001', 'sz399006', 'sh000688', 'hsi'] as const
const US_MAP_ORDER = ['dji', 'ixic', 'gspc'] as const

const store = useMarketStore()
const chartStore = useChartStore()
const { priceClass } = useTrendClass()
const viewMode = ref<'map' | 'grid'>('grid')
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

const mapItems = computed(() =>
  indices.value.filter(
    (item) => item.id !== 'gold' && MAP_FOCUS_REGIONS.has(item.meta.region),
  ),
)

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
      <h2>全球速览</h2>
      <div class="view-actions" aria-label="指数视图切换">
        <button
          type="button"
          class="view-btn"
          :class="{ active: viewMode === 'grid' }"
          aria-label="方块视图"
          @click="viewMode = 'grid'"
        >
          ▦
        </button>
        <button
          type="button"
          class="view-btn"
          :class="{ active: viewMode === 'map' }"
          aria-label="地图视图"
          @click="viewMode = 'map'"
        >
          ◫
        </button>
      </div>
    </header>

    <p v-if="indicesCachedHint" class="indices-meta" :class="{ stale: indicesStale || equityState === 'degraded' }">
      {{ indicesCachedHint }}
    </p>
    <p v-else-if="indicesLoadingSlow" class="indices-loading">
      指数拉取较慢（数据源限流），请稍候或约 2 分钟后刷新…
    </p>
    <p v-else-if="indicesLoading" class="indices-loading">指数加载中…</p>
    <p v-else-if="indicesFailed" class="indices-loading indices-failed">
      指数暂不可用，约 2 分钟后自动重试
    </p>

    <div v-if="viewMode === 'map'" class="world-preview">
      <section class="map-column map-column-asia" aria-label="亚洲市场">
        <h3 class="map-column-title">亚洲市场</h3>
        <div class="map-bubble-row">
          <button
            v-for="item in asiaMapItems"
            :key="item.id"
            type="button"
            class="map-bubble"
            :class="[bubbleClass(item), { stale: item.stale }]"
            @click="openChart(item)"
          >
            <span>{{ item.meta.shortName ?? item.name }}</span>
            <strong>{{ formatPct(item.changePct) }}</strong>
          </button>
        </div>
      </section>
      <section class="map-column map-column-us" aria-label="美国市场">
        <h3 class="map-column-title">美国市场</h3>
        <div class="map-bubble-row">
          <button
            v-for="item in usMapItems"
            :key="item.id"
            type="button"
            class="map-bubble"
            :class="[bubbleClass(item), { stale: item.stale }]"
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
  gap: 8px;
  padding: 0 2px 6px;
}

.panel-head h2 {
  margin: 0;
  font-size: 15px;
  line-height: 1.3;
  color: #eaecef;
}

.indices-meta {
  margin: 0 0 10px;
  max-width: 100%;
  font-size: 11px;
  line-height: 1.35;
  color: #848e9c;
  text-align: right;
}

.indices-meta.stale {
  color: #f0b90b;
}

.indices-loading {
  margin: 0 0 8px;
  padding: 12px 10px;
  font-size: 12px;
  color: #848e9c;
  text-align: center;
  background: #151a20;
  border-radius: 6px;
}

.indices-failed {
  color: #f0b90b;
}

.view-actions {
  display: flex;
  gap: 6px;
}

.view-btn {
  width: 30px;
  height: 30px;
  border: 1px solid #2b3139;
  border-radius: 6px;
  background: #151a20;
  color: #848e9c;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}

.view-btn.active {
  color: #f0b90b;
  border-color: rgba(240, 185, 11, 0.7);
  background: rgba(240, 185, 11, 0.08);
}

.world-preview {
  position: relative;
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: clamp(220px, 32vw, 280px);
  overflow: hidden;
  border-radius: 8px;
  border: 1px solid #232a33;
  background: #101418;
}

.world-preview::before {
  content: "";
  position: absolute;
  inset: 12px 50%;
  width: 1px;
  transform: translateX(-50%);
  background: linear-gradient(180deg, transparent, rgba(132, 142, 156, 0.22), transparent);
  pointer-events: none;
  z-index: 2;
}

.map-column {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  min-width: 0;
  padding: 12px 10px 14px;
}

.map-column::before {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.95;
}

.map-column::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.28;
  background-image: radial-gradient(rgba(148, 158, 172, 0.22) 0.6px, transparent 0.6px);
  background-size: 16px 16px;
}

.map-column-asia::before {
  background:
    radial-gradient(ellipse 90% 80% at 18% 42%, rgba(42, 118, 168, 0.14), transparent 68%),
    linear-gradient(160deg, rgba(16, 22, 30, 0.98), rgba(12, 16, 22, 0.92));
}

.map-column-us::before {
  background:
    radial-gradient(ellipse 90% 80% at 82% 44%, rgba(168, 128, 42, 0.12), transparent 68%),
    linear-gradient(200deg, rgba(14, 18, 24, 0.92), rgba(18, 22, 28, 0.98));
}

.map-column-title {
  position: relative;
  z-index: 1;
  margin: 0 0 10px;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.2;
  color: rgba(184, 194, 208, 0.72);
  letter-spacing: 0.04em;
}

.map-bubble-row {
  position: relative;
  z-index: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 10px;
  flex: 1;
  min-height: 0;
}

.map-column-asia .map-bubble-row,
.map-column-us .map-bubble-row {
  align-content: center;
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
  background: #1e2329;
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
  background: rgba(43, 49, 57, 0.5);
}

.index-card.clickable:active,
.index-row:not(.disabled):active {
  background: rgba(43, 49, 57, 0.72);
}

.index-card.disabled,
.index-row.disabled {
  cursor: default;
}

.index-title {
  margin: 0;
  font-size: 12px;
  font-weight: bold;
  color: #eaecef;
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
  background: #151a20;
}

.region-group + .region-group {
  border-top: 1px solid #20262e;
}

.region-group h3 {
  margin: 0;
  padding: 9px 10px 5px;
  text-align: left;
  font-size: 12px;
  line-height: 1.3;
  color: #848e9c;
}

.index-row {
  width: 100%;
  display: grid;
  grid-template-columns: minmax(84px, 1.1fr) minmax(86px, 0.9fr) minmax(64px, 0.7fr);
  align-items: center;
  gap: 6px;
  min-height: 42px;
  border: none;
  border-top: 1px solid #20262e;
  background: transparent;
  color: #eaecef;
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
  color: #f6465d;
}

.index-price.down,
.index-chg.down,
.row-change.down {
  color: #0ecb81;
}

.index-price.flat,
.index-chg.flat,
.row-change.flat {
  color: #848e9c;
}

@media (max-width: 430px) {
  .world-preview {
    min-height: 248px;
  }

  .map-column {
    padding: 10px 6px 12px;
  }

  .map-bubble-row {
    gap: 8px;
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
