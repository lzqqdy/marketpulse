# Research: AI 行情分析助手形态选型

**Feature**: `007-ai-assistant`  
**Date**: 2026-07-23  
**Status**: Accepted for V1

## 问题

MarketPulse 需要「分析行情 + 前端持续对话」的 AI 能力。开源领域存在两类主流方案，需选定 V1 范式与边界。

## 选项对比

| 方案 | 代表 | 交互 | 优点 | 缺点 | 与 MP 匹配 |
|------|------|------|------|------|------------|
| A. 对话 Copilot | OpenBB Agent Rita | 多轮聊天 + SSE + 看板上下文 + tools | 贴合看板；可追问；流式体验好；成本可控 | 深度不如「模拟交易台」一次长跑 | **高** |
| B. 多 Agent 交易台 | TradingAgents、ai-hedge-fund | 选标的 → 流水线 → BUY/HOLD/SELL | 社区热、结构化结论、可辩论 | 慢、贵；非持续对话；难嵌看板闲聊 | 中（适合三期按钮） |
| C. 自动交易 OS | NOFX、OpenProphet | 心跳循环 + 下单 | 端到端执行 | 超范围、风控重、个人站风险高 | **低** |
| D. 纯 Prompt 无工具 | 单次 ChatCompletion | 聊天 | 实现快 | 价格幻觉；不可信 | **否决** |

## 决策

**V1 采用方案 A（OpenBB Rita 范式）**：

1. 单 Agent + tool calling 循环（不必上 LangGraph 多图）。
2. `POST` chat + **SSE** 流式事件。
3. 请求携带 **page context**（focus symbol 等）。
4. 会话与消息持久化，支持续聊。
5. 工具封装现有 `MarketDataService`，禁止直连交易所。

**明确不做（V1）**：方案 B 全量流水线、方案 C 下单、方案 D 无工具瞎聊。

**预留**：三期可将「深度分析」做成一次性 job，产出报告后再用同一对话 UI 追问（A 宿主 + B 插件）。

## Rita 范式映射到 MarketPulse

| Rita / OpenBB | MarketPulse V1 |
|---------------|----------------|
| Workspace chat UI | `web/src/features/ai/` 对话抽屉或页 |
| `POST /v1/query` + SSE | `POST /api/v1/ai/chat` + SSE |
| Dashboard / widget context | `context.focusSymbol` / `page` / `visibleSymbols` |
| Widget data tools | `get_quote`、`get_snapshot_summary`、`get_klines_summary`、`get_express_news`、`get_market_breadth` |
| Citations / artifacts | 一期：文本内引用 symbol/新闻标题；二期：可点卡片打开 K 线抽屉 |
| MCP 扩展工具 | 一期不做；配置预留后续 |

## 关键风险

| 风险 | 缓解 |
|------|------|
| LLM 费用 | `ai.enabled`、日配额、`max_tool_rounds`、K 线摘要化 |
| 幻觉报价 | 强制 tool；system prompt；无工具结果不报精确价 |
| 拖垮行情进程 | 独立模块、超时、不共享行情热路径锁 |
| 上下文爆炸 | 历史截断；工具结果裁剪 |
| Key 泄露 | 仅服务端 config / env |

## 结论

先抄 Rita 的**对话形态与契约**，不抄其 Workspace 专有 widget 协议全文；工具与数据用 MarketPulse 已有行情服务。此决策写入 [spec.md](./spec.md)「已确认决策」。

## LLM Provider（V1）

| 项 | 取值 |
|----|------|
| Provider | DeepSeek（用户现有 API Key） |
| Base URL | `https://api.deepseek.com` |
| 默认 model | `deepseek-v4-flash`（便宜、适合高频率对话+工具） |
| 可选 model | `deepseek-v4-pro`（更强推理，更贵、并发更低） |
| 协议 | OpenAI Chat Completions；`tools` / `tool_calls` / `stream` 均支持 |
| 注意 | 不存在 API model id `deepseek-v4`；thinking 模式做 tool loop 时需回传 `reasoning_content`，V1 默认 **non-thinking** 简化实现 |

官方入口：https://api-docs.deepseek.com/ · 定价：https://api-docs.deepseek.com/quick_start/pricing/
