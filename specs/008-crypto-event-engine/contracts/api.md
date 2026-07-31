# API Contract: Crypto Event Engine

**Feature**: `008-crypto-event-engine`  
**Status**: Draft  
**Auth**: 下列端点均为**公开只读**，**不需要**登录 token。  
**开关**: `event.enabled=false` 或模块 soft-disabled 时返回 `event_disabled`（HTTP 503）。

实现前将摘要同步到 `docs/RFC-002-api-contract.md`。

## REST

### `GET /api/v1/events`

列出市场事件。

Query（可选）:

| 参数 | 说明 |
|------|------|
| market | 如 `crypto`（一期主要值） |
| type | Event type |
| subType | Event sub_type |
| severity | `LOW` / `MEDIUM` / `HIGH` / `EXTREME`（可多值，实现可用逗号） |
| status | `DETECTED` / `ACTIVE` / `DEESCALATING` / `RESOLVED` |
| symbol | 如 `BTC` |
| startTime | Unix 秒或 RFC3339（实现钉死一种，推荐 Unix 秒） |
| endTime | 同上 |
| limit | 默认 20，最大 100 |
| cursor | 可选游标；若一期用 page 则 `page` + `pageSize` |

**200**

```json
{
  "items": [
    {
      "id": "evt_01JABC",
      "type": "CRYPTO_MOVE",
      "subType": "FLASH_SELLOFF",
      "title": "加密资产快速走弱",
      "description": "BTC 短时下跌伴随放量与爆仓放大",
      "severity": "HIGH",
      "score": 86.2,
      "peakScore": 91.0,
      "status": "ACTIVE",
      "symbols": ["BTC", "ETH"],
      "markets": ["CRYPTO"],
      "startTime": 1753941060,
      "endTime": null,
      "createdAt": 1753941060,
      "updatedAt": 1753941180
    }
  ],
  "nextCursor": "eyJ0IjoxNzUzfQ",
  "limit": 20
}
```

### `GET /api/v1/events/{eventID}`

事件详情（含 signals 摘要）。

**200**

```json
{
  "id": "evt_01JABC",
  "type": "CRYPTO_MOVE",
  "subType": "FLASH_SELLOFF",
  "title": "加密资产快速走弱",
  "description": "BTC 短时下跌伴随放量与爆仓放大",
  "severity": "HIGH",
  "score": 86.2,
  "peakScore": 91.0,
  "status": "ACTIVE",
  "symbols": ["BTC", "ETH"],
  "markets": ["CRYPTO"],
  "startTime": 1753941060,
  "endTime": null,
  "signals": [
    {
      "id": "sig_01",
      "signalType": "PRICE_DROP",
      "symbol": "BTC",
      "market": "CRYPTO",
      "value": -4.2,
      "baseline": 0.8,
      "ratio": 0,
      "changePct": -4.2,
      "window": "15m",
      "direction": "DOWN",
      "timestamp": 1753941060,
      "metadata": {}
    },
    {
      "id": "sig_02",
      "signalType": "VOLUME_SPIKE",
      "symbol": "BTC",
      "market": "CRYPTO",
      "value": 4800,
      "baseline": 1000,
      "ratio": 4.8,
      "changePct": 0,
      "window": "15m",
      "direction": "UP",
      "timestamp": 1753941120,
      "metadata": {}
    },
    {
      "id": "sig_03",
      "signalType": "LIQUIDATION_SPIKE",
      "symbol": "MARKET",
      "market": "CRYPTO",
      "value": 280000000,
      "baseline": 30000000,
      "ratio": 9.3,
      "changePct": 0,
      "window": "1h",
      "direction": "UP",
      "timestamp": 1753941180,
      "metadata": { "note": "exchange_wide_1h_window" }
    }
  ],
  "context": {},
  "createdAt": 1753941060,
  "updatedAt": 1753941180
}
```

**404**: `{ "error": { "code": "event_not_found", "message": "事件不存在" } }`

### `GET /api/v1/events/{eventID}/timeline`

时间线节点（Signal 出现 + 状态变更）。

**200**

```json
{
  "eventId": "evt_01JABC",
  "items": [
    {
      "ts": 1753941060,
      "kind": "signal",
      "label": "BTC 15m 价格下跌",
      "signalType": "PRICE_DROP",
      "symbol": "BTC",
      "payload": { "changePct": -4.2 }
    },
    {
      "ts": 1753941120,
      "kind": "signal",
      "label": "BTC 成交量放大",
      "signalType": "VOLUME_SPIKE",
      "symbol": "BTC",
      "payload": { "ratio": 4.8 }
    },
    {
      "ts": 1753941180,
      "kind": "status",
      "label": "状态变为 ACTIVE",
      "status": "ACTIVE",
      "payload": { "score": 86.2 }
    }
  ]
}
```

## WebSocket

### `WS /ws/v1/events`

公开连接，**无** `token` 查询参数要求。

服务端 → 客户端消息：

#### `event.created`

```json
{
  "type": "event.created",
  "event": {
    "id": "evt_01JABC",
    "type": "CRYPTO_MOVE",
    "subType": "FLASH_SELLOFF",
    "title": "加密资产快速走弱",
    "severity": "HIGH",
    "score": 86.2,
    "status": "ACTIVE",
    "symbols": ["BTC"],
    "markets": ["CRYPTO"],
    "startTime": 1753941060
  }
}
```

#### `event.updated`

```json
{
  "type": "event.updated",
  "event": {
    "id": "evt_01JABC",
    "status": "ACTIVE",
    "score": 94.2,
    "severity": "EXTREME",
    "updatedAt": 1753941300
  }
}
```

#### `event.resolved`

```json
{
  "type": "event.resolved",
  "event": {
    "id": "evt_01JABC",
    "status": "RESOLVED",
    "endTime": 1753942000,
    "score": 42.0,
    "severity": "LOW"
  }
}
```

客户端可发送文本 `ping`；服务端回复 `pong`（与行情 WS 习惯对齐，实现时统一）。

## 错误约定

| HTTP | code | 何时 |
|------|------|------|
| 503 | `event_disabled` | 开关关闭或依赖缺失 soft-skip |
| 404 | `event_not_found` | 详情/时间线 |
| 400 | `invalid_argument` | 查询参数非法 |

## 非目标（契约层）

- 无 `POST/PATCH/DELETE` 用户写接口（一期无订阅）
- 无需要 Bearer 的 Event 专用路由
- 不与 `/ws/v1/alerts/stream` 混用
