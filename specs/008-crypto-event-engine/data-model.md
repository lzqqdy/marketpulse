# Data Model: 008-crypto-event-engine

**Date**: 2026-07-31  
**Status**: Draft  
**Spec**: [spec.md](./spec.md)

## 1. 领域对象

### 1.1 EventSignal

单一、可量化的异常事实。可先于 Event 存在于内存缓冲；落库时关联 `event_id`。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键，如 `sig_<ulid>` |
| event_id | string | 所属 Event；聚合前可空（仅内存） |
| signal_type | string | 见枚举 |
| symbol | string | 如 `BTC` / `ETH`；市场级可用 `MARKET` |
| market | string | 一期固定倾向 `CRYPTO` |
| value | float64 | 当前观测值 |
| baseline | float64 | 基线 |
| ratio | float64 | value/baseline（适用时） |
| change_pct | float64 | 涨跌幅 %（价格类） |
| window | string | 如 `5m` / `15m` / `1h` |
| direction | string | `UP` / `DOWN` / `NEUTRAL` |
| timestamp | time | 信号时刻 |
| metadata | JSON | 扩展（interval、sample_count 等） |

**signal_type 枚举（一期）**

```text
PRICE_DROP
PRICE_SPIKE
VOLUME_SPIKE
VOLATILITY_SPIKE
LIQUIDATION_SPIKE
```

### 1.2 MarketEvent

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键，如 `evt_<ulid>` |
| type | string | 如 `CRYPTO_MOVE` / `PRICE_ANOMALY` |
| sub_type | string | 如 `FLASH_SELLOFF` / `VOLUME_MOVE` / `DROP` |
| title | string | 展示标题（中文模板） |
| description | string | 短描述 |
| severity | string | `NORMAL` / `LOW` / `MEDIUM` / `HIGH` / `EXTREME` |
| score | float64 | 0–100 |
| peak_score | float64 | 生命周期内最高分 |
| status | string | 见生命周期 |
| start_time | time | 开始 |
| end_time | *time | 结束；未结束为空 |
| symbols | JSON/数组 | 主相关标的 |
| markets | JSON/数组 | 一期多为 `["CRYPTO"]` |
| context | JSON | EventContext；一期可 `{}` |
| created_at | time | |
| updated_at | time | |

### 1.3 EventContext（预留）

一期只保留结构，不强制填充：

```text
related_news: []
related_assets: []
macro_events: []
historical_events: []
market_state: {}
```

### 1.4 生命周期

```text
DETECTED → ACTIVE → DEESCALATING → RESOLVED
```

| 状态 | 含义 |
|------|------|
| DETECTED | 首次达触发条件（可极短，或创建时直接 ACTIVE） |
| ACTIVE | 异常持续 |
| DEESCALATING | 强度下降，未恢复正常 |
| RESOLVED | 恢复正常；写入 end_time |

实现建议：创建后若下一检测轮仍满足，立即 `ACTIVE`；避免 UI 长期停在 DETECTED。

## 2. Score 与 Severity

### 2.1 Score（事件强度，非投资置信度）

```text
Score =
  PriceScore       × w_price
+ VolumeScore      × w_volume
+ VolatilityScore  × w_vol
+ LiquidationScore × w_liq
```

一期默认权重（存在分量时归一化）：

| 分量 | 默认权重 |
|------|----------|
| Price | 0.35 |
| Volume | 0.25 |
| Volatility | 0.15 |
| Liquidation | 0.25 |

某类 Signal 不存在时：**从分母剔除**，不按 0 分计入。

各分量 0–100 由 ratio / |changePct| 相对阈值线性或分段映射（实现钉死在 scorer 包，单测覆盖）。

### 2.2 Severity 映射

| Score | Severity |
|-------|----------|
| 0–30 | NORMAL |
| 30–50 | LOW |
| 50–70 | MEDIUM |
| 70–85 | HIGH |
| 85–100 | EXTREME |

列表默认可过滤掉 NORMAL（配置或查询参数）。

## 3. MySQL 表

### 3.1 market_event

```sql
CREATE TABLE market_event (
  id            VARCHAR(64)  NOT NULL PRIMARY KEY,
  type          VARCHAR(64)  NOT NULL,
  sub_type      VARCHAR(64)  NOT NULL DEFAULT '',
  title         VARCHAR(256) NOT NULL,
  description   TEXT         NOT NULL,
  severity      VARCHAR(16)  NOT NULL,
  score         DOUBLE       NOT NULL DEFAULT 0,
  peak_score    DOUBLE       NOT NULL DEFAULT 0,
  status        VARCHAR(32)  NOT NULL,
  start_time    DATETIME(3)  NOT NULL,
  end_time      DATETIME(3)  NULL,
  symbols_json  JSON         NOT NULL,
  markets_json  JSON         NOT NULL,
  context_json  JSON         NOT NULL,
  created_at    DATETIME(3)  NOT NULL,
  updated_at    DATETIME(3)  NOT NULL,
  KEY idx_event_status_start (status, start_time),
  KEY idx_event_type_start (type, start_time),
  KEY idx_event_severity_start (severity, start_time),
  KEY idx_event_created (created_at)
);
```

### 3.2 market_event_signal

```sql
CREATE TABLE market_event_signal (
  id            VARCHAR(64)  NOT NULL PRIMARY KEY,
  event_id      VARCHAR(64)  NOT NULL,
  signal_type   VARCHAR(64)  NOT NULL,
  symbol        VARCHAR(64)  NOT NULL,
  market        VARCHAR(32)  NOT NULL,
  value         DOUBLE       NOT NULL DEFAULT 0,
  baseline      DOUBLE       NOT NULL DEFAULT 0,
  ratio         DOUBLE       NOT NULL DEFAULT 0,
  change_pct    DOUBLE       NOT NULL DEFAULT 0,
  window        VARCHAR(16)  NOT NULL DEFAULT '',
  direction     VARCHAR(16)  NOT NULL DEFAULT '',
  ts            DATETIME(3)  NOT NULL,
  metadata_json JSON         NOT NULL,
  KEY idx_sig_event (event_id),
  KEY idx_sig_symbol_ts (symbol, ts),
  KEY idx_sig_type_ts (signal_type, ts),
  CONSTRAINT fk_sig_event FOREIGN KEY (event_id) REFERENCES market_event(id)
);
```

### 3.3 market_event_relation（可选预留）

一期可不建。若建：

```text
id, event_id, related_event_id, relation_type, score, created_at
```

仅占位，不做图查询 API。

## 4. 配置模型（YAML）

挂在根配置 `event:` 段（示例）：

```yaml
event:
  enabled: false
  evaluate_interval: 20s
  symbols: [BTC, ETH]
  aggregate_window: 10m
  price:
    "5m": { threshold_pct: 2 }
    "15m": { threshold_pct: 3 }
    "1h": { threshold_pct: 5 }
  volume:
    lookback_bars: 20
    ratio_medium: 2
    ratio_high: 3
    ratio_extreme: 5
  volatility:
    lookback_bars: 20
    ratio: 2
  liquidation:
    sample_interval: 1m
    baseline_samples: 60
    ratio: 3
```

## 5. 内存热状态（不落库）

- ACTIVE Event 索引：`symbol+template → eventID`
- Signal 去重键：短 TTL
- Liquidation 环形采样缓冲
- 内部检测任务队列（listener → worker）

## 6. 校验规则

- `score` ∈ [0, 100]
- `status` 仅允许生命周期枚举
- RESOLVED 必须有 `end_time`
- `symbols_json` / `markets_json` 至少一端非空数组
- Signal 关联的 `event_id` 必须存在（落库时）
