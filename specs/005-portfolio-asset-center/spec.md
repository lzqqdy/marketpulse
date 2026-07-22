# Feature Specification: 资产中心（Portfolio / Asset Center）

**Feature Branch**: `005-portfolio-asset-center`  
**Created**: 2026-07-22  
**Status**: Implemented  
**Input**: 用户中心「资产中心」：持仓设置（币价 + 美股参考）与本金；资产总览实时更新；每日资产快照列表。后续基于快照做累计收益/收益率图、资产走势、资产分布。逻辑参考 go-coin / mine-web；老库 `assets_log` 数年历史需可同步到新项目。UI 可参考旧资产页截图，允许按 MarketPulse 视觉体系发挥。

## 背景

- `portfolio` 模块已落地（`internal/portfolio`、`/api/v1/portfolio`、用户中心第三 Tab）。
- 用户中心 Tab：`profile` / `alerts` / `portfolio`。
- 行情估值经 `MarketDataService`，禁止 portfolio 直连交易所或 ingest（Constitution）。
- 老项目 `assets_log` 迁移：`cmd/migrate-assets-log`（按 uid 映射；未映射行记为 skip）。
- 图表报告见 **006**（已完成）。

## 分期范围

| 阶段 | 内容 | 本期 |
|------|------|------|
| **一期** | 持仓/本金设置、实时总览、每日快照列表、日终快照任务、历史数据迁移能力 | ✅ |
| **二期** | 基于快照的报告图：累计收益/收益率、资产走势、资产分布 | ✅ 见 `specs/006-portfolio-asset-reports/` |

## 已确认决策

| # | 议题 | 决策 |
|---|------|------|
| 1 | 模块归属 | 独立 `portfolio`，命名空间 `/api/v1/portfolio`；前端 `web/src/features/portfolio/` |
| 2 | 可设置标的 | **币价（现货/USDT 计价）** + **美股参考（alpha，如 Bitget 代币化标的）**；估值均走现有行情 Store |
| 3 | 本金 | 用户级 **CNY 本金**；与旧 `user.balance` + `assets_log.date='1'` 语义对齐，新表用显式字段，迁移时回填 |
| 4 | 涨跌色 | 遵循用户/系统涨跌色约定（国内习惯默认红涨绿跌）；与用户中心主题一致 |
| 5 | 实时总览刷新 | 持仓页轮询或订阅行情变更后重算；一期 REST 轮询即可（建议 3–10s），不强制独立 WS |
| 6 | 日终快照 | 服务内定时任务（默认 Asia/Shanghai 日切后写「昨日」快照），`portfolio.enabled` 灰度 |
| 7 | 历史迁移 | 提供离线/管理脚本：从旧 MySQL `assets_log`（及可选 `assets`/`balance`）导入；按 `uid` 映射到 MarketPulse `users.id` |
| 8 | 二期图表 | 一期保证快照字段完整、列表可分页导出级查询，图表另开 spec |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 设置持仓与本金 (Priority: P1)

作为已登录用户，我希望在用户中心「资产中心」勾选/添加关注的币种与美股参考标的，填写持仓数量，并设置本金（¥），以便系统按实时价估值。

**Why this priority**: 无持仓与本金则无总览与快照。

**Independent Test**: 登录后添加 BTC/USDT 与一个 alpha 标的数量，设置本金；刷新后设置仍在；总览数字随之变化。

**Acceptance Scenarios**:

1. **Given** 用户已登录且 `portfolio.enabled=true`，**When** 打开资产中心，**Then** 可见持仓表与本金设置入口。
2. **Given** 行情 Store 中有有效报价的标的，**When** 添加该标的并填写数量 ≥ 0，**Then** 持仓持久化成功。
3. **Given** 无报价或不支持的标的类型，**When** 尝试添加，**Then** 拒绝或 UI 不可选。
4. **Given** 用户修改本金 CNY ≥ 0，**When** 保存，**Then** 本金持久化；历史累计收益按新本金口径重算展示（不回溯改写已写入的历史日快照行，除非明确「重建基准」操作——一期不做重建）。
5. **Given** 用户将某标的数量改为 0 或移除，**When** 保存，**Then** 该标的不再计入实时总览；历史快照中的 `asset_detail` 不变。

---

### User Story 2 - 资产总览实时更新 (Priority: P1)

作为已登录用户，我希望看到总资产（USDT / CNY）、U 溢价、今日收益、7 日/30 日/历史累计收益及收益率，且随行情近似实时刷新。

**Why this priority**: 资产中心核心价值；对齐旧 `getAssetsWave` 体验。

**Independent Test**: 持仓非空时总览有数值；改数量或等行情变动后，总资产与今日收益在下一轮刷新内更新。

**Acceptance Scenarios**:

1. **Given** 有持仓与有效 USDT/CNY 汇率，**When** 请求总览，**Then** 返回总资产 USDT、约合 CNY、U 溢价百分比（若汇率字段可得）。
2. **Given** 存在最近一条日快照，**When** 计算今日收益，**Then** `今日收益(CNY) ≈ 当前总CNY - 最近快照 total_value_cny`，并给出百分比。
3. **Given** 存在对应日期快照，**When** 计算 7 日/30 日收益，**Then** 相对约 7/30 天前快照的 CNY 差值与比率；缺失则展示占位「—」。
4. **Given** 已设置本金 > 0，**When** 计算历史收益，**Then** `历史收益 = 当前总CNY - 本金`，收益率 = 收益/本金。
5. **Given** 本金为 0 且总资产 > 0，**When** 展示历史收益率，**Then** 不除零崩溃（展示「—」或「∞」之一，产品选「—」更安全）。

---

### User Story 3 - 每日资产快照列表 (Priority: P1)

作为已登录用户，我希望分页查看每日快照（日期、总额、日涨跌、日收益率、累计收益、累计收益率），风格接近旧「详情」表。

**Why this priority**: 承接历史数据与二期图表的数据源。

**Independent Test**: 有快照数据时列表按日期倒序分页；仅本人数据；涨跌色正确。

**Acceptance Scenarios**:

1. **Given** 用户有多条日快照，**When** 打开快照列表，**Then** 默认按日期 desc 分页（如 10/20/50 条每页）。
2. **Given** 快照行，**When** 渲染，**Then** 展示 Date / Total(CNY 或双币种) / Daily / D% / Profits / P%；正红负绿（或随用户涨跌色）。
3. **Given** 迁移或本金基准行（旧 `date='1'`），**When** 列表展示，**Then** 可标记为「本金」或单独不在日表展示（一期建议：日表默认过滤基准行，本金在设置区展示）。
4. **Given** 用户 A，**When** 请求列表，**Then** 不可见用户 B 数据。

---

### User Story 4 - 日终自动快照 (Priority: P1)

作为系统，我需要在每个交易日结束后为有持仓（或已开启资产中心）的用户写入一条昨日资产快照，保证列表与后续报告连续。

**Why this priority**: 无自动快照则仅靠迁移历史，新数据会断档。

**Independent Test**: 触发一次日终任务后，昨日日期出现一条 `(user_id, date)` 唯一记录，含 total、日收益、累计、asset_detail JSON。

**Acceptance Scenarios**:

1. **Given** 用户有持仓，**When** 日终任务运行，**Then** 写入 `date=昨日` 快照；重复执行同日幂等（upsert 或跳过）。
2. **Given** 存在上一日快照，**When** 计算日收益，**Then** 相对上一有效日快照（非本金基准行）的 CNY 差额与比率。
3. **Given** 存在本金基准，**When** 计算累计收益，**Then** 相对本金 CNY（或首条有效快照，迁移策略见 data-model）。
4. **Given** `portfolio.enabled=false`，**When** 定时触发，**Then** 不写库、不影响行情进程。

---

### User Story 5 - 历史 assets_log 同步 (Priority: P1)

作为运营/开发者，我希望把老项目 `assets_log`（及可选持仓/本金）导入 MarketPulse，使用户在新资产中心看到连续历史。

**Why this priority**: 用户明确要求数年历史可同步。

**Independent Test**: 对测试库导入样例行后，快照列表出现对应日期与金额；`(user_id, date)` 不重复。

**Acceptance Scenarios**:

1. **Given** 旧库导出或直连只读，**When** 运行迁移工具并指定 uid 映射，**Then** 日快照字段落入新表，语义对齐。
2. **Given** 旧行 `date='1'`，**When** 迁移，**Then** 写入本金设置或基准快照标记，不与普通日混淆。
3. **Given** 同一 `(mapped_uid, date)` 已存在，**When** 再次导入，**Then** 可配置 skip / overwrite；默认 skip 防误伤。
4. **Given** `asset_detail` JSON，**When** 迁移，**Then** 原样或规范化存储，列表/二期分布图可读。

---

### User Story 6 - 灰度与权限 (Priority: P2)

作为运维，我希望资产中心可开关；未登录或关闭时不可访问业务 API。

**Acceptance Scenarios**:

1. **Given** 未登录，**When** 访问 portfolio API，**Then** 401。
2. **Given** `portfolio.enabled=false`，**When** 访问，**Then** 明确禁用错误；用户中心可不展示或展示不可用说明。
3. **Given** MySQL 未配置，**When** 开启 portfolio，**Then** 启动失败或功能降级策略与 alerts/users 一致（文档写明）。

---

### Edge Cases

- 持仓数量非法（负、非数字）→ 400。
- 某标的临时无价 → 该标的估值记为不可用，总览可部分合计并提示缺失标的；日终快照跳过无价用户或按上一日价策略（一期：**跳过该用户并打日志**，避免脏快照）。
- USDT/CNY 缺失 → 用配置默认汇率并标记 `rate_fallback=true`。
- 仅有本金无持仓 → 总览可为 0；仍允许看历史快照。
- 时区：快照日期与日切一律 **Asia/Shanghai**。
- 大额历史导入 → 批量 insert，不阻塞 HTTP 热路径。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统 MUST 在用户中心提供「资产中心」板块（Tab 或子路由），需登录。
- **FR-002**: 系统 MUST 支持用户对 **crypto** 与 **alpha（美股参考）** 标的的持仓增删改（数量）；标的必须来自行情可报价集合。
- **FR-003**: 系统 MUST 支持用户设置 **本金（CNY）** 并持久化。
- **FR-004**: 系统 MUST 提供资产总览：总资产 USDT/CNY、U 溢价（可得时）、今日/7 日/30 日/历史收益与收益率。
- **FR-005**: 系统 MUST 用当前行情报价 × 持仓数量实时（或近实时）估值；USDT 数量按 1:1 USDT。
- **FR-006**: 系统 MUST 提供每日快照分页列表（含排序字段约定）；默认过滤本金基准行。
- **FR-007**: 系统 MUST 日终为符合条件用户写入昨日快照（幂等），含 `asset_detail`。
- **FR-008**: 系统 MUST 提供历史 `assets_log` 迁移方案（脚本 + 字段映射 + uid 映射）。
- **FR-009**: 系统 MUST 以 `portfolio.enabled`（及依赖 mysql）支持灰度与回滚。
- **FR-010**: 估值与快照计算 MUST NOT 直连交易所；只读 MarketData 公开接口/服务。
- **FR-011**: 图表报告 UI 已迁至 **006** 并完成；本 spec（005）范围止于持仓/本金/总览/日快照/迁移。

### Non-Functional

- **NFR-001**: 总览接口在个人持仓规模（≤100 标的）下 P99 < 200ms（不含网络到浏览器）。
- **NFR-002**: 日终任务失败可重跑；不因单用户失败中断全量。
- **NFR-003**: 行情主路径不受 portfolio 故障影响（独立模块、独立开关）。

### Out of Scope（一期）

- 交易所 API 同步持仓 / 充提流水
- 成本价 FIFO、已实现/未实现分离账本
- 资产报告图表（二期）
- 多币种本金、多账户组合
- 目标价幻想收益（旧 `nowIsFantasyTime`）——可三期再议
- 手动改全局 USDT 场外价（旧 `setUsdtPrice`）——一期只用行情模块 OTC 汇率；不做全局脏写

## Success Criteria

- 用户可在 MarketPulse 用户中心完成「设持仓 → 看总览 → 看日快照」闭环。
- 旧 `assets_log` 样例数据可导入并在列表中正确展示。
- `portfolio.enabled=false` 可一键关闭且行情/告警不受影响。
- 文档与契约齐全，可进入 `/speckit-tasks` → 实现。

## 参考

- 旧 UI：`mine-web-master/view/index/assets.html`、资产分析页
- 旧总览：`Index.php::getAssetsWave`
- 旧日终：`go-coin-master/crontab/job/assets.go`
- 模块边界：`docs/MODULES.md`
- 同行灰度：`specs/004-alert-push/`
