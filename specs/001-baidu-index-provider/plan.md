# Implementation Plan: 百度财经指数数据源切换

**Branch**: `001-baidu-index-provider` | **Date**: 2026-07-10 | **Spec**: [spec.md](./spec.md)

## Summary

在 `internal/marketdata/ingest/baidu/` 新增百度财经 provider，将指数行情与 K 线的主数据源切换为百度；WS 优先、HTTP 轮询降级、腾讯/东财作为最终备用。复用现有 `equityCache`、provider 链、`equityBreaker` 机制，最小化对 API 层的改动。

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: 标准库 `net/http`、`gorilla/websocket`（若项目已有则复用，否则用 `nhooyr.io/websocket` 或 `gorilla/websocket`）  
**Storage**: 内存 `equityCache` + `klineCache`（已有）  
**Testing**: `go test -buildvcs=false ./internal/marketdata/ingest/...`  
**Target Platform**: marketd Linux/macOS  
**Constraints**: 不改 RFC-002 对外契约；provider 逻辑仅在 ingest 层

## Constitution Check

| 原则 | 符合性 |
| --- | --- |
| 模块边界 | ✅ 改动限于 `internal/marketdata/ingest/` |
| 契约优先 | ✅ 对外 API 不变，仅内部 source 字段 |
| 最小改动 | ✅ 复用 provider 链、缓存、熔断 |
| 测试 | ✅ 解析/映射/降级单测 |

## Project Structure

```text
internal/marketdata/ingest/
├── baidu/
│   ├── client.go       # HTTP client + 限流 + 通用信封
│   ├── types.go        # API 响应类型
│   ├── quote.go        # FetchBaiduQuotes (HTTP)
│   ├── kline.go        # FetchBaiduKlines
│   ├── websocket.go    # WS 连接、订阅、推送
│   ├── mapping.go      # IndexDef ↔ 百度参数
│   ├── baidu_test.go
│   └── testdata/       # 录制响应夹具
├── equity/
│   ├── index.go        # 扩展 BaiduCode/BaiduMarket 字段
│   └── kline_cache.go  # 接入 baidu 优先链
├── slow.go             # fetchEquityProvider 加 baidu case
├── service.go          # 启动 baidu WS goroutine
└── provider_health.go  # 注册 baidu_index

internal/config/
└── config.go           # BaiduConfig + 默认 providers

config/config.example.yaml
docs/DATA_SOURCES.md    # 沉淀型文档更新
```

## Phase 0: 符号映射验证

实现前用 curl 验证 `research.md` 中 14 个指数的百度 code/market 映射，修正 `DefaultIndices` 中不准确项。商品类（gold/silver/crude）若百度无数据则 `HasBaidu()=false`。

## Phase 1: HTTP Provider（MVP）

1. `baidu/client.go` + `quote.go` — 批量拉取指数快照
2. `equity/index.go` — 扩展 `IndexDef` 百度字段
3. `slow.go` — `case "baidu"` + `equityProviderHealthName`
4. `config.go` — `BaiduConfig`，默认 providers `baidu,tencent,eastmoney`
5. 单测：响应解析、映射、空结果降级

## Phase 2: K 线切换

1. `baidu/kline.go` — 解析 `getstockquotation` marketData
2. `kline_cache.go` — `fetchIndexKlines` 优先 baidu
3. 单测：周期映射、CSV 解析、降级链

## Phase 3: WebSocket

1. `baidu/websocket.go` — 连接、subscribe、patch、重连
2. `service.go` — `startBaiduIndexWS(ctx)` goroutine
3. WS 推送写入 `equityCache`；失败标记 degraded 并依赖 HTTP poll
4. 单测：消息解析（夹具），连接逻辑 mock

## Phase 4: 文档与验收

1. 更新 `docs/DATA_SOURCES.md`
2. 全量 `go test`
3. 本地 `make dev-api` 验证 snapshot + index klines + provider status

## 回滚方案

```yaml
ingest:
  baidu:
    enabled: false
  equity:
    providers:
      - tencent
      - eastmoney
```

重启 `marketd` 即恢复切换前行为。

## 风险缓解

| 风险 | 缓解 |
| --- | --- |
| 指数 WS 不支持 | HTTP-only 模式，WS 失败不阻塞 |
| 商品映射失败 | `HasBaidu()` 跳过，走腾讯/东财 |
| 百度接口变更 | 解析隔离 + testdata 夹具 |
