# Feature Specification: AI 行情分析助手（对话 Copilot）

**Feature Branch**: `007-ai-assistant`  
**Created**: 2026-07-23  
**Status**: Implemented（一期）  
**Input**: 在 MarketPulse 内做对话型行情分析 Agent（抄 OpenBB Rita 范式）：前端持续多轮对话；Agent 通过工具读取现有行情/快讯等数据作答；可绑定当前页面正在查看的标的上下文。不做自动交易、不做 TradingAgents 多角色辩论流水线（可作为后续可选深度分析）。

## 背景

- `docs/MODULES.md` 已规划独立 `ai` 模块：分析 jobs、prompts、模型响应、insights；只读 `marketdata` / `portfolio` / `users` 公开边界。
- RFC-001 将「AI 分析助手」列为规划项；RFC-004 Phase G。
- 行情能力已具备：`Snapshot` / `Quote` / `Klines` / `MarketCenter` / `ExpressNews` 等，经 `MarketDataService` 对外。
- 用户会话、MySQL、灰度开关模式已在 `users` / `alerts` / `portfolio` 验证。
- 开源调研结论：对话持续交互对齐 **OpenBB Agent Rita**（SSE + 上下文 + 工具）；不采用 TradingAgents 作为 V1 主路径。详见 [research.md](./research.md)。

## 分期范围

| 阶段 | 内容 | 本期 |
|------|------|------|
| **一期（对话 Copilot）** | 登录用户多轮对话；SSE 流式；工具拉行情/K 线摘要/快讯/盘面广度；页面 context；会话持久化；灰度开关与日配额 | ✅ |
| **二期** | 持仓解读工具（依赖 `portfolio`）；引用卡片增强；简单技术指标摘要；会话标题自动生成 | ⏳ |
| **三期（可选）** | 「深度分析」按钮触发精简多角色流水线（TradingAgents 风格）；报告落库后再聊 | ❌ 本期不做 |

## 已确认决策

| # | 议题 | 决策 |
|---|------|------|
| 1 | 产品形态 | **对话 Copilot**（OpenBB Rita 范式），非批量研报流水线、非自动交易 |
| 2 | 模块归属 | 独立 `ai`；API `/api/v1/ai/*`；前端 `web/src/features/ai/` |
| 3 | 数据访问 | **只读** `MarketDataService`（及二期 `portfolio.Service`）；禁止直连交易所 / ingest |
| 4 | 交互通道 | **SSE**（`POST` + `text/event-stream`）；不用行情 WS 承载 LLM token |
| 5 | 持续对话 | MySQL 存会话与消息；请求可带 `conversationId` 续聊 |
| 6 | 页面上下文 | 请求体带 `context`（focus symbol、page、visible symbols 等）；类似 Rita 的 dashboard/widget context |
| 7 | 鉴权 | 与 users 相同 session；未登录 401；`ai.enabled=false` 业务禁用 |
| 8 | 依赖链 | `ai` 需要 LLM 配置 +（会话持久化）`mysql` + `users`；portfolio 工具为可选增强 |
| 9 | 模型接入 | **V1 固定 DeepSeek**：OpenAI-compatible `https://api.deepseek.com` + tool calling；默认模型 `deepseek-v4-flash`（可选 `deepseek-v4-pro`）。无裸 ID `deepseek-v4`。 |
| 10 | 价格 grounding | 数字结论 MUST 来自工具返回；Prompt 明确禁止编造行情 |
| 11 | 免责 | 所有回复附带「非投资建议」类声明（系统层或尾注） |
| 12 | 配额 | 按用户日配额（Redis 计数优先；无 Redis 时降级为 MySQL/内存策略在 plan 钉死） |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 打开助手并提问行情 (Priority: P1)

作为已登录用户，我希望在看板打开 AI 助手，用自然语言询问行情（如「BTC 现在怎么样」「今天市场宽幅如何」），并看到基于真实数据的流式回答。

**Why this priority**: 无单轮可用对话则无产品价值。

**Independent Test**: `ai.enabled=true` 且配置有效 LLM Key；登录后发送一条与当前盘面相关的问题；界面出现流式文本；回答中的价格/涨跌可与看板对照一致（容许延迟）。

**Acceptance Scenarios**:

1. **Given** 用户已登录且 `ai.enabled=true`，**When** 打开助手并发送消息，**Then** 开始 SSE 流式输出助手回复。
2. **Given** 问题需要报价，**When** Agent 调用行情工具，**Then** 回复中的关键数字与工具结果一致，而非模型臆造。
3. **Given** LLM 或工具失败，**When** 请求结束，**Then** 前端收到明确错误事件，会话不处于「永久加载」；已生成的部分文本可保留或回滚策略一致（实现选定并文档化，一期推荐保留已流式内容 + 错误提示）。
4. **Given** `ai.enabled=false`，**When** 调用 chat API，**Then** 返回业务禁用错误；UI 不展示或展示不可用说明。

---

### User Story 2 - 多轮持续对话 (Priority: P1)

作为已登录用户，我希望在同一会话中追问（如「那以太坊呢」「换成 4 小时级别怎么看」），助手记得上文，无需重复说明标的。

**Why this priority**: 明确要求「前端支持持续对话」；Rita 范式的核心。

**Independent Test**: 同一 `conversationId` 连发两问；第二问可不提标的仍指向上文标的；刷新页面后打开该会话，历史消息仍在。

**Acceptance Scenarios**:

1. **Given** 首轮已创建会话，**When** 带 `conversationId` 发送追问，**Then** 助手结合历史消息作答。
2. **Given** 用户刷新浏览器，**When** 打开同一会话，**Then** 可见完整历史（用户/助手消息；工具中间态可折叠或仅存服务端）。
3. **Given** 用户新建会话，**When** 发送消息，**Then** 不污染旧会话上下文。
4. **Given** 用户 A 的 `conversationId`，**When** 用户 B 访问，**Then** 404/403，不可读。

---

### User Story 3 - 绑定当前页面上下文 (Priority: P1)

作为正在看某标的 K 线或行情表的用户，我希望说「这个怎么看」时，助手默认分析我当前聚焦的标的，而不是反问「哪个」。

**Why this priority**: Rita 相对 TradingAgents 的关键差异；贴合看板产品。

**Independent Test**: 前端 focus 为 `BTCUSDT` 时发送「这个怎么看」；服务端收到 `context.focusSymbol=BTCUSDT`；回复明确围绕该标的。

**Acceptance Scenarios**:

1. **Given** 请求带 `context.focusSymbol`，**When** 用户消息含指代（「这个/它」），**Then** 助手解析为该标的。
2. **Given** 消息显式写出另一标的，**When** 与 context 冲突，**Then** 以用户消息显式标的为准。
3. **Given** 无 context 且消息未指明标的，**When** 需要标的才能答，**Then** 助手可追问或基于大盘快照作答（不编造个股价）。

---

### User Story 4 - 会话列表与管理 (Priority: P2)

作为已登录用户，我希望看到历史会话列表，切换会话，并删除不需要的会话。

**Why this priority**: 持续使用必备；可略后于 P1 对话闭环，但仍属一期。

**Independent Test**: 产生 ≥2 个会话后列表可见；切换加载对应消息；删除后列表与详情不可再访问。

**Acceptance Scenarios**:

1. **Given** 用户有多个会话，**When** 打开列表，**Then** 按更新时间倒序展示标题与时间。
2. **Given** 选中会话，**When** 拉取消息，**Then** 仅该会话消息。
3. **Given** 删除会话，**When** 确认，**Then** 会话及消息不可再读（硬删或软删一期可选，推荐软删）。

---

### User Story 5 - 工具增强的分析质量 (Priority: P1)

作为用户，我希望助手在需要时自动拉取报价、K 线摘要、快讯、市场广度，而不是空谈。

**Why this priority**: grounding 是可信分析底线。

**Independent Test**: 问「最近有什么快讯影响行情」时，链路出现 news 工具调用；回复可引用快讯标题/时间。

**Acceptance Scenarios**:

1. **Given** 询问现价/涨跌，**When** Agent 运行，**Then** 至少一次 quote/snapshot 类工具调用（除非上下文已有极短 TTL 缓存且标注时间）。
2. **Given** 询问走势，**When** Agent 运行，**Then** 可调用 klines 摘要工具，不把超长原始 K 线全量塞进最终用户可见消息。
3. **Given** 工具返回空/失败，**When** 回复，**Then** 说明数据不可用，不伪造数字。

---

### User Story 6 - 灰度、配额与安全 (Priority: P2)

作为运维/个人站长，我希望可关闭 AI、限制每人每日调用次数，避免费用失控；AI 故障不影响行情主路径。

**Acceptance Scenarios**:

1. **Given** 未登录，**When** 访问 AI API，**Then** 401。
2. **Given** 超过日配额，**When** 再发 chat，**Then** 429 或业务码 `ai_quota_exceeded`，文案可读。
3. **Given** MySQL/LLM 未配置却 `ai.enabled=true`，**When** 启动或首次请求，**Then** soft-skip 或明确错误（与 alerts/portfolio 风格对齐，plan 钉死）。
4. **Given** AI 模块 panic/超时，**When** 行情 WS/REST，**Then** 仍正常。

---

### Edge Cases

- LLM 流中断 / 客户端取消：服务端取消上游请求；会话写入已完成部分或标记 `incomplete`（实现选定）。
- 超长历史：截断/摘要策略，保证上下文窗口不爆（plan 钉死保留最近 N 轮 + 系统摘要可选）。
- 工具死循环：`max_tool_rounds` 上限。
- 敏感操作：拒绝「帮我下单/改密码/导出他人数据」等；不提供交易执行工具。
- 标的不存在或无报价：明确告知。
- 并发同会话双请求：一期可串行拒绝第二请求（409）或排队；推荐 **同会话单 flight**。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统 MUST 提供登录用户可用的 AI 对话界面（抽屉或独立入口），支持发送消息与流式展示回复。
- **FR-002**: 系统 MUST 支持多轮会话：创建、续聊、列会话、取消息、删除会话。
- **FR-003**: 系统 MUST 通过 SSE 流式返回助手输出（及可选 tool 进度事件）。
- **FR-004**: 系统 MUST 接受可选 `context`（至少支持 `focusSymbol`），用于消解指代。
- **FR-005**: 系统 MUST 以 tool calling 访问行情只读能力；一期工具至少覆盖：报价、盘面/宏观摘要、K 线摘要、快讯、市场广度（center）。
- **FR-006**: 系统 MUST NOT 让 `ai` 模块直连交易所或 provider 包。
- **FR-007**: 系统 MUST 以 `ai.enabled` 支持灰度关闭；关闭后行情不受影响。
- **FR-008**: 系统 MUST 实施每用户日配额。
- **FR-009**: 系统 MUST 在助手回复中体现非投资建议免责（系统固定尾注或等价机制）。
- **FR-010**: 系统 MUST 隔离用户数据：仅本人会话可读可删。
- **FR-011**: 一期 MUST NOT 提供下单、改持仓、改告警规则等写操作工具。

### Key Entities

- **Conversation（会话）**: 属于某用户；有标题、时间、状态。
- **Message（消息）**: 属于某会话；角色含 user / assistant / system / tool；可含工具调用元数据。
- **PageContext（页面上下文）**: 单次请求附带的看板焦点信息，不强制持久化（可快照写入该轮 user 消息旁元数据）。
- **ToolResult（工具结果）**: Agent 内部使用的结构化行情摘要，对用户可折叠展示。

### Non-Functional

- **NFR-001**: 行情主路径（WS/snapshot）MUST NOT 因 AI 故障不可用。
- **NFR-002**: 单次 chat 须有超时（建议 ≤ 120s 可配置）；工具单次超时更短。
- **NFR-003**: 写入 Prompt 的工具 payload 应裁剪（K 线摘要化），控制 token 成本。
- **NFR-004**: API Key 仅服务端配置，不得下发前端。

### Out of Scope（一期）

- 自动交易 / 券商或交易所下单
- TradingAgents 多角色辩论与回测
- 公开免登录试用（可后续 guest 配额）
- 语音输入、多模态看图识 K 线
- 微调自有金融大模型
- 修改 `docs/providers` 百度内部 Agent API（不依赖其闭源能力）

## Success Criteria

- **SC-001**: 登录用户可完成「打开助手 → 提问 → 流式看到基于工具数据的回答」闭环。
- **SC-002**: 同一会话追问无需重复标的时，助手仍能正确指代（在提供 context 或上文明确时）。
- **SC-003**: 刷新后历史会话可恢复。
- **SC-004**: `ai.enabled=false` 可一键关闭且行情/告警/资产不受影响。
- **SC-005**: 文档齐全（spec / research / data-model / contracts / plan），可进入 `/speckit-tasks` → 实现。

## Assumptions

- 个人/小团队部署；并发用户很少。
- 运营者可自备 OpenAI-compatible API Key。
- 用户已理解输出为分析辅助、非投资建议。
- 一期中文为默认交互语言（Prompt 可配置）。

## 参考

- 模块边界：`docs/MODULES.md`（`ai`）
- 架构：`docs/RFC-001-architecture.md`
- 同行灰度：`specs/004-alert-push/`、`specs/005-portfolio-asset-center/`
- 范式参考：OpenBB Agent Rita（对话 + SSE + context + tools）
- 调研：`specs/007-ai-assistant/research.md`
