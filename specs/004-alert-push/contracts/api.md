# API Contract: 推送告警

**Feature**: `004-alert-push`  
**Status**: Implemented  
**Auth**: 除特别说明外均需登录（Bearer / `X-Session-Token`），与 users 模块一致。  
**开关**: `alerts.enabled=false` 时返回 `alerts_disabled`（HTTP 503）。

已同步摘要到 `docs/RFC-002-api-contract.md` §11。`assetType` ∈ `spot|index|alpha`。

## REST

### `GET /api/v1/alerts/rules`

列出当前用户未删除规则。

Query（可选）: `status=active|disabled`、`assetType`、`symbol`、`ruleType`、`page`、`pageSize`、`sortBy`、`sortOrder`

**200**

```json
{
  "items": [
    {
      "id": 1,
      "assetType": "spot",
      "symbol": "BTCUSDT",
      "field": "price",
      "ruleType": 1,
      "params": { "target": 100000 },
      "channels": ["in_app", "pushplus"],
      "frequency": "loop",
      "intervalMinutes": 10,
      "setPrice": "97000.12",
      "status": "active",
      "triggerCount": 0,
      "createdAt": 1715850000,
      "updatedAt": 1715850000
    },
    {
      "id": 2,
      "assetType": "alpha",
      "symbol": "nvda",
      "field": "price",
      "ruleType": 1,
      "params": { "target": 150 },
      "channels": ["in_app", "email"],
      "frequency": "once",
      "intervalMinutes": 10,
      "setPrice": "140.5",
      "status": "active",
      "triggerCount": 0,
      "createdAt": 1715850100,
      "updatedAt": 1715850100
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 2
}
```

### `POST /api/v1/alerts/rules`

创建规则。服务端拉取当前报价做「已满足则拒绝」。

**Body**

```json
{
  "assetType": "spot",
  "symbol": "BTCUSDT",
  "field": "price",
  "ruleType": 1,
  "params": { "target": 100000 },
  "channels": ["in_app", "email"],
  "frequency": "once",
  "intervalMinutes": 10
}
```

校验：

- `ruleType` ∈ 1..5；`channels` 非空且 ⊆ 允许集合。
- `frequency=loop` 时 `intervalMinutes` ∈ `[loop_interval_min, loop_interval_max]`。
- 无有效报价 / 数据源不健康 → 错误。
- **当前已满足触发条件 → 错误**（如 `alert_condition_already_met`）。

**201**: 返回创建后的 rule 对象。

### `PATCH /api/v1/alerts/rules/:id`

可更新：`params`（部分类型）、`channels`、`frequency`、`intervalMinutes`、`status`。  
归属校验：必须为当前用户且未删除。  
若修改阈值导致「当前已满足」，**一期建议直接拒绝**（与创建一致，避免立即刷屏）；或仅允许改 status/channels — 实现选「拒绝改参至已满足」并在 tasks 写明。

### `DELETE /api/v1/alerts/rules/:id`

软删：`is_deleted=1`，并从内存索引移除。

### `GET /api/v1/alerts/deliveries`

Query: `page`（默认 1）、`pageSize`（默认 20，上限 100），可选 `ruleId`、`channel`。

**200**

```json
{
  "items": [
    {
      "id": 10,
      "ruleId": 1,
      "assetType": "spot",
      "symbol": "BTCUSDT",
      "ruleType": 1,
      "channel": "in_app",
      "triggerValue": "100001.5",
      "title": "BTCUSDT 上涨触达",
      "body": "...",
      "status": "success",
      "errorMsg": "",
      "createdAt": 1710000100
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### 通道资料

继续使用既有：

- `GET/PUT /api/v1/users/me` 字段 `email`、`wechatPushToken`

## WebSocket

### `GET /ws/v1/alerts/stream`

- 必须携带有效 session（query token 或与现有 auth 方式对齐，实现时统一）。
- 连接成功后：
  1. 服务端 `inbox` 未读批量推送（或先发 `inbox_snapshot` 再 `alert` 事件）。
  2. 之后实时 `alert` 事件。

**服务端 → 客户端消息示例**

```json
{
  "type": "alert",
  "data": {
    "deliveryId": 10,
    "ruleId": 1,
    "title": "...",
    "body": "...",
    "symbol": "BTCUSDT",
    "createdAt": 1710000100
  }
}
```

```json
{
  "type": "inbox_snapshot",
  "data": { "items": [ /* 同上结构数组 */ ] }
}
```

### `POST /api/v1/alerts/inbox/ack`（可选 REST）

Body: `{ "deliveryIds": [10, 11] }`  
从 Redis inbox 移除已读；WS 也可支持 `{"type":"ack","deliveryIds":[...]}`。

二选一即可，tasks 中定一种，推荐 **WS ack + REST ack 都支持** 便于断线。

## 错误码（建议）

| code | 含义 |
|------|------|
| `alerts_disabled` | 模块未开启 |
| `alert_condition_already_met` | 创建/改参时条件已满足 |
| `alert_invalid_params` | 参数非法 |
| `alert_symbol_unavailable` | 无报价或源不健康 |
| `alert_not_found` | 规则不存在或不属于当前用户 |
| `alert_channel_misconfigured` | 可选：创建时强校验；本期以 warn + 触发时 skipped 为主 |

## 前端约定

- 用户中心 Tab `alerts`：规则列表 + 创建表单 + 推送记录子视图。
- 布局根挂载 `AlertToastHost`：仅登录后连接 `/ws/v1/alerts/stream`。
- 标的选择器数据来自现有 market snapshot（与首页同源），无数据项不可选。
