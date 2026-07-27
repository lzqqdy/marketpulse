<script setup lang="ts">
import { computed } from 'vue'
import MetricHelp from '@/components/MetricHelp.vue'
import { useMarketStore } from '@/features/market/stores/market'
import { useTrendClass } from '@/features/market/composables/useTrendClass'
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
  help?: string
}

const cards = computed<MetricCard[]>(() => [
  // 按列填充 2 行网格，整体横滑；首屏 3×2 满格
  {
    title: '总市值',
    main: '$' + formatNumber(m.value.totalMarketCapUsd, 2),
    sub: formatPct(m.value.totalMarketCapChange24hPct),
    subClass: priceClass(m.value.totalMarketCapChange24hPct),
    help: '加密市场全部代币流通市值合计，反映整体市场规模。下方数字为近 24 小时涨跌幅。',
  },
  {
    title: 'BTC溢价',
    main: formatPremiumPct(funding.value.premiumPct),
    // 手机窄列放不下 "Mark … / Idx …"，只保留两个价避免换行撕开
    sub:
      funding.value.markPrice && funding.value.indexPrice
        ? `${formatNumber(funding.value.markPrice, 0)}/${formatNumber(funding.value.indexPrice, 0)}`
        : '',
    subClass:
      (funding.value.premiumPct ?? 0) > 0 ? 'up' : (funding.value.premiumPct ?? 0) < 0 ? 'down' : 'flat',
    help: 'BTC 永续合约标记价相对现货指数价的溢价/折价。\n正值：合约价高于现货，偏多头拥挤；负值：合约价低于现货，偏空头或资金承压。\n下方为标记价/指数价。',
  },
  {
    title: '情绪',
    main: String(m.value.fearGreed.value),
    sub: fearLabelZh[m.value.fearGreed.label] ?? m.value.fearGreed.label,
    subClass: m.value.fearGreed.value >= 55 ? 'down' : m.value.fearGreed.value <= 45 ? 'up' : 'flat',
    help: '恐惧与贪婪指数（Fear & Greed，0–100）。\n数值越低越恐惧，越高越贪婪；常用来观察市场情绪是否过热或过度悲观。',
  },
  {
    title: '大户多空',
    main: topLongShort.value.ratio ? topLongShort.value.ratio.toFixed(2) : '--',
    sub: topLongShort.value.ratio
      ? `多${topLongShort.value.longAccountPct.toFixed(1)}% / 空${topLongShort.value.shortAccountPct.toFixed(1)}%`
      : '',
    subClass: topLongShort.value.ratio >= 1 ? 'up' : 'down',
    help: '大户账户的多空持仓比（多仓账户占比 / 空仓账户占比）。\n大于 1 表示大户整体偏多，小于 1 偏空。下方为多/空账户占比。',
  },
  {
    title: '资金费率',
    main: fundingLabel(funding.value.rate),
    sub: formatNextFunding(funding.value.nextFundingTime),
    subClass: 'flat',
    help: '永续合约定期结算的资金费率。\n费率为正：多头向空头付费；为负：空头向多头付费。用来衡量多空杠杆拥挤程度。下方为下次结算时间。',
  },
  {
    title: '美元',
    main: '$ ' + rates.value.usdCny.toFixed(2),
    sub: 'U ' + rates.value.usdtCny.toFixed(2),
    subClass: 'usd',
    help: '外汇与稳定币兑人民币参考价。\n上方为 USD/CNY（美元），下方为 USDT/CNY（U 溢价相关）。两者价差可侧面反映换汇成本与市场情绪。',
  },
  {
    title: '持仓量',
    main: '$' + formatCompactCnyUnit(openInterest.value.valueUsd, 1),
    sub: formatPct(openInterest.value.changePct),
    subClass: priceClass(openInterest.value.changePct),
    help: 'BTC 永续未平仓合约名义价值（Open Interest），衡量杠杆资金规模。\n持仓量上升通常表示新开仓增多；下方为变化幅度。',
  },
  {
    title: '爆仓',
    main: '$' + formatCompactCnyUnit(liquidations.value.totalUsd, 0),
    sub: `近1h 多${formatCompactCnyUnit(liquidations.value.longUsd, 0)} / 空${formatCompactCnyUnit(liquidations.value.shortUsd, 0)}`,
    subClass: liquidations.value.longUsd >= liquidations.value.shortUsd ? 'down' : 'up',
    help: '近 1 小时强制平仓（爆仓）金额合计。\n多头爆仓多常见于急跌，空头爆仓多常见于急涨；可用来观察短期踩踏方向。',
  },
  {
    title: '主动买卖',
    main: takerBuySell.value.ratio.toFixed(2),
    sub: `买${formatCompactCnyUnit(takerBuySell.value.buyVol, 0)} / 卖${formatCompactCnyUnit(takerBuySell.value.sellVol, 0)}`,
    subClass: takerBuySell.value.ratio >= 1 ? 'up' : 'down',
    help: 'Taker 主动买入成交量 / 主动卖出成交量。\n大于 1 表示主动买盘更强，小于 1 表示主动卖盘更强。下方为买卖成交额。',
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
    help: '主要稳定币流通市值合计。\n规模上升通常意味着更多法币/稳定币进入加密市场，可侧面反映场内可用流动性。',
  },
  // 横向滑动区：次要 / 与首屏重复度较高
  {
    title: '多空比',
    main: m.value.longShort?.ratio ? m.value.longShort.ratio.toFixed(2) : '--',
    sub: longShortText.value,
    subClass: 'flat',
    help: '全市场账户多空比（偏散户视角）。\n与「大户多空」对照看：两边背离时，常提示多空力量分布不均。',
  },
  {
    title: 'BTC占比',
    main: m.value.btcDominancePct.toFixed(1) + '%',
    sub: `ETH ${m.value.ethDominancePct.toFixed(1)}%`,
    subClass: 'flat',
    help: '比特币市值占加密总市值的比例（BTC Dominance）。\n占比上升常对应资金更偏避险/集中 BTC；下降则山寨币相对更活跃。下方为 ETH 占比。',
  },
  {
    title: '24h成交额',
    main: '$' + formatNumber(m.value.totalVolume24hUsd, 2),
    sub: '',
    subClass: 'flat',
    help: '加密市场近 24 小时成交额合计，反映整体交投活跃度。成交额放大时常伴随波动加剧。',
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
          <p class="index-title">
            <span>{{ card.title }}</span>
            <MetricHelp v-if="card.help" :title="card.title" :text="card.help" />
          </p>
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
  color: var(--text);
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
  /* 约 3.2 列宽：完整显示 3 列，右侧露出第 4 列一角，提示可横滑 */
  --col-w: calc((100cqw - 8px) / 3.2);
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
  /* 右侧多留一点，避免最后一列贴边看不出还能滑 */
  padding-right: 10px;
  box-sizing: content-box;
}

.card {
  background: var(--card);
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
  color: var(--text);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  max-width: 100%;
}

.index-box {
  margin: 3px 0 0;
  font-size: 12px;
  font-weight: bold;
  line-height: 1.35;
  color: var(--text);
}

.index-box.up {
  color: var(--up);
}

.index-box.down {
  color: var(--down);
}

.index-desc {
  margin: 2px 0 0;
  font-size: 9px;
  font-weight: bold;
  line-height: 1.3;
  color: var(--muted);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
}

.index-desc.up {
  color: var(--up);
}

.index-desc.down {
  color: var(--down);
}

.index-desc.usd {
  color: var(--usd);
}

@media (max-width: 680px) {
  .macro-grid {
    gap: 4px;
    padding: 6px 6px 6px 0;
  }

  .card {
    min-height: 58px;
    padding: 5px 4px;
  }

  .index-title {
    font-size: 10px;
  }

  .index-box {
    font-size: 12px;
  }

  .index-desc {
    font-size: 9px;
  }
}

@media (max-width: 360px) {
  .metric-scroll {
    --col-w: calc((100cqw - 4px) / 3.2);
  }

  .macro-grid {
    gap: 2px;
    padding-right: 8px;
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
