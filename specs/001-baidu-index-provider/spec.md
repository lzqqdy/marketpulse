# Feature Specification: 百度财经指数数据源切换

**Feature Branch**: `001-baidu-index-provider`

**Created**: 2026-07-10

**Status**: Draft — 待开发确认

**Input**: 将 MarketPulse 指数行情与指数 K 线的主数据源切换为百度财经（Baidu Finance），腾讯与东方财富降为备用；优先 WebSocket，WS 不可用时降级 HTTP 轮询，百度全不可用时再降级至现有腾讯/东方财富方案。

## 背景

- 百度财经 API 已完成调研，文档位于 `docs/providers/baidu_finance.md` 与 `openapi.yaml`
- 当前指数行情：Tencent（primary）→ Eastmoney（fallback），REST 轮询
- 当前指数 K 线：Eastmoney（primary），日 K 与 Tencent 竞速；Tencent 作为部分指数的补充

## User Scenarios & Testing

### User Story 1 - 指数实时行情以百度为主 (Priority: P1)

作为行情看板用户，我希望全球指数（上证、恒生、道指、黄金等）优先从百度财经获取，以便获得更稳定、更低延迟的数据。

**Why this priority**: 指数行情是看板核心展示，直接影响用户体验。

**Independent Test**: 启动 `marketd`，观察 `/api/v1/market/snapshot` 中 `indices` 字段来源；`GET /api/v1/market/providers/status` 中 `baidu_index` 为 primary 且 `current_used=true`。

**Acceptance Scenarios**:

1. **Given** 百度 WS 可用，**When** 服务启动，**Then** 指数行情通过 WS 推送更新，`baidu_index` 状态 healthy
2. **Given** 百度 WS 连接失败，**When** 超过重连阈值，**Then** 自动降级为百度 HTTP 轮询（`getquotation`），不中断指数展示
3. **Given** 百度 HTTP 也失败（熔断），**When** 轮询周期到达，**Then** 按 `tencent` → `eastmoney` 顺序拉取缺失指数，整体状态 degraded 但仍有数据
4. **Given** 仅部分指数百度无映射，**When** 拉取该指数，**Then** 跳过百度直接走备用源，不影响其他指数

---

### User Story 2 - 指数 K 线以百度为主 (Priority: P1)

作为看板用户，我在 K 线抽屉查看指数历史走势时，希望优先使用百度 `getstockquotation` 接口。

**Why this priority**: K 线与实时行情同属指数数据域，需同步切换以保持一致性。

**Independent Test**: `GET /api/v1/market/index/klines?id=sh000001&interval=1d` 返回 `source` 字段含 `baidu`；百度不可用时返回 `eastmoney` 或 `tencent`。

**Acceptance Scenarios**:

1. **Given** 百度 K 线 API 正常，**When** 请求 `sh000001` 日 K，**Then** 返回数据且 `source=baidu`
2. **Given** 百度 K 线失败，**When** 请求同一指数，**Then** 降级 Eastmoney，再失败则 Tencent（日 K 竞速保留）
3. **Given** 缓存命中，**When** TTL 内重复请求，**Then** 使用缓存，不重复打上游

---

### User Story 3 - Provider 健康与可观测 (Priority: P2)

作为运维/开发者，我希望在 provider status API 中看到百度源的健康状态、角色与降级链路。

**Acceptance Scenarios**:

1. **Given** 服务运行，**When** 调用 `GET /api/v1/market/providers/status`，**Then** 包含 `baidu_index`（primary）、`tencent_index`（fallback）、`eastmoney_index`（fallback）
2. **Given** 百度 WS 降级到 HTTP，**When** 查看 status，**Then** `baidu_index` 仍 healthy，`last_error` 记录 WS 降级原因（如有）

---

### Edge Cases

- 百度 WS 对指数类标的可能不支持（调研矩阵标注「—」）→ 实现时探测，不支持则直接用 HTTP，不阻塞启动
- A 股收盘后、美股交易中：按现有 `equity.NextPollInterval` 动态调频
- 百度限流或无数据：熔断器与现有 `equityBreaker` 机制一致
- 商品类指数（gold/silver/crude）：需验证百度 `financeType=futures` 映射，无映射则跳过百度

## Requirements

### Functional Requirements

- **FR-001**: 系统 MUST 将百度设为指数行情第一优先级数据源
- **FR-002**: 系统 MUST 尝试百度 WS（`wss://finance-ws.pae.baidu.com`），失败时降级百度 HTTP 轮询
- **FR-003**: 系统 MUST 在百度不可用时，按 `tencent` → `eastmoney` 顺序降级
- **FR-004**: 系统 MUST 将百度设为指数 K 线第一优先级，保留 Eastmoney/Tencent 备用链
- **FR-005**: 系统 MUST 在 `IndexDef` 中维护百度 code/market/financeType 映射
- **FR-006**: 系统 MUST 注册 `baidu_index` provider 健康指标
- **FR-007**: 系统 MUST 不修改对外 API 契约（RFC-002），仅变更内部 `source` 字段值
- **FR-008**: 系统 MUST 通过配置开关控制百度源启用（默认启用）

### Non-Functional Requirements

- **NFR-001**: 百度 HTTP 请求速率 ≤ 10 req/s（自建限流）
- **NFR-002**: WS 重连最多 5 次，间隔 3s；与调研文档一致
- **NFR-003**: 改动限于 `internal/marketdata/ingest/`，不侵入 API handler 业务逻辑
- **NFR-004**: 新增代码须有单元测试（解析、映射、降级逻辑）

## Success Criteria

- [ ] 默认配置下指数行情 `providers` 顺序为 `baidu,tencent,eastmoney`
- [ ] 百度可用时 snapshot 指数数据主要来自百度
- [ ] 百度全挂时行为与切换前一致（腾讯+东财兜底）
- [ ] 指数 K 线 `source` 优先显示 `baidu`
- [ ] `go test -buildvcs=false ./...` 通过
- [ ] `docs/DATA_SOURCES.md` 同步更新（沉淀型文档，非 SDD 产物）

## Out of Scope

- 板块排行、快讯、热搜等其他百度能力（Phase 2）
- 前端 UI 变更
- 加密货币、美股参考（Bitget/Alpha）数据源变更

## References

- `docs/providers/baidu_finance.md`
- `docs/providers/openapi.yaml`
- `docs/DATA_SOURCES.md`
- `docs/RFC-002-api-contract.md`
- `internal/marketdata/ingest/equity/`
