# Implementation Plan: AI 行情分析助手（对话 Copilot）

**Branch**: `007-ai-assistant` | **Date**: 2026-07-23 | **Spec**: [spec.md](./spec.md)

## Summary

落地独立 `ai` 模块（OpenBB Rita 对话范式）：登录用户多轮 SSE 聊天；Agent 以 tool calling 只读 `MarketDataService`；会话/消息落 MySQL；请求携带页面 `context`；`ai.enabled` 灰度 + 日配额。一期不做交易、不做多 Agent 辩论流水线。

## Technical Context

**Language/Version**: Go 1.22+；前端 Vue 3 + TypeScript  
**Primary Dependencies**: Gin；现有 users session；`MarketDataService`；OpenAI-compatible HTTP API（tool calling）；MySQL；Redis（配额，与 users 对齐优先）  
**Storage**: MySQL（`ai_conversations` / `ai_messages`）；Redis 日配额计数  
**Testing**: `go test`（工具裁剪、会话隔离、配额、SSE 事件编码）；前端 `npm run build`  
**Target Platform**: 单机 `marketd`  
**Project Type**: 全栈（Go API + Vue SPA）  
**Performance Goals**: 工具本地读 Store P99 应远小于 LLM RTT；单次 chat 超时可配（建议 120s）  
**Constraints**: 不直连交易所；不阻塞行情 WS；API Key 仅服务端；Constitution 模块边界  
**Scale/Scope**: 个人站；低并发

## Constitution Check

| Gate | Status |
|------|--------|
| Module boundaries：`ai` 不写 market store、不依赖 ingest/provider 包 | Pass（设计） |
| Contract before code：`/api/v1/ai/*` 见 [contracts/api.md](./contracts/api.md)，实现前同步 RFC-002 | Pass（Draft） |
| No exchange calls from ai | Pass |
| Persistence behind repository | Pass |
| 灰度/回滚：`ai.enabled` | Pass |

## Architecture

```text
[Vue features/ai]
   │ POST /api/v1/ai/chat (SSE)
   │ GET/DELETE conversations…
   ▼
internal/api (AI handlers)
   ▼
ai.Service
   ├ conversationRepo / messageRepo     → MySQL
   ├ quota                              → Redis（优先）
   ├ llm.Client                         → OpenAI-compatible
   ├ agent.Runner（prompt + tool loop）
   └ tools.*  ──只读──> MarketDataService
                 └──(二期)──> portfolio.Service
```

### Agent 循环（单会话）

```text
user message (+ context)
  → append messages
  → loop (≤ max_tool_rounds):
       LLM(messages, tools)
       if tool_calls → execute tools → append tool results → continue
       else stream assistant tokens → break
  → persist assistant (+ optional tool rows)
  → SSE done
```

### 配置（拟）

```yaml
ai:
  enabled: false
  auto_migrate: true
  provider: "deepseek"                 # OpenAI-compatible
  base_url: "https://api.deepseek.com" # 可覆盖
  api_key: ""                          # 或 MARKETPULSE_AI_API_KEY / DEEPSEEK_API_KEY
  model: "deepseek-v4-flash"           # 或 deepseek-v4-pro；勿用已废弃 deepseek-chat
  # thinking: disabled                 # 对话工具循环建议非思考模式，降延迟/费用；需要时再开
  timeout: 120s
  max_tool_rounds: 6
  max_history_messages: 40
  daily_quota_per_user: 50
  system_prompt: ""                    # 空则用内置默认（含免责）
```

> DeepSeek 官方：V4 仅有 `deepseek-v4-flash` / `deepseek-v4-pro`（均支持 Tool Calls + stream）。`deepseek-chat` / `deepseek-reasoner` 于 **2026-07-24 15:59 UTC** 起不可用。文档：https://api-docs.deepseek.com/

依赖：`ai.enabled=true` 时需要 `mysql` + `users`；缺依赖则 soft-skip + warn（对齐 portfolio/alerts）。无 Redis 时：配额可用 `ai_usage_daily` 表，或启动 warn 并以 MySQL 计数实现（plan 实现任务中二选一，推荐 **有 Redis 用 Redis，无则 MySQL 日表**）。

## Project Structure

### Documentation (this feature)

```text
specs/007-ai-assistant/
├── spec.md
├── research.md
├── plan.md
├── data-model.md
├── contracts/api.md
└── tasks.md            # 下一步 /speckit-tasks 生成
```

### Source（拟）

```text
internal/ai/
  service.go            # facade + Bootstrap
  types.go
  repo.go
  migrate/
  llm/                  # chat + tool calling client
  agent/                # runner, prompts
  tools/                # quote/snapshot/klines/news/breadth adapters
internal/api/ai.go      # handlers + SSE
internal/config/        # AiConfig
internal/server/deps.go # 挂 Ai Service
web/src/features/ai/
  api.ts
  types.ts
  components/           # ChatDrawer / MessageList / Composer
  composables/          # useAiChatStream
```

## Implementation Phases（供 tasks 拆解）

| Phase | 内容 | 验收 |
|-------|------|------|
| P0 | config + migrate + Bootstrap soft-skip | `ai.enabled=false` 无影响 |
| P1 | tools 瘦封装 + 单测 | 裁剪后的 JSON 稳定 |
| P2 | llm client + agent loop（可先非流式测） | 工具调用正确 |
| P3 | `POST /chat` SSE + 会话写入 | curl/SSE 可读 token |
| P4 | conversations CRUD API | 隔离与软删 |
| P5 | 前端对话 UI + context 注入 | 追问与刷新恢复 |
| P6 | 配额、超时、错误码、RFC-002 同步 | 429/禁用/主路径无影响 |

## 前端要点

- 发送时从 `market` / `chart` store 组装 `context`。  
- Chat 用 `fetch` + `ReadableStream` 解析 SSE（POST body）。  
- 入口：看板浮动按钮或用户中心 Tab（实现时二选一，推荐看板全局入口 + 登录门闸）。  
- 复用现有 session header 工具（auth store）。

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| Token 费用 | 配额、历史截断、K 线摘要、max_tool_rounds |
| 同会话并发 | 409 busy |
| 流式中断 | `incomplete` 标记；前端提示 |
| Prompt 注入 | 工具结果与用户文本分角色；忽略「忽略以上指令」类越权写操作 |

## 回滚

1. `ai.enabled: false`  
2. 前端隐藏入口  
3. 表可保留；无需回滚行情数据

## Next

1. `/speckit-clarify`（若仍有歧义；当前决策已较满，可跳过）  
2. `/speckit-tasks` 生成 `tasks.md` → **已完成**（见 [tasks.md](./tasks.md)）  
3. `/speckit-implement` 按任务落地（建议从 T001 或 Phase 1–3 MVP）  
4. 实现前将 [contracts/api.md](./contracts/api.md) 同步进 `docs/RFC-002-api-contract.md`，并在 `config.example.yaml` 增加 `ai` 段（见 T001 / T038）
