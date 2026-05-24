import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { fetchHealthz, fetchSnapshot, type HealthzResponse } from '@/features/market/api/http'
import { connectMarketStream } from '@/features/market/api/marketWs'
import { createEmptySnapshot, createMockSnapshot } from '@/features/market/mock/data'
import type { AlphaSnapshot, IndexQuote, MarketSnapshot, Quote } from '@/features/market/types/market'
import { formatNumber } from '@/utils/format'

const COIN_ICONS: Record<string, string> = {
  BTC: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/btc.svg',
  ETH: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/eth.svg',
  BNB: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/bnb.svg',
  LTC: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/ltc.svg',
  FIL: 'https://cdn.jsdelivr.net/npm/cryptocurrency-icons@0.18.1/svg/color/fil.svg',
}

function enrichQuotes(quotes: Quote[]): Quote[] {
  return quotes.map((q, i) => ({
    ...q,
    rank: q.rank || i + 1,
    iconUrl: q.iconUrl || COIN_ICONS[q.symbol],
    updatedAt: typeof q.updatedAt === 'string' ? q.updatedAt : new Date().toISOString(),
  }))
}

function normalizeIndices(
  next: IndexQuote[] | null | undefined,
  prev: IndexQuote[] | null | undefined,
): IndexQuote[] {
  if (Array.isArray(next) && next.length > 0) return next
  if (Array.isArray(prev) && prev.length > 0) return prev
  return []
}

function normalizeAlpha(
  next: AlphaSnapshot | null | undefined,
  prev: AlphaSnapshot | null | undefined,
): AlphaSnapshot {
  if (next && (next.indices?.length || next.stocks?.length)) return next
  if (prev) return prev
  return { indices: [], stocks: [] }
}

export const useMarketStore = defineStore('market', () => {
  const snapshot = ref<MarketSnapshot>(createEmptySnapshot())
  const wsStatus = ref<'mock' | 'connecting' | 'open' | 'closed'>('connecting')
  const live = ref(false)
  const ingestHealth = ref<HealthzResponse | null>(null)
  const nowMs = ref(Date.now())

  let disconnectWs: (() => void) | null = null
  let healthTimer: ReturnType<typeof setInterval> | null = null
  let clockTimer: ReturnType<typeof setInterval> | null = null
  let resumeListenerInstalled = false
  let initialFallbackTimer: ReturnType<typeof setTimeout> | null = null

  const STALE_QUOTE_MS = 45_000

  const quotes = computed(() => snapshot.value.quotes)
  const rates = computed(() => snapshot.value.rates)
  const macro = computed(() => snapshot.value.macro)
  const indices = computed(() => snapshot.value.indices ?? [])
  const alpha = computed(() => snapshot.value.alpha ?? { indices: [], stocks: [] })

  const updatedAtLabel = computed(() => {
    const t = snapshot.value.quotes[0]?.updatedAt
    if (!t) return '--'
    const d = new Date(t)
    return Number.isNaN(d.getTime()) ? '--' : d.toLocaleString('zh-CN', { hour12: false })
  })

  const binanceWsState = computed(() => ingestHealth.value?.ingest?.binance_ws ?? 'unknown')

  const quotesStale = computed(() => {
    const t = snapshot.value.quotes[0]?.updatedAt
    if (!t) return true
    const ms = new Date(t).getTime()
    return Number.isNaN(ms) || nowMs.value - ms > STALE_QUOTE_MS
  })

  /** 仅当 Binance 入站正常且报价未过期时为 true */
  const binanceFeedLive = computed(
    () => binanceWsState.value === 'connected' && !quotesStale.value,
  )

  const feedStatus = computed<'live' | 'stale' | 'reconnecting' | 'offline'>(() => {
    if (!live.value) return 'offline'
    const s = binanceWsState.value
    if (s === 'connected' && !quotesStale.value) return 'live'
    if (s === 'connecting' || s === 'reconnecting' || s === 'starting') return 'reconnecting'
    if (s === 'connected' && quotesStale.value) return 'stale'
    return 'offline'
  })

  function marketCapLabel(symbol: string): string {
    const q = snapshot.value.quotes.find((row) => row.symbol === symbol)
    if (!q?.marketCapUsd) return '--'
    return formatNumber(q.marketCapUsd, 2)
  }

  function applySnapshot(data: Partial<MarketSnapshot>) {
    const prev = snapshot.value
    snapshot.value = {
      version: data.version ?? prev.version,
      ts: data.ts ?? prev.ts,
      quotes: enrichQuotes(data.quotes ?? prev.quotes ?? []),
      rates: data.rates ?? prev.rates,
      indices: normalizeIndices(data.indices, prev.indices),
      alpha: normalizeAlpha(data.alpha, prev.alpha),
      macro: data.macro ?? prev.macro,
    }
  }

  function applyQuotes(data: Quote[], version: number) {
    const enriched = enrichQuotes(data)
    snapshot.value = {
      ...snapshot.value,
      quotes: enriched,
      version,
    }
  }

  function stopHealthPoll() {
    if (healthTimer) {
      clearInterval(healthTimer)
      healthTimer = null
    }
  }

  async function pollHealth() {
    try {
      ingestHealth.value = await fetchHealthz()
    } catch {
      /* ignore */
    }
  }

  function startHealthPoll() {
    stopHealthPoll()
    void pollHealth()
    healthTimer = setInterval(() => void pollHealth(), 10_000)
  }

  function stopClock() {
    if (clockTimer) {
      clearInterval(clockTimer)
      clockTimer = null
    }
  }

  function startClock() {
    stopClock()
    nowMs.value = Date.now()
    clockTimer = setInterval(() => {
      nowMs.value = Date.now()
    }, 5_000)
  }

  async function refreshSnapshot() {
    try {
      const snap = await fetchSnapshot()
      applySnapshot(snap)
    } catch {
      // 后台恢复时网络可能还没唤醒，交给 WS 自动重连继续补。
    }
  }

  function disconnectStream() {
    disconnectWs?.()
    disconnectWs = null
  }

  function connectStream() {
    disconnectStream()
    wsStatus.value = 'connecting'
    disconnectWs = connectMarketStream(['quotes', 'rates', 'indices', 'alpha', 'macro'], {
      onOpen: () => {
        wsStatus.value = 'open'
      },
      onSnapshot: (msg) => {
        applySnapshot(msg.data)
        wsStatus.value = 'open'
      },
      onQuotes: (msg) => {
        applyQuotes(msg.data, msg.version)
        wsStatus.value = 'open'
      },
      onRates: (msg) => {
        snapshot.value = {
          ...snapshot.value,
          rates: msg.data,
          version: msg.version,
        }
      },
      onIndices: (msg) => {
        if (!Array.isArray(msg.data)) return
        snapshot.value = {
          ...snapshot.value,
          indices: msg.data,
          version: msg.version,
        }
      },
      onMacro: (msg) => {
        snapshot.value = {
          ...snapshot.value,
          macro: msg.data,
          version: msg.version,
        }
      },
      onAlpha: (msg) => {
        snapshot.value = {
          ...snapshot.value,
          alpha: msg.data,
          version: msg.version,
        }
      },
      onClose: () => {
        wsStatus.value = 'closed'
      },
      onError: () => {
        if (wsStatus.value === 'open' || snapshot.value.quotes.length > 0) {
          return
        }
        initMock()
      },
    })
  }

  function handlePageResume() {
    if (!live.value) return
    nowMs.value = Date.now()
    void refreshSnapshot()
    void pollHealth()
    if (wsStatus.value === 'closed' || quotesStale.value) {
      connectStream()
    }
  }

  function installResumeListeners() {
    if (resumeListenerInstalled) return
    resumeListenerInstalled = true
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('focus', handlePageResume)
    window.addEventListener('pageshow', handlePageResume)
  }

  function removeResumeListeners() {
    if (!resumeListenerInstalled) return
    resumeListenerInstalled = false
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    window.removeEventListener('focus', handlePageResume)
    window.removeEventListener('pageshow', handlePageResume)
  }

  function handleVisibilityChange() {
    if (document.visibilityState === 'visible') {
      handlePageResume()
    }
  }

  function clearInitialFallback() {
    if (initialFallbackTimer) {
      clearTimeout(initialFallbackTimer)
      initialFallbackTimer = null
    }
  }

  function teardown() {
    clearInitialFallback()
    disconnectStream()
    stopHealthPoll()
    stopClock()
    stopMockTick()
    removeResumeListeners()
  }

  /** 连接真实行情：REST 首屏 + WS 推送 */
  async function initLive() {
    teardown()
    live.value = true
    wsStatus.value = 'connecting'
    snapshot.value = createEmptySnapshot()

    startClock()
    installResumeListeners()
    await refreshSnapshot()
    startHealthPoll()
    connectStream()

    initialFallbackTimer = setTimeout(() => {
      initialFallbackTimer = null
      if (live.value && wsStatus.value === 'connecting' && snapshot.value.quotes.length === 0) {
        initMock()
      }
    }, 5000)
  }

  function initMock() {
    teardown()
    live.value = false
    snapshot.value = createMockSnapshot()
    wsStatus.value = 'mock'
    startMockTick()
  }

  let tickTimer: ReturnType<typeof setInterval> | null = null

  function startMockTick(intervalMs = 2500) {
    stopMockTick()
    tickTimer = setInterval(() => {
      const list = snapshot.value.quotes
      const q = list[Math.floor(Math.random() * list.length)]
      if (!q) return
      const delta = (Math.random() - 0.5) * q.priceUsdt * 0.0012
      applyQuotes(
        list.map((row) =>
          row.symbol === q.symbol
            ? {
                ...row,
                priceUsdt: Math.max(0.0001, row.priceUsdt + delta),
                priceCny: (row.priceCny / row.priceUsdt) * Math.max(0.0001, row.priceUsdt + delta),
                changeDayPct: row.changeDayPct,
                change24hPct: row.change24hPct,
              }
            : row,
        ),
        snapshot.value.version + 1,
      )
    }, intervalMs)
  }

  function stopMockTick() {
    if (tickTimer) {
      clearInterval(tickTimer)
      tickTimer = null
    }
  }

  function bumpQuote(symbol: string, patch: Partial<Quote>) {
    applyQuotes(
      snapshot.value.quotes.map((q) =>
        q.symbol === symbol
          ? {
              ...q,
              ...patch,
              updatedAt: new Date().toISOString(),
              priceCny:
                patch.priceUsdt != null
                  ? patch.priceUsdt * (snapshot.value.rates.usdtCny || 7.26)
                  : q.priceCny,
            }
          : q,
      ),
      snapshot.value.version + 1,
    )
  }

  return {
    snapshot,
    wsStatus,
    feedStatus,
    binanceFeedLive,
    binanceWsState,
    ingestHealth,
    live,
    quotes,
    rates,
    macro,
    indices,
    alpha,
    updatedAtLabel,
    marketCapLabel,
    applySnapshot,
    applyQuotes,
    bumpQuote,
    initLive,
    initMock,
    stopMockTick,
    teardown,
  }
})
