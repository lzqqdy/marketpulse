# 行情中心面板 — 实施计划（草案）

## 目标

在全球速览下方新增「行情中心」，对标百度 finance 行情中心，支持 A股/港股/美股 Tab 切换，含涨跌分布、热力图、主力净流入、热门板块四块。

## 技术决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 数据获取 | On-Demand API | 面板按需加载，非全局 snapshot |
| 缓存 | 服务端 30s TTL | 防 403 重试、减上游压力；非 ingest |
| 上游 | 百度 Finance CDN | 直连 403，复用 baidu client |
| API 形态 | 聚合 `GET /api/v1/market/center` + 热力图子接口 | Tab 首屏 1 次；sortKey 切换懒加载 |
| 前端位置 | IndexGrid 下方 | 用户指定 |

## 阶段

### Phase 1 — 后端（优先）

1. `baidu/market_center.go` — 四个 fetch + 市场参数映射
2. `market_center_cache.go` — TTL cache
3. `marketdata.Service.MarketCenter(market)`
4. `api/market_center.go` + 路由
   - `GET /api/v1/market/center?market=`
   - `GET /api/v1/market/center/heatmap?market=&sortKey=amount|volume|marketValue`
5. 单测：mock JSON 解析 + ab/hk/us 映射

### Phase 2 — 前端

1. `MarketCenterPanel.vue` — Tab + 加载/错误态
2. 四个子组件 + `marketCenter.ts` API
3. 接入 `MarketDashboard.vue`
4. 60s 自动刷新（交易时段）

### Phase 3 — 打磨

1. Provider 健康 `baidu_market_center`
2. 热力图 sortKey 交互
3. 移动端布局

## 不在本期

- 板块详情页 / 成分股钻取（待定；参考百度 `/block/{market}-{code}`，Phase 2+）
- ingest 写入 snapshot / WS 推送
- 新加坡/期货等其他百度 Tab

## 依赖

- `specs/002-market-center-panel/research.md`
- 现有 `baidu/client.go` CDN 回退
