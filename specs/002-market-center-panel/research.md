# 行情中心面板 — 技术调研

**日期**: 2026-07-10  
**参考页面**: https://finance.baidu.com/?quotationMarket=us  
**范围**: A 股 / 港股 / 美股，涨跌分布 + 热力图 + 主力净流入 + 热门板块

---

## 1. 目标 UI（对标百度）

```
行情中心
[A股] [港股] [美股]          ← Tab 切换 market=ab|hk|us

┌─ 涨跌分布 ─────────────────────────────┐
│ 成交额 3.39万亿                        │
│ 柱状图：涨停…跌停 9 档                 │
│ 上涨 3512 / 平盘 85 / 下跌 1612        │
└────────────────────────────────────────┘

┌─ 热力图 ─────────── [成交额▼] ────────┐
│ Treemap：行业板块，面积=排序字段        │
└────────────────────────────────────────┘

┌─ 主力净流入 ───────── [行业▼] ────────┐
│ 柱状图：净流入红 / 净流出绿             │
└────────────────────────────────────────┘

┌─ 热门板块 ────────────────────────────┐
│ [行业板块][概念板块][地域板块]          │
│ 横向卡片：板块名 + 涨跌幅 + 领涨股 + 迷你分时 │
└────────────────────────────────────────┘
```

**插入位置（前端）**: `MarketDashboard.vue` 侧栏，`IndexGrid`（全球速览）下方、`AlphaStockPanel` 上方。

---

## 2. 上游接口验证

> 直连 `finance.pae.baidu.com` 多数返回 **403**；与指数行情相同，后端需走 **CDN 代理**（`sp0.baidu.com/.../finance.pae.baidu.com`）并带 `Referer` + `User-Agent`。

### 2.1 涨跌分布

```
GET /sapi/v1/marketquote?bizType=chgdiagram&market={ab|hk|us}&finClientType=pc
```

| 市场 | 可用 | 备注 |
|------|------|------|
| ab | ✅ | 含 `total.title=成交额`、`total.price=3.39万亿` |
| hk | ✅ | 有 ratio + diagram，**无成交额 total** |
| us | ✅ | 同上 |

**Result 结构**:

```json
{
  "chgdiagram": {
    "total": { "title": "成交额", "price": "3.39万亿" },
    "ratio": { "up": 3512, "down": 1612, "balance": 85 },
    "diagram": [
      { "title": "涨停", "status": "up", "count": 92 },
      { "title": "涨停-5%", "status": "up", "count": 286 }
    ]
  }
}
```

- A 股 diagram 9 档（含涨停/跌停中文标签）
- 港美股 diagram 档位标签不同（如 `>7%`、`7%-5%`），前端需按 `title` 原样展示

### 2.2 热力图

```
GET /vapi/v2/blocks?style=heatmap&market={market}&typeCode={typeCode}&sortKey={amount|volume|marketValue}&sortType=desc&pn=0&rn=20&finClientType=pc
```

| 市场 | typeCode | 实测 |
|------|----------|------|
| ab | `HY`（行业） | ✅ 20 条，含 amount/pxChangeRate/logo |
| hk | `HSHY` | ✅（`HY` 返回空） |
| us | `HY` | ✅ |

**单条字段**: `code`, `name`, `market`, `amount`, `volume`, `marketValue`, `lastPx`, `pxChangeRate`, `pxChange`, `logo`, `rawData`

**交互**: 下拉切换 `sortKey` → 重新请求（与百度一致）

### 2.3 主力净流入

```
GET /sapi/v1/marketquote?bizType=fundflow&rn=12&market={market}&finClientType=pc
```

| 市场 | 可用 | blockType |
|------|------|-----------|
| ab | ✅ | 3 组：`HY`行业 / `GN`概念 / `DY`地域 |
| hk | ✅ | 仅 1 组：行业（12 条） |
| us | ✅ | 仅 1 组：行业（12 条） |

**结构**:

```json
{
  "fundflow": [
    {
      "blockType": "HY",
      "blockTypeName": "行业",
      "data": [
        {
          "code": "650000",
          "name": "国防军工",
          "mainNetTurnover": "+78.85亿",
          "rawData": { "mainNetTurnover": 7884525184 }
        }
      ]
    }
  ]
}
```

- 港美股下拉「行业」即可，无需概念/地域 Tab
- 排序：百度默认已排好；展示 top 净流入 + 净流出

### 2.4 热门板块

```
GET /vapi/v1/blocks/overview?hasTrend=1&market={market}&finClientType=pc
```

| 市场 | blocks 类型 | 备注 |
|------|-------------|------|
| ab | `HY`, `GN`, `DY` | 含 `minuteData` 迷你分时 |
| hk | `HSHY` | 映射为「行业板块」Tab |
| us | `HY` | 映射为「行业板块」Tab |

**单条字段（行业）**:

```json
{
  "code": "650100",
  "name": "航天装备Ⅱ",
  "price": 8932.24,
  "ratio": { "value": "+10.36%", "status": "up" },
  "rise_first": [{ "code": "688523", "name": "航天环宇科技", "ratio": { "value": "+20.01%" } }],
  "minuteData": "..."
}
```

- `hasTrend=1` 必须带，否则无 sparkline 数据
- 领涨股在 `rise_first[0]`，不是 `leadStock` 字段

---

## 3. 后端架构建议

### 3.1 用户倾向：接口转发 — 是否可行？

**可行，但不建议「裸透传」或「零缓存」。**

| 方案 | 评价 |
|------|------|
| 浏览器直连百度 | ❌ CORS + 403，不可行 |
| 后端裸 reverse proxy | ⚠️ 可用但暴露上游结构、难做字段归一化、无 403 兜底 |
| **后端 Typed Fetch + 短 TTL 缓存** | ✅ **推荐** |
| 纳入 ingest 轮询写入 snapshot | ❌ 过重；非全局必需、刷新频率与指数不同 |

### 3.2 与现有指数 ingest 的差异

| 维度 | 指数行情（现有） | 行情中心（新） |
|------|------------------|----------------|
| 消费方式 | 全局 snapshot + WS 广播 | 用户打开面板 / 切 Tab 才需要 |
| 刷新 | 30s 轮询 + WS | 交易时段 30~60s 足够 |
| 是否进 store | 是 | **否**（按需 API 即可） |
| CDN 代理 | 需要 | **同样需要** |
| 缓存 | equityCache + TTL | **服务端 15~30s 内存缓存** |

**结论**: 不做 ingest 轮询；做 **On-Demand API + 短 TTL 服务端缓存**。  
这不是「和其他数据一样的缓存转发」，而是更轻的 **read-through cache**：

- 缓存 key: `market + module + params`（如 `ab:heatmap:HY:amount`）
- TTL: 交易时段 **30s**，非交易时段 **5min**（或前端降低刷新频率）
- 403 时自动重试 CDN base（复用 `baidu/client.go` 模式）

### 3.3 推荐 API 设计

**方案 A（推荐）— 聚合接口，减少前端往返**

```
GET /api/v1/market/center?market=ab
```

响应:

```json
{
  "market": "ab",
  "source": "baidu",
  "fetchedAt": 1783668000,
  "chgdiagram": { ... },
  "heatmap": { "sortKey": "amount", "items": [...] },
  "fundflow": { "blockType": "HY", "items": [...] },
  "overview": { "tabs": [{ "type": "HY", "name": "行业板块", "items": [...] }] }
}
```

- Tab 切换只打 **1 次** 后端
- 热力图 `sortKey` 变更：`GET /api/v1/market/center/heatmap?market=ab&sortKey=volume`（或 query 参数）

**方案 B — 四个独立接口**（与百度 1:1，灵活但 Tab 切换 4 请求）

```
GET /api/v1/market/center/chgdiagram?market=ab
GET /api/v1/market/center/heatmap?market=ab&typeCode=HY&sortKey=amount
GET /api/v1/market/center/fundflow?market=ab&blockType=HY
GET /api/v1/market/center/overview?market=ab
```

**建议**: 首版用 **方案 A 聚合** + 热力图 sortKey **懒加载子接口**（见 §3.5）。

### 3.5 热力图 sortKey：聚合还是拆分？

**结论：首屏跟聚合走，切换时单独请求。**

| 时机 | 请求 | 说明 |
|------|------|------|
| Tab 切到 A股/港股/美股 | `GET /api/v1/market/center?market=ab` | 含 chgdiagram + fundflow + overview + **heatmap(amount 默认)** |
| 热力图下拉切 成交量/市值 | `GET /api/v1/market/center/heatmap?market=ab&sortKey=volume` | 仅 20 条，轻量 |
| 同 sortKey 30s 内再切回 | 命中服务端缓存 | key = `market+typeCode+sortKey` |

**为何不三种 sortKey 全塞进聚合？**

- 百度交互是**切换时再请求**，不是一次拉三套
- 三套各 20 条 ≈ 3 倍 payload，首屏没必要
- 用户多数只看默认「成交额」

**聚合响应里 heatmap 形态**:

```json
{
  "heatmap": {
    "sortKey": "amount",
    "typeCode": "HY",
    "items": [ { "code", "name", "amount", "volume", "marketValue", "pxChangeRate", "logo" } ]
  }
}
```

子接口返回相同结构，仅 `sortKey` / `items` 不同。

### 3.6 板块详情钻取（待定 · Phase 2+）

百度板块详情页示例：

`https://finance.baidu.com/block/us-US1305?name=半导体`

后续可能做「板块 / 指数 / 股票」统一详情，本期 **不做钻取**，热力图/热门板块卡片不可点击或仅展示 tooltip。

预留：

- 路由：`/block/:market-:code` 或 drawer
- 上游：成分股 `sapi/v1/constituents`、板块资金流、板块 K 线等（调研已有）

### 3.4 代码落点（已实现）

```
internal/marketdata/marketcenter/
  client.go             # 聚合 Baidu API 数据
  cache.go              # 短 TTL cache
  refresher.go          # 后台预热 ab/hk/us
  types.go              # 响应结构
  market.go             # 市场代码规范化

internal/marketdata/
  service.go            # MarketCenter(market) 门面

internal/api/
  market_center.go      # Handler
  routes.go             # 注册路由

web/src/features/market/
  components/MarketCenterPanel.vue
  api/marketCenter.ts
  types/marketCenter.ts
```

复用 `baidu/client.go` CDN fallback，**不新建 ingest goroutine**。

### 3.5 Provider 健康

- 注册 `baidu_market_center`（auxiliary）
- 聚合接口失败时记 `ReportFailure`；成功记 `ReportSuccess`
- 不阻塞现有 `baidu_index` 健康状态

---

## 4. 前端方案

### 4.1 组件结构（已实现）

```
web/src/features/market/
  components/
    MarketCenterPanel.vue       # 容器 + Tab + 四模块内联
  api/
    marketCenter.ts
  types/
    marketCenter.ts
```

> 初版将四个子模块内联在 `MarketCenterPanel.vue` 中，未拆分为独立子组件。

### 4.2 交互

| 操作 | 行为 |
|------|------|
| 切换 A股/港股/美股 | 重新请求 `GET /center?market=` |
| 热力图下拉 sortKey | 仅刷新 heatmap 子接口或带 query 的聚合 |
| 主力净流入下拉 blockType | ab 切换 HY/GN/DY；hk/us 固定行业 |
| 热门板块 Tab | 读 overview 内对应 type，**无需再请求**（overview 一次返回多 Tab） |
| 自动刷新 | 交易时段每 60s `refresh`（可配置） |

### 4.3 图表库

- 项目若已有 ECharts：热力图用 treemap，涨跌/资金流用 bar
- 若无：CSS flex treemap（MVP）+ 简单 div 柱状图

---

## 5. 市场参数映射表

| UI Tab | `market` | 热力图 typeCode | 热门板块 types | 主力净流入 types |
|--------|----------|-----------------|----------------|------------------|
| A股 | `ab` | `HY` | HY, GN, DY | HY, GN, DY |
| 港股 | `hk` | `HSHY` | HSHY → 展示为「行业」 | 行业（单组） |
| 美股 | `us` | `HY` | HY → 展示为「行业」 | 行业（单组） |

---

## 6. 风险与限制

1. **403 / CDN**: 必须与指数接口同样处理 CDN 回退
2. **港股热力图**: `typeCode` 不是 `HY` 而是 `HSHY`，需映射
3. **港美股涨跌分布**: 无 A 股式「成交额」汇总，UI 需隐藏或显示「—」
4. **收盘后**: 数据仍为当日收盘统计，可展示但不再跳动；无需 WS
5. **百度字段变更**: 做 adapter 归一化，前端不依赖原始字段名
6. **rate limit**: 短 TTL 缓存 + Tab 切换防抖

---

## 7. 工作量粗估

| 任务 | 估时 |
|------|------|
| 后端 baidu adapter + cache + 聚合 API | 1~1.5d |
| 前端 4 模块 + Tab 切换 | 1.5~2d |
| 联调 + 三市场边界 | 0.5d |
| **合计** | **3~4d** |

---

## 8. 结论

1. ✅ 四个百度接口均已验证可用（经 CDN），三市场均支持。
2. ✅ 后端采用 Typed Fetch + 短 TTL 缓存，未纳入 ingest 轮询。
3. ✅ 前端 Tab 切换打 1 个聚合 API，热力图排序单独刷新。
4. ✅ 已插入 `IndexGrid` 下方，见 `MarketDashboard.vue`。

**状态**: 已实现（2026-07-11）。详见 `plan.md`。
