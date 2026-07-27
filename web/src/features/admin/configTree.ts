import { dump, load } from 'js-yaml'
import type { ConfigGap, FieldSchema } from './types'

export type ConfigTree = Record<string, unknown>

export function parseYamlTree(text: string): ConfigTree {
  const doc = load(text)
  if (!doc || typeof doc !== 'object' || Array.isArray(doc)) {
    throw new Error('配置根节点必须是对象')
  }
  return doc as ConfigTree
}

export function dumpYamlTree(tree: ConfigTree): string {
  return dump(tree, {
    lineWidth: 120,
    noRefs: true,
    quotingType: '"',
  })
}

export function getAtPath(tree: ConfigTree, path: string): unknown {
  const parts = path.split('.').filter(Boolean)
  let cur: unknown = tree
  for (const p of parts) {
    if (!cur || typeof cur !== 'object' || Array.isArray(cur)) return undefined
    cur = (cur as ConfigTree)[p]
  }
  return cur
}

export function setAtPath(tree: ConfigTree, path: string, value: unknown): void {
  const parts = path.split('.').filter(Boolean)
  let cur: ConfigTree = tree
  for (let i = 0; i < parts.length - 1; i++) {
    const p = parts[i]
    const next = cur[p]
    if (!next || typeof next !== 'object' || Array.isArray(next)) {
      cur[p] = {}
    }
    cur = cur[p] as ConfigTree
  }
  cur[parts[parts.length - 1]] = value
}

export function applySelectedGaps(
  tree: ConfigTree,
  gaps: ConfigGap[],
  selected: Record<string, boolean>,
  overrides: Record<string, unknown>,
): ConfigTree {
  const clone = structuredClone(tree) as ConfigTree
  for (const g of gaps) {
    if (!selected[g.path]) continue
    if (g.kind === 'missing_in_example') continue
    const value = Object.prototype.hasOwnProperty.call(overrides, g.path)
      ? overrides[g.path]
      : g.exampleValue
    setAtPath(clone, g.path, value)
  }
  return clone
}

export function schemaMap(schema: FieldSchema[]): Map<string, FieldSchema> {
  const m = new Map<string, FieldSchema>()
  for (const s of schema) m.set(s.path, s)
  return m
}

export function kindLabel(kind: string): string {
  switch (kind) {
    case 'missing_in_live':
      return 'example 有 / live 无'
    case 'missing_in_example':
      return '仅 live 存在'
    case 'type_mismatch':
      return '类型不一致'
    default:
      return kind
  }
}

export function formatPreview(value: unknown): string {
  if (value === undefined || value === null) return '—'
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}
