import { computed, type Ref } from 'vue'

/** 大陆习惯：涨红跌绿 */
export function useTrendClass() {
  const priceClass = (value: number) => {
    if (value > 0) return 'up'
    if (value < 0) return 'down'
    return 'flat'
  }

  const badgeClass = (value: number) => {
    if (value > 0) return 'badge-up'
    if (value < 0) return 'badge-down'
    return 'badge-flat'
  }

  return { priceClass, badgeClass }
}

export function useFlashKey(symbol: Ref<string>, price: Ref<number>) {
  return computed(() => `${symbol.value}:${price.value}`)
}
