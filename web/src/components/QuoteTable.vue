<script setup lang="ts">
import { computed } from 'vue'
import { useMarketStore } from '@/stores/market'
import { useChartStore } from '@/stores/chart'
import { useTrendClass } from '@/composables/useTrendClass'
import { formatPct, formatPriceUsdt, formatRank } from '@/utils/format'

const store = useMarketStore()
const chartStore = useChartStore()

function openChart(symbol: string) {
  chartStore.open(symbol)
}
const { priceClass, badgeClass } = useTrendClass()

const rows = computed(() => store.quotes)

const ICON_FALLBACK: Record<string, string> = {
  BTC: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/btc.svg',
  ETH: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/eth.svg',
  BNB: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/bnb.svg',
  LTC: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/ltc.svg',
  FIL: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/fil.svg',
}

function onIconError(event: Event, symbol: string) {
  const img = event.target as HTMLImageElement
  const fallback = ICON_FALLBACK[symbol]
  if (fallback && img.src !== fallback) {
    img.src = fallback
  }
}

function formatCny(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return formatPriceUsdt(value)
}
</script>

<template>
  <section class="quote-section">
    <table class="quote-table">
      <caption class="sr-only">实时币价</caption>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.symbol"
          class="quote-row"
          @click="openChart(row.symbol)"
        >
          <td class="col-icon">
            <img
              v-if="row.iconUrl"
              :src="row.iconUrl"
              :alt="row.symbol"
              width="15"
              height="15"
              loading="lazy"
              @error="onIconError($event, row.symbol)"
            />
            <p class="rank-line">
              <span class="rank">{{ formatRank(row.rank) }}</span>
            </p>
          </td>
          <td class="col-coin">
            <span class="coin-label">{{ row.symbol }}</span>
            <p class="supply">{{ store.marketCapLabel(row.symbol) }}</p>
          </td>
          <td class="col-price" :class="priceClass(row.changeDayPct)">
            {{ formatPriceUsdt(row.priceUsdt) }}
            <p class="cny">¥{{ formatCny(row.priceCny) }}</p>
          </td>
          <td class="col-pct">
            <span class="badge" :class="badgeClass(row.changeDayPct)">
              {{ formatPct(row.changeDayPct) }}
            </span>
          </td>
          <td class="col-pct">
            <span class="badge" :class="badgeClass(row.change24hPct)">
              {{ formatPct(row.change24hPct) }}
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<style scoped>
/* 一比一对齐旧站 index.html tbody 币价表 */
.quote-section {
  width: 100%;
  max-width: 420px;
  margin: 0 auto;
}

.quote-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

.quote-table td {
  padding: 8px 5px;
  vertical-align: middle;
  border-top: none;
}

.col-icon {
  width: 9%;
  padding: 8px 2px 8px 4px;
  text-align: center;
}

.col-icon img {
  display: block;
  width: 15px;
  height: 15px;
  margin: 0 auto 3px;
}

.rank-line {
  margin: 0;
  line-height: 1.1;
}

.rank {
  font-size: 12px;
  color: #5f5f5f;
}

.col-coin {
  width: 18%;
  padding: 8px 0 8px 0;
  font-weight: bold;
  color: #dbaa6a;
  text-align: left;
  vertical-align: middle;
}

.coin-label {
  display: inline-block;
  font-size: 15px;
  line-height: 1.3;
  color: #cd8518;
}

.supply {
  margin: 4px 0 0;
  padding: 0;
  font-size: 10px;
  line-height: 1.2;
  color: #858a90;
  font-weight: normal;
}

.col-price {
  width: 32%;
  padding: 8px 14px 8px 0;
  font-weight: bold;
  font-size: 15px;
  line-height: 1.3;
  text-align: left;
  white-space: nowrap;
}

.col-price.up {
  color: #f6465d;
}

.col-price.down {
  color: #0ecb81;
}

.col-price.flat {
  color: #eaecef;
}

.cny {
  margin: 4px 0 0;
  padding: 0;
  font-size: 12px;
  line-height: 1.2;
  color: #8c9fad;
  font-weight: normal;
}

.col-pct {
  width: 20%;
  padding: 8px 4px;
  text-align: center;
}

.col-price + .col-pct {
  padding-left: 8px;
}

.badge {
  display: block;
  width: 100%;
  box-sizing: border-box;
  font-size: 12px;
  font-weight: bold;
  line-height: 40px;
  border-radius: 3px;
  color: #fff;
  text-align: center;
}

.badge-up {
  background-color: rgb(248, 73, 96);
}

.badge-down {
  background-color: rgb(2, 192, 118);
}

.badge-flat {
  background-color: #5f5f5f;
}

.quote-row {
  cursor: pointer;
}

.quote-row:hover {
  background: rgba(43, 49, 57, 0.25);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}
</style>
