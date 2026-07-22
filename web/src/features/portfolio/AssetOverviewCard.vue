<script setup lang="ts">
import { computed } from 'vue'
import type { PnLWindow, PortfolioOverview } from './types'

const props = defineProps<{
  overview: PortfolioOverview | null
  loading?: boolean
}>()

const { priceClass } = useLocalTrend()

function useLocalTrend() {
  const priceClass = (value: number | null | undefined) => {
    if (value == null || !Number.isFinite(value) || value === 0) return 'flat'
    return value > 0 ? 'up' : 'down'
  }
  return { priceClass }
}

function fmtMoney(n: number | null | undefined, digits = 2): string {
  if (n == null || !Number.isFinite(n)) return '—'
  return n.toLocaleString('en-US', { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

function fmtSigned(n: number | null | undefined, digits = 2): string {
  if (n == null || !Number.isFinite(n)) return '—'
  const body = Math.abs(n).toLocaleString('en-US', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
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

function windowText(w: PnLWindow | null | undefined): string {
  if (!w) return '—'
  return `${fmtSigned(w.pnlCny)} / ${fmtPct(w.pnlPct)}`
}

function windowClass(w: PnLWindow | null | undefined): string {
  return priceClass(w?.pnlCny)
}

const premium = computed(() => props.overview?.usdtPremiumPct ?? 0)
</script>

<template>
  <section class="overview-card">
    <h2 class="overview-title">资产总览</h2>
    <p v-if="loading && !overview" class="hint">加载中…</p>
    <template v-else-if="overview">
      <div class="block block-center">
        <div class="label">总资产(U溢价: {{ premium.toFixed(2) }}%)</div>
        <div class="total-line">
          <span class="total-main">{{ fmtMoney(overview.totalUsdt) }}</span>
          <span class="total-sub">≈ {{ fmtMoney(overview.totalCny) }} CNY</span>
        </div>
        <p v-if="overview.rateFallback" class="hint warn">汇率回退默认值</p>
        <p v-if="overview.missingSymbols?.length" class="hint warn">
          缺价: {{ overview.missingSymbols.join(', ') }}
        </p>
      </div>

      <div class="block block-center">
        <div class="label">今日收益(CNY)</div>
        <div class="pnl-line" :class="windowClass(overview.today)">
          {{ windowText(overview.today) }}
        </div>
      </div>

      <div class="period-grid">
        <div class="period">
          <div class="label">近7日收益</div>
          <div class="pnl-sm" :class="windowClass(overview.d7)">{{ windowText(overview.d7) }}</div>
        </div>
        <div class="period">
          <div class="label">近30日收益</div>
          <div class="pnl-sm" :class="windowClass(overview.d30)">{{ windowText(overview.d30) }}</div>
        </div>
        <div class="period">
          <div class="label">累计收益</div>
          <div class="pnl-sm" :class="windowClass(overview.allTime)">{{ windowText(overview.allTime) }}</div>
        </div>
      </div>
    </template>
    <p v-else class="hint">暂无总览数据</p>
  </section>
</template>

<style scoped>
.overview-card {
  border: 1px solid var(--line);
  border-radius: 8px;
  background: color-mix(in srgb, var(--panel) 92%, transparent);
  padding: 16px 14px 12px;
  min-width: 0;
  max-width: 100%;
  box-sizing: border-box;
}

.overview-title {
  margin: 0 0 14px;
  text-align: center;
  font-size: 15px;
  font-weight: 700;
  color: var(--coin);
}

.block {
  margin-bottom: 14px;
  min-width: 0;
}

.block-center {
  text-align: center;
}

.block-center .total-line {
  justify-content: center;
}

.block-center .hint {
  text-align: center;
}

.label {
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 6px;
}

.total-line {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.total-main {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-strong);
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.total-sub {
  font-size: 13px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.pnl-line {
  font-size: 20px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.pnl-sm {
  font-size: 12px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
  line-height: 1.35;
}

.period-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0;
  border-top: 1px solid var(--line);
  margin-top: 4px;
  min-width: 0;
}

.period {
  padding: 10px 6px 4px;
  text-align: center;
  min-width: 0;
}

.period + .period {
  border-left: 1px solid var(--line);
}

.hint {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.hint.warn {
  color: var(--warning);
  margin-top: 6px;
}

.up {
  color: var(--up);
}

.down {
  color: var(--down);
}

.flat {
  color: var(--text);
}

@media (max-width: 680px) {
  .total-main {
    font-size: 18px;
  }

  .pnl-line {
    font-size: 16px;
  }

  .period-grid {
    grid-template-columns: 1fr;
  }

  .period + .period {
    border-left: none;
    border-top: 1px solid var(--line);
  }

  .pnl-sm {
    font-size: 13px;
  }
}
</style>
