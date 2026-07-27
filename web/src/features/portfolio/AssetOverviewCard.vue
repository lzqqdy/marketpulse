<script setup lang="ts">
import { computed } from 'vue'
import MetricHelp from '@/components/MetricHelp.vue'
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
        <div class="label">
          总资产(U溢价: {{ premium.toFixed(2) }}%)
          <MetricHelp
            title="U溢价"
            text="U溢价 = (USDT/CNY − USD/CNY) / USD/CNY。&#10;正值表示 USDT 相对美元更贵，换汇成本偏高；负值则相反。总资产按 USDT 与人民币双口径展示。"
          />
        </div>
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
        <div class="label">
          今日收益(CNY)
          <MetricHelp
            title="今日收益"
            text="今日收益 = 当前总资产(CNY) − 最近一日资产快照总资产。&#10;依赖每日快照；若当日尚未生成快照或新录入持仓，数值可能暂时偏差。收益率按相对昨日快照计算。"
          />
        </div>
        <div class="label-hint">相对昨日快照</div>
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
          <div class="label">
            累计收益
            <MetricHelp
              title="累计收益"
              text="相对持仓本金累计的盈亏（CNY）与收益率。&#10;本金来自持仓录入的成本口径，与「今日收益」的快照对比算法不同。"
            />
          </div>
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
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
}

.label-hint {
  font-size: 11px;
  color: var(--muted-2, var(--muted));
  margin: -4px 0 6px;
  opacity: 0.9;
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
