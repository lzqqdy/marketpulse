# Implementation Plan: 资产中心（Portfolio）

**Branch**: `005-portfolio-asset-center` | **Date**: 2026-07-22 | **Spec**: [spec.md](./spec.md)

## Summary

落地独立 `portfolio` 模块：持仓与本金落 MySQL；实时总览用 `MarketDataService` 报价现算；日终 cron 写快照；提供旧库 `assets_log` 迁移命令。用户中心新增「资产中心」Tab（持仓编辑 + 总览卡片 + 快照表）。一期不做图表。

## Technical Context

**Language/Version**: Go 1.22；前端 Vue 3 + TypeScript  
**Primary Dependencies**: Gin、现有 users 会话、marketdata 只读服务、MySQL；可选 cron（进程内）  
**Storage**: MySQL（持仓、设置/本金、日快照）；无强依赖 Redis（一期）  
**Testing**: `go test`（估值公式、日收益、迁移映射）；`npm run build`  
**Target Platform**: 单机 `marketd`  
**Project Type**: 全栈（Go API + Vue SPA）  
**Performance Goals**: 总览现算 ≤100 标的；日终串行/有限并发按用户  
**Constraints**: 不直连交易所；`portfolio.enabled` 灰度；兼容旧 `assets_log` 语义  
**Scale/Scope**: 个人/小团队；单用户持仓很少

## Constitution Check

| Gate | Status |
|------|--------|
| Module boundaries：portfolio 不写 market store、不依赖 ingest | Pass |
| Contract before code：新增 `/api/v1/portfolio/*` 同步 RFC-002 / contracts | Pass（实现前更新） |
| No exchange calls from portfolio | Pass |
| Persistence behind repository | Pass |
| 灰度/回滚：`portfolio.enabled` | Pass |

## 与旧系统对齐策略

| 旧能力 | 新实现 |
|--------|--------|
| `assets` 持仓 | `portfolio_holdings` |
| `user.balance` 本金 | `portfolio_settings.principal_cny` |
| `assets_log` 日快照 | `portfolio_snapshots`（字段对齐，便于迁移） |
| `assets_log.date='1'` 本金行 | 迁移入 settings；快照表可用 `kind=principal` 或直接不落日表 |
| `getAssetsWave` | `GET /api/v1/portfolio/overview` |
| `setAssets` | `PUT /api/v1/portfolio/holdings` |
| `assetLog` | `GET /api/v1/portfolio/snapshots` |
| cron `AssetProfitAnalysis` | `internal/portfolio/jobs/daily_snapshot.go` |
| 美股 yfinance 指数 | **改用** MarketPulse alpha / 指数报价（不复刻 yfinance） |
| 手动 `setUsdtPrice` | **不做**；用 OTC USDT/CNY |

公式（与旧逻辑兼容，细节见 [data-model.md](./data-model.md)）：

```text
total_usdt = Σ qty × price_usdt   (USDT 标的 price=1；alpha 按 Store 报价折合)
total_cny  = total_usdt × usdt_cny
today_pnl  = total_cny - last_snapshot.total_value_cny
pnl_7d     = total_cny - snapshot(date≈today-7).total_value_cny
pnl_30d    = total_cny - snapshot(date≈today-30).total_value_cny
hist_pnl   = total_cny - principal_cny
```

日终快照：

```text
date               = 昨日 (Asia/Shanghai)
total_value        = 当日估值 USDT
total_value_cny    = 当日估值 CNY
daily_profit       = total_value_cny - prev_day.total_value_cny
daily_profit_rate  = daily_profit / prev_day.total_value_cny
total_profit       = total_value_cny - principal_cny
total_profit_rate  = total_profit / principal_cny
asset_detail       = JSON[{symbol, asset_type, qty, price, value_usdt, value_cny}, ...]
```

## Architecture

```text
[Vue 资产中心]
   │ REST（session）
   ▼
/api/v1/portfolio/*
   │
   ▼
portfolio.Service
   ├ holdingsRepo / settingsRepo / snapshotRepo  → MySQL
   ├ MarketDataService.Quote / Rates            → 只读估值
   └ DailySnapshotJob（cron）                   → 写 snapshots
```

```text
迁移：
旧 MySQL assets_log (+ assets, user.balance)
        │
        ▼
cmd 或 scripts/migrate_assets_log
        │ uid map
        ▼
portfolio_snapshots / portfolio_holdings / portfolio_settings
```

## Project Structure

### Documentation (this feature)

```text
specs/005-portfolio-asset-center/
├── spec.md
├── plan.md                 # 本文件
├── data-model.md
├── research.md
├── contracts/
│   └── api.md
└── tasks.md                # 后续生成
```

### Source Code（拟新增）

```text
internal/portfolio/
  migrate/001_portfolio.sql
  types.go
  repo.go
  service.go              # holdings + settings + overview
  valuation.go            # 纯函数估值，便于单测
  jobs/daily_snapshot.go
  migrate_legacy.go       # 或 cmd/tools/migrate-assets-log
internal/api/portfolio.go
internal/config/          # portfolio.enabled, cron, default usdt_cny
cmd/marketd/              # wire
web/src/features/portfolio/
  api.ts
  types.ts
  AssetHoldingsPanel.vue
  AssetOverviewCard.vue
  AssetSnapshotsPanel.vue
  AssetCenterPanel.vue    # 组合上述
web/src/features/auth/views/UserCenterView.vue  # 增 Tab
```

## 前端交互（一期）

参考旧资产页，适配 MarketPulse 暗色用户中心：

1. **持仓表**：Coin/Symbol | 类型 | Num（可编辑） | Value(U) | Estimated(¥) | 日内估变（可选）
2. **资产总览卡**：总资产 + U溢价；今日收益；7日 / 30日 / 历史
3. **快照表**：分页 + 涨跌色；本金在设置区，不混入日表

交互：数量失焦或点保存即 PUT；总览定时刷新（如 5s）。

## 实现顺序（建议）

1. DDL + repo + 配置开关  
2. holdings/settings API + 单测估值  
3. overview API  
4. snapshots 列表 API + daily job  
5. 迁移命令 + 样例验证  
6. 前端三块 UI 接入用户中心  
7. 更新 `docs/MODULES.md` 状态、`RFC-002`、`docs/README.md`

## 灰度与回滚

| 手段 | 说明 |
|------|------|
| `portfolio.enabled: false` | 关闭 API 与 cron；前端隐藏 Tab |
| DB 回滚 | 删表或保留数据仅关开关（推荐关开关） |
| 迁移回滚 | 按 `user_id` 删除导入批次（迁移写 `source=legacy` 便于清理） |

## 风险

| 风险 | 缓解 |
|------|------|
| 旧 date='1' / 收益率小数 vs 百分数混用 | 迁移规范化；见 research |
| alpha 报价单位与 crypto 不一致 | valuation 统一折 USDT |
| 日终无价 | 跳过用户 + 告警日志；次日可补跑 |
| uid 映射错误 | 迁移要求显式 map 文件，禁止默认 uid=1 盲导 |
| 与 alerts 同时依赖 MySQL | 启动顺序与 users 一致；互不影响表 |

## 二期预留

- `GET /api/v1/portfolio/reports/series?from&to`（由 snapshots 聚合）
- 前端 ECharts/lightweight-charts：累计收益、收益率、资产走势、饼图分布（读 `asset_detail`）
- 不在一期实现，避免拖慢闭环
