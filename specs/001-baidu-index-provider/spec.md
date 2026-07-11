# Feature Specification: 百度财经指数数据源切换

**Feature Branch**: `001-baidu-index-provider`

**Created**: 2026-07-10

**Status**: ✅ 已实现（2026-07-11）

**Input**: 将 MarketPulse 指数行情与指数 K 线的主数据源切换为百度财经（Baidu Finance），腾讯与东方财富降为备用；优先 WebSocket，WS 不可用时降级 HTTP 轮询，百度全不可用时再降级至现有腾讯/东方财富方案。

## 背景

- 百度财经 API 已完成调研，文档位于 `docs/providers/baidu_finance.md` 与 `openapi.yaml`
- ~~当前指数行情：Tencent（primary）→ Eastmoney（fallback），REST 轮询~~
- **已实现**：Baidu（primary，WS + REST）→ Tencent（fallback）→ Eastmoney（fallback）

## User Scenarios & Testing

### User Story 1 - 指数实时行情以百度为主 (Priority: P1) ✅

作为行情看板用户，我希望全球指数（上证、恒生、道指、黄金等）优先从百度财经获取，以便获得更稳定、更低延迟的数据。

**Independent Test**: 启动 `marketd`，观察 `/api/v1/market/snapshot` 中 `indices` 字段 `source=baidu`；`GET /api/v1/market/providers/status` 中 `baidu_index` 为 primary 且 `current_used=true`。

**Acceptance Scenarios**:

1. ✅ 百度 WS 可用时，指数行情通过 WS 推送更新，`baidu_index` 状态 healthy
2. ✅ 百度 WS 连接失败时，自动降级为百度 HTTP 轮询（`getquotation`）
3. ✅ 百度 HTTP 也失败（熔断）时，按 `tencent` → `eastmoney` 顺序拉取
4. ✅ 仅部分指数百度无映射时，跳过百度直接走备用源

---

### User Story 2 - 指数 K 线以百度为主 (Priority: P1) ✅

**Independent Test**: `GET /api/v1/market/index-klines?id=sh000001&interval=1d` 返回 `source` 含 `baidu`。

**Acceptance Scenarios**:

1. ✅ 百度 K 线 API 正常时，`source=baidu`
2. ✅ 百度 K 线失败时，降级 Eastmoney，再失败则 Tencent
3. ✅ 缓存命中时，TTL 内不重复打上游

---

### User Story 3 - Provider 健康与可观测 (Priority: P2) ✅

**Acceptance Scenarios**:

1. ✅ `GET /api/v1/market/providers/status` 包含 `baidu_index`（primary）、`tencent_index`（fallback）、`eastmoney_index`（fallback）
2. ✅ 百度 WS 降级到 HTTP 时，`baidu_index` 仍 healthy

## 实现落点

| 组件 | 路径 |
|------|------|
| Baidu HTTP client | `internal/marketdata/ingest/baidu/client.go` |
| Baidu WS | `internal/marketdata/ingest/baidu/websocket.go` |
| 指数映射 | `internal/marketdata/ingest/baidu/mapping.go` |
| K 线 | `internal/marketdata/ingest/baidu/kline.go`（通过 equity kline_cache 调用） |
| Provider 注册 | `internal/marketdata/ingest/provider_health.go` |
| 配置 | `config/config.example.yaml` → `ingest.baidu` |

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
