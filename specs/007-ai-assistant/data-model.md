# Data Model: AI 行情分析助手

**Feature**: `007-ai-assistant`  
**Date**: 2026-07-23

## 设计原则

1. **会话与消息为权威历史**：续聊以 DB 为准，不只靠前端内存。  
2. **工具细节可存可折**：完整 tool 轨迹利于排障；对用户默认折叠。  
3. **不污染 market store / users 表**：AI 状态仅住 `ai_*` 表。  
4. **context 按轮可选快照**：page context 写入该轮 user 消息的 `metadata`，便于回放「当时在看什么」。  
5. **软删优先**：会话 `deleted_at`；列表默认过滤。

## 表结构（拟）

### `ai_conversations`

| 列 | 类型 | 说明 |
|----|------|------|
| `id` | BIGINT PK AI | |
| `public_id` | CHAR(26) 或 VARCHAR(36) NOT NULL | 对外暴露 ID（ULID/UUID）；避免枚举自增 |
| `user_id` | BIGINT NOT NULL | `users.id` |
| `title` | VARCHAR(128) NOT NULL DEFAULT '' | 空则前端显示「新对话」；二期可自动生成 |
| `status` | VARCHAR(16) NOT NULL DEFAULT `active` | `active` \| `archived` |
| `created_at` | DATETIME(3) NOT NULL | |
| `updated_at` | DATETIME(3) NOT NULL | 每轮 chat 更新 |
| `deleted_at` | DATETIME(3) NULL | 软删 |
| UNIQUE | (`public_id`) | |
| INDEX | (`user_id`, `updated_at` DESC) | 列表 |
| INDEX | (`user_id`, `deleted_at`) | |

### `ai_messages`

| 列 | 类型 | 说明 |
|----|------|------|
| `id` | BIGINT PK AI | |
| `conversation_id` | BIGINT NOT NULL | FK → `ai_conversations.id` |
| `role` | VARCHAR(16) NOT NULL | `user` \| `assistant` \| `system` \| `tool` |
| `content` | MEDIUMTEXT NOT NULL | 展示用文本；tool 角色可存 JSON 字符串 |
| `metadata` | JSON NULL | 见下方约定 |
| `token_prompt` | INT NULL | 可选统计 |
| `token_completion` | INT NULL | 可选统计 |
| `created_at` | DATETIME(3) NOT NULL | |
| INDEX | (`conversation_id`, `id`) | 按插入序拉取 |

> 一期可不建物理 FK（与 portfolio 风格一致时以应用层保证）；若项目其它表用 FK 则跟随。

## `metadata` JSON 约定

### user 消息

```json
{
  "context": {
    "focusSymbol": "BTCUSDT",
    "assetClass": "crypto",
    "page": "dashboard",
    "visibleSymbols": ["BTCUSDT", "ETHUSDT"]
  },
  "clientMessageId": "optional-uuid"
}
```

### assistant 消息

```json
{
  "finishReason": "stop",
  "incomplete": false,
  "model": "deepseek-chat",
  "citations": [
    { "type": "symbol", "value": "BTCUSDT" },
    { "type": "news", "title": "...", "time": "2026-07-23T10:00:00+08:00" }
  ]
}
```

### tool 消息（可选落库）

```json
{
  "toolCallId": "call_xxx",
  "name": "get_quote",
  "arguments": { "symbol": "BTCUSDT" },
  "ok": true,
  "latencyMs": 12
}
```

`content` 存工具返回的**已裁剪** JSON 文本。

## 配额与限流（非业务表）

| 机制 | 存储 | Key 示例 |
|------|------|----------|
| 日配额 | Redis（优先） | `ai:quota:{userId}:{yyyyMMdd}` INCR，TTL 到次日上海时区 |
| 无 Redis | MySQL 计数表或拒绝开启配额严格模式 | 见 plan；一期若强依赖 Redis 则与 users 对齐要求写清 |

可选表（仅当不用 Redis 时）：

### `ai_usage_daily`（可选）

| 列 | 类型 | 说明 |
|----|------|------|
| `user_id` | BIGINT | |
| `day` | DATE | Asia/Shanghai |
| `chat_count` | INT NOT NULL DEFAULT 0 | |
| UNIQUE | (`user_id`, `day`) | |

## 与其它模块关系

```text
users.id ──< ai_conversations.user_id
ai_conversations.id ──< ai_messages.conversation_id

ai ──读──> MarketDataService（无表耦合）
ai ──读──> portfolio.Service（二期，无表耦合）
```

- 不把对话写入 `portfolio_*` / `alert_*`。  
- 不在 `marketdata` 内存 Store 增加 AI 字段。

## 保留与清理

- 一期：软删会话即可；不做自动归档 job。  
- 后续可加：N 天未更新会话归档；硬删工具消息仅留 assistant 摘要。

## 迁移

- `ai.auto_migrate=true` 时启动执行（对齐 portfolio/alerts）。  
- 迁移脚本目录建议：`internal/ai/migrate/`。
