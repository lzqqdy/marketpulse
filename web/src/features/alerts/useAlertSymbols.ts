import { computed, ref } from 'vue'
import { fetchSnapshot } from '@/features/market/api/http'

const FALLBACK_SPOT = ['BTC', 'ETH', 'BNB', 'LTC', 'FIL']
const FALLBACK_INDEX = [
  { id: 'sh000001', name: '上证' },
  { id: 'sz399001', name: '深证' },
  { id: 'sz399006', name: '创业板' },
  { id: 'sh000300', name: '沪深300' },
  { id: 'hsi', name: '恒生' },
  { id: 'ks11', name: '韩国综指' },
]

export function useAlertSymbols() {
  const spotSymbols = ref<string[]>([])
  const indexSymbols = ref<{ id: string; name: string }[]>([])
  const loading = ref(false)
  const hint = ref('')

  const formSymbolOptions = computed(() => ({
    spot: spotSymbols.value.map((s) => ({
      value: s,
      label: s.includes('USDT') ? s : `${s} / ${s}USDT`,
    })),
    index: indexSymbols.value.map((i) => ({
      value: i.id,
      label: i.name && i.name !== i.id ? `${i.name} (${i.id})` : i.id,
    })),
  }))

  function optionsForAssetType(assetType: string): { value: string; label: string }[] {
    if (assetType === 'spot') return formSymbolOptions.value.spot
    if (assetType === 'index') return formSymbolOptions.value.index
    // 全部：合并，index 在前易区分
    return [...formSymbolOptions.value.spot, ...formSymbolOptions.value.index]
  }

  async function loadSymbols() {
    loading.value = true
    hint.value = ''
    try {
      const snap = await fetchSnapshot()
      const spots = (snap.quotes ?? [])
        .map((q) => q.symbol)
        .filter(Boolean)
        .sort((a, b) => a.localeCompare(b))
      spotSymbols.value = spots.length ? spots : [...FALLBACK_SPOT]
      const idxs = (snap.indices ?? [])
        .filter((i) => i.id && !i.stale)
        .map((i) => ({ id: i.id, name: i.name || i.id }))
      indexSymbols.value = idxs.length ? idxs : [...FALLBACK_INDEX]
      if (!spots.length || !idxs.length) {
        hint.value = '行情快照暂无完整标的，已使用常用列表'
      }
    } catch {
      spotSymbols.value = [...FALLBACK_SPOT]
      indexSymbols.value = [...FALLBACK_INDEX]
      hint.value = '行情接口不可用，已使用常用标的列表'
    } finally {
      loading.value = false
    }
  }

  return {
    spotSymbols,
    indexSymbols,
    loading,
    hint,
    formSymbolOptions,
    optionsForAssetType,
    loadSymbols,
  }
}
