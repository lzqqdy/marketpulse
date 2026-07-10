# Contract: 指数 Provider 降级链

**Scope**: 内部 ingest 契约，不改变 RFC-002 对外 API。

## 行情 Provider 链

```
baidu (WS → HTTP) → tencent → eastmoney
```

### 行为规则

1. `pollEquity` 遍历 `cfg.Ingest.Equity.Providers` 顺序不变，新增 `case "baidu"`
2. 百度 WS 模式下，`pollEquity` 仍运行但主要做健康检查与 HTTP 补洞；WS goroutine 负责推送更新 `equityCache`
3. 仅对 `equityCache.expiredDefs()` 返回的缺失/过期指数请求上游
4. `baidu` 熔断 open 时跳过，记录 `equity_baidu` circuit_open

## K 线 Provider 链

```
baidu HTTP → eastmoney → tencent (仅 1d 竞速)
```

### source 字段值

| 场景 | `KlineResponse.source` |
| --- | --- |
| 百度成功 | `baidu` |
| 百度缓存 | `baidu_cache` / `baidu_cache_stale` |
| 东财成功 | `eastmoney`（保持现有） |
| 腾讯成功 | `tencent`（保持现有） |

## Provider Status API

`GET /api/v1/market/providers/status` 响应新增：

```json
{
  "name": "baidu_index",
  "label": "Baidu Finance",
  "category": "index",
  "role": "primary",
  "status": "healthy",
  "current_used": true
}
```

`tencent_index.role` 变更为 `fallback`，`eastmoney_index.role` 保持 `fallback`。

## 配置回滚

将 `ingest.equity.providers` 改回 `tencent,eastmoney` 并设 `ingest.baidu.enabled: false` 即可完全回退，无需改代码。
