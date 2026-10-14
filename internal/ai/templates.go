package ai

import "fmt"

var analysisTemplates = map[string]string{
	"market_brief":     "盘面简报：结论、关键指标与数据时间、市场分化、风险和未知。区分交易日涨跌与24小时涨跌。",
	"symbol_review":    "单标复盘：先明确标的及现货/参考口径，再给报价、走势、相关新闻、风险。相关报道不能直接当作涨跌原因。",
	"comparison":       "标的对比：按一致资产口径查询用户指定标的；列出价格、涨跌周期、走势和风险，不将不同时段或币种直接排名。",
	"portfolio_review": "资产体检：只读本人资产，列出估值、集中度、缺失/过期价格、收益口径和风险。没有现金流核算不能称为真实投资回报率；不修改持仓。",
	"event_review":     "异动复盘：查询市场事件，列出检测时间、状态、信号窗口、事实及待验证解释。事件检测和新闻都不能证明涨跌因果。",
}

func templateInstruction(id string) (string, error) {
	if id == "" {
		return "", nil
	}
	text, ok := analysisTemplates[id]
	if !ok {
		return "", fmt.Errorf("%w: unknown templateId", ErrInvalidInput)
	}
	return "\n\n[analysis_template]\n" + text + "\n[/analysis_template]", nil
}
