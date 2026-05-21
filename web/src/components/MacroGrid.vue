<script setup lang="ts">
import { computed } from 'vue'
import { useMarketStore } from '@/stores/market'
import { useTrendClass } from '@/composables/useTrendClass'
import { formatNumber, formatPct } from '@/utils/format'

const store = useMarketStore()
const { priceClass } = useTrendClass()

const m = computed(() => store.macro)
const rates = computed(() => store.rates)

const fearLabelZh: Record<string, string> = {
  'Extreme Fear': '极度恐惧',
  Fear: '恐惧',
  Neutral: '中立',
  Greed: '贪婪',
  'Extreme Greed': '极度贪婪',
}

const longShortText = computed(() => {
  const ratio = m.value.longShort
  if (!ratio?.ratio) return ''
  return `多${ratio.longAccountPct.toFixed(1)}% / 空${ratio.shortAccountPct.toFixed(1)}%`
})

const mockTopLongShort = {
  symbol: 'BTCUSDT',
  ratio: 1.21,
  longAccountPct: 54.8,
  shortAccountPct: 45.2,
  updatedAt: new Date().toISOString(),
}

const mockFunding = {
  symbol: 'BTCUSDT',
  rate: 0.000126,
  markPrice: 78045.8,
  indexPrice: 78083.5,
  premiumPct: -0.048,
  nextFundingTime: new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString(),
  updatedAt: new Date().toISOString(),
}

const mockOpenInterest = {
  symbol: 'BTCUSDT',
  valueUsd: 38.74e9,
  changePct: 2.36,
  updatedAt: new Date().toISOString(),
}

const mockTakerBuySell = {
  symbol: 'BTCUSDT',
  ratio: 1.18,
  buyVol: 16.8e9,
  sellVol: 14.2e9,
  updatedAt: new Date().toISOString(),
}

const mockLiquidations = {
  window: '1h',
  longUsd: 28.6e6,
  shortUsd: 17.9e6,
  totalUsd: 46.5e6,
  updatedAt: new Date().toISOString(),
}

const funding = computed(() => m.value.funding ?? mockFunding)
const topLongShort = computed(() => m.value.topLongShort ?? mockTopLongShort)
const openInterest = computed(() => m.value.openInterest ?? mockOpenInterest)
const takerBuySell = computed(() => m.value.takerBuySell ?? mockTakerBuySell)
const liquidations = computed(() => m.value.liquidations ?? mockLiquidations)

function fundingLabel(rate: number) {
  return (rate * 100).toFixed(4) + '%'
}

function formatNextFunding(iso: string) {
  if (!iso) return ''
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return ''
  return (
    '下次 ' +
    t.toLocaleString('zh-CN', {
      month: 'numeric',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    })
  )
}

function formatPremiumPct(pct: number | undefined) {
  if (pct == null || !Number.isFinite(pct)) return '--'
  const sign = pct > 0 ? '+' : ''
  return sign + pct.toFixed(3) + '%'
}

function formatCompactCnyUnit(value: number, precision = 1) {
  if (!Number.isFinite(value)) return '--'
  const abs = Math.abs(value)
  if (abs >= 1e12) return `${(value / 1e12).toFixed(precision)}万亿`
  if (abs >= 1e8) return `${(value / 1e8).toFixed(precision)}亿`
  if (abs >= 1e4) return `${(value / 1e4).toFixed(precision)}万`
  return value.toFixed(precision)
}

interface MetricCard {
  title: string
  main: string
  sub: string
  subClass: string
}

const cards = computed<MetricCard[]>(() => [
  // 按列填充 2 行网格，整体横滑；首屏 3×2 满格
  {
    title: '总市值',
    main: '$' + formatNumber(m.value.totalMarketCapUsd, 2),
    sub: formatPct(m.value.totalMarketCapChange24hPct),
    subClass: priceClass(m.value.totalMarketCapChange24hPct),
  },
  {
    title: 'BTC溢价',
    main: formatPremiumPct(funding.value.premiumPct),
    sub:
      funding.value.markPrice && funding.value.indexPrice
        ? `Mark ${formatNumber(funding.value.markPrice, 0)} / Idx ${formatNumber(funding.value.indexPrice, 0)}`
        : '',
    subClass:
      (funding.value.premiumPct ?? 0) > 0 ? 'up' : (funding.value.premiumPct ?? 0) < 0 ? 'down' : 'flat',
  },
  {
    title: '情绪',
    main: String(m.value.fearGreed.value),
    sub: fearLabelZh[m.value.fearGreed.label] ?? m.value.fearGreed.label,
    subClass: m.value.fearGreed.value >= 55 ? 'down' : m.value.fearGreed.value <= 45 ? 'up' : 'flat',
  },
  {
    title: '大户多空',
    main: topLongShort.value.ratio ? topLongShort.value.ratio.toFixed(2) : '--',
    sub: topLongShort.value.ratio
      ? `多${topLongShort.value.longAccountPct.toFixed(1)}% / 空${topLongShort.value.shortAccountPct.toFixed(1)}%`
      : '',
    subClass: topLongShort.value.ratio >= 1 ? 'up' : 'down',
  },
  {
    title: '资金费率',
    main: fundingLabel(funding.value.rate),
    sub: formatNextFunding(funding.value.nextFundingTime),
    subClass: 'flat',
  },
  {
    title: '美元',
    main: '$ ' + rates.value.usdCny.toFixed(2),
    sub: 'U ' + rates.value.usdtCny.toFixed(2),
    subClass: 'usd',
  },
  {
    title: '持仓量',
    main: '$' + formatCompactCnyUnit(openInterest.value.valueUsd, 1),
    sub: formatPct(openInterest.value.changePct),
    subClass: priceClass(openInterest.value.changePct),
  },
  {
    title: '爆仓',
    main: '$' + formatCompactCnyUnit(liquidations.value.totalUsd, 0),
    sub: `近1h 多${formatCompactCnyUnit(liquidations.value.longUsd, 0)} / 空${formatCompactCnyUnit(liquidations.value.shortUsd, 0)}`,
    subClass: liquidations.value.longUsd >= liquidations.value.shortUsd ? 'down' : 'up',
  },
  {
    title: '主动买卖',
    main: takerBuySell.value.ratio.toFixed(2),
    sub: `买${formatCompactCnyUnit(takerBuySell.value.buyVol, 0)} / 卖${formatCompactCnyUnit(takerBuySell.value.sellVol, 0)}`,
    subClass: takerBuySell.value.ratio >= 1 ? 'up' : 'down',
  },
  {
    title: '稳定币',
    main:
      m.value.stablecoinMarketCapUsd && m.value.stablecoinMarketCapUsd > 0
        ? '$' + formatCompactCnyUnit(m.value.stablecoinMarketCapUsd, 1)
        : '--',
    sub:
      m.value.stablecoinMarketCapUsd && m.value.stablecoinMarketCapUsd > 0
        ? formatPct(m.value.stablecoinMarketCapChange24hPct ?? 0)
        : '',
    subClass: priceClass(m.value.stablecoinMarketCapChange24hPct ?? 0),
  },
  // 横向滑动区：次要 / 与首屏重复度较高
  {
    title: '多空比',
    main: m.value.longShort?.ratio ? m.value.longShort.ratio.toFixed(2) : '--',
    sub: longShortText.value,
    subClass: 'flat',
  },
  {
    title: 'BTC占比',
    main: m.value.btcDominancePct.toFixed(1) + '%',
    sub: `ETH ${m.value.ethDominancePct.toFixed(1)}%`,
    subClass: 'flat',
  },
  {
    title: '24h成交额',
    main: '$' + formatNumber(m.value.totalVolume24hUsd, 2),
    sub: '',
    subClass: 'flat',
  },
])

</script>

<template>
  <section class="coin-metrics" aria-label="币圈指标">
    <div class="section-head">
      <h2>币圈指标</h2>
    </div>
    <div class="metric-scroll">
      <div class="macro-grid">
        <article v-for="card in cards" :key="card.title" class="card">
          <p class="index-title">{{ card.title }}</p>
          <p class="index-box" :class="card.subClass === 'usd' ? '' : card.subClass">{{ card.main }}</p>
          <p v-if="card.sub" class="index-desc" :class="card.subClass">{{ card.sub }}</p>
        </article>
      </div>
    </div>
  </section>
</template>

<style scoped>
.coin-metrics {
  width: 100%;
  min-width: 0;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 0 6px;
  padding: 0 2px;
}

.section-head h2 {
  margin: 0;
  font-size: 13px;
  line-height: 1.3;
  color: #eaecef;
}

.metric-scroll {
  width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  scroll-snap-type: x proximity;
  padding-bottom: 2px;
  container-type: inline-size;
  --col-w: calc((100cqw - 8px) / 3);
}

.metric-scroll::-webkit-scrollbar {
  display: none;
}

.macro-grid {
  display: grid;
  grid-template-rows: repeat(2, auto);
  grid-auto-flow: column;
  grid-auto-columns: var(--col-w);
  gap: 4px;
  width: max-content;
}

.card {
  background: #1e2329;
  border-radius: 4px;
  min-height: 68px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 6px 5px;
  cursor: default;
  min-width: 0;
  scroll-snap-align: start;
}

.index-title {
  margin: 0;
  font-weight: bold;
  font-size: 11px;
  line-height: 1.3;
  color: #eaecef;
}

.index-box {
  margin: 3px 0 0;
  font-size: 12px;
  font-weight: bold;
  line-height: 1.35;
  color: #eaecef;
}

.index-box.up {
  color: #f6465d;
}

.index-box.down {
  color: #0ecb81;
}

.index-desc {
  margin: 2px 0 0;
  font-size: 9px;
  font-weight: bold;
  line-height: 1.3;
  color: #848e9c;
}

.index-desc.up {
  color: #f6465d;
}

.index-desc.down {
  color: #0ecb81;
}

.index-desc.usd {
  color: #008543;
}

@media (max-width: 360px) {
  .metric-scroll {
    --col-w: calc((100cqw - 4px) / 3);
  }

  .macro-grid {
    gap: 2px;
  }

  .index-title {
    font-size: 10px;
  }
}

@media (min-width: 760px) {
  .card {
    min-height: 72px;
    padding: 7px 6px;
  }

  .index-title,
  .index-box {
    font-size: 12px;
  }

  .index-desc {
    font-size: 10px;
  }
}
</style>
