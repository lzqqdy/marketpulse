import type { AiAssetClass, AiEvidence, AiToolResult, ToolStatus } from '../types'

export function chatAssetClass(kind: 'crypto' | 'index' | 'alpha', symbol: string): AiAssetClass {
  if (kind === 'alpha') return 'us_contract'
  if (kind === 'index' && symbol.toLowerCase().startsWith('us-')) return 'us_equity'
  return kind
}

export function toolResultPhase(result: AiToolResult): ToolStatus['phase'] {
  if (result.status === 'partial') return 'partial'
  if (result.status === 'stale') return 'stale'
  if (result.status === 'unavailable') return 'error'
  return result.ok ? 'done' : 'error'
}

export function mergeEvidence(current: AiEvidence[], incoming: AiEvidence[]): AiEvidence[] {
  const map = new Map(current.map((item) => [item.id, item]))
  for (const item of incoming) map.set(item.id, item)
  return [...map.values()]
}

export function restoreToolResults(metadata: unknown, now = Date.now()): AiToolResult[] {
  if (!metadata || typeof metadata !== 'object' || !('toolResults' in metadata)) return []
  const values = metadata.toolResults
  if (!Array.isArray(values)) return []
  return values.filter((value): value is AiToolResult =>
    !!value && typeof value === 'object' && typeof value.name === 'string' && typeof value.ok === 'boolean',
  ).map((value) => {
    const records = (Array.isArray(value.evidence) ? value.evidence : []).filter((item): item is AiEvidence =>
      !!item && typeof item.id === 'string' && typeof item.kind === 'string' && !!item.facts && typeof item.facts === 'object' && !Array.isArray(item.facts),
    ).map((item) => {
      const expires = Date.parse(item.expiresAt)
      const expired = !Number.isFinite(expires) || expires <= now
      return expired && item.status !== 'unavailable' ? { ...item, status: 'stale' as const } : item
    })
    const expired = records.some((item) => item.status === 'stale')
    const allExpired = records.length > 0 && records.every((item) => item.status === 'stale')
    const status = value.status === 'unavailable' ? value.status
      : allExpired ? 'stale' : expired ? 'partial' : value.status
    return { ...value, evidence: records, ok: expired ? false : value.ok, status }
  })
}
