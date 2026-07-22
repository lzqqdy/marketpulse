<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/features/auth/stores/auth'
import AllocationDonut from './AllocationDonut.vue'
import * as api from './api'
import { usePortfolioLineChart } from './composables/usePortfolioLineChart'
import type { AllocationResult, ReportRange, ReportSeriesResult } from './types'

const RANGES: { id: ReportRange; label: string }[] = [
  { id: '7d', label: '7日' },
  { id: '30d', label: '30日' },
  { id: '90d', label: '90日' },
  { id: '180d', label: '180日' },
  { id: '1y', label: '近一年' },
  { id: 'all', label: '全部' },
]

const auth = useAuthStore()
const range = ref<ReportRange>('30d')
const series = ref<ReportSeriesResult | null>(null)
const allocation = ref<AllocationResult | null>(null)
const loading = ref(false)
const error = ref('')

const trendEl = ref<HTMLElement | null>(null)
const profitEl = ref<HTMLElement | null>(null)
const rateEl = ref<HTMLElement | null>(null)
const dailyEl = ref<HTMLElement | null>(null)

const trendPoints = computed(() =>
  (series.value?.points ?? []).map((p) => ({ date: p.date, value: p.totalValueCny })),
)
const profitPoints = computed(() =>
  (series.value?.points ?? []).map((p) => ({ date: p.date, value: p.totalProfit })),
)
const ratePoints = computed(() =>
  (series.value?.points ?? []).map((p) => ({ date: p.date, value: p.totalProfitRate * 100 })),
)
const dailyPoints = computed(() =>
  (series.value?.points ?? []).map((p) => ({ date: p.date, value: p.dailyProfit })),
)

usePortfolioLineChart(trendEl, trendPoints, { kind: 'area' })
usePortfolioLineChart(profitEl, profitPoints, {
  kind: 'line',
  lineColor: '#3b82f6',
})
usePortfolioLineChart(rateEl, ratePoints, {
  kind: 'line',
  lineColor: '#a78bfa',
})
usePortfolioLineChart(dailyEl, dailyPoints, { kind: 'histogram', signedBars: true })

const summaryClass = computed(() => {
  const v = series.value?.summary.pnlCny
  if (v == null || v === 0) return 'flat'
  return v > 0 ? 'up' : 'down'
})

function fmtMoney(n: number | null | undefined): string {
  if (n == null || !Number.isFinite(n)) return '—'
  return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function fmtSigned(n: number | null | undefined): string {
  if (n == null || !Number.isFinite(n)) return '—'
  const body = Math.abs(n).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
  if (n > 0) return `+${body}`
  if (n < 0) return `-${body}`
  return body
}

function fmtPct(n: number | null | undefined): string {
  if (n == null || !Number.isFinite(n)) return '—'
  const body = Math.abs(n).toFixed(2)
  if (n > 0) return `+${body}%`
  if (n < 0) return `-${body}%`
  return `${body}%`
}

async function load() {
  if (!auth.token) return
  loading.value = true
  error.value = ''
  try {
    const [s, a] = await Promise.all([
      api.getReportSeries(auth.token, range.value),
      api.getReportAllocation(auth.token),
    ])
    series.value = s
    allocation.value = a
  } catch (e) {
    error.value = e instanceof Error ? e.message : '报告加载失败'
  } finally {
    loading.value = false
  }
}

async function setRange(id: ReportRange) {
  range.value = id
  await load()
}

onMounted(() => {
  void load()
})

defineExpose({ reload: load })
</script>

<template>
  <div class="reports">
    <div class="reports-head">
      <h2 class="title">资产报告</h2>
      <div class="ranges" role="tablist" aria-label="时间范围">
        <button
          v-for="r in RANGES"
          :key="r.id"
          type="button"
          class="range-btn"
          :class="{ active: range === r.id }"
          @click="setRange(r.id)"
        >
          {{ r.label }}
        </button>
      </div>
    </div>

    <p v-if="error" class="banner-err">{{ error }}</p>
    <p v-else-if="loading && !series" class="hint">加载中…</p>

    <template v-if="series">
      <div class="summary">
        <div class="sum-item">
          <div class="label">区间起点</div>
          <div class="value">¥{{ fmtMoney(series.summary.startCny) }}</div>
        </div>
        <div class="sum-item">
          <div class="label">区间终点</div>
          <div class="value">¥{{ fmtMoney(series.summary.endCny) }}</div>
        </div>
        <div class="sum-item">
          <div class="label">区间盈亏</div>
          <div class="value" :class="summaryClass">
            {{ fmtSigned(series.summary.pnlCny) }}
            <span class="pct">/ {{ fmtPct(series.summary.pnlPct) }}</span>
          </div>
        </div>
        <div class="sum-item muted">
          <div class="label">样本区间</div>
          <div class="value small">{{ series.from || '—' }} → {{ series.to || '—' }}</div>
        </div>
      </div>

      <p v-if="!series.points.length" class="hint">暂无快照数据，日终任务写入后将出现走势图</p>

      <div class="chart-grid">
        <section class="chart-card">
          <h3>资产净值走势</h3>
          <div ref="trendEl" class="chart-el" />
        </section>
        <section class="chart-card">
          <h3>累计收益 (CNY)</h3>
          <div ref="profitEl" class="chart-el" />
        </section>
        <section class="chart-card">
          <h3>累计收益率 (%)</h3>
          <div ref="rateEl" class="chart-el" />
        </section>
        <section class="chart-card">
          <h3>每日盈亏 (CNY)</h3>
          <div ref="dailyEl" class="chart-el" />
        </section>
      </div>
    </template>

    <section v-if="allocation" class="alloc-card">
      <h3>当前资产分布</h3>
      <p v-if="allocation.missingSymbols?.length" class="hint warn">
        缺价未计入: {{ allocation.missingSymbols.join(', ') }}
      </p>
      <AllocationDonut :items="allocation.items" :total-cny="allocation.totalCny" />
    </section>
  </div>
</template>

<style scoped>
.reports {
  display: grid;
  gap: 16px;
  min-width: 0;
}

.reports-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.title {
  margin: 0;
  font-size: 16px;
  font-weight: 650;
  color: var(--text);
}

.ranges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.range-btn {
  border: 1px solid var(--line);
  background: transparent;
  color: var(--muted);
  border-radius: 6px;
  padding: 5px 10px;
  font-size: 12px;
  cursor: pointer;
}

.range-btn.active {
  color: var(--text);
  border-color: color-mix(in srgb, var(--accent) 55%, var(--line));
  background: color-mix(in srgb, var(--accent) 14%, transparent);
}

.summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.sum-item {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 10px 12px;
  background: color-mix(in srgb, var(--panel) 88%, transparent);
}

.sum-item.muted .value {
  color: var(--muted);
}

.label {
  font-size: 11px;
  color: var(--muted);
  margin-bottom: 4px;
}

.value {
  font-size: 15px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  color: var(--text);
}

.value.small {
  font-size: 12px;
  font-weight: 500;
}

.value .pct {
  font-size: 12px;
  font-weight: 500;
  margin-left: 4px;
}

.value.up {
  color: var(--up);
}

.value.down {
  color: var(--down);
}

.value.flat {
  color: var(--text);
}

.chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.chart-card,
.alloc-card {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 12px;
  min-width: 0;
  background: color-mix(in srgb, var(--panel) 92%, transparent);
}

.chart-card h3,
.alloc-card h3 {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}

.chart-el {
  width: 100%;
  height: 240px;
}

.hint {
  margin: 0;
  font-size: 13px;
  color: var(--muted);
}

.hint.warn {
  color: var(--warning);
  margin-bottom: 8px;
}

.banner-err {
  margin: 0;
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid color-mix(in srgb, var(--warning) 45%, var(--line));
  color: var(--warning);
  font-size: 13px;
  background: color-mix(in srgb, var(--warning) 10%, transparent);
}

@media (max-width: 900px) {
  .summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .chart-grid {
    grid-template-columns: 1fr;
  }
}
</style>
