# Implementation Plan: 推送告警（Alert Push）

**Branch**: `004-alert-push` | **Date**: 2026-07-14 | **Spec**: [spec.md](./spec.md)

## Summary

新增独立 `alerts` 模块：规则与推送记录落 MySQL；冷却 / 站内未读 inbox / 外发限流走 Redis；滚动 5 分钟高低与规则热索引走内存。评测消费 MarketStore 变更，投递异步，不影响行情主路径。通道：`in_app`（含离线）、SMTP 邮件、PushPlus。

## Technical Context

**Language/Version**: Go 1.22；前端 Vue 3 + TypeScript  
**Primary Dependencies**: Gin、gorilla/websocket、go-redis、MySQL、现有 users / marketdata  
**Storage**: MySQL（`alert_rules`、`alert_deliveries`）；Redis（冷却、inbox、限流）；内存（5m 窗口、按 symbol 规则索引）  
**Testing**: `go test`（评测/创建校验单测）；前端 `npm run build`；可选 SMTP/PushPlus mock  
**Target Platform**: 单机 `marketd`（与现网一致）  
**Project Type**: 全栈 Web（Go API + Vue SPA）  
**Performance Goals**: Store 回调内评测不阻塞写入；外发进 worker；币价路径评测尝试 <1s  
**Constraints**: 不直连交易所；`alerts` 依赖 `users`+mysql+redis；可配置关闭  
**Scale/Scope**: 个人/小团队看板量级；按 symbol 索引规则，避免全表秒扫

## Constitution Check

| Gate | Status |
|------|--------|
| Module boundaries：alerts 不写入 market store，不依赖 ingest 内部 | Pass |
| Contract before code：新增 `/api/v1/alerts/*` 与 WS 需同步 RFC-002 | Pass（实现前更新） |
| No exchange calls from alerts | Pass |
| Persistence behind repository | Pass |
| 灰度/回滚：`alerts.enabled` | Pass |

## Storage 分工（原则落地）

| 数据 | 存储 | 理由 |
|------|------|------|
| 告警规则 | MySQL `alert_rules` | 用户配置，需持久、备份、跨重启 |
| 推送记录 | MySQL `alert_deliveries` | 用户可查历史，需持久 |
| 规则冷却 | Redis `mp:alert:cd:{rule_id}` | 高频读写，TTL 自然过期 |
| 站内未读 inbox | Redis List/Stream `mp:alert:inbox:{user_id}` | 离线补达，非主流程，可限长 |
| 外发限流 | Redis | 保护 SMTP/PushPlus |
| 滚动 5m high/low | 进程内存 | 跟行情同进程，最低延迟；重启后窗口重建（短暂不触发可接受） |
| Active 规则按 symbol 索引 | 进程内存（CRUD/启动从 MySQL 加载） | 热路径；非权威源 |

## Architecture

```text
MarketStore.onChange(symbol)
        │
        ▼
AlertEvaluator（内存索引取 rules → 判定 → Redis 冷却检查）
        │ hit
        ▼
AlertDispatcher（异步）
   ├ in_app  → Redis inbox + 若在线则 WS push
   ├ email   → SMTP worker
   └ pushplus→ HTTP worker
        │
        └─ INSERT alert_deliveries
```

- type5：`WindowTracker` 在每次报价更新时维护 `symbol → ring(high,low,expiry)`。
- 冷却 key 必须含 `rule_id`，避免同币多规则互锁（参考项目坑）。

## Project Structure

### Documentation (this feature)

```text
specs/004-alert-push/
├── spec.md
├── plan.md              # 本文件
├── data-model.md
├── contracts/
│   └── api.md
└── tasks.md             # 后续 /speckit-tasks 生成
```

### Source Code

```text
internal/alerts/
  migrate/001_alerts.sql
  types.go
  repo.go
  service.go          # CRUD + create-time 条件校验
  evaluator.go
  window5m.go
  dispatcher.go
  channels/
    inapp.go
    email.go
    pushplus.go
  cooldown.go         # Redis
  inbox.go            # Redis
internal/api/alerts.go
internal/config/      # alerts + smtp
cmd/marketd/          # wire 开关与启动
web/src/features/alerts/
  api.ts
  types.ts
  AlertRulesPanel.vue
  AlertDeliveriesPanel.vue
  AlertToastHost.vue  # 布局层全局挂载
```

**Structure Decision**: 新模块 `internal/alerts` + `web/src/features/alerts`，符合 MODULES / VIBE_GUIDE。

## API / WS（摘要）

详见 [contracts/api.md](./contracts/api.md)。

- REST：`/api/v1/alerts/rules`、`/api/v1/alerts/deliveries`
- WS：`/ws/v1/alerts/stream`（登录必填；推送 + 拉未读）
- 通道资料继续 `PUT /api/v1/users/me`

## Config（草案）

```yaml
alerts:
  enabled: false          # 默认关，灰度时开
  daily_timezone: Asia/Shanghai
  loop_interval_min: 1
  loop_interval_max: 1440
  inbox_max_len: 100
smtp:
  host: ""
  port: 465
  username: ""
  password: ""
  from: ""
  # 注意：禁止使用参考项目的 rtmp 命名
```

## Complexity Tracking

无 Constitution 违规需开例外。相对参考项目的故意简化：废弃 scene 素数积；规则/记录分表；事件驱动替代秒级全表扫描。

## 实现顺序建议

1. DDL + repo + config 开关  
2. Rules CRUD + 创建时条件校验（依赖 MarketDataService.Quote/Snapshot）  
3. Window5m + Evaluator + Redis 冷却  
4. Dispatcher：in_app → email → pushplus  
5. Deliveries 查询 API  
6. 前端告警页 + 全局 Toast + WS  
7. 打开 `alerts.enabled` 灰度  

## 验证与回滚

- 验证：单测覆盖 type1–5 判定与「已满足禁止创建」；手工触发站内离线补达；关闭开关确认行情无变化。  
- 回滚：`alerts.enabled: false` 重启即可。
