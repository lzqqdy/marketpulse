# Contract: AI Assistant API

**Status**: Implemented（一期，2026-07-23）

**Feature**: `007-ai-assistant`  
**Base**: `/api/v1/ai`  
**Auth**: 与 users 相同 session（`Authorization: Bearer …` 或 `X-Session-Token`）；未登录 401  
**Gate**: `ai.enabled=false` → 403/503 + `code=ai_disabled`（与 alerts/portfolio 对齐）

## 约定

- JSON 字段 **camelCase**（对齐 portfolio 契约）。  
- 对外会话 ID 使用 `conversationId` = `ai_conversations.public_id`。  
- Chat 成功路径为 **SSE**；错误可在建立流前返回普通 JSON，或在流内发 `event: error`。

---

## Endpoints

### POST `/api/v1/ai/chat`

发送一条用户消息；可选续聊；**SSE** 流式返回。

**Request**

```json
{
  "conversationId": "01JXXXXOPTIONAL",
  "message": "这个怎么看？",
  "context": {
    "focusSymbol": "BTCUSDT",
    "assetClass": "crypto",
    "page": "dashboard",
    "visibleSymbols": ["BTCUSDT", "ETHUSDT"]
  }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `message` | 是 | 非空，长度上限实现定（建议 ≤ 4000 字） |
| `conversationId` | 否 | 缺省则创建新会话 |
| `context` | 否 | 页面上下文；见 spec |

**Response**：`Content-Type: text/event-stream`

事件类型（`event:` 行；`data:` 为 JSON）：

| event | data 示例 | 说明 |
|-------|-----------|------|
| `meta` | `{"conversationId":"…","messageId":123}` | 流开始；便于前端绑定会话 |
| `token` | `{"text":"……"}` | 助手文本增量 |
| `tool_start` | `{"name":"get_quote","arguments":{…}}` | 可选 |
| `tool_result` | `{"name":"get_quote","ok":true,"summary":"…"}` | 可选；勿回传超大原始 payload |
| `done` | `{"finishReason":"stop","conversationId":"…"}` | 正常结束 |
| `error` | `{"code":"ai_upstream","message":"…"}` | 失败结束 |

**Notes**

- 同一 `conversationId` 同时只允许一个 in-flight chat；冲突 → 流前 **409** + `code=ai_conversation_busy`。  
- 超日配额 → 流前 **429** + `code=ai_quota_exceeded`。  
- 客户端断开：服务端取消 LLM/工具（尽力而为）。

---

### GET `/api/v1/ai/conversations`

列出会话（默认排除软删）。

**Query**：`page`（默认 1）、`pageSize`（默认 20，最大 50）

**Response 200**

```json
{
  "total": 3,
  "page": 1,
  "pageSize": 20,
  "items": [
    {
      "conversationId": "01JXXXX",
      "title": "BTC 走势",
      "updatedAt": "2026-07-23T20:00:00+08:00",
      "createdAt": "2026-07-23T19:50:00+08:00"
    }
  ]
}
```

---

### GET `/api/v1/ai/conversations/:conversationId/messages`

拉取某会话消息（用户可见角色：`user` / `assistant`；`tool`/`system` 默认不返回，或经 `?include=tools` 可选）。

**Query**：`limit`（默认 100）、`beforeId`（可选，向上翻页）

**Response 200**

```json
{
  "conversationId": "01JXXXX",
  "messages": [
    {
      "id": 1,
      "role": "user",
      "content": "这个怎么看？",
      "metadata": {
        "context": { "focusSymbol": "BTCUSDT" }
      },
      "createdAt": "2026-07-23T19:50:01+08:00"
    },
    {
      "id": 2,
      "role": "assistant",
      "content": "……",
      "metadata": { "finishReason": "stop" },
      "createdAt": "2026-07-23T19:50:15+08:00"
    }
  ]
}
```

非本人或不存在 → **404**。

---

### DELETE `/api/v1/ai/conversations/:conversationId`

软删会话。

**Response 204**（或 200 + `{ "ok": true }`，与项目其它 DELETE 风格对齐）

---

### PATCH `/api/v1/ai/conversations/:conversationId`（可选一期）

```json
{ "title": "BTC 复盘" }
```

---

## Errors

| 场景 | HTTP | code |
|------|------|------|
| 未登录 | 401 | （沿用 users） |
| 功能关闭 | 403/503 | `ai_disabled` |
| 校验失败 | 400 | `invalid_request` |
| 会话忙碌 | 409 | `ai_conversation_busy` |
| 日配额用尽 | 429 | `ai_quota_exceeded` |
| LLM/配置错误 | 502/503 | `ai_upstream` / `ai_misconfigured` |
| 会话不存在/无权限 | 404 | `not_found` |

---

## 工具面（非 HTTP，Agent 内部）

一期 Agent 可调用的逻辑工具（经 `MarketDataService`，返回已裁剪结构）：

| name | 用途 | 输入（示意） |
|------|------|--------------|
| `get_quote` | 单标报价 | `symbol`, 可选 `assetClass` |
| `get_snapshot_summary` | 宏观/盘面摘要 + Top 涨跌 | 可选 `limit` |
| `get_klines_summary` | K 线统计摘要 | `symbol`, `interval`, `limit` |
| `get_express_news` | 快讯列表摘要 | `tag?`, `limit` |
| `get_market_breadth` | 市场广度/热力摘要 | `market`=`cn`\|`hk`\|`us` |

二期：`get_portfolio_overview`（需 portfolio.enabled + 本人）。

**禁止**：下单、改持仓、改告警、读他人数据、任意 Shell。

---

## 非目标端点（一期不做）

- WebSocket chat  
- 公开免登录 chat  
- 深度分析 job：`POST /api/v1/ai/deep-analyze`（三期）
