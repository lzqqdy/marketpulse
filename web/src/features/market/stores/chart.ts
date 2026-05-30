import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { connectKlineWs } from '@/features/market/api/klineWs'
import { fetchIndexKlines } from '@/features/market/api/klines'
import { generateMockKlines } from '@/features/market/mock/kline'
import type { Candle, KlineInterval } from '@/features/market/types/chart'
import { KLINE_FETCH_LIMIT } from '@/features/market/types/chart'
import type { AlphaQuote, IndexQuote } from '@/features/market/types/market'
import { useMarketStore } from '@/features/market/stores/market'

function mergeCandle(candles: Candle[], c: Candle): Candle[] {
  if (candles.length === 0) return [c]
  const last = candles[candles.length - 1]
  if (last.time === c.time) {
    const next = candles.slice()
    next[next.length - 1] = c
    return next
  }
  if (c.time > last.time) return [...candles, c]
  return candles.map((x) => (x.time === c.time ? c : x))
}

export const useChartStore = defineStore('chart', () => {
  const visible = ref(false)
  const symbol = ref('BTC')
  const kind = ref<'crypto' | 'index' | 'alpha'>('crypto')
  const displayName = ref('BTC')
  const pairLabel = ref('USDT')
  const indexQuote = ref<IndexQuote | null>(null)
  const alphaQuote = ref<AlphaQuote | null>(null)
  const interval = ref<KlineInterval>('1d')
  const loading = ref(false)
  const error = ref('')
  const candles = ref<Candle[]>([])
  const source = ref('')
  const wsLive = ref(false)

  let disconnect: (() => void) | null = null

  const market = useMarketStore()

  const quote = computed(() =>
    kind.value === 'crypto' ? market.quotes.find((q) => q.symbol === symbol.value) : undefined,
  )

  const changePct = computed(() =>
    kind.value === 'crypto'
      ? quote.value?.change24hPct ?? 0
      : kind.value === 'alpha'
        ? alphaQuote.value?.changeDayPct ?? 0
        : indexQuote.value?.changePct ?? 0,
  )

  function teardownWs() {
    disconnect?.()
    disconnect = null
    wsLive.value = false
  }

  function useMockStream() {
    teardownWs()
    loading.value = true
    source.value = 'mock'
    wsLive.value = false
    candles.value = generateMockKlines(symbol.value, interval.value, KLINE_FETCH_LIMIT)
    loading.value = false

    let price = candles.value[candles.value.length - 1]?.close ?? 1000
    const timer = setInterval(() => {
      if (!visible.value) {
        clearInterval(timer)
        return
      }
      const last = candles.value[candles.value.length - 1]
      if (!last) return
      const drift = (Math.random() - 0.5) * price * 0.006
      price = Math.max(0.01, price + drift)
      const c: Candle = {
        time: last.time,
        open: last.open,
        high: Math.max(last.high, price),
        low: Math.min(last.low, price),
        close: price,
        volume: last.volume + Math.random() * 100,
      }
      candles.value = mergeCandle(candles.value, c)
    }, 2000)
    disconnect = () => clearInterval(timer)
  }

  function connectWs() {
    teardownWs()
    loading.value = true
    error.value = ''
    source.value = ''

    disconnect = connectKlineWs(symbol.value, interval.value, {
      onOpen: () => {
        wsLive.value = true
      },
      onSnapshot: (msg) => {
        candles.value = msg.candles
        source.value = kind.value === 'alpha' ? msg.source : msg.source + '-ws'
        loading.value = false
        error.value = ''
      },
      onUpdate: (msg) => {
        candles.value = mergeCandle(candles.value, msg.candle)
        if (kind.value === 'crypto') {
          market.bumpQuote(msg.symbol, { priceUsdt: msg.candle.close })
        }
      },
      onError: (msg) => {
        if (candles.value.length === 0) {
          error.value = msg.message
          loading.value = false
          if (msg.code === 'WS_ERROR' || msg.code === 'PARSE_ERROR') {
            useMockStream()
          }
        }
      },
      onClose: () => {
        wsLive.value = false
      },
    })

    // 若后端未启动，WS 很快失败且无 snapshot → 降级 mock
    setTimeout(() => {
      if (visible.value && loading.value && candles.value.length === 0) {
        useMockStream()
      }
    }, 4000)
  }

  function open(sym: string) {
    const changed = kind.value !== 'crypto' || symbol.value !== sym
    kind.value = 'crypto'
    symbol.value = sym
    displayName.value = sym
    pairLabel.value = 'USDT'
    indexQuote.value = null
    alphaQuote.value = null
    if (changed) candles.value = []
    visible.value = true
    connectWs()
  }

  function openAlpha(item: AlphaQuote) {
    const changed = kind.value !== 'alpha' || symbol.value !== item.symbol
    kind.value = 'alpha'
    symbol.value = item.symbol
    displayName.value = item.name
    pairLabel.value = 'USDT'
    indexQuote.value = null
    alphaQuote.value = item
    if (changed) candles.value = []
    visible.value = true
    connectWs()
  }

  async function loadIndexKlines() {
    teardownWs()
    loading.value = true
    error.value = ''
    source.value = ''
    wsLive.value = false

    try {
      const res = await fetchIndexKlines(symbol.value, interval.value, KLINE_FETCH_LIMIT)
      candles.value = res.candles
      source.value = res.source
      pairLabel.value = res.pair
      error.value = ''
    } catch {
      candles.value = []
      error.value = '暂无该指数 K 线数据'
    } finally {
      loading.value = false
    }
  }

  function openIndex(item: IndexQuote) {
    const changed = kind.value !== 'index' || symbol.value !== item.id
    kind.value = 'index'
    symbol.value = item.id
    displayName.value = item.name
    pairLabel.value = item.id
    indexQuote.value = item
    alphaQuote.value = null
    if (changed) candles.value = []
    if (!['15m', '1h', '1d', '1w'].includes(interval.value)) {
      interval.value = '1d'
    }
    visible.value = true
    void loadIndexKlines()
  }

  function setInterval(iv: KlineInterval) {
    interval.value = iv
    if (kind.value === 'index') {
      void loadIndexKlines()
      return
    }
    connectWs()
  }

  function close() {
    visible.value = false
    teardownWs()
  }

  function reload() {
    if (kind.value === 'index') {
      void loadIndexKlines()
      return
    }
    connectWs()
  }

  return {
    visible,
    symbol,
    kind,
    displayName,
    pairLabel,
    indexQuote,
    alphaQuote,
    interval,
    loading,
    error,
    candles,
    source,
    wsLive,
    quote,
    changePct,
    open,
    openAlpha,
    openIndex,
    close,
    reload,
    setInterval,
  }
})
