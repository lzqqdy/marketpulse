<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useAuthStore } from '@/features/auth/stores/auth'
import AssetHoldingsPanel from './AssetHoldingsPanel.vue'
import AssetOverviewCard from './AssetOverviewCard.vue'
import AssetSnapshotsPanel from './AssetSnapshotsPanel.vue'
import * as api from './api'
import type { PortfolioOverview } from './types'

const auth = useAuthStore()
const overview = ref<PortfolioOverview | null>(null)
const loading = ref(false)
const error = ref('')
const holdingsRef = ref<{ reload: () => Promise<void> } | null>(null)
const snapshotsRef = ref<{ reload: () => Promise<void> } | null>(null)

let timer: ReturnType<typeof setInterval> | null = null

async function refreshOverview() {
  if (!auth.token) return
  loading.value = true
  try {
    overview.value = await api.getOverview(auth.token)
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '总览加载失败'
  } finally {
    loading.value = false
  }
}

async function onSaved() {
  await refreshOverview()
  await snapshotsRef.value?.reload()
}

onMounted(async () => {
  await refreshOverview()
  timer = setInterval(() => {
    void refreshOverview()
  }, 5000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="asset-center">
    <p v-if="error" class="banner-err">{{ error }}</p>
    <section class="user-card">
      <AssetHoldingsPanel ref="holdingsRef" @saved="onSaved" />
    </section>
    <section class="user-card">
      <AssetOverviewCard :overview="overview" :loading="loading" />
    </section>
    <section class="user-card">
      <AssetSnapshotsPanel ref="snapshotsRef" />
    </section>
  </div>
</template>

<style scoped>
.asset-center {
  display: grid;
  gap: 16px;
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
</style>
