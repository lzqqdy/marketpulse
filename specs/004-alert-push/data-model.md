# Data Model: 推送告警

**Feature**: `004-alert-push`  
**原则**: MySQL 仅必要持久表；冷却 / inbox / 限流用 Redis；滚动窗口与规则热索引用内存。

## MySQL

### `alert_rules`

| 列 | 类型 | 说明 |
|----|------|------|
| id | BIGINT PK AI | |
| user_id | BIGINT NOT NULL | 对应 users.id |
| asset_type | VARCHAR(32) NOT NULL | `spot` / `index` / `alpha`（美股参考） /（预留 `fx` `macro`） |
| symbol | VARCHAR(64) NOT NULL | 如 `BTCUSDT`、指数 id |
| field | VARCHAR(32) NOT NULL DEFAULT `price` | 监控字段 |
| rule_type | TINYINT UNSIGNED NOT NULL | 1–5 |
| params | JSON NOT NULL | 见下方 params 约定 |
| channels | JSON NOT NULL | `["in_app","email","pushplus"]` |
| frequency | VARCHAR(16) NOT NULL | `once` / `loop` / `daily` |
| interval_minutes | INT UNSIGNED NOT NULL DEFAULT 10 | 仅 loop 有意义 |
| set_price | DECIMAL(24,8) NOT NULL | 创建时价格快照 |
| status | VARCHAR(16) NOT NULL | `active` / `disabled` |
| last_triggered_at | BIGINT UNSIGNED NULL | unix 秒，可空 |
| trigger_count | INT UNSIGNED NOT NULL DEFAULT 0 | 冗余计数，可自 deliveries 聚合 |
| created_at | BIGINT UNSIGNED NOT NULL | |
| updated_at | BIGINT UNSIGNED NOT NULL | |
| is_deleted | TINYINT NOT NULL DEFAULT 0 | |

索引建议：

- `(user_id, is_deleted, status)`
- `(asset_type, symbol, status, is_deleted)` — 供启动加载 / 重建内存索引

#### `params` JSON 约定

| rule_type | params 字段 |
|-----------|-------------|
| 1 | `{"target": number}` → 上涨至 |
| 2 | `{"target": number}` → 下跌至 |
| 3 | `{"range": number, "upper": number, "lower": number}` 创建时写入上下界 |
| 4 | `{"ampl": number, "upper": number, "lower": number}` ampl 为百分比数值，如 `5` 表示 5% |
| 5 | `{"rapid_chg": number}` 滚动 5 分钟振幅阈值 % |

创建时除写入 params 外保存 `set_price`。

### `alert_deliveries`

| 列 | 类型 | 说明 |
|----|------|------|
| id | BIGINT PK AI | |
| rule_id | BIGINT NOT NULL | |
| user_id | BIGINT NOT NULL | |
| asset_type | VARCHAR(32) NOT NULL | 冗余便于列表 |
| symbol | VARCHAR(64) NOT NULL | |
| rule_type | TINYINT UNSIGNED NOT NULL | |
| channel | VARCHAR(16) NOT NULL | `in_app` / `email` / `pushplus` |
| trigger_value | DECIMAL(24,8) NOT NULL | 触发时观测值（价或振幅%） |
| title | VARCHAR(256) NOT NULL | |
| body | VARCHAR(1024) NOT NULL | |
| status | VARCHAR(16) NOT NULL | `success` / `failed` / `skipped` |
| error_msg | VARCHAR(512) NOT NULL DEFAULT '' | |
| created_at | BIGINT UNSIGNED NOT NULL | |

索引：`(user_id, created_at DESC)`、`(rule_id, created_at DESC)`。

> 不把「冷却截止时间」「未读 inbox」写入 MySQL。

## Redis

| Key | 结构 | TTL / 约束 | 用途 |
|-----|------|------------|------|
| `mp:alert:cd:{rule_id}` | string（任意占位） | once: 可短 TTL 或依赖 status；loop: `interval_minutes`；daily: 至当日结束+缓冲 | 推送冷却 |
| `mp:alert:inbox:{user_id}` | List（JSON 元素） | 无 TTL；`LTRIM` 限制 `inbox_max_len` | 站内未读离线队列 |
| `mp:alert:rl:email` / `mp:alert:rl:pushplus` | 计数或令牌 | 按分钟窗口 | 全局外发限流（可选） |

Inbox 元素建议：

```json
{
  "delivery_id": 123,
  "rule_id": 1,
  "title": "...",
  "body": "...",
  "symbol": "BTCUSDT",
  "created_at": 1710000000
}
```

## Memory

| 结构 | 说明 |
|------|------|
| `map[assetType:symbol][]RuleRef` | active 且未删规则；CRUD/启停时更新 |
| `map[symbol]*Window5m` | 滚动窗口：`high`/`low`/`points`（带时间戳价点，剔除 >5min） |

进程重启：从 MySQL 加载 active 规则重建索引；5m 窗口冷启动，短期 type5 可能不触发（可接受）。

## 与参考表 `alert_push` 对照

| 旧字段 | 新模型 |
|--------|--------|
| 单表混规则+状态+隐式历史 | 拆 `alert_rules` + `alert_deliveries` |
| scene 素数积 | `channels` JSON |
| frequency 0/1/2 | `once`/`loop`/`daily` + `interval_minutes` |
| lower/upper/ampl 列 | `params` JSON + 必要上下界快照 |
| 冷却靠 `{coin}-{mobile}_{type}` | `mp:alert:cd:{rule_id}` |

## 创建时「已满足则禁止」校验

| type | 禁止条件（有有效报价时） |
|------|--------------------------|
| 1 | `price >= target` |
| 2 | `price <= target` |
| 3/4 | `price >= upper OR price <= lower` |
| 5 | 当前滚动窗口振幅 `% >= rapid_chg`（窗口不足则视为未满足，允许创建） |
