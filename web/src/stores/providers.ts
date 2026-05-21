import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchProviderStatus } from '@/api/providers'
import type { ProviderHealth, ProviderStatusResponse } from '@/types/providers'

const POLL_INTERVAL_MS = 30_000

export const useProviderStore = defineStore('providers', () => {
  const status = ref<ProviderStatusResponse | null>(null)
  const loading = ref(false)
  const lastError = ref('')
  let timer: ReturnType<typeof setInterval> | null = null
  let resumeListenerInstalled = false

  const providers = computed(() => status.value?.providers ?? [])
  const overall = computed(() => status.value?.overall ?? null)
  const abnormalCount = computed(
    () => providers.value.filter((p) => !['healthy', 'disabled'].includes(p.status)).length,
  )

  const groups = computed(() => {
    const labels: Record<string, string> = {
      crypto: '实时行情',
      index: '指数行情',
      forex: '汇率',
      macro: '宏观指标',
      derivatives: '衍生品',
      alpha: 'Alpha',
      other: '其他',
    }
    const order = ['crypto', 'index', 'forex', 'macro', 'derivatives', 'alpha', 'other']
    const bucket = new Map<string, ProviderHealth[]>()
    for (const provider of providers.value) {
      const key = provider.category || 'other'
      bucket.set(key, [...(bucket.get(key) ?? []), provider])
    }
    return [...bucket.entries()]
      .sort(([a], [b]) => {
        const ai = order.indexOf(a)
        const bi = order.indexOf(b)
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
      })
      .map(([category, rows]) => ({
        category,
        label: labels[category] ?? category,
        rows,
      }))
  })

  async function refresh() {
    loading.value = true
    try {
      status.value = await fetchProviderStatus()
      lastError.value = ''
    } catch (err) {
      lastError.value = err instanceof Error ? err.message : '数据源状态请求失败'
    } finally {
      loading.value = false
    }
  }

  function start() {
    stop()
    void refresh()
    timer = setInterval(() => void refresh(), POLL_INTERVAL_MS)
    installResumeListeners()
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    removeResumeListeners()
  }

  function handleResume() {
    void refresh()
  }

  function handleVisibilityChange() {
    if (document.visibilityState === 'visible') {
      handleResume()
    }
  }

  function installResumeListeners() {
    if (resumeListenerInstalled) return
    resumeListenerInstalled = true
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('focus', handleResume)
    window.addEventListener('pageshow', handleResume)
  }

  function removeResumeListeners() {
    if (!resumeListenerInstalled) return
    resumeListenerInstalled = false
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    window.removeEventListener('focus', handleResume)
    window.removeEventListener('pageshow', handleResume)
  }

  return {
    status,
    providers,
    overall,
    groups,
    abnormalCount,
    loading,
    lastError,
    refresh,
    start,
    stop,
  }
})
