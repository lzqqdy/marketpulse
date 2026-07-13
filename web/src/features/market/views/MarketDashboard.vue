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
      <QuoteTable />
      <MacroGrid />
      <IndexGrid />
      <MarketCenterPanel />
      <AlphaStockPanel />
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
  gap: 12px;
  width: 100%;
}

.dashboard-panels > * {
  min-width: 0;
}

.footer-note {
  text-align: center;
  font-size: 11px;
  color: var(--muted-2);
  margin: 16px 0 0;
}

@media (min-width: 900px) {
  .dashboard-panels {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
    align-items: start;
  }

  .footer-note {
    margin-top: 20px;
  }
}

@media (min-width: 1040px) {
  .dashboard-panels {
    gap: 20px;
  }
}
</style>
