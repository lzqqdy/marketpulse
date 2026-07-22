<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import StatusBar from '@/features/market/components/StatusBar.vue'
import ProviderStatusWidget from '@/features/market/components/ProviderStatusWidget.vue'
import QuoteTable from '@/features/market/components/QuoteTable.vue'
import MacroGrid from '@/features/market/components/MacroGrid.vue'
import IndexGrid from '@/features/market/components/IndexGrid.vue'
import MarketCenterPanel from '@/features/market/components/MarketCenterPanel.vue'
import ExpressNewsPanel from '@/features/market/components/ExpressNewsPanel.vue'
import AlphaStockPanel from '@/features/market/components/AlphaStockPanel.vue'
import KlineDrawer from '@/features/market/components/KlineDrawer.vue'
import { useMarketStore } from '@/features/market/stores/market'
import { useProviderStore } from '@/features/market/stores/providers'
import { useScreenWakeLock } from '@/features/market/composables/useScreenWakeLock'

const store = useMarketStore()
const providerStore = useProviderStore()

useScreenWakeLock()

onMounted(() => {
  store.initLive()
  providerStore.start()
})

onUnmounted(() => {
  store.teardown()
  store.stopMockTick()
  providerStore.stop()
})
</script>

<template>
  <div class="dashboard">
    <StatusBar />
    <ProviderStatusWidget />
    <main class="dashboard-panels">
      <!--
        PC：左右两列各自纵向堆叠，避免 CSS Grid 同行等高把左侧撑出大空白。
        移动：dash-col 用 display:contents，子模块按阅读顺序回落到单列。
      -->
      <div class="dash-col dash-col-main">
        <QuoteTable />
        <IndexGrid />
        <AlphaStockPanel />
      </div>
      <div class="dash-col dash-col-side">
        <MacroGrid />
        <MarketCenterPanel />
      </div>
    </main>
    <ExpressNewsPanel />
    <p class="footer-note">点击币种行查看 K 线 · 行情 Binance WS 实时</p>
    <KlineDrawer />
  </div>
</template>

<style scoped>
.dashboard {
  width: 100%;
}

.dashboard-panels {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.dash-col {
  display: contents;
}

.dash-col > * {
  min-width: 0;
}

.footer-note {
  text-align: center;
  font-size: 11px;
  color: var(--muted-2);
  margin: 12px 0 0;
}

@media (min-width: 900px) {
  .dashboard-panels {
    display: flex;
    flex-direction: row;
    align-items: flex-start;
    gap: 12px;
  }

  .dash-col {
    display: flex;
    flex-direction: column;
    flex: 1 1 0;
    gap: 10px;
    min-width: 0;
  }

  .footer-note {
    margin-top: 14px;
  }
}

@media (min-width: 1040px) {
  .dashboard-panels {
    gap: 14px;
  }

  .dash-col {
    gap: 12px;
  }
}

/*
 * 移动端阅读顺序：币价 → 宏观 → 指数 → 行情中心 → 美股参考
 * display:contents 时用 order 重排，避免双列 DOM 顺序影响手机。
 */
@media (max-width: 899px) {
  .dash-col-main > :nth-child(1) {
    order: 1;
  }
  .dash-col-side > :nth-child(1) {
    order: 2;
  }
  .dash-col-main > :nth-child(2) {
    order: 3;
  }
  .dash-col-side > :nth-child(2) {
    order: 4;
  }
  .dash-col-main > :nth-child(3) {
    order: 5;
  }
}
</style>
