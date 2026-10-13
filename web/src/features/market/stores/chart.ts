import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { connectKlineWs } from '@/features/market/api/klineWs'
import { fetchIndexKlines } from '@/features/market/api/klines'
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
  let requestVersion = 0
  let controller: AbortController | null = null
  let timeout: ReturnType<typeof setTimeout> | null = null
  let retry: ReturnType<typeof setTimeout> | null = null

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
    requestVersion++
    controller?.abort()
    controller = null
    if (timeout) clearTimeout(timeout)
    if (retry) clearTimeout(retry)
    timeout = retry = null
    disconnect?.()
    disconnect = null
    wsLive.value = false
  }

  function connectWs() {
    teardownWs()
    const version = requestVersion
    const requestedSymbol = symbol.value
    const requestedInterval = normalizeInterval(interval.value)
    const current = () => visible.value && version === requestVersion
    function reconnect(message: string) {
      if (!current() || retry) return
      loading.value = false
      wsLive.value = false
      error.value = message
      retry = setTimeout(() => { if (current()) connectWs() }, 3000)
    }
    loading.value = true
    error.value = ''
    source.value = ''
    interval.value = normalizeInterval(interval.value)

    disconnect = connectKlineWs(symbol.value, interval.value, {
      onOpen: () => {
        if (!current()) return
      },
      onSnapshot: (msg) => {
        if (!current() || msg.symbol !== requestedSymbol || msg.interval !== requestedInterval) return
        if (timeout) clearTimeout(timeout)
        if (retry) clearTimeout(retry)
        timeout = retry = null
        wsLive.value = true
        candles.value = msg.candles
        source.value = kind.value === 'alpha' ? msg.source : msg.source + '-ws'
        loading.value = false
        error.value = ''
      },
      onUpdate: (msg) => {
        if (!current() || msg.symbol !== requestedSymbol || msg.interval !== requestedInterval) return
        candles.value = mergeCandle(candles.value, msg.candle)
        if (kind.value === 'crypto') {
          market.bumpQuote(msg.symbol, { priceUsdt: msg.candle.close })
        }
      },
      onError: (msg) => {
        if (!current()) return
        wsLive.value = false
        error.value = msg.message
        loading.value = false
        if (timeout) clearTimeout(timeout)
        timeout = null
        if (msg.code === 'WS_ERROR' || msg.code === 'PARSE_ERROR') reconnect(msg.message)
      },
      onClose: () => {
        reconnect('连接已断开，正在重连')
      },
    })

    timeout = setTimeout(() => reconnect('行情加载超时，正在重试'), 15000)
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

  function normalizeInterval(iv: unknown): KlineInterval {
    const allowed: KlineInterval[] = ['1m', '5m', '15m', '1h', '1d', '1w']
    if (typeof iv === 'string' && allowed.includes(iv as KlineInterval)) {
      return iv as KlineInterval
    }
    return '1d'
  }

  async function loadIndexKlines() {
    teardownWs()
    const version = requestVersion
    controller = new AbortController()
    const signal = controller.signal
    loading.value = true
    error.value = ''
    source.value = ''
    wsLive.value = false
    interval.value = normalizeInterval(interval.value)

    try {
      const res = await fetchIndexKlines(symbol.value, interval.value, KLINE_FETCH_LIMIT, signal)
      if (version !== requestVersion || !visible.value) return
      candles.value = res.candles
      source.value = res.source
      pairLabel.value = res.pair
      error.value = ''
    } catch {
      if (version !== requestVersion || !visible.value) return
      error.value = '暂无该指数 K 线数据'
    } finally {
      if (version === requestVersion) loading.value = false
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
    interval.value = normalizeInterval(interval.value)
    if (!['15m', '1h', '1d', '1w'].includes(interval.value)) {
      interval.value = '1d'
    }
    visible.value = true
    void loadIndexKlines()
  }

  function setKlineInterval(iv: KlineInterval) {
    if (interval.value !== normalizeInterval(iv)) candles.value = []
    interval.value = normalizeInterval(iv)
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
    setKlineInterval,
  }
})
