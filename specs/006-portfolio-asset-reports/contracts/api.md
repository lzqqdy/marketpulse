# Contract: Portfolio Reports API

## GET `/api/v1/portfolio/reports/series`

Query: `range=7d|30d|90d|180d|1y|all`（默认 `30d`）

```json
{
  "range": "30d",
  "from": "2026-06-22",
  "to": "2026-07-21",
  "summary": {
    "startCny": 70000,
    "endCny": 72865.34,
    "pnlCny": 2865.34,
    "pnlPct": 4.09
  },
  "points": [
    {
      "date": "2026-06-22",
      "totalValueCny": 70000,
      "totalValue": 10447.76,
      "dailyProfit": 120.5,
      "dailyProfitRate": 0.0017,
      "totalProfit": 50000,
      "totalProfitRate": 2.5
    }
  ]
}
```

- `pnlPct` 为百分数展示单位（4.09 = 4.09%）；无起点时为 `null`。
- `points` 按 `date` 升序；`all` 时 `from` 为最早快照日。

## GET `/api/v1/portfolio/reports/allocation`

```json
{
  "totalCny": 75128.58,
  "totalUsdt": 11213.22,
  "items": [
    {
      "assetType": "crypto",
      "symbol": "BTC",
      "valueCny": 66321.58,
      "valueUsdt": 9898.74,
      "weightPct": 88.28
    }
  ],
  "missingSymbols": []
}
```

- `weightPct` 百分数；仅有价持仓参与合计。
