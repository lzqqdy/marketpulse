# RFC-002：API / WebSocket 契约

| 字段 | 内容 |
|------|------|
| 状态 | Active |
| 依赖 | RFC-001 |
| 日期 | 2026-05-16 |
| 最后对齐 | 2026-07-11 |

> 实现前后端时以此为准；变更需更新本文并 bump 修订记录。

---

## 1. 通用约定

- 行情模块 canonical 路径：`/api/v1/market/*` 与 `/ws/v1/market/*`
- 旧行情路径保留兼容：`/api/v1/snapshot`、`/api/v1/klines`、`/ws/v1/stream` 等
- 时间：Unix 毫秒 `ts`，ISO8601 字符串 `updatedAt`
- 数字：JSON number，前端展示自行格式化
- 错误 REST：

```json
{ "error": { "code": "INVALID_SYMBOL", "message": "..." } }
```

常见错误码：`INVALID_SYMBOL`、`INVALID_INDEX`、`INVALID_INTERVAL`、`INVALID_MARKET`、`UPSTREAM_ERROR`

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
      "marketCapUsd": 1.9e12,
      "volume24hUsd": 3.2e10,
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
      "source": "baidu",
      "stale": false,
      "fetchedAt": "2026-05-16T12:00:00+08:00",
      "updatedAt": "2026-05-16T12:00:00+08:00"
    }
  ],
  "alpha": {
    "indices": [
      {
        "id": "qqq",
        "name": "纳指ETF",
        "symbol": "QQQUSDT",
        "price": 520.5,
        "change24hPct": 0.8,
        "changeDayPct": 0.3,
        "volume": 1234567,
        "markPrice": 520.6,
        "indexPrice": 520.4,
        "fundingRate": 0.0001,
        "updatedAt": "2026-05-16T12:00:00+08:00",
        "source": "bitget",
        "category": "index"
      }
    ],
    "stocks": [],
    "updatedAt": "2026-05-16T12:00:00+08:00",
    "source": "bitget"
  },
  "macro": {
    "totalMarketCapUsd": 2.45e12,
    "totalVolume24hUsd": 8.9e10,
    "totalMarketCapChange24hPct": 1.1,
    "fearGreed": { "value": 55, "label": "Neutral" },
    "btcDominancePct": 52.3,
    "ethDominancePct": 17.8,
    "stablecoinMarketCapUsd": 1.5e11,
    "stablecoinMarketCapChange24hPct": 0.2,
    "longShort": { "symbol": "BTCUSDT", "ratio": 1.05, "longAccountPct": 51.2, "shortAccountPct": 48.8, "updatedAt": "..." },
    "topLongShort": { "symbol": "BTCUSDT", "ratio": 1.1, "longAccountPct": 52.4, "shortAccountPct": 47.6, "updatedAt": "..." },
    "funding": { "symbol": "BTCUSDT", "rate": 0.0001, "markPrice": 97000, "indexPrice": 96950, "premiumPct": 0.05, "nextFundingTime": "...", "updatedAt": "..." },
    "openInterest": { "symbol": "BTCUSDT", "valueUsd": 2.5e10, "changePct": 1.2, "updatedAt": "..." },
    "takerBuySell": { "symbol": "BTCUSDT", "ratio": 1.02, "buyVol": 5000, "sellVol": 4900, "updatedAt": "..." },
    "liquidations": { "window": "1h", "longUsd": 5e7, "shortUsd": 3e7, "totalUsd": 8e7, "updatedAt": "..." }
  }
}
```

> `alpha` 为美股参考面板数据（UI 文案「美股参考」）。`source` 字段标识实际数据源（`bitget` / `binance-alpha`）。

---

## 3. GET /api/v1/market/providers/status

兼容路径：`GET /api/v1/providers/status`

数据源健康度。

### Response 200

```json
{
  "overall": {
    "status": "healthy",
    "healthy": 10,
    "total": 13,
    "avgLatencyMs": 120,
    "updatedAt": "2026-05-16T12:00:00+08:00"
  },
  "providers": [
    {
      "name": "binance_spot_ws",
      "label": "Binance Spot WS",
      "category": "crypto",
      "status": "healthy",
      "role": "primary",
      "currentUsed": true,
      "latencyMs": 0,
      "lastSuccessAt": "2026-05-16T12:00:00+08:00",
      "failCount": 0,
      "circuitOpen": false,
      "staleSeconds": 0
    }
  ]
}
```

Provider 状态枚举：`healthy`、`stale`、`circuit_open`、`unavailable`、`standby`、`disabled`。

完整 provider 列表见 `docs/DATA_SOURCES.md`。

---

## 4. GET /api/v1/market/klines

兼容路径：`GET /api/v1/klines`

K 线历史。支持 crypto 和 alpha 标的。

| 参数 | 默认 | 说明 |
|------|------|------|
| `symbol` | 必填 | 如 `BTC`、`NVDAUSDT` |
| `interval` | `1h` | `1m` `5m` `15m` `1h` `4h` `1d` `1w` |
| `limit` | `300` | 条数 |

### Response 200

```json
{
  "symbol": "BTC",
  "pair": "BTCUSDT",
  "interval": "1h",
  "source": "binance",
  "candles": [
    {
      "time": 1715850000,
      "open": 97000,
      "high": 97500,
      "low": 96800,
      "close": 97200,
      "volume": 1234.5,
      "quoteVolume": 1.2e8,
      "tradeCount": 5000,
      "takerBuyBaseVolume": 600,
      "takerBuyQuoteVolume": 5.8e7,
      "closed": true
    }
  ]
}
```

---

## 5. GET /api/v1/market/index-klines

兼容路径：`GET /api/v1/index-klines`

指数 K 线（REST only，无 WebSocket）。

| 参数 | 默认 | 说明 |
|------|------|------|
| `id` | 必填 | 如 `sh000001`、`hsi` |
| `interval` | `1d` | `15m` `1h` `1d` `1w` |
| `limit` | `300` | 条数 |

数据源优先级：Baidu → Eastmoney → Tencent（日 K 竞速）。

---

## 6. GET /api/v1/market/center

行情中心聚合数据（仅新命名空间，无 legacy 路径）。

| 参数 | 默认 | 说明 |
|------|------|------|
| `market` | `ab` | `ab`（A股）`hk`（港股）`us`（美股） |

### Response 200

```json
{
  "market": "ab",
  "source": "baidu",
  "fetchedAt": 1783668000,
  "chgdiagram": {
    "totalTitle": "成交额",
    "totalValue": "3.39万亿",
    "up": 3512,
    "down": 1612,
    "balance": 85,
    "bars": [{ "title": "涨停", "status": "up", "count": 92 }]
  },
  "heatmap": {
    "sortKey": "amount",
    "typeCode": "HY",
    "items": [{ "code": "650000", "name": "国防军工", "pxChangeRate": 2.5, "metricValue": "78.85亿" }]
  },
  "fundflow": {
    "groups": [{ "blockType": "HY", "blockTypeName": "行业", "items": [{ "code": "650000", "name": "国防军工", "mainNetTurnover": "+78.85亿", "netAmount": 7884525184 }] }]
  },
  "overview": {
    "tabs": [{ "type": "HY", "name": "行业板块", "items": [{ "code": "650100", "name": "航天装备Ⅱ", "price": 8932.24, "changePct": 10.36, "leadStock": { "code": "688523", "name": "航天环宇科技", "changePct": 20.01 }, "trend": [8930, 8935, 8940] }] }]
  }
}
```

前置条件：`ingest.baidu.enabled=true`，否则返回 `UPSTREAM_ERROR`。

---

## 7. GET /api/v1/market/center/heatmap

热力图子接口（sortKey 切换时懒加载）。

| 参数 | 默认 | 说明 |
|------|------|------|
| `market` | `ab` | `ab` `hk` `us` |
| `sortKey` | `amount` | `amount` `volume` `marketValue` |

响应结构与 §6 中 `heatmap` 字段相同。

---

## 8. GET /api/v1/market/expressnews

7×24 财经快讯（按需转发百度 `expressnews`，服务端缓存）。

| 参数 | 默认 | 说明 |
|------|------|------|
| `tag` | `""` | `""` `A股` `港股` `美股` `异动` |
| `pn` | `0` | 页码，从 0 起 |
| `rn` | `20` | 每页条数，最大 50 |
| `filterByUserStocks` | `0` | `1` 时仅返回自选股相关快讯 |

```json
{
  "tag": "",
  "pn": 0,
  "rn": 20,
  "source": "baidu",
  "fetchedAt": 1783758335,
  "hasMore": true,
  "items": [
    {
      "id": "http://dps.baidu.com/data/finance_stock_express_news/...",
      "title": "腾讯洽谈Manus相关事宜",
      "body": "7月11日，有消息称腾讯正在洽谈成为Manus股东。(Finscope)",
      "publishTime": 1783758335,
      "provider": "Finscope",
      "entities": [
        {
          "code": "00700",
          "name": "腾讯控股",
          "market": "hk",
          "ratio": "-2.00%",
          "changePct": -2,
          "logoUrl": "https://..."
        }
      ]
    }
  ]
}
```

缓存 TTL：首页有新快讯 30s；首页无变化 2min；历史页 10min。

前置条件：`ingest.baidu.enabled=true`，否则返回 `UPSTREAM_ERROR`。

---

## 9. GET /healthz

```json
{
  "status": "ok",
  "uptimeSec": 3600,
  "symbolCount": 5,
  "storeVersion": 1024,
  "appMode": "debug",
  "ingest": {
    "binance_ws": "connected",
    "otc": "ok",
    "forex": "ok",
    "equity": "ok",
    "equity_baidu": "ok",
    "equity_baidu_ws": "connected",
    "equity_tencent": "ok",
    "equity_eastmoney": "ok",
    "macro": "ok",
    "crypto_meta": "ok",
    "long_short": "ok",
    "funding": "ok",
    "open_interest": "ok",
    "liquidations": "ok",
    "liquidations_ws": "connected",
    "sge_gold": "ok",
    "market_center": "ok",
    "expressnews": "ok",
    "alpha_poll": "ok"
  }
}
```

`ingest` 值为字符串状态：`starting`、`ok`、`error`、`connected`、`disconnected`、`reconnecting`、`degraded`、`circuit_open`、`disabled`。

---

## 10. WebSocket /ws/v1/market/stream

兼容路径：`WS /ws/v1/stream`

### 连接

```
wss://{host}/ws/v1/market/stream?channels=quotes,rates,indices,macro,alpha
```

默认频道：`quotes`。可用频道：`quotes`、`rates`、`indices`、`macro`、`alpha`。

### 服务端 → 客户端

**连接时首包 snapshot**

```json
{ "type": "snapshot", "data": { /* 完整 Snapshot 对象 */ } }
```

**quotes**

```json
{
  "type": "quotes",
  "version": 42,
  "ts": 1715850000123,
  "data": [{ "symbol": "BTC", "priceUsdt": 97001.0, "change24hPct": 1.24 }]
}
```

**rates / indices / macro / alpha** — 结构同 snapshot 对应子对象，含 `version` 和 `ts`。

**pong**

```json
{ "type": "pong", "ts": 1715850000000 }
```

推送机制：Store 变更后 100ms debounce 广播。

### 客户端 → 服务端

```json
{ "op": "ping" }
```

> 频道在连接时通过 query 参数指定，不支持运行时 `subscribe` 操作。

---

## 10. WebSocket /ws/v1/market/kline

兼容路径：`WS /ws/v1/kline`

```
wss://{host}/ws/v1/market/kline?symbol=BTC&interval=1h
```

连接后行为：

1. REST 拉取历史 K 线
2. 推送 `kline_snapshot` 全量
3. 订阅 Binance/Bitget kline WS，推送 `kline_update` 增量
4. Alpha 标的降级时 REST 轮询（30s）

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

**error**

```json
{ "type": "error", "code": "INVALID_SYMBOL", "message": "..." }
```

切换周期 = 断开并重连新 `interval`。

---

## 11. 前端 TypeScript 类型

实现位置：

| 类型 | 文件 |
|------|------|
| `MarketSnapshot`、`Quote`、`IndexQuote`、`AlphaSnapshot`、`MacroSnapshot` | `web/src/features/market/types/market.ts` |
| `Candle`、`KlineInterval` | `web/src/features/market/types/chart.ts` |
| `MarketCenterResponse` | `web/src/features/market/types/marketCenter.ts` |
| `ProviderStatusResponse` | `web/src/features/market/types/providers.ts` |

字段与本文保持一致。

---

## 12. 版本与兼容

- `snapshot.version` 与 WS `version` 单调递增
- 前端忽略 `version` 小于本地的包
- API 路径带 `/v1`，破坏性变更升 `/v2`
- Legacy 路径与 canonical 路径返回相同数据，新客户端应使用 `/api/v1/market/*`

---

## 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 0.1 | 2026-05-16 | 草案 |
| 0.2 | 2026-05-24 | 增加 market canonical namespace，保留旧路径兼容 |
| 1.0 | 2026-07-11 | 对齐实现：alpha、macro 衍生品、providers/status、index-klines、market/center、expressnews、healthz 字段 |
