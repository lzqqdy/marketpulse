# Tasks: 百度财经指数数据源切换

**Input**: spec.md, plan.md, research.md, data-model.md, contracts/

## Phase 1: 基础与映射 (Foundational)

- [ ] T001 [P] curl 验证 14 个指数的百度 code/market 映射，更新 `research.md` 中不准确项
- [ ] T002 扩展 `internal/marketdata/ingest/equity/index.go`：`BaiduCode`、`BaiduMarket`、`BaiduFinanceType`、`HasBaidu()`
- [ ] T003 [P] 新增 `internal/config/config.go` 中 `BaiduConfig` 结构体与默认值
- [ ] T004 更新 `config/config.example.yaml`：`ingest.baidu` 段 + `providers: [baidu, tencent, eastmoney]`
- [ ] T005 更新 `internal/config/config_test.go` 验证新默认 providers

**Checkpoint**: 配置与映射就绪

---

## Phase 2: User Story 1 — 指数行情百度主源 (P1)

- [ ] T006 [P] [US1] 创建 `internal/marketdata/ingest/baidu/client.go`：HTTP client、Referer、限流、APIResponse 信封
- [ ] T007 [P] [US1] 创建 `internal/marketdata/ingest/baidu/types.go`：QuoteResult、WS 消息类型
- [ ] T008 [US1] 创建 `internal/marketdata/ingest/baidu/quote.go`：`FetchBaiduQuotes(client, defs)` → `map[string]store.IndexQuote`
- [ ] T009 [US1] 创建 `internal/marketdata/ingest/baidu/mapping.go`：IndexDef → 订阅/请求参数
- [ ] T010 [US1] 修改 `internal/marketdata/ingest/slow.go`：`fetchEquityProvider` 增加 `case "baidu"`
- [ ] T011 [US1] 修改 `internal/marketdata/ingest/slow.go`：`equityProviderHealthName` 增加 `baidu_index`
- [ ] T012 [US1] 修改 `internal/marketdata/ingest/provider_health.go`：注册 `baidu_index` primary，`tencent_index` 改 fallback
- [ ] T013 [P] [US1] 创建 `internal/marketdata/ingest/baidu/baidu_test.go` + `testdata/`：quote 解析单测

**Checkpoint**: HTTP 行情百度主源可用，WS 未接入时 HTTP 轮询工作

---

## Phase 3: User Story 1 — WebSocket 降级链 (P1)

- [ ] T014 [US1] 创建 `internal/marketdata/ingest/baidu/websocket.go`：连接、subscribe、patch、重连、消息解析
- [ ] T015 [US1] 修改 `internal/marketdata/ingest/service.go`：启动 `startBaiduIndexWS(ctx)`，推送更新 `equityCache`
- [ ] T016 [US1] WS 失败时设置 `equity_baidu_ws=degraded`，HTTP poll 接管
- [ ] T017 [P] [US1] WS 消息解析单测（testdata 夹具）

**Checkpoint**: WS 优先 → HTTP 降级 → 腾讯/东财兜底全链路通

---

## Phase 4: User Story 2 — 指数 K 线百度主源 (P1)

- [ ] T018 [P] [US2] 创建 `internal/marketdata/ingest/baidu/kline.go`：`FetchBaiduKlines` + marketData CSV 解析
- [ ] T019 [US2] 修改 `internal/marketdata/ingest/equity/kline_cache.go`：`fetchIndexKlines` 优先 baidu
- [ ] T020 [US2] K 线周期映射：1d/1w/1h 支持，15m 不支持则跳过百度
- [ ] T021 [P] [US2] K 线解析与降级链单测

**Checkpoint**: 指数 K 线 source 优先 `baidu`

---

## Phase 5: User Story 3 — 文档与验收 (P2)

- [ ] T022 [P] [US3] 更新 `docs/DATA_SOURCES.md`：百度 primary、降级链、provider 名称
- [ ] T023 [US3] 运行 `go test -buildvcs=false ./...` 全量通过
- [ ] T024 [US3] 按 `quickstart.md` 本地验证 snapshot / klines / provider status

---

## Dependencies

```text
T001-T005 → T006-T013 → T014-T017
T001-T005 → T018-T021
T013 + T021 → T022-T024
```

## Parallel Opportunities

- T006/T007/T009 可并行（不同文件）
- T013/T017/T021 测试可并行
- Phase 3 (WS) 与 Phase 4 (K线) 可在 Phase 2 完成后并行

## MVP Scope

**最小可交付**: Phase 1 + Phase 2（HTTP 行情 + K 线），不含 WS。  
**完整交付**: Phase 1–5。
