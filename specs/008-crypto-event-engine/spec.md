# Feature Specification: Crypto Event Engine（市场异动引擎 · 加密优先 MVP）

**Feature Branch**: `008-crypto-event-engine`  
**Created**: 2026-07-31  
**Status**: Implemented（一期）  
**Input**: 将 MarketPulse 从「展示市场数据」升级为「理解市场发生了什么」。一期仅覆盖加密市场：规则检测异常 → Signal → 聚合成 MarketEvent → 公共 REST/WS + 看板异动面板。不做个人推送、不用 LLM 发现异常、不做自动交易。原始需求参考 `MarketPulse_Event_Engine_v1.0_需求文档.md`（GPT 草案，已按现网能力裁剪）。

## 背景

- MarketPulse 已具备较完整加密行情：Spot Quote WS、K 线、宏观、Binance 爆仓滚动聚合、快讯等，经 `MarketDataService` 对外。
- 现有 `alerts` 解决「用户设定条件后个人提醒」；尚无系统级「市场刚刚发生了什么」的结构化事件层。
- `docs/MODULES.md` 依赖方向要求业务模块只消费 `MarketDataService`，禁止直连 Provider / ingest。
- 用户确认：加密优先；一期不做个人推送与订阅；上线观察效果后再迭代 A 股/美股/跨市场与个性化推送。

## 分期范围

| 阶段 | 内容 | 本期 |
|------|------|------|
| **一期（Crypto MVP）** | Price / Volume / Volatility / Liquidation Detector；同窗聚合；Score/Severity/生命周期；MySQL 持久化；公共 REST + 公共 WS；看板异动面板；`event.enabled` 灰度 | ✅ |
| **二期** | 用户订阅过滤 + 多通道推送（复用 alerts 投递）；指数/美股参考 Price 异动；简化 Cross-Market；Event Context 挂载 Related News | ⏳ |
| **三期** | A/港/美 Fund Flow；完整 RISK_OFF；Historical similarity；AI tools（search_events / get_event）与解释 Agent | ❌ 本期不做 |

## 已确认决策

| # | 议题 | 决策 |
|---|------|------|
| 1 | 市场范围 | **加密优先**；关注标的由配置列表决定（默认含 BTC、ETH 等） |
| 2 | 个人推送 | **一期不做**；不做登录订阅、邮箱、PushPlus |
| 3 | 公共实时通道 | `/ws/v1/events` **公开、无需登录**（对齐行情 WS，不对齐 alerts WS） |
| 4 | 列表/详情鉴权 | REST **公开可读**；无需登录 |
| 5 | 检测方式 | **确定性规则**（绝对阈值、ratio；可演进 Z-Score）；**禁止 LLM 做检测** |
| 6 | 模块归属 | 独立 `internal/event`；API `/api/v1/events*`；前端 `web/src/features/events/` |
| 7 | 数据访问 | **只读** `MarketDataService`（Listener + Snapshot + Klines + Macro.Liquidations） |
| 8 | 与 alerts 边界 | Event = 系统公共事实；Alert = 用户私有规则提醒；一期互不调用 |
| 9 | 持久化 | MySQL 存 Event / Signal；缺 MySQL 时 soft-disable（与 alerts/ai 风格一致） |
| 10 | 爆仓粒度 | 消费现有**全市场 1h 滚动** `Macro.Liquidations`；与标的价格信号为**同时间窗关联**，不宣称「BTC 专属 15m 爆仓」 |
| 11 | 短窗指标 | Price/Volume/Volatility **必须基于 K 线**，不得仅用 Snapshot 日涨跌/24h 量 |
| 12 | 聚合目标 | 同窗多 Signal 合成单一 Event（如 `CRYPTO_FLASH_SELLOFF`），避免三条孤立异动 |
| 13 | AI | 一期不接 Agent；可预留 Context 字段为空；LLM 只解释、不发现（后续） |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 自动发现加密异常 Signal (Priority: P1)

作为系统，我希望在配置标的出现显著价格、成交量、波动或爆仓异常时，自动产出结构化 Signal，而无需用户配置规则、也无需 LLM。

**Why this priority**: 无 Signal 则无后续 Event；是引擎输入层。

**Independent Test**: 注入或等待满足阈值的 K 线/爆仓条件；仓库或内存缓冲中出现对应 `PRICE_DROP` / `VOLUME_SPIKE` / `VOLATILITY_SPIKE` / `LIQUIDATION_SPIKE`，字段含 value/baseline/ratio/changePct/timestamp。

**Acceptance Scenarios**:

1. **Given** `event.enabled=true` 且 MySQL 可用，**When** 某配置 symbol 的 15m 涨跌幅超过配置阈值，**Then** 产出 `PRICE_DROP` 或 `PRICE_SPIKE` Signal。
2. **Given** 当前窗口成交量相对近 N 根同周期均量 ratio ≥ 配置阈值，**When** 检测轮次运行，**Then** 产出 `VOLUME_SPIKE`。
3. **Given** 实现波动或 ATR 相对历史基线异常，**When** 检测轮次运行，**Then** 产出 `VOLATILITY_SPIKE`。
4. **Given** 全市场爆仓滚动窗口相对引擎维护的基线 ratio 超阈值，**When** 检测轮次运行，**Then** 产出 `LIQUIDATION_SPIKE`（market 可标为 `CRYPTO`，symbol 可为聚合占位如 `MARKET`）。
5. **Given** 未达任何阈值，**When** 检测轮次运行，**Then** 不产生噪声 Signal。

---

### User Story 2 - 同窗聚合为单一 MarketEvent (Priority: P1)

作为用户，我希望 BTC 急跌伴随放量与爆仓放大时，看到一条「Crypto Flash Sell-off」类事件，而不是三条互不相关的异动。

**Why this priority**: 聚合是 Event Engine 相对简单告警的核心差异；需求验收的关键叙事。

**Independent Test**: 在聚合时间窗内注入 PRICE_DROP + VOLUME_SPIKE + LIQUIDATION_SPIKE；系统仅创建（或更新）一条 `CRYPTO_FLASH_SELLOFF`（或等价 type/subType），其 `signals` 含上述三者。

**Acceptance Scenarios**:

1. **Given** 同 symbol（或规则允许的关联组）在聚合窗内出现 Price+Volume+Liquidation 且方向一致（下跌侧），**When** Aggregator 运行，**Then** 生成/更新一条 Flash Sell-off 类 Event，而非三个独立顶层 Event。
2. **Given** 仅出现 Price 异常，**When** Aggregator 运行，**Then** 可生成较轻量的 `PRICE_ANOMALY` 类 Event（或按模板规则），不误标为 Flash Sell-off。
3. **Given** 已有 ACTIVE 的同模板 Event 未结束，**When** 新相关 Signal 到达，**Then** 并入该 Event（更新 score/signals/status），不重复开单。
4. **Given** 聚合窗外的孤立 Signal，**When** 评估，**Then** 不与已 RESOLVED 的历史 Event 错误合并。

---

### User Story 3 - 公开查看 Event 列表与详情 (Priority: P1)

作为访客（无需登录），我希望在看板看到市场异动列表，并点进详情查看分数、严重度、受影响标的、Signal 明细与时间线。

**Why this priority**: 用户可感知的产品闭环；一期交付面。

**Independent Test**: 未登录打开首页；异动面板列出 ACTIVE/近期 Event；点开详情接口返回完整结构；过滤参数（severity/status/symbol）生效。

**Acceptance Scenarios**:

1. **Given** 存在 ACTIVE Event，**When** `GET /api/v1/events`，**Then** 返回分页/游标列表，含 title、severity、score、symbols、status、startTime。
2. **Given** 已知 eventID，**When** `GET /api/v1/events/{id}`，**Then** 返回 summary、signals、score、status、时间字段。
3. **Given** `GET /api/v1/events/{id}/timeline`，**When** 请求成功，**Then** 按时间序返回 Signal/状态变更节点。
4. **Given** `event.enabled=false` 或模块 soft-disabled，**When** 调用 API，**Then** 返回业务禁用（如 503 / `event_disabled`）；UI 隐藏或展示不可用说明。
5. **Given** 未登录访客，**When** 访问上述只读 API，**Then** **不**要求 token。

---

### User Story 4 - 公共 WebSocket 实时更新 (Priority: P1)

作为打开看板的访客，我希望异动面板在新 Event 创建、分数/状态更新、结束时自动刷新，无需手动刷新页面。

**Why this priority**: 与「市场正在发生」的实时性一致；对齐行情公开 WS 体验。

**Independent Test**: 连接 `/ws/v1/events`（无 token）；造一条 Event；客户端收到 `event.created`；随后 score 变化收到 `event.updated`；RESOLVED 收到 `event.resolved`。

**Acceptance Scenarios**:

1. **Given** 客户端已连接公共 Event WS，**When** 新 Event 持久化成功，**Then** 广播 `event.created`（含摘要字段）。
2. **Given** ACTIVE Event 的 score/status/signals 更新，**When** 提交成功，**Then** 广播 `event.updated`。
3. **Given** Event 进入 RESOLVED，**When** 状态落库，**Then** 广播 `event.resolved`。
4. **Given** 未登录，**When** 连接 `/ws/v1/events`，**Then** 允许升级并接收广播（**不**校验 users session）。

---

### User Story 5 - 生命周期与强度评分 (Priority: P2)

作为用户，我希望看到事件是否仍在持续、严重程度如何，以及强度分数如何随信号变化。

**Why this priority**: 完成「发生了什么 → 是否还在持续 → 多严重」叙事；可略后于列表闭环，但仍属一期。

**Independent Test**: 事件从 DETECTED→ACTIVE；指标回落→DEESCALATING→RESOLVED；Score 在 0–100，Severity 映射一致；缺某类 Signal 时权重归一化而非当 0。

**Acceptance Scenarios**:

1. **Given** 首次达触发条件，**When** 创建 Event，**Then** status 为 `DETECTED` 或直接 `ACTIVE`（实现选定一种并文档化；推荐 DETECTED 后迅速 ACTIVE）。
2. **Given** 异常仍超阈值，**When** 后续轮次，**Then** 保持/进入 `ACTIVE`，并维护 `peakScore`。
3. **Given** 强度下降但仍未正常，**When** 评估，**Then** `DEESCALATING`。
4. **Given** 指标回到正常范围，**When** 评估，**Then** `RESOLVED` 且写入 `endTime`。
5. **Given** 仅部分 Signal 类型存在，**When** 计算 Score，**Then** 对存在分量做权重归一化。

---

### User Story 6 - 配置化阈值与灰度 (Priority: P3)

作为运维，我希望用配置调整检测阈值与关注标的，并能一键关闭 Event Engine，且不影响行情主路径。

**Why this priority**: 可运维性；避免硬编码导致误报无法调。

**Independent Test**: 修改 YAML 阈值后重启（或热载若已支持）行为变化；`event.enabled=false` 时检测停止、API 禁用、行情正常。

**Acceptance Scenarios**:

1. **Given** 配置中的 price 15m 阈值提高，**When** 原临界行情，**Then** 不再触发（或按新阈值触发）。
2. **Given** `event.enabled=false`，**When** 启动或运行中关闭，**Then** 不写入新 Event；行情 WS/REST 不受影响。
3. **Given** MySQL 不可用但配置 `enabled=true`，**When** 启动，**Then** soft-skip 并打日志（与 alerts/ai 一致）。

---

### Edge Cases

- K 线接口短暂失败：本轮跳过该 symbol，不写脏 Signal；下次重试。
- 休市/无更新：加密 7×24；若某标的报价 stale，检测跳过或降低置信（实现选定）。
- 爆仓 WS 断开：`Liquidations` 过期时不发 `LIQUIDATION_SPIKE`，或仅用价格/量能聚合降级模板。
- 高频行情导致检测过密：必须节流（建议 15–30s 一轮或 listener 合并）。
- 时钟回拨 / 重复 Signal：同一 (signalType, symbol, window) 短时去重。
- 历史 Event 查询跨度过大：limit/cursor 强制上限。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统 MUST 对配置的加密标的运行 Price / Volume / Volatility 异常检测，并产出独立 Signal。
- **FR-002**: 系统 MUST 基于 `Macro.Liquidations`（及引擎内基线）检测 Liquidation 异常 Signal。
- **FR-003**: 系统 MUST 在配置时间窗内将相关 Signal 聚合为 MarketEvent，支持 Flash Sell-off 类模板。
- **FR-004**: 系统 MUST 持久化 Event 与其 Signals（MySQL），并支持按 status/severity/symbol/time 查询。
- **FR-005**: 系统 MUST 提供公开 `GET /api/v1/events`、`GET /api/v1/events/{id}`、`GET /api/v1/events/{id}/timeline`。
- **FR-006**: 系统 MUST 提供公开 `WS /ws/v1/events`，广播 created/updated/resolved。
- **FR-007**: 系统 MUST 维护 Event 生命周期与 Score（0–100）及 Severity 映射。
- **FR-008**: 系统 MUST 通过 `event.enabled` 灰度；MUST NOT 在 `event` 模块内直连交易所或 ingest。
- **FR-009**: 系统 MUST 将检测阈值与关注标的配置化（YAML），禁止唯一硬编码路径。
- **FR-010**: 前端 MUST 在看板展示异动列表（及详情入口），访客无需登录即可查看。
- **FR-011**: 一期 MUST NOT 实现个人推送、用户订阅偏好、邮箱/PushPlus 投递。
- **FR-012**: 一期 MUST NOT 使用 LLM 判定是否发生异常。

### Key Entities

- **EventSignal**: 单一异常事实（类型、标的、市场、数值、基线、ratio、时间）。
- **MarketEvent**: 聚合后的市场事件（类型、标题、严重度、分数、状态、时间、symbols、signals）。
- **EventContext**（预留）: 关联新闻/资产/宏观/历史事件；一期可空结构。
- **DetectionRuleConfig**: 阈值与窗口配置。

### Non-Functional

- **NFR-001**: Event 检测与落库 MUST NOT 阻塞行情 ingest / 行情 WS 主路径。
- **NFR-002**: 检测循环 MUST 有节流；单轮对单 symbol 的 K 线拉取应有超时。
- **NFR-003**: 模块故障（panic 隔离或 recover）不得拖垮 `marketd` 行情服务。
- **NFR-004**: 公共 WS 为广播模型，无需 per-user Hub 鉴权状态。

### Out of Scope（一期）

- 个人推送 / 用户订阅 / 多通道通知
- LLM 自动检测异常
- 自动交易 / 买卖建议 / 下单
- 完整 Cross-Market RISK_OFF、A/港/美 Fund Flow Detector
- 完整 Event Graph、Vector DB、RAG
- Multi-Agent / Bull-Bear 辩论
- AI Analysis 完整 Agent（可留 UI 入口 disabled）

## Success Criteria

- **SC-001**: 加密急跌+放量+爆仓场景下，系统能聚合成单一 Flash Sell-off 类 Event（可用回放/注入测试验证）。
- **SC-002**: 未登录用户可在看板看到异动列表与详情，并经公共 WS 收到更新。
- **SC-003**: `event.enabled=false` 可关闭引擎且行情/告警/AI 不受影响。
- **SC-004**: 文档齐全（spec / research / data-model / contracts / plan / tasks / quickstart），可进入 `/speckit-implement`。

## Assumptions

- 个人/小团队部署；并发低；MySQL 在启用 event 时可用。
- Binance K 线与爆仓数据源在验收环境基本健康。
- 一期默认中文标题/文案模板。
- 阈值初值允许误报，上线后凭配置调参。

## 参考

- 模块边界：`docs/MODULES.md`
- 数据源：`docs/DATA_SOURCES.md`
- 同行模块：`specs/004-alert-push/`（评测/灰度范式，非推送复用）
- 架构：`docs/RFC-001-architecture.md`
- 契约同步目标（实现时）：`docs/RFC-002-api-contract.md`
