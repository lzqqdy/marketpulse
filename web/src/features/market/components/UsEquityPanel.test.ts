import { afterEach, describe, expect, it, vi } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia } from 'pinia'
import UsEquityPanel from './UsEquityPanel.vue'
import { useMarketStore } from '../stores/market'

afterEach(() => vi.unstubAllGlobals())

async function renderPanel(wide: boolean, count = 12) {
  vi.stubGlobal('window', { matchMedia: () => ({ matches: wide }) })
  const pinia = createPinia()
  const store = useMarketStore(pinia)
  const tickers = ['qqq', 'spy', 'aapl', 'msft', 'nvda', 'amzn', 'googl', 'meta', 'tsla', 'mu', 'tsm', 'skhy']
  store.snapshot.indices = tickers.slice(0, count).map((ticker) => ({
    id: `us-${ticker}`, name: ticker, price: 100, changePct: 1, updatedAt: new Date().toISOString(),
  }))
  return renderToString(createSSRApp(UsEquityPanel).use(pinia))
}

describe('US equity responsive defaults', () => {
  it('shows all mobile cards by default and keeps the collapse control', async () => {
    const html = await renderPanel(false)
    expect(html.match(/class="us-card"/g)).toHaveLength(12)
    expect(html).toContain('aria-label="收起美股列表"')
    expect(html).toContain('aria-expanded="true"')
  })

  it('preserves the desktop three-row collapsed default', async () => {
    const html = await renderPanel(true)
    expect(html.match(/class="us-card"/g)).toHaveLength(9)
    expect(html).toContain('aria-label="展开美股列表"')
    expect(html).toContain('aria-expanded="false"')
  })

  it('omits the toggle when all rows fit in the collapsed layout', async () => {
    const html = await renderPanel(false, 6)
    expect(html.match(/class="us-card"/g)).toHaveLength(6)
    expect(html).not.toContain('class="us-toggle"')
  })
})
