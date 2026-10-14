import { describe, expect, it } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import AssistantTrace from './AssistantTrace.vue'
import type { AiEvidence } from '../types'

describe('assistant trace presentation', () => {
  it('renders expiry, factual detail, currency and semantic limitations safely', async () => {
    const record: AiEvidence = { id: 'ev_known', kind: 'quote', source: 'tencent', symbol: 'AAPL', status: 'ok', asOf: '2026-01-01T00:00:00Z', fetchedAt: '2026-01-01T00:00:00Z', expiresAt: '2026-01-01T00:05:00Z', asOfBasis: 'data_time', currency: 'USD', changeBasis: 'trading_session', facts: { price: 200, title: '<script>bad</script>' }, limitations: ['不是合约报价'], url: 'javascript:alert(1)' }
    const html = await renderToString(createSSRApp(AssistantTrace, { message: { id: 'a', role: 'assistant', content: '', evidence: [record], toolSteps: [{ id: 'call', name: 'get_quote', label: '报价', phase: 'partial', durationMs: 120 }] }, selectedId: record.id }))
    expect(html).toContain('历史参考')
    expect(html).toContain('交易日涨跌')
    expect(html).toContain('USD')
    expect(html).toContain('部分可用')
    expect(html).toContain('0.1s')
    expect(html).not.toContain('<script>')
    expect(html).not.toContain('javascript:')
  })
})
