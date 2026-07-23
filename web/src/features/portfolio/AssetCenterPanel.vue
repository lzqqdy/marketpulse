<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useAuthStore } from '@/features/auth/stores/auth'
import AssetHoldingsPanel from './AssetHoldingsPanel.vue'
import AssetOverviewCard from './AssetOverviewCard.vue'
import AssetReportsPanel from './AssetReportsPanel.vue'
import AssetSnapshotsPanel from './AssetSnapshotsPanel.vue'
import * as api from './api'
import type { PortfolioOverview } from './types'

type CenterTab = 'overview' | 'reports'

const auth = useAuthStore()
const centerTab = ref<CenterTab>('overview')
const overview = ref<PortfolioOverview | null>(null)
const loading = ref(false)
const error = ref('')
const holdingsRef = ref<{ reload: () => Promise<void> } | null>(null)
const snapshotsRef = ref<{ reload: () => Promise<void> } | null>(null)
const reportsRef = ref<{ reload: () => Promise<void> } | null>(null)

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
  if (centerTab.value === 'reports') {
    await reportsRef.value?.reload()
  }
}

function switchTab(tab: CenterTab) {
  centerTab.value = tab
  if (tab === 'reports') {
    void reportsRef.value?.reload()
  }
}

onMounted(async () => {
  await refreshOverview()
  timer = setInterval(() => {
    if (centerTab.value === 'overview') {
      void refreshOverview()
    }
  }, 5000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="asset-center">
    <div class="subtabs" role="tablist" aria-label="资产中心分区">
      <button
        type="button"
        class="subtab"
        :class="{ active: centerTab === 'overview' }"
        role="tab"
        :aria-selected="centerTab === 'overview'"
        @click="switchTab('overview')"
      >
        总览
      </button>
      <button
        type="button"
        class="subtab"
        :class="{ active: centerTab === 'reports' }"
        role="tab"
        :aria-selected="centerTab === 'reports'"
        @click="switchTab('reports')"
      >
        资产报告
      </button>
    </div>

    <p v-if="error && centerTab === 'overview'" class="banner-err">{{ error }}</p>

    <template v-if="centerTab === 'overview'">
      <section class="user-card">
        <AssetOverviewCard :overview="overview" :loading="loading" />
      </section>
      <section class="user-card">
        <AssetHoldingsPanel ref="holdingsRef" @saved="onSaved" />
      </section>
      <section class="user-card">
        <AssetSnapshotsPanel ref="snapshotsRef" />
      </section>
    </template>

    <section v-else class="user-card">
      <AssetReportsPanel ref="reportsRef" />
    </section>
  </div>
</template>

<style scoped>
.asset-center {
  display: grid;
  gap: 16px;
  min-width: 0;
  max-width: 100%;
}

.subtabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.subtab {
  border: 1px solid var(--line);
  background: transparent;
  color: var(--muted);
  border-radius: 8px;
  padding: 7px 14px;
  font-size: 13px;
  cursor: pointer;
}

.subtab.active {
  color: var(--text);
  border-color: color-mix(in srgb, var(--accent) 55%, var(--line));
  background: color-mix(in srgb, var(--accent) 14%, transparent);
  font-weight: 600;
}

.asset-center :deep(.user-card) {
  min-width: 0;
  max-width: 100%;
  box-sizing: border-box;
  overflow-x: hidden;
}

.banner-err {
  margin: 0;
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid color-mix(in srgb, var(--warning) 45%, var(--line));
  color: var(--warning);
  font-size: 13px;
  background: color-mix(in srgb, var(--warning) 10%, transparent);
  overflow-wrap: anywhere;
}

@media (max-width: 680px) {
  .asset-center {
    gap: 12px;
  }

  .asset-center :deep(.user-card) {
    padding: 12px;
  }

  .subtabs {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0;
    border: 1px solid var(--line);
    border-radius: 8px;
    overflow: hidden;
  }

  .subtab {
    border: 0;
    border-radius: 0;
    padding: 10px 8px;
    text-align: center;
  }

  .subtab + .subtab {
    border-left: 1px solid var(--line);
  }
}
</style>
