# Implementation Plan: Crypto Event Engine（加密优先 MVP）

**Branch**: `008-crypto-event-engine` | **Date**: 2026-07-31 | **Spec**: [spec.md](./spec.md)

## Summary

在现有 Go 单体中新增 `internal/event`：只读 `MarketDataService`，用规则 Detector 产出 Signal，模板 Aggregator 合成 MarketEvent，MySQL 持久化，公开 REST + 公开 WS 广播，Vue 看板挂载异动面板。一期不做个人推送、不做 LLM 检测、不做跨市场完整 RISK_OFF。

## Technical Context

**Language/Version**: Go 1.22+；前端 Vue 3 + TypeScript  
**Primary Dependencies**: Gin；`MarketDataService`；MySQL（`platform/mysql`）；现有 WS upgrader 模式  
**Storage**: MySQL（`market_event` / `market_event_signal`）；内存热状态（ACTIVE 索引、爆仓基线环）  
**Testing**: `go test`（detector/aggregator/scorer/lifecycle）；前端 `npm run build`  
**Target Platform**: 单机 `marketd`  
**Project Type**: 全栈（Go API + Vue SPA）  
**Performance Goals**: 检测节流 15–30s；listener 回调只入队；不阻塞行情 WS  
**Constraints**: Constitution 模块边界；不直连 Provider；公开 API 无 auth  
**Scale/Scope**: 个人站；关注标的少量（配置列表）

## Constitution Check

| Gate | Status |
|------|--------|
| Module boundaries：`event` 不写 market store、不依赖 ingest/provider | Pass（设计） |
| Contract before code：见 [contracts/api.md](./contracts/api.md)；实现前同步 RFC-002 | Pass（Draft） |
| No exchange calls from event | Pass |
| Persistence behind repository | Pass |
| 灰度/回滚：`event.enabled` + MySQL soft-skip | Pass |

## Architecture

```text
MarketDataService
  ├ AddListener ──► event.Engine (queue + throttle)
  ├ Klines ───────► Price / Volume / Volatility detectors
  └ Snapshot.Macro.Liquidations ─► Liquidation detector + baseline ring
           │
           ▼
    Signals ─► Aggregator (templates + window)
           │
           ▼
    Scorer + Lifecycle
           │
           ├► repository (MySQL)
           └► Hub (broadcast WS)
                    │
        ┌───────────┼───────────┐
        ▼           ▼           ▼
   REST handlers  /ws/v1/events  web features/events
```

## Project Structure

### Documentation (this feature)

```text
specs/008-crypto-event-engine/
├── spec.md
├── research.md
├── data-model.md
├── plan.md              # this file
├── quickstart.md
├── tasks.md
└── contracts/api.md
```

### Source Code（实现阶段）

```text
internal/event/
├── types.go
├── config.go            # 从根 Config 映射
├── service.go           # 对外 Service 接口
├── engine.go            # 调度：listener / ticker / 入队
├── detector/
│   ├── price.go
│   ├── volume.go
│   ├── volatility.go
│   └── liquidation.go
├── aggregator/
│   └── aggregator.go
├── scorer/
│   └── scorer.go
├── lifecycle/
│   └── lifecycle.go
├── repository/
│   └── mysql.go
├── hub/
│   └── hub.go           # 公共广播
├── migrate/
│   └── 001_event.sql
└── bootstrap.go

internal/api/events.go
internal/config/config.go          # 增加 EventConfig
internal/server/deps.go            # 注入
cmd/marketd/main.go                # Bootstrap soft-skip

web/src/features/events/
├── api.ts
├── types.ts
├── useEventsStream.ts
└── components/
    ├── EventPanel.vue
    └── EventDetailDrawer.vue      # 或 Modal
```

## 实现要点

### Detector

- 输入：配置 symbols + interval 列表；调用 `Klines`
- 输出：`EventSignal` 列表（本轮）
- Liquidation：读 Snapshot；更新环形基线；ratio 超阈则产出 `LIQUIDATION_SPIKE`（symbol=`MARKET`）

### Aggregator

- 输入：本轮新 Signal + 当前 ACTIVE 集合
- 匹配 [research.md](./research.md) 模板；merge 或 create
- 聚合窗：`event.aggregate_window`（默认 10m）

### Scorer / Lifecycle

- 按 [data-model.md](./data-model.md) 权重归一化
- 状态机：DETECTED→ACTIVE→DEESCALATING→RESOLVED
- 每次持久化变更触发 Hub 广播

### API / WS

- 契约：[contracts/api.md](./contracts/api.md)
- Hub：连接列表 mutex + 广播；失败连接移除

### Frontend

- `EventPanel` 挂 `MarketDashboard` 侧栏
- 首屏 REST；其后 WS 增量
- `event` 在 ingestHealth / providers 中显示 disabled 时隐藏面板（对齐 AI FAB 模式）

### Config / Bootstrap

```text
event.enabled + mysql → Bootstrap
缺依赖 → soft-skip，日志提示
```

## 复杂度与风险

| 风险 | 缓解 |
|------|------|
| K 线拉取过多 | 少 symbol；节流；缓存最近 K 线短 TTL |
| 爆仓非按币种 | UI/文案标注全市场；metadata.note |
| 误报 | 阈值配置化；上线后调参 |
| listener 阻塞 | 只入队，worker 执行检测 |

## Phase 顺序（实现）

1. 表结构 + repository + types  
2. detectors + scorer 单测  
3. aggregator + lifecycle + engine  
4. REST + WS + main 装配  
5. 前端面板  
6. 同步 RFC-002 / MODULES / config.example  
7. 回放或注入验收 Flash Sell-off  

## 参考

- [research.md](./research.md)
- [data-model.md](./data-model.md)
- [contracts/api.md](./contracts/api.md)
- `internal/alerts/evaluator.go`（节流评测范式）
- `docs/MODULES.md`、`docs/DATA_SOURCES.md`
