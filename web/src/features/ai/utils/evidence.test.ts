import { describe, expect, it } from 'vitest'
import type { AiEvidence } from '../types'
import { chatAssetClass, mergeEvidence, restoreToolResults, toolResultPhase } from './evidence'

const record: AiEvidence = {
  id: 'ev-1', kind: 'quote', instrumentId: 'us-aapl', assetClass: 'us_equity',
  source: 'tencent', asOf: '2026-10-13T12:00:00Z', fetchedAt: '2026-10-13T12:00:00Z',
  expiresAt: '2026-10-13T12:02:00Z', status: 'ok', facts: { price: 200 },
}

describe('AI evidence and tool result compatibility', () => {
  it('keeps US cash equity and contract focus separate', () => {
    expect(chatAssetClass('index', 'us-aapl')).toBe('us_equity')
    expect(chatAssetClass('alpha', 'aapl')).toBe('us_contract')
    expect(chatAssetClass('index', 'sh000001')).toBe('index')
    expect(chatAssetClass('crypto', 'BTC')).toBe('crypto')
  })

  it('never displays partial, stale or failed tools as success', () => {
    expect(toolResultPhase({ name: 'get_quote', ok: true, status: 'partial' })).toBe('partial')
    expect(toolResultPhase({ name: 'get_quote', ok: false, status: 'stale' })).toBe('stale')
    expect(toolResultPhase({ name: 'get_quote', ok: true, status: 'unavailable' })).toBe('error')
    expect(toolResultPhase({ name: 'get_quote', ok: false })).toBe('error')
    expect(toolResultPhase({ name: 'get_quote', ok: true })).toBe('done')
  })

  it('keeps structured facts when deduplicating repeated evidence', () => {
    const updated = { ...record, facts: { price: 202 } }
    const merged = mergeEvidence([record], [updated])
    expect(merged).toEqual([updated])
    expect(record.facts.price).toBe(200)
  })

  it('restores complete facts and marks expired evidence as historical', () => {
    const metadata = { toolResults: [{ name: 'get_quote', ok: true, status: 'ok', evidence: [record] }] }
    const fresh = restoreToolResults(metadata, Date.parse('2026-10-13T12:01:00Z'))
    expect(fresh[0].ok).toBe(true)
    const old = restoreToolResults(metadata, Date.parse('2026-10-13T12:02:00Z'))
    expect(old[0].ok).toBe(false)
    expect(old[0].evidence?.[0].status).toBe('stale')
    expect(old[0].evidence?.[0].facts.price).toBe(200)
    expect(record.status).toBe('ok')
  })

  it('accepts legacy messages without inventing citations', () => {
    expect(restoreToolResults({ grounding: 'legacy' })).toEqual([])
    expect(restoreToolResults(null)).toEqual([])
    expect(restoreToolResults({ toolResults: [null, {}, { name: 'bad' }] })).toEqual([])
  })

  it('distinguishes mixed evidence from an entirely expired result', () => {
    const fresh = { ...record, id: 'ev-fresh', expiresAt: '2026-10-13T13:00:00Z' }
    const metadata = { toolResults: [{ name: 'get_symbol_brief', ok: true, status: 'ok', evidence: [record, fresh] }] }
    const results = restoreToolResults(metadata, Date.parse('2026-10-13T12:03:00Z'))
    expect(results[0].status).toBe('partial')
    expect(results[0].ok).toBe(false)
    expect(results[0].evidence?.[1].status).toBe('ok')
  })
})
