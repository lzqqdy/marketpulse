# 行情中心面板 — 实施计划

**状态**: ✅ 已实现（2026-07-11）

## 目标

在全球速览下方新增「行情中心」，对标百度 finance 行情中心，支持 A股/港股/美股 Tab 切换，含涨跌分布、热力图、主力净流入、热门板块四块。

## 技术决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 数据获取 | On-Demand API | 面板按需加载，非全局 snapshot |
| 缓存 | 服务端 TTL cache | 防 403 重试、减上游压力；非 ingest |
| 上游 | 百度 Finance CDN | 直连 403，复用 baidu client |
| API 形态 | 聚合 `GET /api/v1/market/center` + 热力图子接口 | Tab 首屏 1 次；sortKey 切换懒加载 |
| 前端位置 | IndexGrid 下方 | 用户指定 |

## 实现落点

| 组件 | 实际路径 | 状态 |
|------|----------|------|
| 后端 Client | `internal/marketdata/marketcenter/client.go` | ✅ |
| 缓存 | `internal/marketdata/marketcenter/cache.go` | ✅ |
| 后台预热 | `internal/marketdata/marketcenter/refresher.go` | ✅ |
| API Handler | `internal/api/market_center.go` | ✅ |
| 路由 | `internal/api/routes.go` | ✅ |
| 前端面板 | `web/src/features/market/components/MarketCenterPanel.vue` | ✅ |
| 前端 API | `web/src/features/market/api/marketCenter.ts` | ✅ |
| 前端类型 | `web/src/features/market/types/marketCenter.ts` | ✅ |

## 阶段完成情况

### Phase 1 — 后端 ✅

- [x] Baidu 四个 fetch + 市场参数映射（在 `marketcenter/client.go` 中，复用 `baidu/client.go`）
- [x] TTL cache（`marketcenter/cache.go`）
- [x] `marketdata.Service.MarketCenter(market)`
- [x] `api/market_center.go` + 路由
- [x] 单测：`marketcenter/window_test.go`、`client_test.go`

### Phase 2 — 前端 ✅

- [x] `MarketCenterPanel.vue` — Tab + 加载/错误态 + 四模块内联
- [x] `marketCenter.ts` API 客户端
- [x] 接入 `MarketDashboard.vue`
- [x] 60s 自动刷新

### Phase 3 — 打磨（部分）

- [ ] Provider 健康 `baidu_market_center`（未注册独立 provider）
- [x] 热力图 sortKey 交互
- [x] 移动端布局

## 不在本期

- 板块详情页 / 成分股钻取
- ingest 写入 snapshot / WS 推送
- 新加坡/期货等其他百度 Tab

## 依赖

- `specs/002-market-center-panel/research.md`
- 现有 `baidu/client.go` CDN 回退
