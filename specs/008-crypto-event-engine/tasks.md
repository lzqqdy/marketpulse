# Tasks: 008-crypto-event-engine

**Input**: `specs/008-crypto-event-engine/`  
**Status**: Implemented（一期 Crypto MVP，2026-07-31）  
**Prerequisites**: [spec.md](./spec.md), [plan.md](./plan.md), [data-model.md](./data-model.md), [contracts/api.md](./contracts/api.md)

## Format

- **[P]**: 可并行
- **[USn]**: 对应用户故事

---

## Phase 1: Setup

**Purpose**: 包骨架与配置挂钩

- [x] T001 创建 `internal/event/` 目录结构（types、service、engine、detector、aggregator、scorer、lifecycle、repository、hub、migrate、bootstrap）按 [plan.md](./plan.md)
- [x] T002 [P] 在 `internal/config/config.go` 增加 `EventConfig` 与默认值/校验；`config/config.example.yaml`、`config/config.docker.yaml` 增加 `event:` 段
- [x] T003 [P] 编写 `internal/event/migrate/001_event.sql`（`market_event`、`market_event_signal`）对齐 [data-model.md](./data-model.md)

---

## Phase 2: Foundational

**Purpose**: 阻塞所有用户故事的基础能力

- [x] T004 实现 `repository` MySQL CRUD（CreateEvent、UpdateEvent、List、Get、ListSignals、InsertSignals）
- [x] T005 实现公共广播 `hub`（Register/Unregister/Broadcast）
- [x] T006 实现 `Bootstrap` soft-skip（`event.enabled` + MySQL）；接入 `cmd/marketd/main.go` 与 `internal/server/deps.go`
- [x] T007 [P] 实现 Score 权重归一化与 Severity 映射（`scorer`）+ 单测
- [x] T008 [P] 实现生命周期状态迁移（`lifecycle`）+ 单测

---

## Phase 3: US1 — Detectors

**Purpose**: 产出 Signal

- [x] T009 [US1] 实现 `detector/price.go`（基于 Klines 窗口 changePct 与配置阈值）
- [x] T010 [P] [US1] 实现 `detector/volume.go`（volume ratio）
- [x] T011 [P] [US1] 实现 `detector/volatility.go`（简化 ATR/std vs 基线）
- [x] T012 [P] [US1] 实现 `detector/liquidation.go`（Macro.Liquidations + 内存环形基线）
- [x] T013 [US1] Detector 单测（表格驱动：达阈/未达阈/K线不足跳过）

---

## Phase 4: US2 — Aggregator

**Purpose**: 同窗合成为单一 Event

- [x] T014 [US2] 实现模板匹配（flash_selloff / volume_move / price_only）与聚合窗 merge
- [x] T015 [US2] ACTIVE 索引：同 template+主 symbol 未结束则更新而非新建
- [x] T016 [US2] Aggregator 单测：三 Signal → 一 Flash Sell-off；仅 Price → price_only；窗外不合并

---

## Phase 5: Engine 调度

**Purpose**: 接入行情、节流跑检测

- [x] T017 实现 `engine`：AddListener 入队 + `evaluate_interval` ticker；调用 detectors → aggregator → scorer → lifecycle → repo → hub
- [x] T018 确保 listener 回调不阻塞（只投递）；Klines 调用带超时
- [x] T019 Engine 集成测试或可注入假 `MarketDataService` 的单测

---

## Phase 6: US3 — REST API

**Purpose**: 公开列表/详情/时间线

- [x] T020 [US3] 实现 `internal/api/events.go`：List / Get / Timeline，对齐 [contracts/api.md](./contracts/api.md)
- [x] T021 [US3] 在 `internal/api/routes.go` 注册 `/api/v1/events*`；`event_disabled` → 503
- [x] T022 [P] [US3] Handler 测试（禁用/空列表/详情 404）

---

## Phase 7: US4 — Public WebSocket

**Purpose**: 公开实时推送

- [x] T023 [US4] 实现 `GET /ws/v1/events` upgrade + hub 订阅；无 token 校验
- [x] T024 [US4] 在 Create/Update/Resolve 路径广播 `event.created` / `event.updated` / `event.resolved`
- [x] T025 [P] [US4] ping/pong 与断线清理

---

## Phase 8: US5 — 生命周期与评分打通

**Purpose**: 状态与分数在 API 可见

- [x] T026 [US5] 引擎路径写入 peak_score、status 迁移、end_time
- [x] T027 [US5] 详情/列表字段含 peakScore、severity、status；RESOLVED 含 endTime

---

## Phase 9: US6 + Frontend

**Purpose**: 配置可运维 + 看板可见

- [x] T028 [US6] 确认阈值全部来自 config；文档化热更是否需要重启（一期允许重启生效）
- [x] T029 [P] 前端 `web/src/features/events/types.ts` + `api.ts`
- [x] T030 [P] [US4] `useEventsStream.ts` 连接 `/ws/v1/events`
- [x] T031 [US3] `EventPanel.vue` + 详情抽屉/面板；挂载 `MarketDashboard`
- [x] T032 ingestHealth / 状态栏体现 event disabled 时隐藏面板

---

## Phase 10: Docs & Quality Gates

- [x] T033 同步 `docs/RFC-002-api-contract.md` Event 章节（摘自 contracts）
- [x] T034 更新 `docs/MODULES.md` Current Module Map（实现后将 Draft 标为 Implemented）
- [x] T035 [P] 更新根 `README.md` 功能表一行「市场异动」
- [x] T036 `go test -buildvcs=false ./internal/event/...` 与相关 api 测试通过
- [x] T037 `cd web && npm run build` 通过

---

## 验收清单（实现完成后勾选）

1. [ ] 注入/回放：PRICE_DROP + VOLUME_SPIKE + LIQUIDATION_SPIKE → 单一 FLASH_SELLOFF
2. [ ] 未登录：`GET /api/v1/events` 与看板面板可用
3. [ ] `WS /ws/v1/events` 无 token 可收 created/updated/resolved
4. [ ] `event.enabled: false` → API 503，行情正常
5. [ ] 无个人推送相关代码路径（无订阅表、无邮箱投递）

---

## 二期占位（不在本期实现）

- [ ] T100 用户订阅偏好 + 复用 alerts 投递通道
- [ ] T101 指数 / 美股参考 Price Detector
- [ ] T102 简化 Cross-Market RISK_OFF
- [ ] T103 Event Context 挂载 express news
- [ ] T104 AI tools：`search_events` / `get_event`
