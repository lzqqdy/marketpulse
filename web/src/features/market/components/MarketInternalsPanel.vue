<script setup lang="ts">
import { computed } from 'vue'
import { useMarketStore } from '@/features/market/stores/market'
import { useTrendClass } from '@/features/market/composables/useTrendClass'
import { formatPct } from '@/utils/format'
import type { SectorQuote } from '@/features/market/types/market'

const store = useMarketStore()
const { priceClass } = useTrendClass()

const cn = computed(() => store.internals?.cn)
const breadth = computed(() => cn.value?.breadth)
const wind = computed(() => cn.value?.wind)
const hasData = computed(() => (breadth.value?.total ?? 0) > 0)

const upPct = computed(() => breadth.value?.up_pct ?? 0)
const downPct = computed(() => breadth.value?.down_pct ?? 0)

const strongIndustry = computed(() => topSectors(cn.value?.industry ?? [], 5, true))
const weakIndustry = computed(() => topSectors(cn.value?.industry ?? [], 5, false))
const strongConcept = computed(() => topSectors(cn.value?.concept ?? [], 5, true))

const updatedLabel = computed(() => {
  const t = cn.value?.updatedAt
  if (!t) return ''
  const d = new Date(t)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleString('zh-CN', { hour12: false })
})

function topSectors(rows: SectorQuote[], n: number, desc: boolean): SectorQuote[] {
  const cp = [...rows]
  cp.sort((a, b) => (desc ? b.change_pct - a.change_pct : a.change_pct - b.change_pct))
  return cp.slice(0, n)
}
</script>

<template>
  <section class="internals-panel card">
    <header class="panel-head">
      <div>
        <h3 class="panel-title">A 股市场内部结构</h3>
        <p class="panel-sub">宽度 · 板块 · 风向</p>
      </div>
      <span v-if="updatedLabel" class="updated">{{ updatedLabel }}</span>
    </header>

    <p v-if="!hasData" class="empty">等待 A 股宽度与板块数据…</p>

    <template v-else>
      <div class="breadth-grid">
        <div class="breadth-stat up">
          <span class="label">上涨</span>
          <strong>{{ breadth?.up ?? 0 }}</strong>
        </div>
        <div class="breadth-stat flat">
          <span class="label">平盘</span>
          <strong>{{ breadth?.flat ?? 0 }}</strong>
        </div>
        <div class="breadth-stat down">
          <span class="label">下跌</span>
          <strong>{{ breadth?.down ?? 0 }}</strong>
        </div>
        <div class="breadth-stat">
          <span class="label">涨停</span>
          <strong>{{ breadth?.limit_up ?? 0 }}</strong>
        </div>
        <div class="breadth-stat">
          <span class="label">跌停</span>
          <strong>{{ breadth?.limit_down ?? 0 }}</strong>
        </div>
      </div>

      <div class="bar-block">
        <div class="bar-labels">
          <span>上涨比例 {{ upPct.toFixed(1) }}%</span>
          <span>下跌 {{ downPct.toFixed(1) }}%</span>
        </div>
        <div class="bar-track">
          <div class="bar-up" :style="{ width: `${Math.min(100, upPct)}%` }" />
          <div class="bar-down" :style="{ width: `${Math.min(100, downPct)}%` }" />
        </div>
      </div>

      <div v-if="wind?.summary" class="wind-box">
        <p class="wind-title">市场风向</p>
        <p class="wind-summary">{{ wind.summary }}</p>
        <div v-if="wind.tags?.length" class="wind-tags">
          <span v-for="tag in wind.tags" :key="tag" class="tag">{{ tag }}</span>
        </div>
      </div>

      <div class="sector-columns">
        <div class="sector-col">
          <h4>强势行业 Top5</h4>
          <ul>
            <li v-for="row in strongIndustry" :key="row.code">
              <span>{{ row.name }}</span>
              <span :class="priceClass(row.change_pct)">{{ formatPct(row.change_pct) }}</span>
            </li>
          </ul>
        </div>
        <div class="sector-col">
          <h4>弱势行业 Top5</h4>
          <ul>
            <li v-for="row in weakIndustry" :key="row.code">
              <span>{{ row.name }}</span>
              <span :class="priceClass(row.change_pct)">{{ formatPct(row.change_pct) }}</span>
            </li>
          </ul>
        </div>
        <div class="sector-col wide">
          <h4>强势概念 Top5</h4>
          <ul>
            <li v-for="row in strongConcept" :key="row.code">
              <span>{{ row.name }}</span>
              <span :class="priceClass(row.change_pct)">{{ formatPct(row.change_pct) }}</span>
            </li>
          </ul>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.internals-panel {
  padding: 14px 16px;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 12px;
}

.panel-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.panel-sub {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted, #8b949e);
}

.updated {
  font-size: 11px;
  color: var(--text-muted, #8b949e);
  white-space: nowrap;
}

.empty {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted, #8b949e);
}

.breadth-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 12px;
}

.breadth-stat {
  background: rgba(255, 255, 255, 0.03);
  border-radius: 8px;
  padding: 8px;
  text-align: center;
}

.breadth-stat .label {
  display: block;
  font-size: 11px;
  color: var(--text-muted, #8b949e);
}

.breadth-stat strong {
  font-size: 16px;
}

.bar-block {
  margin-bottom: 12px;
}

.bar-labels {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--text-muted, #8b949e);
  margin-bottom: 4px;
}

.bar-track {
  display: flex;
  height: 8px;
  border-radius: 999px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.06);
}

.bar-up {
  background: #3fb950;
}

.bar-down {
  background: #f85149;
}

.wind-box {
  background: rgba(56, 139, 253, 0.08);
  border: 1px solid rgba(56, 139, 253, 0.18);
  border-radius: 10px;
  padding: 10px 12px;
  margin-bottom: 12px;
}

.wind-title {
  margin: 0 0 4px;
  font-size: 12px;
  color: var(--text-muted, #8b949e);
}

.wind-summary {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
}

.wind-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.tag {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
}

.sector-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.sector-col.wide {
  grid-column: 1 / -1;
}

.sector-col h4 {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--text-muted, #8b949e);
}

.sector-col ul {
  list-style: none;
  margin: 0;
  padding: 0;
}

.sector-col li {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  padding: 4px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

@media (max-width: 720px) {
  .breadth-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
