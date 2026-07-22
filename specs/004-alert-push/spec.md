# Feature Specification: 推送告警（Alert Push）

**Feature Branch**: `004-alert-push`  
**Created**: 2026-07-14  
**Status**: Implemented  
**Input**: 对首页可正常获取数据源的标的（币价、指数、美股参考等）支持规则告警；检查频率对齐数据更新频率；通道含站内弹窗 / 邮箱 / PushPlus；频率含一次 / 循环 / 每日一次；与用户绑定；可操作配置 UI + 推送记录。参考 go-coin / mine-web，仅作参考不照搬。

## 背景

- `alerts` 模块已落地（`internal/alerts`、`/api/v1/alerts`、用户中心「价格告警」Tab、全局 `AlertToastHost`）。
- 用户资料含 `email`、`wechatPushToken`，供邮件 / PushPlus 通道使用。
- 行情统一进入内存 Store；告警经 `MarketDataService` / Store 变更事件评测，禁止直连交易所或 ingest 内部（Constitution）。
- 标的类型：`spot`（现货）、`index`（全球指数）、`alpha`（美股参考）。

## 已确认决策

| # | 议题 | 决策 |
|---|------|------|
| 1 | 5 分钟剧烈波动 | 使用**滚动 5 分钟高低振幅**：`(high-low)/low*100`，非固定 5m K 线根 |
| 2 | 触发类型范围 | 一期一次做齐 **type 1–5** |
| 3 | 站内弹窗 | 支持**离线**：触发写入未读 inbox；用户登录/回站后全局补弹或列表可见 |
| 4 | 已满足条件创建 | **禁止创建**（拒绝并提示当前已触达） |
| 5 | 存储原则 | **MySQL 仅存必要持久表**；冷却、inbox、评测热索引、投递限流等尽量 **Redis / 内存**，不阻塞行情主流程 |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 创建并管理告警规则 (Priority: P1)

作为已登录用户，我希望对首页可用标的快速配置价格/波动告警，并选择通道与频率，以便在条件满足时收到通知。

**Why this priority**: 无规则则无告警闭环；是功能核心。

**Independent Test**: 登录后在用户中心「告警」页创建一条 BTC 上涨触达规则，列表可见且可启停/删除；创建时若现价已 ≥ 阈值则接口返回业务错误。

**Acceptance Scenarios**:

1. **Given** 用户已登录且标的数据源健康，**When** 选择标的 + type1–5 + 通道 + 频率并保存，**Then** 规则持久化成功且出现在规则列表。
2. **Given** 当前价已满足触达条件，**When** 提交创建，**Then** 拒绝创建并提示「当前已满足条件，无法创建」。
3. **Given** 已有规则，**When** 切换启停或软删，**Then** 停用/删除后不再触发；未读 inbox 不受软删历史记录影响（记录仍可查）。
4. **Given** 勾选邮箱或 PushPlus 但未配置对应字段，**When** 保存规则，**Then** 允许保存并提示该通道暂不可用；触发时跳过该通道并记失败原因。

---

### User Story 2 - 条件命中后多通道推送 (Priority: P1)

作为已登录用户，我希望规则命中时按所选通道收到通知（站内 / 邮箱 / 微信 PushPlus），且遵守频率策略。

**Why this priority**: 推送是告警价值所在。

**Independent Test**: 造一条易触发的 once 规则；评测命中后站内未读 +1；若配置了 email/token 则对应投递记录为 success/failed。

**Acceptance Scenarios**:

1. **Given** 规则 status=active 且冷却未挡住，**When** 标的报价更新导致条件成立，**Then** 按 channels 投递并各写一条推送记录。
2. **Given** frequency=`once`，**When** 首次成功触发，**Then** 规则变为不可继续推送（disabled），需用户手动再开。
3. **Given** frequency=`loop` 且 `interval_minutes=N`，**When** 触发后，**Then** N 分钟内同规则不再推；到期后若仍满足条件则再次推送。
4. **Given** frequency=`daily`，**When** 当日已推送过，**Then** 当日不再推；次日（用户时区，默认 Asia/Shanghai）可再推。
5. **Given** 某一通道投递失败，**When** 其他通道成功，**Then** 不影响其他通道；失败写入 deliveries.error。

---

### User Story 3 - 站内弹窗（含离线补达）(Priority: P1)

作为登录用户，我希望在**任意已登录页面**看到告警弹窗；若触发时不在线，回站后仍能看到未读告警。

**Why this priority**: 用户明确要求全局站内弹窗 + 离线能力。

**Independent Test**: 触发含 `in_app` 的规则后断开会话；重新登录进入任意页，未读弹窗或未读列表出现对应文案，确认后未读清除。

**Acceptance Scenarios**:

1. **Given** 用户在线且订阅告警通道，**When** `in_app` 投递，**Then** 全局弹窗展示标题/内容，不局限于告警页。
2. **Given** 用户离线，**When** 规则命中 `in_app`，**Then** 消息进入未读 inbox（Redis），不丢失。
3. **Given** 存在未读 inbox，**When** 用户登录并建立告警连接，**Then** 拉取未读并弹窗/展示；用户确认或标记已读后未读移除。
4. **Given** 未登录访客，**When** 任意页面，**Then** 不建立告警推送连接、不弹用户告警。

---

### User Story 4 - 五种触发类型 (Priority: P1)

作为用户，我希望用同一套流程配置上涨/下跌/区间/振幅%/滚动 5 分钟剧烈波动五类规则。

**Why this priority**: 已确认一期做齐 1–5。

**Independent Test**: 五类规则均可 CRUD；评测逻辑对假报价符合各自定义（见 FR）。

**Acceptance Scenarios**:

1. **type=1 上涨触达**：现价 `>=` 设定价 X 时触发；创建时现价已 `>=X` 则禁止。
2. **type=2 下跌触达**：现价 `<=` 设定价 X 时触发；创建时现价已 `<=X` 则禁止。
3. **type=3 区间触达**：以创建时现价为中心、± 绝对价差 R；现价触及上界或下界触发；创建时若已在界外则禁止。
4. **type=4 振幅比例**：以创建时现价为中心、± ampl% 得到上下界；触及触发；创建时已在界外则禁止。
5. **type=5 滚动 5 分钟剧烈波动**：近 5 分钟窗口内 `(high-low)/low*100 >= rapid_chg%` 触发；创建时若窗口振幅已达标则禁止。

---

### User Story 5 - 推送记录查询 (Priority: P2)

作为用户，我希望查看历史推送（时间、标的、通道、成败、文案），便于核对是否漏推。

**Why this priority**: 可观测与排障；依赖投递已发生。

**Independent Test**: 至少产生 1 条 delivery 后，列表 API/UI 可见且仅本人数据。

**Acceptance Scenarios**:

1. **Given** 已有投递，**When** 打开推送记录，**Then** 按时间倒序分页展示。
2. **Given** 用户 A，**When** 请求记录，**Then** 不可见用户 B 的记录。

---

### User Story 6 - 标的覆盖与评测节奏 (Priority: P2)

作为用户，我希望凡首页能正常拿到数据的币价、指数等均可设告警，且检查节奏跟随该数据源的更新频率。

**Why this priority**: 对齐产品目标；可分期落地标的，但契约需统一。

**Independent Test**: 对 `asset_type=spot` / `index` / `alpha`（美股参考）各建一条规则，随 Store 更新进入评测；无数据/不健康源不可选或创建失败。

**Acceptance Scenarios**:

1. **Given** 标的在 snapshot 中有有效报价，**When** 创建规则，**Then** 允许选择该标的。
2. **Given** 数据源不可用或无报价，**When** 尝试创建，**Then** 拒绝或 UI 不可选。
3. **Given** 币价 WS 更新 / 指数按 poll TTL 更新，**When** Store 变更，**Then** 仅评测关联该 symbol 的 active 规则（事件驱动，非全表秒扫阻塞行情）。

---

### Edge Cases

- 规则参数非法（负数振幅、interval=0、空 channels）→ 400 业务校验错误。
- PushPlus / SMTP 超时或限流 → 该通道失败入记录；可重试策略一期不做自动重试（避免刷屏），用户可依赖 loop 下次触发。
- 用户修改 email/token 后 → 下次投递用新值；已发出记录不回写。
- `alerts.enabled=false` 或 users/mysql/redis 未开 → 告警 API 明确禁用错误；行情主流程不受影响。
- 同一用户同标的多条不同 type 规则 → 各自独立评测与冷却（按 rule_id）。
- type5 窗口内样本不足（标的刚出现）→ 不触发；创建时按「当前窗口振幅未达阈值」允许（若仍不足达标）。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统 MUST 提供登录用户的告警规则 CRUD（软删）与启停。
- **FR-002**: 系统 MUST 支持 rule_type **1–5**（上涨触达、下跌触达、区间触达、振幅%、滚动 5 分钟剧烈波动），语义见 User Story 4。
- **FR-003**: 系统 MUST 在创建时校验「当前是否已满足触发条件」；已满足则 **禁止创建**。
- **FR-004**: 系统 MUST 支持推送频率：`once` / `loop`（自定义 `interval_minutes`）/ `daily`（默认时区 Asia/Shanghai）。
- **FR-005**: 系统 MUST 支持通道多选：`in_app`、`email`、`pushplus`；缺配置通道跳过并记失败。
- **FR-006**: 系统 MUST 将告警评测挂在行情 Store 变更（或等价公开服务事件）上，**评测频率对齐该标的数据更新频率**；禁止 alerts 直连交易所 API。
- **FR-007**: 系统 MUST 为 type=5 维护每标的**滚动 5 分钟** high/low（内存为主，可按需 Redis 辅助），按振幅公式判定。
- **FR-008**: 系统 MUST 提供站内通道：在线实时推送 + **离线未读 inbox**；登录后任意页可弹/可见。
- **FR-009**: 系统 MUST 支持 SMTP 邮件推送到用户绑定邮箱；配置项使用 `smtp.*`（参考项目 `rtmp` 命名不采用）。
- **FR-010**: 系统 MUST 支持 PushPlus 一对一推送（用户 token = `wechat_push_token`），对接 [PushPlus 文档](https://www.pushplus.plus/doc/)。
- **FR-011**: 系统 MUST 持久化推送记录供用户查询（分页）；记录归属校验 uid。
- **FR-012**: 冷却与未读 inbox、投递限流等非必要持久状态 MUST 优先使用 **Redis**；评测热路径可用 **内存索引**；MySQL 仅存必要表（规则 + 推送记录）。
- **FR-013**: 告警模块 MUST 可通过配置开关关闭且不影响行情主流程（灰度/回滚）。
- **FR-014**: 前端 MUST 提供易用告警设置（标的点选、类型切换表单、通道/频率、启停）与推送记录视图；用户中心「价格告警」Tab + 全局 Toast 已落地。
- **FR-015**: `loop.interval_minutes` MUST 允许用户配置；产品默认档位建议 5/10/30/60，允许自定义但需有上下界（建议 1–1440）。

### Key Entities

- **AlertRule（告警规则）**：归属用户；绑定 `asset_type` + `symbol`（+ 可选 field）；含 rule_type、阈值参数、channels、frequency、status、创建时价格快照等。
- **AlertDelivery（推送记录）**：一次通道投递的结果；关联 rule 与 user；含触发值、文案、channel、status、error。
- **InAppInboxItem（站内未读）**：非 MySQL 实体；Redis 中按用户存储的未读告警，供离线补达。
- **SymbolWindow5m（滚动窗口）**：非 MySQL；每标的近 5 分钟价点 high/low，支撑 type=5。
- **UserNotifyProfile**：复用 `users.email`、`users.wechat_push_token`，不新建用户通道表。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 登录用户可在 2 分钟内完成一条 type1–5 规则的创建（含通道与频率）。
- **SC-002**: 币价类规则在报价更新后 **1 秒内**完成评测尝试（不含外部邮件/PushPlus 网络耗时）；外发异步，不阻塞 Store 写入。
- **SC-003**: 离线触发的 `in_app` 告警，用户重新登录后 **首次建立告警连接 5 秒内**可见未读。
- **SC-004**: `alerts.enabled=false` 时，行情 snapshot/WS 行为与开启前一致（回归可验证）。
- **SC-005**: 推送记录可追溯每次通道成败；用户抽查与演示无「有触发无记录」缺口。

## Assumptions

- 标的覆盖首页 **现货报价 + 全球指数 + 美股参考（alpha）**；汇率/宏观等可用同一 `asset_type` 模型扩展，UI 可后续加选项。
- 不做短信、不做浏览器原生 Desktop Notification；站内弹窗 ≠ Web Notification API。
- 邮箱不强制验证码绑定（填写即视为可投递），与现有 `PUT /me` 一致。
- PushPlus 使用一对一消息接口；用户自备 token，额度/封禁由第三方负责，产品侧做好错误提示。
- `daily` 按 `Asia/Shanghai` 自然日；暂不开放用户自定义时区。
- `interval_minutes` 默认推荐 10；上下界 1–1440。
- 参考表 `alert_push` 的 scene 素数积、全表秒扫、冷却不含 rule_id **不采用**。
- 灰度可通过 `alerts.enabled` 与（可选）用户白名单配置实现。

## 范围外（Out of Scope）

- type 6/7（开盘波动、24h 波动）— 可后续增量。
- 短信、语音、企业微信等其他 PushPlus 子渠道封装。
- 告警规则分享、模板市场、多人订阅同一规则。
- 投递失败自动重试队列（一期仅记失败）。

## 回滚方案

1. 配置 `alerts.enabled: false` 并重启 → API/WS 告警关闭，评测与外发停止。
2. 前端告警入口可继续隐藏或展示「未启用」。
3. MySQL 表保留不影响行情；无需删表即可回滚行为。
