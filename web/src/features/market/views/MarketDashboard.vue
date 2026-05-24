<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import StatusBar from '@/features/market/components/StatusBar.vue'
import ProviderStatusWidget from '@/features/market/components/ProviderStatusWidget.vue'
import QuoteTable from '@/features/market/components/QuoteTable.vue'
import MacroGrid from '@/features/market/components/MacroGrid.vue'
import IndexGrid from '@/features/market/components/IndexGrid.vue'
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
    <main class="dashboard-grid">
      <section class="market-column">
        <QuoteTable />
      </section>
      <aside class="side-column">
        <MacroGrid />
        <IndexGrid />
        <AlphaStockPanel />
      </aside>
    </main>
    <p class="footer-note">点击币种行查看 K 线 · 行情 Binance WS 实时</p>
    <KlineDrawer />
  </div>
</template>

<style scoped>
.dashboard {
  width: 100%;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
  align-items: start;
  width: 100%;
}

.market-column,
.side-column {
  min-width: 0;
}

.market-column {
  display: flex;
  justify-content: center;
}

.side-column {
  display: grid;
  gap: 12px;
}

.footer-note {
  text-align: center;
  font-size: 11px;
  color: var(--muted-2);
  margin: 16px 0 0;
}

@media (min-width: 900px) {
  .dashboard-grid {
    grid-template-columns: minmax(400px, 1.2fr) minmax(300px, 0.8fr);
    gap: 16px;
  }

  .footer-note {
    margin-top: 20px;
  }
}

@media (min-width: 1040px) {
  .dashboard-grid {
    grid-template-columns: minmax(520px, 1.25fr) minmax(420px, 0.75fr);
    gap: 20px;
  }
}
</style>
