# RFC-002：API / WebSocket 契约（草案）

| 字段 | 内容 |
|------|------|
| 状态 | Draft |
| 依赖 | RFC-001 |
| 日期 | 2026-05-16 |

> 实现前后端时以此为准；变更需更新本文并 bump `version` 说明。

---

## 1. 通用约定

- 基础路径：`/api/v1`
- 行情模块 canonical 路径：`/api/v1/market/*` 与 `/ws/v1/market/*`
- 旧行情路径暂时保留兼容：`/api/v1/snapshot`、`/api/v1/klines`、`/ws/v1/stream` 等
- 时间：Unix 毫秒 `ts`，ISO8601 字符串 `updatedAt`
- 数字：JSON number，前端展示自行格式化
- 错误 REST：

```json
{ "error": { "code": "NOT_FOUND", "message": "..." } }
```

---

## 2. GET /api/v1/market/snapshot

兼容路径：`GET /api/v1/snapshot`

首屏全量数据。

### Response 200

```json
{
  "version": 1,
  "ts": 1715850000000,
  "quotes": [
    {
      "symbol": "BTC",
      "priceUsdt": 97000.12,
      "priceCny": 704521.88,
      "changeDayPct": 0.85,
      "change24hPct": 1.23,
      "rank": 1,
      "iconUrl": "https://...",
      "updatedAt": "2026-05-16T12:00:01+08:00"
    }
  ],
  "rates": {
    "usdtCny": 7.26,
    "usdCny": 7.24,
    "updatedAt": "2026-05-16T12:00:00+08:00"
  },
  "indices": [
    {
      "id": "sh000001",
      "name": "上证",
      "price": 3123.45,
      "changePct": -0.32,
      "updatedAt": "2026-05-16T12:00:00+08:00"
    }
  ],
  "macro": {
    "totalMarketCapUsd": 2.45e12,
    "totalVolume24hUsd": 8.9e10,
    "totalMarketCapChange24hPct": 1.1,
    "fearGreed": { "value": 55, "label": "Neutral" },
    "btcDominancePct": 52.3,
    "ethDominancePct": 17.8
  },
  "internals": {
    "cn": {
      "breadth": {
        "total": 5123,
        "up": 3120,
        "down": 1680,
        "flat": 323,
        "up_pct": 60.9,
        "down_pct": 32.8,
        "advance_decline_ratio": 1.86,
        "median_change_pct": 0.42,
        "equal_weight_change_pct": 0.38,
        "up_turnover_pct": 58.2,
        "limit_up": 45,
        "limit_down": 12,
        "updatedAt": "2026-05-16T12:00:00+08:00",
        "source": "eastmoney"
      },
      "industry": [],
      "concept": [],
      "wind": {
        "summary": "市场宽度偏强，赚钱效应较好",
        "tags": ["宽度偏强"],
        "updatedAt": "2026-05-16T12:00:00+08:00"
      },
      "updatedAt": "2026-05-16T12:00:00+08:00"
    }
  }
}
```

---

## 2a. GET /api/v1/market/internals

A 股市场内部结构（宽度 + 板块 + 风向）。

查询参数：

| 参数 | 默认 | 说明 |
|------|------|------|
| `market` | `cn` | 当前仅支持 `cn` |

### Response 200

结构与 snapshot 中 `internals` 相同（`MarketInternals`）。

---

## 2b. GET /api/v1/market/breadth

仅返回 `internals.cn.breadth`（`MarketBreadth`）。

查询参数：同 internals（`market=cn`）。

---

## 2c. GET /api/v1/market/sectors

行业或概念板块排行。

查询参数：

| 参数 | 默认 | 说明 |
|------|------|------|
| `market` | `cn` | 当前仅支持 `cn` |
| `type` | 必填 | `industry` 或 `concept` |

### Response 200

```json
{
  "type": "industry",
  "sectors": [
    {
      "code": "BK0475",
      "name": "银行",
      "change_pct": 1.23,
      "turnover_rate": 0.85,
      "up_count": 28,
      "down_count": 4,
      "leader_name": "招商银行",
      "leader_change_pct": 2.1
    }
  ],
  "updatedAt": "2026-05-16T12:00:00+08:00"
}
```

---

## 2d. GET /api/v1/market/wind

规则化市场风向摘要（`MarketWind`）。

查询参数：同 internals（`market=cn`）。

---

## 3. GET /api/v1/market/klines

兼容路径：`GET /api/v1/klines`

K 线历史（Binance REST 代理）。

查询参数：

| 参数 | 默认 | 说明 |
|------|------|------|
| `symbol` | 必填 | 基础币种，如 `BTC` |
| `interval` | `1h` | `1m` `5m` `15m` `1h` `4h` `1d` `1w` |
| `limit` | `300` | 条数，最大 1000 |

### Response 200

```json
{
  "symbol": "BTC",
  "pair": "BTCUSDT",
  "interval": "1h",
  "source": "binance",
  "candles": [
    { "time": 1715850000, "open": 97000, "high": 97500, "low": 96800, "close": 97200, "volume": 1234.5 }
  ]
}
```

---

## 4. GET /healthz

```json
{
  "status": "ok",
  "uptimeSec": 3600,
  "ingest": {
    "binanceWs": "connected",
    "lastQuoteTs": 1715850000000,
    "forex": "ok",
    "equity": "ok"
  }
}
```

---

## 4. WebSocket /ws/v1/market/kline（K 线，推荐）

兼容路径：`WS /ws/v1/kline`

连接后服务端行为：

1. 内部 **一次** REST 拉取历史 K 线（非前端轮询）
2. 推送 `kline_snapshot` 全量
3. 订阅 Binance `@kline_{interval}`，推送 `kline_update` 增量

```
wss://{host}/ws/v1/market/kline?symbol=BTC&interval=1h
```

**kline_snapshot**

```json
{
  "type": "kline_snapshot",
  "symbol": "BTC",
  "interval": "1h",
  "source": "binance",
  "candles": [{ "time": 1715850000, "open": 97000, "high": 97500, "low": 96800, "close": 97200, "volume": 1234.5 }]
}
```

**kline_update**

```json
{
  "type": "kline_update",
  "symbol": "BTC",
  "interval": "1h",
  "candle": { "time": 1715853600, "open": 97200, "high": 97300, "low": 97100, "close": 97250, "volume": 100 }
}
```

切换周期 = 断开并重连新 `interval`（非轮询）。

---

## 5. WebSocket /ws/v1/market/stream

兼容路径：`WS /ws/v1/stream`

### 连接

```
wss://{host}/ws/v1/market/stream?channels=quotes,indices,macro,rates
```

### 客户端 → 服务端

```json
{ "op": "subscribe", "channels": ["quotes"] }
{ "op": "ping" }
```

### 服务端 → 客户端

**quotes（高频，可只含变更 symbol）**

```json
{
  "type": "quotes",
  "version": 42,
  "ts": 1715850000123,
  "data": [
    { "symbol": "BTC", "priceUsdt": 97001.0, "change24hPct": 1.24 }
  ]
}
```

**rates**

```json
{
  "type": "rates",
  "version": 10,
  "ts": 1715850000000,
  "data": { "usdtCny": 7.26, "usdCny": 7.24 }
}
```

**indices / macro** — 结构同 snapshot 子对象。

**pong**

```json
{ "type": "pong", "ts": 1715850000000 }
```

**error**

```json
{ "type": "error", "code": "INVALID_CHANNEL", "message": "..." }
```

---

## 5. 前端 TypeScript 类型（镜像）

实现时放在 `web/src/features/market/types/market.ts`，字段与本文保持一致。

---

## 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 0.1 | 2026-05-16 | 草案 |
| 0.2 | 2026-05-24 | 增加 market canonical namespace，保留旧路径兼容 |
| 0.3 | 2026-06-24 | 增加 A 股 Market Internals（breadth/sectors/wind）接口与 snapshot 字段 |
