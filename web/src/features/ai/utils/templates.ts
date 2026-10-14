import type { AnalysisTemplateId, PageContext } from '../types'

export const ANALYSIS_TEMPLATES: { id: AnalysisTemplateId; label: string }[] = [
  { id: 'market_brief', label: '盘面简报' }, { id: 'symbol_review', label: '单标复盘' },
  { id: 'comparison', label: '标的对比' }, { id: 'portfolio_review', label: '资产体检' },
  { id: 'event_review', label: '异动复盘' },
]

export function templatePrompt(id: AnalysisTemplateId, context: PageContext): string {
  const symbol = context.focusSymbol || 'BTC'
  const reference = context.assetClass === 'us_contract' || context.assetClass === 'alpha' ? '合约参考' : context.assetClass === 'us_equity' ? '股票现货' : ''
  const prompts: Record<AnalysisTemplateId, string> = {
    market_brief: '结合最新盘面做一份简报，列出关键数据、市场分化和风险。',
    symbol_review: `复盘 ${symbol}${reference ? ` 的${reference}` : ''}：报价、走势、相关新闻和风险，注明数据时间。`,
    comparison: '对比 BTC 和 ETH 的报价、走势及风险，使用一致的数据口径。',
    portfolio_review: '只读分析我的资产：估值、集中度、收益口径、价格缺失或过期情况，以及主要风险。',
    event_review: '复盘最近24小时的加密市场异动，区分检测事实、可能解释和待验证信息。',
  }
  return prompts[id]
}
