import type { AiToolResult, ChatMessage, ToolStep } from '../types'
import { toolResultPhase } from './evidence'

const LABELS: Record<string, string> = {
  get_symbol_brief: '标的简报', get_quote: '报价', get_snapshot_summary: '盘面摘要',
  get_macro_metrics: '币圈指标', get_index_board: '指数看板', get_alpha_board: '美股参考',
  get_klines_summary: 'K线摘要', compare_symbols: '标的对比', get_express_news: '快讯',
  get_market_breadth: '行情中心', get_portfolio_overview: '本人资产', get_market_events: '市场事件',
}

export function toolLabel(name: string) { return LABELS[name] || name }

export const PHASE_LABELS = {
  running: '查询中', done: '已获取', error: '失败', partial: '部分可用', stale: '历史数据', interrupted: '已中断',
}

function resultNote(result: AiToolResult): string | undefined {
  if (!result.summary || result.status === 'ok') return undefined
  try {
    const summary = JSON.parse(result.summary)
    const issues: unknown[] = Array.isArray(summary.issues) ? summary.issues : []
    const notes = [summary.error, ...issues.flatMap((item) => {
      if (!item || typeof item !== 'object') return []
      const row = item as Record<string, unknown>
      return [row.error, row.hint]
    })].filter((value): value is string => typeof value === 'string' && !!value)
    return [...new Set(notes)].slice(0, 2).join('；').slice(0, 450) || undefined
  } catch { return result.summary.slice(0, 450) }
}

export function startTool(message: ChatMessage, name: string, callId?: string): ToolStep {
  const steps = message.toolSteps ??= []
  const id = callId || `legacy-${steps.length}`
  const step: ToolStep = { id, name, label: toolLabel(name), phase: 'running' }
  steps.push(step)
  return step
}

export function finishTool(message: ChatMessage, result: AiToolResult): ToolStep {
  const steps = message.toolSteps ??= []
  const step = steps.find((item) => result.toolCallId ? item.id === result.toolCallId : item.name === result.name && item.phase === 'running')
    ?? startTool(message, result.name, result.toolCallId)
  step.phase = toolResultPhase(result)
  step.durationMs = result.durationMs
  step.note = resultNote(result)
  const results = message.toolResults ??= []
  const index = result.toolCallId ? results.findIndex((item) => item.toolCallId === result.toolCallId) : -1
  if (index >= 0) results[index] = result
  else results.push(result)
  return step
}

export function interruptTools(message: ChatMessage) {
  for (const step of message.toolSteps ?? []) if (step.phase === 'running') step.phase = 'interrupted'
}

export function restoredSteps(results: AiToolResult[]): ToolStep[] {
  return results.map((result, i) => ({ id: result.toolCallId || `legacy-${i}`, name: result.name, label: toolLabel(result.name), phase: toolResultPhase(result), durationMs: result.durationMs, note: resultNote(result) }))
}
