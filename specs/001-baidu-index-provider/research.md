# Research: 百度财经指数数据源

**Feature**: 001-baidu-index-provider  
**Date**: 2026-07-10  
**Status**: ✅ 已实现（2026-07-11）

## 实现后摘要

| 能力 | Primary | Fallback | 代码入口 |
| --- | --- | --- | --- |
| 指数行情 | **Baidu**（WS + REST） | Tencent → Eastmoney | `slow.go` → `pollEquity` + `baidu/websocket.go` |
| 指数 K 线 | **Baidu** | Eastmoney → Tencent | `kline_cache.go` → `fetchIndexKlines` |
| Provider 链 | `baidu,tencent,eastmoney` | — | `config.go` |

## 调研时现有实现（已替换）

| 能力 | 当时 Primary | 当时 Fallback |
| --- | --- | --- |
| 指数行情 | Tencent | Eastmoney |
| 指数 K 线 | Eastmoney | Tencent（日 K 竞速） |

Provider 降级模式（已有）：按 `providers` 列表顺序，只拉 `equityCache` 中过期的指数；熔断器 `equityBreaker` 按 provider 隔离。

## 百度 API 选型

### 指数实时行情

| 优先级 | 接口 | 说明 |
| --- | --- | --- |
| 1 | WS `wss://finance-ws.pae.baidu.com` | `product=snapshot`，订阅 `financeType=index`；调研矩阵对指数 WS 标注不确定，需实测 |
| 2 | HTTP `GET /vapi/v1/getquotation` | 单标的快照，含 `cur.price/ratio/increase` |
| 3 | HTTP `GET /api/indexbanner` | 仅覆盖部分 A 股指数，不适合作为全球主源 |

**决策**: WS 优先；若指数订阅无推送或连接失败，切换 HTTP `getquotation` 批量/并发拉取。

### 指数 K 线

| 优先级 | 接口 | 说明 |
| --- | --- | --- |
| 1 | HTTP `GET /selfselect/getstockquotation` | `group=quotation_kline_{market}` + `isIndex=1` + `eprop/ktype` |
| — | `/sapi/v1/get_candlestick_event` | **非 K 线**，技术形态事件，不使用 |

**决策**: K 线仅 HTTP（百度无 K 线 WS 文档）；接入 `getstockquotation`，解析 `marketData` CSV。

### 市场状态门控

- `GET /sapi/v1/marketquote?bizType=marketStatus` → `websocketEnabled` 字段决定是否尝试 WS
- 轮询间隔参考调研：交易时段 5s，非交易 30s（与现有 `equity.ActiveTTL/InactiveTTL` 对齐）

## 指数 Symbol 映射（初稿，实现时 curl 验证）

| MarketPulse ID | 百度 code | market | financeType | 备注 |
| --- | --- | --- | --- | --- |
| sh000001 | 000001 | ab | index | 上证 |
| sz399001 | 399001 | ab | index | 深证 |
| sz399006 | 399006 | ab | index | 创业板 |
| sh000300 | 000300 | ab | index | 沪深300 |
| sh000688 | 000688 | ab | index | 科创50 |
| hsi | HSI | hk | index | 恒生 |
| dji | DJI | us | index | 道指 |
| ixic | IXIC | us | index | 纳斯达克 |
| gspc | SPX | us | index | 标普500 |
| n225 | N225 | global | index | 日经 |
| ks11 | KS11 | global | index | KOSPI |
| gold | GC | global | futures | 国际黄金，待验证 |
| silver | SI | global | futures | 国际白银，待验证 |
| crude | CL | global | futures | WTI，待验证 |

无百度映射的指数：provider 链自动跳过 `baidu`，走腾讯/东财。

## K 线周期映射

| MarketPulse interval | 百度 eprop | ktype |
| --- | --- | --- |
| 15m | — | 不支持则跳过百度 |
| 1h | 60minK | — |
| 1d | dayK | 1 |
| 1w | weekK | 2 |

百度不支持的周期：直接走 Eastmoney 备用链（与现有一致）。

## 风险

1. **无官方 API**：接口可能变更 → 解析层隔离 + 测试夹具
2. **指数 WS 不确定**：实现时先探测，不行则 HTTP-only，不阻塞交付
3. **商品期货映射**：可能需 `financeType=futures` + 不同 code → 实现阶段 curl 验证，失败则商品指数不走百度
4. **合规**：内部工具，限速 + Referer，不作为唯一数据源（已有双备用）

## 决策记录

| 决策 | 选择 | 理由 |
| --- | --- | --- |
| 包结构 | `internal/marketdata/ingest/baidu/` | 与调研建议一致，provider 逻辑隔离 |
| WS 生命周期 | 独立 goroutine in ingest service | 类似 Binance WS 模式 |
| 配置 | 扩展 `ingest.equity.providers` + 新增 `ingest.baidu` | 最小改动，复用现有 provider 链 |
| K 线缓存 | 扩展现有 `klineCache`，source 字段区分 baidu | 避免重复缓存层 |
