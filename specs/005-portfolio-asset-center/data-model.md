# Data Model: 资产中心

**Feature**: `005-portfolio-asset-center`  
**Date**: 2026-07-22

## 设计原则

1. **语义对齐旧 `assets` / `assets_log`**，便于迁移与用户心智一致。  
2. **本金显式化**：不再依赖魔法 `date='1'` 作为唯一真相；迁移时双写兼容。  
3. **快照不可变历史**：改本金/持仓不回溯改已写入日快照（除非运维重建工具，一期不做）。  
4. **标的类型显式**：`asset_type` + `symbol`，支持 crypto 与 alpha。

## 表结构（拟）

### `portfolio_settings`

用户级资产设置（一本金一行）。

| 列 | 类型 | 说明 |
|----|------|------|
| `user_id` | BIGINT PK | 对应 `users.id` |
| `principal_cny` | DECIMAL(18,2) NOT NULL DEFAULT 0 | 本金（¥） |
| `principal_usdt` | DECIMAL(18,8) NULL | 可选缓存：本金/汇率 |
| `created_at` | DATETIME | |
| `updated_at` | DATETIME | |

### `portfolio_holdings`

| 列 | 类型 | 说明 |
|----|------|------|
| `id` | BIGINT PK AI | |
| `user_id` | BIGINT NOT NULL | |
| `asset_type` | VARCHAR(16) NOT NULL | `crypto` \| `alpha` |
| `symbol` | VARCHAR(32) NOT NULL | 如 `BTC`、`BTCUSDT`、`NVDAUSDT`（与行情 Store 键约定统一，实现时定一种并文档化） |
| `quantity` | DECIMAL(24,8) NOT NULL DEFAULT 0 | 持仓数量 |
| `target_price` | DECIMAL(24,8) NULL | 一期可空；兼容旧 target_price |
| `created_at` / `updated_at` | DATETIME | |
| UNIQUE | (`user_id`,`asset_type`,`symbol`) | |

> 旧表 `freeze_num` 一期不迁移业务含义（可忽略或写入 detail 备注）。

### `portfolio_snapshots`

对齐旧 `assets_log`。

| 列 | 类型 | 说明 |
|----|------|------|
| `id` | BIGINT PK AI | |
| `user_id` | BIGINT NOT NULL | |
| `date` | DATE NOT NULL | 快照业务日（上海） |
| `kind` | VARCHAR(16) NOT NULL DEFAULT `daily` | `daily` \| `principal` |
| `total_value` | DECIMAL(18,8) NOT NULL | 总值 USDT |
| `total_value_cny` | DECIMAL(18,2) NOT NULL | 总值 CNY |
| `daily_profit` | DECIMAL(18,2) NOT NULL DEFAULT 0 | 日收益 CNY |
| `daily_profit_rate` | DECIMAL(18,8) NOT NULL DEFAULT 0 | **小数**（0.0127 = 1.27%），与旧库一致 |
| `total_profit` | DECIMAL(18,2) NOT NULL DEFAULT 0 | 累计收益 CNY |
| `total_profit_rate` | DECIMAL(18,8) NOT NULL DEFAULT 0 | **小数** |
| `asset_detail` | JSON/TEXT NULL | 持仓明细快照 |
| `source` | VARCHAR(16) NOT NULL DEFAULT `system` | `system` \| `legacy` \| `manual` |
| `created_at` | DATETIME | 插入时间（替代旧 ctime） |
| UNIQUE | (`user_id`,`date`,`kind`) | |

列表 API 默认：`kind='daily'`。

### 本金基准行策略

| 场景 | 行为 |
|------|------|
| 用户设置本金 | 更新 `portfolio_settings`；可选 upsert `kind=principal` 快照（`date` 可用账户启用日或固定 epoch date 由实现选定，**推荐只用 settings，principal 快照仅服务迁移对照**） |
| 迁移旧 `date='1'` | `principal_cny = total_value_cny`；写 settings；可选写 `kind=principal` |
| 累计收益计算 | **权威：`portfolio_settings.principal_cny`**；若为 0 则回退「最早 daily 快照」或显示 — |

## `asset_detail` JSON 约定

```json
[
  {
    "asset_type": "crypto",
    "symbol": "BTC",
    "quantity": 0.15,
    "price_usdt": 65991.6,
    "value_usdt": 9898.74,
    "value_cny": 66321.58
  },
  {
    "asset_type": "alpha",
    "symbol": "NVDAUSDT",
    "quantity": 10,
    "price_usdt": 120.5,
    "value_usdt": 1205,
    "value_cny": 8073.5
  }
]
```

迁移旧格式 `[{"coin":"BTC","freeze_num":0,"total_num":0.16,...}]`：导入时规范化为上式；保留原始可放 `raw` 字段可选。

## 旧表 → 新表映射

### `assets_log` → `portfolio_snapshots`

| 旧列 | 新列 | 转换 |
|------|------|------|
| uid | user_id | **映射表** |
| date（`YYYY-MM-DD`） | date + kind=`daily` | |
| date=`1` | settings + 可选 kind=`principal` | 不进日列表 |
| total_value | total_value | |
| total_value_cny | total_value_cny | |
| daily_profit | daily_profit | |
| daily_profit_rate | daily_profit_rate | 保持小数 |
| total_profit | total_profit | |
| total_profit_rate | total_profit_rate | 旧库偶见已是倍数（如 2.32=232%）；迁移时检测 `>1` 且历史收益合理则视为小数倍数已正确；展示层统一 `*100` 加 `%` |
| asset_detail | asset_detail | JSON 规范化 |
| ctime | created_at | Unix → DATETIME |
| — | source=`legacy` | |

### `assets` → `portfolio_holdings`

| 旧列 | 新列 |
|------|------|
| uid | user_id（映射） |
| coin | symbol；asset_type=`crypto`（旧无 alpha） |
| total_num | quantity |
| target_price | target_price |

### `user.balance` → `portfolio_settings.principal_cny`

若与 `date=1` 冲突：以 **较新 updated / 非空 balance** 为准，写迁移报告。

## 估值符号约定（实现前钉死）

建议：

- crypto：Store 键与看板一致（如 `BTCUSDT` 或 base `BTC`），`valuation.go` 内统一 resolve。  
- alpha：与 `AlphaStockPanel` / config `alpha.stocks` 符号一致。  
- USDT：quantity 计 USDT，price=1。

## 索引

- `portfolio_holdings(user_id)`  
- `portfolio_snapshots(user_id, date DESC)`  
- `portfolio_snapshots(user_id, kind, date)`

## 与 users 关系

- 不修改 `users` 表结构加 balance（避免污染身份模块）。  
- 本金只住 `portfolio_settings`。
