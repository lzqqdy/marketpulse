# Contract: Portfolio API

**Status**: Implemented（已同步 `docs/RFC-002-api-contract.md` §11.1）

**Feature**: `005-portfolio-asset-center`  
**Base**: `/api/v1/portfolio`  
**Auth**: 与 users 相同 session；未登录 401  
**Gate**: `portfolio.enabled=false` → 403/503 业务禁用（与 alerts 风格对齐）

> 实现前将定稿内容同步进 `docs/RFC-002-api-contract.md`。

## Endpoints

### GET `/api/v1/portfolio/holdings`

返回当前用户持仓列表（可含实时估值字段）。

**Response 200**

```json
{
  "holdings": [
    {
      "assetType": "crypto",
      "symbol": "BTC",
      "quantity": 0.15,
      "priceUsdt": 65991.6,
      "valueUsdt": 9898.74,
      "valueCny": 66321.58,
      "changeCny": -656.65
    }
  ],
  "principalCny": 20000,
  "usdtCny": 6.7,
  "usdtPremiumPct": 0.0
}
```

### PUT `/api/v1/portfolio/holdings`

全量或批量 upsert 持仓（实现选定一种；推荐 **按 symbol upsert + 显式 delete 列表**）。

**Request**

```json
{
  "holdings": [
    { "assetType": "crypto", "symbol": "BTC", "quantity": 0.15 },
    { "assetType": "alpha", "symbol": "NVDAUSDT", "quantity": 10 }
  ]
}
```

### PUT `/api/v1/portfolio/settings`

```json
{ "principalCny": 20000 }
```

### GET `/api/v1/portfolio/overview`

实时总览（对齐旧资产总览卡）。

```json
{
  "totalUsdt": 11213.22,
  "totalCny": 75128.58,
  "usdtCny": 6.7,
  "usdtPremiumPct": 0.0,
  "today": { "pnlCny": 2263.24, "pnlPct": 3.11 },
  "d7": { "pnlCny": 3452.23, "pnlPct": 4.82 },
  "d30": { "pnlCny": 4777.64, "pnlPct": 6.79 },
  "allTime": { "pnlCny": 55128.58, "pnlPct": 275.64 },
  "missingSymbols": []
}
```

缺失窗口收益时对应对象可为 `null`。

### GET `/api/v1/portfolio/snapshots`

Query: `page` `pageSize` `from` `to` `sort=date` `order=desc`

```json
{
  "total": 1200,
  "page": 1,
  "pageSize": 10,
  "items": [
    {
      "date": "2026-07-21",
      "totalValue": 10880.12,
      "totalValueCny": 72865.34,
      "dailyProfit": 1174.01,
      "dailyProfitRate": 0.0164,
      "totalProfit": 52865.34,
      "totalProfitRate": 2.643
    }
  ]
}
```

默认仅 `kind=daily`。

### GET `/api/v1/portfolio/eligible-symbols`（可选）

返回可添加的 crypto / alpha 符号列表（来自行情 Store / 配置），供前端选择器。

## Errors

| 场景 | 码 |
|------|-----|
| 未登录 | 401 |
| 功能关闭 | 403/503 + code=`portfolio_disabled` |
| 校验失败 | 400 |
| 符号无报价不可添加 | 400 + code=`symbol_unavailable` |

## 非 HTTP

- 日终任务：进程内 cron，无公开触发（可另加 admin 内部接口，一期可选）。  
- 迁移：CLI，非 REST。

## 二期（不实现，仅占位）

- `GET /api/v1/portfolio/reports/series`
- `GET /api/v1/portfolio/reports/allocation`
