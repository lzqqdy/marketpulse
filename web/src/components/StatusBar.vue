<script setup lang="ts">
import { computed } from 'vue'
import { useMarketStore } from '@/stores/market'

const store = useMarketStore()

const statusText = computed(() => {
  switch (store.feedStatus) {
    case 'live':
      return 'Binance 实时'
    case 'reconnecting':
      return '行情重连中…'
    case 'stale':
      return '行情延迟'
    case 'offline':
      return store.live ? '行情已中断' : '演示数据'
    default:
      return '连接中…'
  }
})

const statusClass = computed(() => {
  switch (store.feedStatus) {
    case 'live':
      return 'open'
    case 'reconnecting':
      return 'connecting'
    case 'stale':
      return 'stale'
    case 'offline':
      return store.live ? 'closed' : 'mock'
    default:
      return 'connecting'
  }
})
</script>

<template>
  <header class="status-bar">
    <span class="tag-label link">实时币价</span>
    <span class="tag-label dot" :class="statusClass">{{ statusText }}</span>
    <span class="tag-label">{{ store.updatedAtLabel }}</span>
  </header>
</template>

<style scoped>
.status-bar {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: 8px;
  margin: 4px 0 10px;
  min-height: 24px;
}

.tag-label {
  font-size: 12px;
  color: var(--muted);
}

.link {
  color: var(--coin);
  cursor: pointer;
}

.dot::before {
  content: '';
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 4px;
  vertical-align: middle;
  background: var(--muted);
}

.dot.open::before {
  background: var(--down);
}

.dot.connecting::before,
.dot.stale::before {
  background: var(--warning);
}

.dot.mock::before {
  background: var(--muted);
}

.dot.closed::before {
  background: var(--up);
}

@media (min-width: 760px) {
  .status-bar {
    justify-content: flex-start;
    margin-bottom: 14px;
  }

  .tag-label {
    font-size: 13px;
  }
}
</style>
