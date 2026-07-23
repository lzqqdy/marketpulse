# Tasks: 007-ai-assistant

**Input**: `specs/007-ai-assistant/`  
**Status**: ✅ 一期 Phase 1–9 已完成（2026-07-23）  
**Default model**: `deepseek-v4-flash`（config 可改 `deepseek-v4-pro`）

## Phase 1–3

- [x] T001–T020 Setup / Foundational / US1 MVP

## Phase 4–5

- [x] T021–T026 工具 grounding + 页面 context（含 T023 单测补充）

## Phase 6–7

- [x] T027–T029 续聊 + `GET .../messages` + localStorage / 刷新恢复
- [x] T030–T033 会话列表 / 软删 / PATCH 标题 / 前端侧栏

## Phase 8

- [x] T034 日配额（Redis 优先，`ai_usage_daily` 回退）
- [x] T035 chat `timeout` + 工具 15s timeout
- [x] T036 错误码（含 SSE 前 JSON 返回 429/409）
- [x] T037 无写操作工具 + 免责尾注兜底

## Phase 9

- [x] T038 RFC-002 §11.2
- [x] T039 MODULES / README / RFC-004
- [x] T040 config.example + config.docker 已含 `ai` 段
- [x] T041 免责尾注
- [x] T042 `go test ./internal/ai/...` + `npm run build` 通过

## 验收

1. `ai.enabled: true` + Key + mysql/users → 登录 → AI 抽屉提问 → SSE 流式回答  
2. 同会话追问；刷新后历史恢复（localStorage + GET messages）  
3. 会话侧栏切换 / 删除 / 新建  
4. 超配额 429；`ai.enabled: false` 行情不受影响  
5. `model: deepseek-v4-pro` 仅改配置即可  

## 二期占位

- [ ] T100 portfolio 工具 + K 线引用卡片  
- [ ] T101 自动标题 / 技术指标  
- [ ] T102 深度分析流水线  
