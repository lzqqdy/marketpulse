import { describe, expect, it } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia } from 'pinia'
import MacroGrid from './MacroGrid.vue'
import { useMarketStore } from '../stores/market'

describe('missing macro data', () => {
  it('renders placeholders, never component-local demonstration prices', async () => {
    const app = createSSRApp(MacroGrid).use(createPinia())
    const html = await renderToString(app)
    expect(html).not.toContain('0.0126%')
    expect(html).not.toContain('387.4')
    expect(html).not.toContain('4650')
    expect(html.match(/>--</g)).toHaveLength(13)
  })

  it('distinguishes a real zero funding rate from an uninitialized field', async () => {
    const pinia = createPinia()
    const store = useMarketStore(pinia)
    store.snapshot.macro.funding = { symbol: 'BTCUSDT', rate: 0, nextFundingTime: '', updatedAt: '0001-01-01T00:00:00Z' }
    expect(await renderToString(createSSRApp(MacroGrid).use(pinia))).not.toContain('0.0000%')
    store.snapshot.macro.funding.updatedAt = new Date().toISOString()
    expect(await renderToString(createSSRApp(MacroGrid).use(pinia))).toContain('0.0000%')
  })
})
