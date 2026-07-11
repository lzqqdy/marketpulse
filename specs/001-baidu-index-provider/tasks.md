# Tasks: 百度财经指数数据源切换

**Input**: spec.md, plan.md, research.md, data-model.md, contracts/

**状态**: ✅ 全部完成（2026-07-11）

## Phase 1: 基础与映射 (Foundational) ✅

- [x] T001 [P] curl 验证 14 个指数的百度 code/market 映射
- [x] T002 扩展 `internal/marketdata/ingest/equity/index.go`：`BaiduCode`、`BaiduMarket`、`BaiduFinanceType`、`HasBaidu()`
- [x] T003 [P] 新增 `internal/config/config.go` 中 `BaiduConfig` 结构体与默认值
- [x] T004 更新 `config/config.example.yaml`：`ingest.baidu` 段 + `providers: [baidu, tencent, eastmoney]`
- [x] T005 更新 `internal/config/config_test.go` 验证新默认 providers

## Phase 2: User Story 1 — 指数行情百度主源 (P1) ✅

- [x] T006 [P] [US1] 创建 `internal/marketdata/ingest/baidu/client.go`
- [x] T007 [P] [US1] 创建 `internal/marketdata/ingest/baidu/types.go`
- [x] T008 [US1] 创建 `internal/marketdata/ingest/baidu/quote.go`（通过 index_ref.go 等实现）
- [x] T009 [US1] 创建 `internal/marketdata/ingest/baidu/mapping.go`
- [x] T010 [US1] 修改 `internal/marketdata/ingest/slow.go`：`fetchEquityProvider` 增加 `case "baidu"`
- [x] T011 [US1] 修改 `slow.go`：`equityProviderHealthName` 增加 `baidu_index`
- [x] T012 [US1] 修改 `provider_health.go`：注册 `baidu_index` primary
- [x] T013 [P] [US1] 创建 `baidu/baidu_test.go` + testdata

## Phase 3: User Story 1 — WebSocket 降级链 (P1) ✅

- [x] T014 [US1] 创建 `internal/marketdata/ingest/baidu/websocket.go`
- [x] T015 [US1] 修改 `service.go`：启动 baidu WS goroutine
- [x] T016 [US1] WS 失败时 HTTP poll 接管
- [x] T017 [P] [US1] WS 消息解析单测

## Phase 4: User Story 2 — 指数 K 线百度主源 (P1) ✅

- [x] T018 [P] [US2] Baidu K 线 fetch（通过 equity kline_cache 优先链）
- [x] T019 [US2] 修改 `kline_cache.go`：`fetchIndexKlines` 优先 baidu
- [x] T020 [US2] K 线周期映射：1d/1w/1h 支持，15m 走 Eastmoney
- [x] T021 [P] [US2] K 线解析与降级链单测

## Phase 5: User Story 3 — 文档与验收 (P2) ✅

- [x] T022 [P] [US3] 更新 `docs/DATA_SOURCES.md`
- [x] T023 [US3] 运行 `go test -buildvcs=false ./...` 全量通过
- [x] T024 [US3] 本地验证 snapshot / index-klines / provider status
