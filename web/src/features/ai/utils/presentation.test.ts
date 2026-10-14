import { describe, expect, it } from 'vitest'
import type { AiEvidence, ChatMessage } from '../types'
import { renderAssistantHtml } from './renderMarkdown'
import { currentEvidence, evidenceTime, safeEvidenceUrl } from './evidencePresentation'
import { finishTool, interruptTools, restoredSteps, startTool } from './toolProgress'
import { ANALYSIS_TEMPLATES, templatePrompt } from './templates'

describe('evidence citations and source safety', () => {
  it('only links evidence belonging to this answer, never model-invented IDs', () => {
    const html = renderAssistantHtml('BTC 价格 [证据:ev_known] [证据:ev_fake] <img onerror=alert(1)>', ['BTC'], ['ev_known'])
    expect(html).toContain('data-evidence-id="ev_known"')
    expect(html).not.toContain('data-evidence-id="ev_fake"')
    expect(html).toContain('来源未验证')
    expect(html).not.toContain('<img')
    expect(html.match(/data-symbol="BTC"/g)).toHaveLength(1)
  })
  it('rejects unsafe external links and expired evidence', () => {
    expect(safeEvidenceUrl('javascript:alert(1)')).toBeUndefined()
    expect(safeEvidenceUrl('https://user:secret@example.com/')).toBeUndefined()
    expect(safeEvidenceUrl('https://example.com/news')).toBe('https://example.com/news')
    const item = { id: 'ev_known', status: 'ok', expiresAt: '2026-01-01T00:00:00Z' } as AiEvidence
    expect(currentEvidence(item, Date.parse('2026-10-14T00:00:00Z')).status).toBe('stale')
    expect(item.status).toBe('ok')
    expect(evidenceTime('0001-01-01T00:00:00Z')).toBe('时点未知')
  })
})

describe('per-answer tool progress', () => {
  it('tracks repeated tools by call ID, deduplicates results and interrupts pending work', () => {
    const message: ChatMessage = { id: 'answer', role: 'assistant', content: '' }
    startTool(message, 'get_quote', 'a')
    startTool(message, 'get_quote', 'b')
    finishTool(message, { name: 'get_quote', toolCallId: 'b', ok: false, status: 'partial', durationMs: 120 })
    finishTool(message, { name: 'get_quote', toolCallId: 'b', ok: false, status: 'partial', durationMs: 120 })
    interruptTools(message)
    expect(message.toolSteps?.map((step) => step.phase)).toEqual(['interrupted', 'partial'])
    expect(message.toolResults).toHaveLength(1)
    expect(restoredSteps(message.toolResults ?? [])[0].phase).toBe('partial')
  })
  it('templates keep spot/reference focus explicit', () => {
    expect(ANALYSIS_TEMPLATES).toHaveLength(5)
    expect(templatePrompt('symbol_review', { focusSymbol: 'AAPL', assetClass: 'us_equity' })).toContain('股票现货')
    expect(templatePrompt('symbol_review', { focusSymbol: 'AAPL', assetClass: 'us_contract' })).toContain('合约参考')
    expect(templatePrompt('portfolio_review', {})).toContain('只读')
  })
})
