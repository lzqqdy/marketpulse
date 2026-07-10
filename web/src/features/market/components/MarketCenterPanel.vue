<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { fetchMarketCenter, fetchMarketCenterHeatmap } from '@/features/market/api/marketCenter'
import { useTrendClass } from '@/features/market/composables/useTrendClass'
import { formatPct } from '@/utils/format'
import type {
  ChgDiagram,
  FundflowGroup,
  HeatmapSortKey,
  MarketCenterResponse,
  MarketCode,
  OverviewItem,
  OverviewTab,
} from '@/features/market/types/marketCenter'
import { HEATMAP_SORT_OPTIONS, MARKET_TABS } from '@/features/market/types/marketCenter'

const { priceClass } = useTrendClass()

const CHG_TRACK_HEIGHT = 100
const FUND_TRACK_HEIGHT = 88
const HEATMAP_COLLAPSED_COUNT = 9

const market = ref<MarketCode>('ab')
const expanded = ref(false)
const heatmapExpanded = ref(false)
const loading = ref(false)
const heatmapLoading = ref(false)
const error = ref('')
const data = ref<MarketCenterResponse | null>(null)
const heatmapSortKey = ref<HeatmapSortKey>('amount')
const fundflowType = ref('')
const overviewTab = ref('')

let refreshTimer: ReturnType<typeof setInterval> | null = null

const chgdiagram = computed<ChgDiagram | null>(() => data.value?.chgdiagram ?? null)
const heatmap = computed(() => data.value?.heatmap ?? null)
const fundflowGroups = computed(() => data.value?.fundflow?.groups ?? [])
const overviewTabs = computed<OverviewTab[]>(() => data.value?.overview?.tabs ?? [])

const activeFundflow = computed<FundflowGroup | null>(() => {
  const groups = fundflowGroups.value
  if (!groups.length) return null
  const found = groups.find((g) => g.blockType === fundflowType.value)
  return found ?? groups[0]
})

const activeOverview = computed<OverviewTab | null>(() => {
  const tabs = overviewTabs.value
  if (!tabs.length) return null
  const found = tabs.find((t) => t.type === overviewTab.value)
  return found ?? tabs[0]
})

const updatedLabel = computed(() => {
  const ts = data.value?.fetchedAt
  if (!ts) return ''
  return new Date(ts * 1000).toLocaleTimeString('zh-CN', { hour12: false })
})

const maxChgCount = computed(() => {
  const bars = chgdiagram.value?.bars ?? []
  return Math.max(1, ...bars.map((b) => b.count))
})

const chgRatio = computed(() => {
  const cd = chgdiagram.value
  if (!cd) return null
  const total = cd.up + cd.down + cd.balance
  if (total <= 0) return null
  return {
    total,
    upPct: (cd.up / total) * 100,
    downPct: (cd.down / total) * 100,
    balancePct: (cd.balance / total) * 100,
  }
})

function chgBarHeight(count: number) {
  if (count <= 0) return 0
  const ratio = count / maxChgCount.value
  return Math.max(16, Math.round(ratio * CHG_TRACK_HEIGHT))
}

function chgBarClass(status: string) {
  if (status === 'up') return 'up'
  if (status === 'down') return 'down'
  return 'flat'
}

const fundflowInflowItems = computed(() => {
  const items = activeFundflow.value?.items ?? []
  return [...items]
    .filter((i) => i.netAmount > 0)
    .sort((a, b) => b.netAmount - a.netAmount)
    .slice(0, 6)
})

const fundflowOutflowItems = computed(() => {
  const items = activeFundflow.value?.items ?? []
  return [...items]
    .filter((i) => i.netAmount < 0)
    .sort((a, b) => a.netAmount - b.netAmount)
    .slice(0, 6)
})

const maxFundflowInflow = computed(() =>
  Math.max(1, ...fundflowInflowItems.value.map((i) => i.netAmount)),
)

const maxFundflowOutflow = computed(() =>
  Math.max(1, ...fundflowOutflowItems.value.map((i) => Math.abs(i.netAmount))),
)

function fundBarHeight(amount: number, side: 'in' | 'out') {
  const abs = Math.abs(amount)
  if (abs <= 0) return 0
  const max = side === 'in' ? maxFundflowInflow.value : maxFundflowOutflow.value
  return Math.max(4, Math.round((abs / max) * FUND_TRACK_HEIGHT))
}

const heatmapItems = computed(() => heatmap.value?.items ?? [])

const visibleHeatmapItems = computed(() =>
  heatmapExpanded.value
    ? heatmapItems.value
    : heatmapItems.value.slice(0, HEATMAP_COLLAPSED_COUNT),
)

const canToggleHeatmap = computed(() => heatmapItems.value.length > HEATMAP_COLLAPSED_COUNT)

const hasExtraSections = computed(() => {
  if (!data.value) return false
  return (
    (heatmap.value?.items?.length ?? 0) > 0 ||
    fundflowGroups.value.length > 0 ||
    overviewTabs.value.length > 0
  )
})

async function loadCenter() {
  const hasData = !!data.value
  if (!hasData) {
    loading.value = true
  }
  error.value = ''
  try {
    const resp = await fetchMarketCenter(market.value)
    data.value = resp
    heatmapSortKey.value = resp.heatmap.sortKey as HeatmapSortKey
    if (!fundflowType.value || !resp.fundflow.groups.some((g) => g.blockType === fundflowType.value)) {
      fundflowType.value = resp.fundflow.groups[0]?.blockType ?? ''
    }
    if (!overviewTab.value || !resp.overview.tabs.some((t) => t.type === overviewTab.value)) {
      overviewTab.value = resp.overview.tabs[0]?.type ?? ''
    }
  } catch (e) {
    if (!hasData) {
      error.value = e instanceof Error ? e.message : '加载失败'
      data.value = null
    }
  } finally {
    loading.value = false
  }
}

async function reloadHeatmap(sortKey: HeatmapSortKey) {
  heatmapSortKey.value = sortKey
  heatmapExpanded.value = false
  heatmapLoading.value = true
  try {
    const resp = await fetchMarketCenterHeatmap(market.value, sortKey)
    if (data.value) {
      data.value = { ...data.value, heatmap: resp }
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : '热力图加载失败'
  } finally {
    heatmapLoading.value = false
  }
}

function trendPath(points: number[] | undefined) {
  if (!points || points.length < 2) return ''
  const min = Math.min(...points)
  const max = Math.max(...points)
  const range = max - min || 1
  const w = 56
  const h = 22
  return points
    .map((p, i) => {
      const x = (i / (points.length - 1)) * w
      const y = h - ((p - min) / range) * h
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

watch(market, () => {
  heatmapExpanded.value = false
  void loadCenter()
})

onMounted(() => {
  void loadCenter()
  refreshTimer = setInterval(() => {
    void loadCenter()
  }, 60_000) // 与指数 ActiveTTL 对齐
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <section class="market-center card">
    <header class="mc-head">
      <div class="mc-title">
        <h2>行情中心</h2>
        <p v-if="updatedLabel" class="mc-meta">更新于 {{ updatedLabel }} · {{ data?.source ?? 'baidu' }}</p>
      </div>
      <div class="market-tabs" role="tablist" aria-label="市场切换">
        <button
          v-for="tab in MARKET_TABS"
          :key="tab.value"
          type="button"
          class="market-tab"
          :class="{ active: market === tab.value }"
          :disabled="loading"
          @click="market = tab.value"
        >
          {{ tab.label }}
        </button>
      </div>
    </header>

    <p v-if="loading && !data" class="mc-loading">行情中心加载中…</p>
    <p v-else-if="error && !data" class="mc-error">{{ error }}</p>

    <template v-else-if="data">
      <!-- 涨跌分布 -->
      <div v-if="chgdiagram" class="mc-block">
        <div class="mc-block-head">
          <h3>涨跌分布</h3>
          <span v-if="chgdiagram.totalValue" class="mc-sub">
            {{ chgdiagram.totalTitle || '成交额' }} {{ chgdiagram.totalValue }}
          </span>
        </div>
        <div class="chg-bars-wrap">
          <div class="chg-bars">
            <div v-for="bar in chgdiagram.bars" :key="bar.title" class="chg-bar-col">
              <div class="chg-bar-track">
                <div
                  class="chg-bar"
                  :class="chgBarClass(bar.status)"
                  :style="{ height: `${chgBarHeight(bar.count)}px` }"
                >
                  <span class="chg-count">{{ bar.count }}</span>
                </div>
              </div>
              <span class="chg-label">{{ bar.title }}</span>
            </div>
          </div>
        </div>
        <div v-if="chgRatio" class="chg-ratio-wrap">
          <div class="chg-ratio-bar" aria-hidden="true">
            <span class="chg-ratio-seg up" :style="{ width: `${chgRatio.upPct}%` }" />
            <span class="chg-ratio-seg flat" :style="{ width: `${chgRatio.balancePct}%` }" />
            <span class="chg-ratio-seg down" :style="{ width: `${chgRatio.downPct}%` }" />
          </div>
          <div class="chg-summary">
            <span class="chg-sum up">上涨 {{ chgdiagram.up }}</span>
            <span class="chg-sum flat">平盘 {{ chgdiagram.balance }}</span>
            <span class="chg-sum down">下跌 {{ chgdiagram.down }}</span>
          </div>
        </div>
      </div>

      <div v-if="expanded" class="mc-extra">
      <!-- 热力图 -->
      <div class="mc-block">
        <div class="mc-block-head">
          <h3>热力图</h3>
          <select
            class="mc-select"
            :value="heatmapSortKey"
            :disabled="heatmapLoading"
            @change="reloadHeatmap(($event.target as HTMLSelectElement).value as HeatmapSortKey)"
          >
            <option v-for="opt in HEATMAP_SORT_OPTIONS" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </div>
        <div v-if="heatmapLoading" class="mc-inline-loading">刷新中…</div>
        <div v-else class="heatmap-grid">
          <div
            v-for="item in visibleHeatmapItems"
            :key="item.code"
            class="heatmap-tile"
            :class="priceClass(item.pxChangeRate)"
            :title="item.name"
          >
            <span class="hm-name">{{ item.name }}</span>
            <strong class="hm-metric">{{ item.metricValue }}</strong>
            <span class="hm-chg">{{ formatPct(item.pxChangeRate) }}</span>
          </div>
        </div>
        <button
          v-if="canToggleHeatmap && !heatmapLoading"
          type="button"
          class="mc-toggle heatmap-toggle"
          :aria-label="heatmapExpanded ? '收起热力图' : '展开热力图'"
          @click="heatmapExpanded = !heatmapExpanded"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true" :class="{ expanded: heatmapExpanded }">
            <path d="m6 9 6 6 6-6" />
          </svg>
        </button>
      </div>

      <!-- 主力净流入 -->
      <div v-if="fundflowGroups.length" class="mc-block">
        <div class="mc-block-head">
          <h3>主力净流入</h3>
          <select v-if="fundflowGroups.length > 1" v-model="fundflowType" class="mc-select">
            <option v-for="g in fundflowGroups" :key="g.blockType" :value="g.blockType">
              {{ g.blockTypeName }}
            </option>
          </select>
          <span v-else class="mc-sub">{{ activeFundflow?.blockTypeName }}</span>
        </div>
        <div class="ff-legend">
          <span class="ff-legend-item up"><i aria-hidden="true" />净流入</span>
          <span class="ff-legend-item down"><i aria-hidden="true" />净流出</span>
        </div>
        <div class="ff-bars-wrap">
          <div class="ff-bars">
            <div
              v-for="item in fundflowInflowItems"
              :key="`in-${item.code}`"
              class="ff-bar-col"
            >
              <span class="ff-val up">{{ item.mainNetTurnover }}</span>
              <div class="ff-bar-track">
                <div
                  class="ff-vbar up"
                  :style="{ height: `${fundBarHeight(item.netAmount, 'in')}px` }"
                />
              </div>
              <span class="ff-name">{{ item.name }}</span>
            </div>
            <div
              v-for="item in fundflowOutflowItems"
              :key="`out-${item.code}`"
              class="ff-bar-col"
            >
              <span class="ff-val down">{{ item.mainNetTurnover }}</span>
              <div class="ff-bar-track">
                <div
                  class="ff-vbar down"
                  :style="{ height: `${fundBarHeight(item.netAmount, 'out')}px` }"
                />
              </div>
              <span class="ff-name">{{ item.name }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 热门板块 -->
      <div v-if="overviewTabs.length" class="mc-block">
        <div class="mc-block-head">
          <h3>热门板块</h3>
        </div>
        <div v-if="overviewTabs.length > 1" class="overview-tabs">
          <button
            v-for="tab in overviewTabs"
            :key="tab.type"
            type="button"
            class="overview-tab"
            :class="{ active: (activeOverview?.type ?? '') === tab.type }"
            @click="overviewTab = tab.type"
          >
            {{ tab.name }}
          </button>
        </div>
        <div class="overview-wrap">
          <div class="overview-scroll">
            <article v-for="item in activeOverview?.items ?? []" :key="item.code" class="overview-card">
              <div class="ov-top">
                <div class="ov-main">
                  <h4>{{ item.name }}</h4>
                  <p class="ov-chg" :class="priceClass(item.changePct)">{{ formatPct(item.changePct) }}</p>
                </div>
                <svg v-if="item.trend?.length" class="ov-spark" viewBox="0 0 56 22" aria-hidden="true">
                  <path :d="trendPath(item.trend)" fill="none" stroke="currentColor" stroke-width="1.5" />
                </svg>
              </div>
              <p v-if="item.leadName" class="ov-lead">
                {{ item.leadName }}
                <span :class="priceClass(item.leadChangePct ?? 0)">{{ formatPct(item.leadChangePct ?? 0) }}</span>
              </p>
            </article>
          </div>
        </div>
      </div>
      </div>

      <button
        v-if="hasExtraSections"
        type="button"
        class="mc-toggle"
        :aria-label="expanded ? '收起行情中心详情' : '展开行情中心详情'"
        @click="expanded = !expanded"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true" :class="{ expanded }">
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>
    </template>
  </section>
</template>

<style scoped>
.market-center {
  width: 100%;
  min-width: 0;
  padding: 10px 12px;
  text-align: left;
  background: var(--card);
  border-radius: 6px;
}

.mc-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 4px;
}

.mc-title {
  min-width: 0;
  flex: 0 0 auto;
}

.mc-head h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
  line-height: 1.3;
}

.mc-meta {
  margin: 4px 0 0;
  font-size: 11px;
  color: var(--muted);
  line-height: 1.35;
}

.market-tabs {
  display: flex;
  gap: 4px;
  flex: 0 0 auto;
  width: 100%;
}

.market-tab {
  border: 1px solid var(--line);
  background: var(--card-soft);
  color: var(--text);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  line-height: 1.2;
  cursor: pointer;
  flex: 1 1 0;
  min-width: 0;
}

.market-tab.active {
  border-color: var(--warning);
  color: var(--warning);
  background: rgba(240, 185, 11, 0.08);
}

.market-tab:disabled {
  opacity: 0.6;
  cursor: default;
}

.mc-loading,
.mc-error,
.mc-inline-loading {
  font-size: 12px;
  color: var(--muted);
  text-align: center;
  padding: 8px 0;
}

.mc-error {
  color: var(--up);
}

.mc-extra .mc-block:first-child {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--line);
}

.mc-toggle {
  display: grid;
  place-items: center;
  width: 28px;
  height: 18px;
  margin: 4px auto 0;
  border: 0;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.mc-toggle:hover {
  color: var(--coin);
}

.mc-toggle svg {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2.2;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: transform 0.18s ease;
}

.mc-toggle svg.expanded {
  transform: rotate(180deg);
}

.mc-block {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--line);
}

.market-center > .mc-block:first-of-type {
  margin-top: 8px;
  padding-top: 0;
  border-top: none;
}

.mc-block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.mc-block-head h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}

.mc-sub {
  font-size: 11px;
  color: var(--muted);
  white-space: nowrap;
}

.mc-select {
  font-size: 11px;
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid var(--line);
  background: var(--card-soft);
  color: var(--text);
  max-width: 100%;
}

.chg-bars-wrap {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  padding: 2px 0;
}

.chg-bars-wrap::-webkit-scrollbar {
  display: none;
}

.chg-bars {
  display: flex;
  justify-content: center;
  align-items: flex-end;
  gap: 4px;
  width: max-content;
  min-width: 100%;
  margin: 0 auto;
}

.chg-bar-col {
  flex: 0 0 30px;
  width: 30px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.chg-bar-track {
  width: 22px;
  height: 100px;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  flex-shrink: 0;
}

.chg-bar {
  width: 100%;
  max-height: 100%;
  border-radius: 3px 3px 0 0;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 3px;
  overflow: hidden;
}

.chg-count {
  font-size: 9px;
  font-weight: 600;
  line-height: 1;
  color: rgba(255, 255, 255, 0.96);
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.28);
  white-space: nowrap;
}

.chg-bar.flat .chg-count {
  color: var(--text);
  text-shadow: none;
}

.chg-bar.up {
  background: var(--up);
}

.chg-bar.down {
  background: var(--down);
}

.chg-bar.flat {
  background: var(--badge-flat);
}

.chg-label {
  font-size: 9px;
  text-align: center;
  line-height: 1.2;
  color: var(--muted);
  min-height: 22px;
  display: flex;
  align-items: flex-start;
  justify-content: center;
}

.chg-ratio-wrap {
  margin-top: 10px;
}

.chg-ratio-bar {
  display: flex;
  width: 100%;
  height: 6px;
  border-radius: 999px;
  overflow: hidden;
  background: var(--hover);
}

.chg-ratio-seg {
  display: block;
  height: 100%;
  min-width: 0;
}

.chg-ratio-seg.up {
  background: var(--up);
}

.chg-ratio-seg.down {
  background: var(--down);
}

.chg-ratio-seg.flat {
  background: var(--badge-flat);
}

.chg-summary {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  gap: 6px;
  align-items: center;
  margin-top: 6px;
  font-size: 11px;
  line-height: 1.3;
}

.chg-sum.up {
  justify-self: start;
  text-align: left;
  color: var(--up);
}

.chg-sum.flat {
  justify-self: center;
  text-align: center;
  color: var(--muted);
  white-space: nowrap;
}

.chg-sum.down {
  justify-self: end;
  text-align: right;
  color: var(--down);
}

.heatmap-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 6px;
}

.heatmap-tile {
  min-width: 0;
  min-height: 54px;
  padding: 6px 8px;
  border-radius: 4px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  font-size: 11px;
  overflow: hidden;
}

.heatmap-tile.up {
  background: color-mix(in srgb, var(--up) 18%, transparent);
  color: var(--text);
}

.heatmap-tile.down {
  background: color-mix(in srgb, var(--down) 18%, transparent);
  color: var(--text);
}

.heatmap-tile.flat {
  background: var(--hover);
  color: var(--muted);
}

.hm-name {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hm-metric {
  font-size: 11px;
  line-height: 1.2;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hm-chg {
  font-size: 10px;
}

.hm-chg.up,
.heatmap-tile.up .hm-chg {
  color: var(--up);
}

.hm-chg.down,
.heatmap-tile.down .hm-chg {
  color: var(--down);
}

.ff-legend {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
  font-size: 10px;
  color: var(--muted);
}

.ff-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.ff-legend-item i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  font-style: normal;
}

.ff-legend-item.up i {
  background: var(--up);
}

.ff-legend-item.down i {
  background: var(--down);
}

.ff-bars-wrap {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  padding: 2px 0;
}

.ff-bars-wrap::-webkit-scrollbar {
  display: none;
}

.ff-bars {
  display: flex;
  justify-content: center;
  align-items: flex-end;
  gap: 4px;
  width: max-content;
  min-width: 100%;
  margin: 0 auto;
}

.ff-bar-col {
  flex: 0 0 52px;
  width: 52px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.ff-val {
  font-size: 9px;
  line-height: 1.2;
  min-height: 11px;
  white-space: nowrap;
  text-align: center;
}

.ff-val.up {
  color: var(--up);
}

.ff-val.down {
  color: var(--down);
}

.ff-bar-track {
  width: 28px;
  height: 88px;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  flex-shrink: 0;
}

.ff-vbar {
  width: 100%;
  max-height: 100%;
  border-radius: 3px 3px 0 0;
}

.ff-vbar.up {
  background: var(--up);
}

.ff-vbar.down {
  background: var(--down);
}

.ff-name {
  font-size: 9px;
  line-height: 1.2;
  color: var(--text);
  text-align: center;
  min-height: 22px;
  max-width: 52px;
  word-break: break-all;
  display: flex;
  align-items: flex-start;
  justify-content: center;
}

.overview-tabs {
  display: flex;
  gap: 6px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.overview-tab {
  border: 1px solid transparent;
  background: var(--hover);
  color: var(--text);
  border-radius: 12px;
  padding: 4px 10px;
  font-size: 11px;
  cursor: pointer;
}

.overview-tab.active {
  border-color: color-mix(in srgb, var(--warning) 35%, transparent);
  background: color-mix(in srgb, var(--warning) 12%, transparent);
  color: var(--warning);
}

.overview-wrap {
  position: relative;
}

.overview-wrap::after {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  width: 18px;
  height: 100%;
  pointer-events: none;
  background: linear-gradient(to left, var(--card), transparent);
}

.overview-scroll {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
  -webkit-overflow-scrolling: touch;
  scroll-snap-type: x proximity;
  scrollbar-width: none;
}

.overview-scroll::-webkit-scrollbar {
  display: none;
}

.overview-card {
  flex: 0 0 136px;
  padding: 10px;
  border-radius: 8px;
  background: var(--card-soft);
  border: 1px solid var(--line);
  scroll-snap-align: start;
}

.ov-top {
  display: flex;
  justify-content: space-between;
  gap: 6px;
  align-items: flex-start;
}

.ov-main {
  min-width: 0;
  flex: 1 1 auto;
}

.overview-card h4 {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.ov-chg {
  margin: 4px 0 0;
  font-size: 13px;
  font-weight: 600;
}

.ov-spark {
  width: 52px;
  height: 22px;
  color: var(--up);
  flex-shrink: 0;
}

.ov-lead {
  margin: 8px 0 0;
  font-size: 10px;
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 480px) {
  .market-center {
    padding: 8px 10px;
  }

  .mc-head {
    gap: 6px;
    margin-bottom: 2px;
  }

  .market-tabs {
    gap: 3px;
  }

  .market-tab {
    padding: 5px 4px;
    font-size: 11px;
  }

  .market-center > .mc-block:first-of-type {
    margin-top: 6px;
  }

  .mc-block-head {
    align-items: flex-start;
  }

  .mc-sub {
    white-space: normal;
  }

  .chg-bars {
    gap: 3px;
  }

  .chg-bar-col {
    flex-basis: 28px;
    width: 28px;
  }

  .chg-bar-track {
    width: 20px;
    height: 92px;
  }

  .chg-count {
    font-size: 9px;
    min-height: 11px;
  }

  .chg-label {
    font-size: 8px;
    min-height: 20px;
    max-width: 30px;
    word-break: break-all;
  }

  .chg-summary {
    font-size: 10px;
    gap: 4px;
  }

  .heatmap-tile {
    min-height: 52px;
    padding: 6px;
  }

  .ff-bar-col {
    flex-basis: 44px;
    width: 44px;
  }

  .ff-bar-track {
    width: 24px;
    height: 76px;
  }

  .ff-val {
    font-size: 8px;
  }

  .ff-name {
    font-size: 8px;
    max-width: 44px;
    min-height: 20px;
  }

  .overview-card {
    flex: 0 0 120px;
    padding: 8px;
  }

  .overview-card h4 {
    font-size: 11px;
    -webkit-line-clamp: 3;
  }

  .ov-chg {
    font-size: 12px;
  }

  .ov-spark {
    width: 44px;
    height: 20px;
  }

  .ov-lead {
    font-size: 9px;
  }
}

@media (min-width: 481px) {
  .market-tabs {
    width: auto;
    margin-left: auto;
    justify-content: flex-end;
  }

  .market-tab {
    flex: 0 0 auto;
  }
}

@media (min-width: 760px) {
  .market-center {
    padding: 12px 14px;
  }

  .mc-head h2 {
    font-size: 16px;
  }

  .heatmap-tile {
    min-height: 58px;
  }

  .overview-card {
    flex-basis: 148px;
  }
}
</style>
