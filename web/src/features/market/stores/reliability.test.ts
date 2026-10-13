import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useChartStore } from './chart'
import { useMarketStore } from './market'
import { connectKlineWs, type KlineWsCallbacks } from '../api/klineWs'
import { connectMarketStream, type MarketWsCallbacks } from '../api/marketWs'
import { fetchIndexKlines } from '../api/klines'
import { fetchSnapshot, fetchHealthz } from '../api/http'
import type { IndexQuote } from '../types/market'
import type { KlineResponse } from '../types/chart'

vi.mock('../api/klineWs', () => ({ connectKlineWs: vi.fn() }))
vi.mock('../api/marketWs', () => ({ connectMarketStream: vi.fn() }))
vi.mock('../api/klines', () => ({ fetchIndexKlines: vi.fn() }))
vi.mock('../api/http', () => ({ fetchSnapshot: vi.fn(), fetchHealthz: vi.fn() }))

let callbacks: KlineWsCallbacks[]
let marketCallbacks: MarketWsCallbacks
const index = (id: string) => ({ id, name: id, price: 100, changePct: 1, updatedAt: '', stale: false }) as IndexQuote
const response = (symbol: string, close: number): KlineResponse => ({ symbol, pair: symbol, interval: '1d', source: 'test', candles: [{ time: 1, open: close, high: close, low: close, close, volume: 1 }] })

beforeEach(() => {
  vi.useFakeTimers()
  vi.resetAllMocks()
  setActivePinia(createPinia())
  vi.stubGlobal('window', { addEventListener: vi.fn(), removeEventListener: vi.fn() })
  vi.stubGlobal('document', { addEventListener: vi.fn(), removeEventListener: vi.fn() })
  callbacks = []
  vi.mocked(connectKlineWs).mockImplementation((_s, _i, cb) => { callbacks.push(cb); return vi.fn() })
  vi.mocked(connectMarketStream).mockImplementation((_c, cb) => { marketCallbacks = cb; return vi.fn() })
  vi.mocked(fetchSnapshot).mockRejectedValue(new Error('offline'))
  vi.mocked(fetchHealthz).mockRejectedValue(new Error('offline'))
})
afterEach(() => {
  useChartStore().close()
  useMarketStore().teardown()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('real data lifecycle', () => {
  it('keeps real connection after startup failure instead of generating prices', async () => {
    const s = useMarketStore()
    await s.initLive()
    marketCallbacks.onError?.()
    await vi.advanceTimersByTimeAsync(6000)
    expect(s.live).toBe(true)
    expect(s.quotes).toEqual([])
    expect(s.wsStatus).not.toBe('mock')
    marketCallbacks.onSnapshot({ type: 'snapshot', data: { ...s.snapshot, quotes: [{ symbol: 'BTC', priceUsdt: 100, updatedAt: new Date().toISOString() } as never] } })
    expect(s.quotes[0].priceUsdt).toBe(100)
  })

  it('ignores late index responses and aborts previous request', async () => {
    let finishOld!: (r: KlineResponse) => void
    vi.mocked(fetchIndexKlines).mockImplementationOnce(() => new Promise(resolve => { finishOld = resolve }))
      .mockResolvedValueOnce(response('hsi', 200))
    const s = useChartStore()
    s.openIndex(index('sh000001'))
    const signal = vi.mocked(fetchIndexKlines).mock.calls[0][3]
    s.openIndex(index('hsi'))
    await Promise.resolve()
    finishOld(response('sh000001', 100))
    await Promise.resolve()
    expect(signal?.aborted).toBe(true)
    expect(s.candles[0].close).toBe(200)
    expect(s.symbol).toBe('hsi')
  })

  it('ignores callbacks from old websocket and clears candles on interval switch', () => {
    const s = useChartStore()
    s.open('BTC')
    callbacks[0].onSnapshot({ type:'kline_snapshot', ...response('BTC',100) })
    s.setKlineInterval('1h')
    expect(s.candles).toEqual([])
    callbacks[0].onSnapshot({ type:'kline_snapshot', ...response('BTC',999) })
    callbacks[0].onClose?.()
    expect(s.candles).toEqual([])
    callbacks[1].onSnapshot({ type:'kline_snapshot', ...response('BTC',200), interval:'1h' })
    expect(s.candles[0].close).toBe(200)
    expect(s.wsLive).toBe(true)
  })

  it('retries failures without mock data and cancels retries on close', async () => {
    const s = useChartStore()
    s.open('BTC')
    callbacks[0].onError({ type:'error', code:'WS_ERROR', message:'offline' })
    expect(s.candles).toEqual([])
    await vi.advanceTimersByTimeAsync(3000)
    expect(callbacks).toHaveLength(2)
    s.close()
    await vi.advanceTimersByTimeAsync(30000)
    expect(callbacks).toHaveLength(2)
    expect(s.source).not.toBe('mock')
  })

  it('does not apply an index response after closing the drawer', async () => {
    let finish!: (r: KlineResponse) => void
    vi.mocked(fetchIndexKlines).mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const s = useChartStore()
    s.openIndex(index('hsi'))
    s.close()
    finish(response('hsi',999))
    await Promise.resolve()
    expect(s.candles).toEqual([])
    expect(s.visible).toBe(false)
  })

  it('invalidates an old timeout when changing instruments', async () => {
    const s = useChartStore()
    s.open('BTC')
    await vi.advanceTimersByTimeAsync(14000)
    s.open('ETH')
    await vi.advanceTimersByTimeAsync(2000)
    expect(callbacks).toHaveLength(2)
    expect(s.loading).toBe(true)
    expect(s.error).toBe('')
  })

  it('marks upstream disconnect as not live and recovers after a fresh snapshot', () => {
    const s = useChartStore()
    s.open('BTC')
    callbacks[0].onSnapshot({ type: 'kline_snapshot', ...response('BTC', 100) })
    expect(s.wsLive).toBe(true)
    callbacks[0].onError({ type: 'error', code: 'UPSTREAM_ERROR', message: 'upstream disconnected' })
    expect(s.wsLive).toBe(false)
    expect(s.candles[0].close).toBe(100)
    callbacks[0].onSnapshot({ type: 'kline_snapshot', ...response('BTC', 200) })
    expect(s.wsLive).toBe(true)
    expect(s.error).toBe('')
    expect(s.candles[0].close).toBe(200)
  })
})
