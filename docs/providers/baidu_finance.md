# 百度财经（FinScope）公开 API 调研报告

> 调研时间：2026-07-10  
> 调研范围：PC Web（`finance.baidu.com` / `finance.pae.baidu.com`）  
> 方法：Chrome DevTools Network 等价分析（页面 HTML + JS Bundle 静态扫描 + curl 匿名探测）  
> 约束：仅公开接口，不研究登录/破解/绕过安全机制

## 文档索引

| 文件 | 说明 |
| --- | --- |
| **[openapi.yaml](./openapi.yaml)** | OpenAPI 3.0 机器可读规范（**111 个 path**，含参数、Schema、稳定性评分） |
| **baidu_finance.md**（本文） | 人类可读调研报告、实测结论、MarketPulse 集成建议 |

### 快速使用 OpenAPI

```bash
# 校验规范
npx @redocly/cli lint docs/providers/openapi.yaml

# 本地预览文档
npx @redocly/cli preview-docs docs/providers/openapi.yaml

# 生成 Go Client（示例）
oapi-codegen -package baidu docs/providers/openapi.yaml > internal/marketdata/ingest/baidu/client_gen.go
```

---

## 1. 总览

### 1.1 域名与入口

| 域名 | 角色 |
| --- | --- |
| `https://finance.baidu.com` | PC Web 前端 SPA 入口（FinScope） |
| `https://finance.pae.baidu.com` | **核心 API 网关**（`/sapi/v1/*`、`/vapi/*`、`/selfselect/*`、`/api/*`） |
| `wss://finance-ws.pae.baidu.com` | 实时行情 WebSocket |
| `https://sp0.baidu.com/5LMDcjW6BwF3otqbppnN2DJv/finance.pae.baidu.com/...` | 百度 CDN 代理层（部分接口经此转发，非必须） |

前端 JS Bundle：

- `https://ms.bdstatic.com/se/finance-pc/static/js/index.99fbdd98.js`（主业务，~2.8MB）
- `https://ms.bdstatic.com/se/finance-pc/static/js/vendors.39ef4848.js`（依赖库，~2.3MB）

### 1.2 通用响应格式

绝大多数接口返回统一 JSON 信封：

```json
{
  "QueryID": "4258857025",
  "ResultCode": "0",
  "Result": { ... },
  "ResultMsg": "Success"
}
```

| 字段 | 含义 |
| --- | --- |
| `ResultCode` | `0` = 成功；`-1` = 服务端错误；`10003`/`10006`/`3001` = 参数/权限/业务错误 |
| `Result` | 业务数据 |
| `QueryID` | 请求追踪 ID |

### 1.3 通用请求参数

| 参数 | 说明 | 常见值 |
| --- | --- | --- |
| `finClientType` | 客户端类型 | `pc` |
| `new_Format` / `newFormat` | 新数据格式开关 | `1` |
| `code` | 证券代码 | `600519`、`00700`、`AAPL` |
| `market` / `market_type` | 市场 | `ab`、`hk`、`us`、`sg`、`global` |
| `financeType` | 资产类型 | `stock`、`etf`、`index`、`block`、`futures`、`foreign`、`bond` |

### 1.4 认证与安全

| 项目 | 结论 |
| --- | --- |
| Cookie | **不需要**（匿名可访问核心行情接口） |
| Referer | **建议携带** `https://finance.baidu.com/`，但实测多数接口无 Referer 也可访问 |
| Origin | 不需要 |
| Token / Sign | **不需要** |
| 时间戳 | 不需要 |
| 登录 | 仅自选股管理（`gethomeinfo`、`addstock` 等）需要登录 |

### 1.5 OpenAPI 接口总表

完整参数与 Schema 见 [openapi.yaml](./openapi.yaml)。下表为按 Tag 分组的 `operationId` 索引：

| Tag | 接口数 | 核心 operationId |
| --- | --- | --- |
| **Quote** | 3 | `getQuotation`, `getStockShort` |
| **Chart** | 1 | `getStockQuotation`（K 线/分时/五日，**最重要**） |
| **Market** | 9 | `getMarketQuote`, `getIndexBanner`, `getBanner`, `getTradeInfo` |
| **Ranking** | 8 | `getHotRank`, `getInstHotStock`, `getAnalysisRank` |
| **Sector** | 9 | `getBlocksV3`, `getBlocksOverview`, `getConstituents` |
| **FundFlow** | 3 | `getFundFlow`, `getIndexMainFundflow` |
| **News** | 5 | `getExpressNews`, `getAnnouncement` |
| **Finance** | 14 | `getAnalysis`, `getWidgets`, `getMainOp` |
| **Technical** | 9 | `getCandlestickEvent`, `getIndicatorsGraph`, `getAnalyse` |
| **ETF** | 1 | `getEtfHoldings` |
| **Fund** | 2 | `getFundRank` |
| **Futures** | 4 | `getFuturesOverview`, `getContracts` |
| **Forex** | 5 | `getExchangeByCode`, `getForeignBanner` |
| **Bond** | 2 | `getBondMeta`, `getBondRelated` |
| **Calendar** | 3 | `getFinanceCalendar`, `getMatrix` |
| **Search** | 4 | `getSug`, `getSugRecommendations` |
| **Community** | 5 | `getStockTalkList`, `getStockVoteRecords` |
| **Watchlist** | 9 | `getHomeInfo`（需登录）, `getTrendData` |
| **Meta** | 6 | `getAppInfo`, `getAsync` |
| **OpenData** | 1 | `getOpenData` |
| **WebSocket** | — | 见 `openapi.yaml` → `x-websocket` |

**未纳入 OpenAPI 的路径**（非公开行情或需登录付费）：

- `/agent/api/*` — AI Agent 内部接口
- `/vapi/rightpaddle/*` — 付费会员
- `/finance/stockaccountlist` — 开户

---

## 2. 枚举定义（来自前端 JS）

### 2.1 financeType（资产类型）

| 值 | 说明 |
| --- | --- |
| `stock` | 股票 |
| `etf` | ETF |
| `index` | 指数 |
| `block` | 板块（行业/概念/地域） |
| `bk` | 板块（别名） |
| `futures` | 期货 |
| `foreign` | 外汇 |
| `global` | 全球指数 |
| `bond` | 债券 |

### 2.2 market（市场）

| 值 | 说明 |
| --- | --- |
| `ab` | A 股（沪深） |
| `hk` | 港股 |
| `us` | 美股 |
| `sg` | 新加坡 |
| `foreign` | 外汇 |
| `global` | 全球 |

### 2.3 period / eprop / ktype（K 线周期）

| period (eprop) | ktype | 说明 | 实测数据量（600519 日 K） |
| --- | --- | --- | --- |
| `dayK` | `1` | 日 K | ~2001 根 |
| `weekK` | `2` | 周 K | ~1254 根 |
| `monthK` | `3` | 月 K | ~299 根 |
| `yearK` | `4` | 年 K | 按年聚合 |
| `30minK` | — | 30 分钟 K | 有限 |
| `60minK` | — | 60 分钟 K | 有限 |
| `120minK` | — | 120 分钟 K | 有限 |
| `quarterK` | — | 季 K | JS 中存在 |
| `halfYearK` | — | 半年 K | JS 中存在 |
| `3yearK` | — | 三年 K | JS 中存在 |

分页参数（K 线加载更多）：

| 参数 | 说明 |
| --- | --- |
| `isPaged` | `1` 启用分页 |
| `count` | 每页条数 |
| `start_time` | Unix 时间戳，向前翻页 |

> **重要澄清**：`/sapi/v1/get_candlestick_event` **不是 K 线接口**，而是蜡烛图技术形态事件（MACD/RSI/BOLL 等信号）。实际 K 线数据走 `/selfselect/getstockquotation`。

---

## 3. API 完整清单

> **机器可读版本**：[openapi.yaml](./openapi.yaml)  
> 以下章节保留实测示例与字段说明；新增接口请优先更新 OpenAPI 文件。

### 3.1 行情 Quote（核心）

#### 3.1.1 实时行情 + 盘口 + 分时

| 属性 | 值 |
| --- | --- |
| **名称** | 股票行情快照（getquotation） |
| **URL** | `GET https://finance.pae.baidu.com/vapi/v1/getquotation` |
| **Method** | GET |
| **Query** | `code`, `market_type`, `financeType`, `group`, `pointType=string`, `new_Format=1`, `finClientType=pc`, `query` |
| **group 值** | `quotation_minute_{market}`（如 `quotation_minute_ab`） |
| **Cookie** | 否 |
| **Referer** | 建议 |
| **稳定性** | ⭐⭐⭐⭐⭐ |
| **用途** | 实时价格、盘口（pankouinfos）、买卖五档、分时 priceinfo |

**返回示例（节选）**：

```json
{
  "ResultCode": 0,
  "Result": {
    "cur": {
      "price": "1204.98",
      "ratio": "+1.93%",
      "increase": "+22.79",
      "volume": "5221255",
      "amount": "62.23亿"
    },
    "pankouinfos": {
      "list": [
        {"ename": "open", "name": "今开", "value": "1182.20"},
        {"ename": "high", "name": "最高", "value": "1204.98"},
        {"ename": "volume", "name": "成交量", "value": "5.22万手"},
        {"ename": "turnoverRatio", "name": "换手率", "value": "0.42%"}
      ]
    },
    "askinfos": [...],
    "buyinfos": [...],
    "useWS": "1"
  }
}
```

#### 3.1.2 分时 / K 线（统一入口）

| 属性 | 值 |
| --- | --- |
| **名称** | 图表数据（getstockquotation） |
| **URL** | `GET https://finance.pae.baidu.com/selfselect/getstockquotation` |
| **Method** | GET |
| **Query** | `group`, `market_type`, `code`, `newFormat=1`, `all=1`, 以及类型标记 |
| **Cookie** | 否 |
| **稳定性** | ⭐⭐⭐⭐⭐ |
| **用途** | 分时、五日、K 线（全市场） |

**group 命名规则**：

| 场景 | group 值 |
| --- | --- |
| A 股分时 | `quotation_minute_ab` |
| A 股 K 线 | `quotation_kline_ab` |
| A 股五日 | `quotation_fiveday_ab` |
| 港股分时 | `quotation_minute_hk` |
| 港股 K 线 | `quotation_kline_hk` |
| 美股分时 | `quotation_minute_us` |
| 美股 K 线 | `quotation_kline_us` |
| 指数分时 | `quotation_index_minute` + `isIndex=1` |
| 板块分时 | `quotation_block_minute` + `isBlock=1` |
| 期货分时 | `quotation_futures_minute` + `isFutures=1` |

**K 线额外参数**：`eprop=dayK|weekK|monthK|yearK`，`ktype=1|2|3|4`

**K 线返回字段**：

| key | 含义 |
| --- | --- |
| `timestamp` | Unix 时间戳 |
| `time` | 日期字符串 |
| `open` | 开盘价 |
| `close` | 收盘价 |
| `high` | 最高价 |
| `low` | 最低价 |
| `volume` | 成交量 |
| `amount` | 成交额 |
| `range` | 涨跌额 |
| `ratio` | 涨跌幅 |
| `turnoverratio` | 换手率 |
| `preClose` | 昨收 |
| `ma5avgprice` / `ma5volume` | MA5 |
| `ma10avgprice` / `ma10volume` | MA10 |
| `ma20avgprice` / `ma20volume` | MA20 |

`marketData` 为分号分隔的 CSV 字符串，每行对应一根 K 线。

**分时返回字段（priceinfo）**：

| 字段 | 含义 |
| --- | --- |
| `time` | Unix 时间戳 |
| `timeKey` | 时分（如 `0930`） |
| `price` | 价格 |
| `ratio` | 涨跌幅 |
| `increase` | 涨跌额 |
| `volume` | 成交量 |
| `avgPrice` | 均价 |
| `amount` | 成交额 |
| `totalVolume` | 累计成交量 |
| `totalAmount` | 累计成交额 |

#### 3.1.3 行情简版

| 属性 | 值 |
| --- | --- |
| **名称** | 股票简版行情 |
| **URL** | `GET https://finance.pae.baidu.com/vapi/stockshort` |
| **Query** | `code`, `market` |
| **用途** | 名称 + 价格 + 涨跌幅（轻量） |

#### 3.1.4 市场状态

| 属性 | 值 |
| --- | --- |
| **名称** | 市场交易状态 |
| **URL** | `GET https://finance.pae.baidu.com/sapi/v1/marketquote` |
| **Query** | `bizType=marketStatus` |
| **用途** | 各市场开闭市状态、WebSocket 开关 |

```json
{
  "Result": {
    "stock": {"ab": {"tradeStatus": "ENDTR", "tradeStatusCN": "已收盘", "websocketEnabled": "1"}},
    "futures": {"us": {"tradeStatus": "TRADE", "tradeStatusCN": "交易中"}},
    "foreign": {"global": {"tradeStatus": "TRADE"}}
  }
}
```

#### 3.1.5 首页指数 Banner

| 属性 | 值 |
| --- | --- |
| **名称** | 首页指数行情条 |
| **URL** | `GET https://finance.pae.baidu.com/api/indexbanner` |
| **用途** | 上证/深证/创业板等指数实时价 + 分时点序列 `p` |

| 属性 | 值 |
| --- | --- |
| **名称** | 首页行情 Banner（含分时） |
| **URL** | `GET https://finance.pae.baidu.com/api/getbanner` |
| **用途** | 类似 indexbanner，含 tabs 切换 |

| 属性 | 值 |
| --- | --- |
| **名称** | 外汇 Banner |
| **URL** | `GET https://finance.pae.baidu.com/api/getforeignbanner` |
| **用途** | USDCNY 等外汇对实时行情 |

#### 3.1.6 全球指数概览

| 属性 | 值 |
| --- | --- |
| **名称** | 指数地图 |
| **URL** | `GET https://finance.pae.baidu.com/vapi/index/overview` |
| **用途** | 全球市场指数地图（index_map） |

| 属性 | 值 |
| --- | --- |
| **名称** | 全球指数排行 |
| **URL** | `GET https://finance.pae.baidu.com/vapi/v1/globalindexrank` |
| **Query** | `pn`, `rn`, `area`（all/asia/america/europeafrica/other） |

#### 3.1.7 沪深港通交易信息

| 属性 | 值 |
| --- | --- |
| **名称** | 沪深港通行情 |
| **URL** | `GET https://finance.pae.baidu.com/vapi/v1/gettradeinfo` |
| **用途** | 沪股通/深股通/港股通指数实时行情 |

---

### 3.2 WebSocket 实时推送

| 属性 | 值 |
| --- | --- |
| **URL** | `wss://finance-ws.pae.baidu.com` |
| **心跳** | 6 秒 |
| **重连** | 最多 5 次，间隔 3 秒 |
| **轮询降级** | WS 不可用时 30 秒 HTTP 轮询 |

**订阅消息格式**：

```json
{
  "source": "pc-web",
  "method": "subscribe",
  "product": "snapshot",
  "items": [
    {"code": "600519", "name": "贵州茅台", "market": "ab", "financeType": "stock"}
  ]
}
```

| product | 说明 |
| --- | --- |
| `snapshot` | 快照（价格/盘口） |
| `tick` | 逐笔（A 股/美股，港股 stock/etf 除外） |
| `adr` | ADR 行情 |

**patch 续订**：每 60 秒发送 `method: "patch"` 保持订阅。

**稳定性**：⭐⭐⭐⭐（协议未文档化，字段可能变更）

---

### 3.3 排行榜 / 热度

| 名称 | URL | 关键参数 | 稳定性 | 备注 |
| --- | --- | --- | --- | --- |
| 热搜股票 | `GET /vapi/v1/hotrank` | `tn=wisexmlnew&dsp=iphone&product=stock` | ⭐⭐⭐⭐⭐ | 实测有数据 |
| 综合排行 | `GET /sapi/v1/ranks` | `bizType`, `market` | ⭐⭐ | 需精确 bizType，否则空 |
| 股票排行 | `GET /sapi/v1/stock_ranks` | `market`, `financeType`, `sortKey`, `sortType` | ⭐⭐ | 参数敏感 |
| 分析排行 | `GET /vapi/v1/analysisrank` | `market`, `pn`, `rn` | ⭐⭐⭐⭐ | 技术面评分排行 |
| 舆情热度排行 | `GET /vapi/sentimentrank` | `market` | ⭐⭐⭐⭐ | AI 舆情排名 |
| 机构热门持股 | `GET /sapi/v1/getinsthotstock` | `market` | ⭐⭐⭐⭐ | 机构增减持 |
| 搜索推荐榜 | `GET /selfselect/listsugrecomm` | `product=search\|plate` | ⭐⭐⭐⭐ | 综合热度 |
| 外汇排行 | `GET /api/getforeignrank` | — | ⭐⭐⭐⭐ | 外汇涨跌幅榜 |
| 基金排行 | `GET /vapi/v1/getfundrank` | — | ⭐⭐⭐ | 基金分类榜单 |
| 期货列表 | `GET /vapi/v1/getfutureslist` | `pn`, `rn` | ⭐⭐ | 实测 null |
| 市场情绪 | `GET /vapi/sentimentlist` | `code`, `market` | ⭐⭐ | 个股舆情，参数敏感 |

**ranks bizType 已知值**：`marketStatus`、`chgdiagram`、`fund_rank`、`mainFundflow`

---

### 3.4 行业 / 概念 / 地域板块

| 名称 | URL | 关键参数 | 稳定性 |
| --- | --- | --- | --- |
| 板块概览 | `GET /vapi/v1/blocks/overview` | `market=ab` | ⭐⭐⭐⭐⭐ |
| 板块列表 v2 | `GET /vapi/v2/blocks` | `market`, `blockType=HY\|GN\|DY` | ⭐⭐⭐⭐⭐ |
| 板块排行 v3 | `GET /vapi/v3/blocks` | `market`, `type_code=HY\|GN\|DY`, `sort_key`, `sort_type`, `pn`, `rn` | ⭐⭐⭐⭐⭐ |
| 指数成分股 | `GET /sapi/v1/constituents` | `code`, `market`, `financeType=index`, `pn`, `rn` | ⭐⭐⭐⭐⭐ |
| 指数关联 | `GET /sapi/v1/index/related` | `code`, `market` | ⭐⭐ |
| 指数波动 | `GET /sapi/v1/index/fluctuation` | `code`, `market` | ⭐⭐ |
| 指数市值 | `GET /sapi/v1/index/marketvalue` | `code`, `market` | ⭐⭐ |
| 指数主力资金 | `GET /sapi/v1/index/main-fundflow` | `code`, `market` | ⭐⭐⭐⭐ |
| 关联指数 | `GET /api/relatedindex` | `market`, `code` | ⭐⭐⭐⭐ |
| 关联板块 | `GET /api/getrelatedblock` | `code`, `market` | ⭐⭐ |
| 行业分类树 | `GET /vapi/v1/getindustrylist` | — | ⭐⭐⭐⭐⭐ |

**板块 type_code**：`HY`（行业）、`GN`（概念）、`DY`（地域）

**v3 blocks sort_key 示例**：`pxChangeRate`、`exmainIn`

---

### 3.5 资金流

| 名称 | URL | 关键参数 | 稳定性 |
| --- | --- | --- | --- |
| 个股资金流 | `GET /vapi/v1/fundflow` | `code`, `market`, `financeType`, `type=fundflow` | ⭐⭐ |
| 主力资金方向 | `GET /vapi/v1/main-flow-direction` | `code`, `market` | ⭐⭐ |
| 指数资金流 | `GET /sapi/v1/index/main-fundflow` | `code`, `market` | ⭐⭐⭐⭐ |
| 板块资金流排行 | `GET /vapi/v3/blocks` | `sort_key=exmainIn` | ⭐⭐⭐⭐⭐ |

---

### 3.6 新闻 / 公告 / 快讯

| 名称 | URL | 关键参数 | 稳定性 |
| --- | --- | --- | --- |
| 7×24 快讯 | `GET /selfselect/expressnews` | `pn`, `rn` | ⭐⭐⭐⭐⭐ |
| 个股新闻 | `GET /selfselect/news` | `code`, `market`, `pn`, `rn` | ⭐⭐⭐ |
| 公司公告 | `GET /sapi/v1/announcement` | `code`, `market`, `pn`, `rn` | ⭐⭐⭐⭐ |
| 个股提示 | `GET /sapi/v1/stock_notice` | `code`, `market` | ⭐⭐ |
| 新闻详情 | `GET /sapi/v1/news_detail` | `nid` | ⭐⭐ |
| 期货新闻 | `GET /vapi/getfuturesnews` | `pn`, `rn` | ⭐⭐ |
| 舆情组件 | `GET /vapi/v1/overviewwidget` | `code`, `market`, `widgetType=sentiment` | ⭐⭐⭐⭐ |

---

### 3.7 财务 / 分析 / 研报

| 名称 | URL | 关键参数 | 稳定性 |
| --- | --- | --- | --- |
| 综合分析 | `GET /vapi/v1/analysis` | `code`, `market`, `financeType` | ⭐⭐⭐⭐ |
| 综合 Widget | `GET /vapi/v1/widgets` | `code`, `market`, `financeType` | ⭐⭐⭐⭐⭐ |
| 概览 Widget | `GET /vapi/v1/overviewwidget` | `code`, `market`, `widgetType` | ⭐⭐⭐⭐ |
| 主营构成 | `GET /vapi/v1/mainop` | `code`, `market`, `type=industry\|product\|region` | ⭐⭐⭐⭐ |
| 相关标的 | `GET /vapi/v1/stockrelatedobjects` | `code`, `market` | ⭐⭐⭐⭐ |
| 全球财务 Widget | `GET /sapi/v1/global/fundamental/widget` | `code`, `market=us` | ⭐⭐⭐⭐ |
| 全球财务行业对比 | `GET /sapi/v1/global/fundamental/industry` | `code`, `market` | ⭐⭐⭐ |
| 全球财务报表 | `GET /sapi/v1/global/fundamental/statement` | `code`, `market`, `statementType=balance\|income\|cashflow` | ⭐⭐⭐ |
| 研报基础 | `GET /vapi/v1/getreportbasics` | `code`, `market` | ⭐⭐ |
| 财务简报 | `GET /vapi/v1/financial_briefing` | `code`, `market` | ⭐⭐ |
| 电话会议纪要 | `GET /sapi/v1/financelsegtranscript/detail` | `code`, `market` | ⭐⭐ |
| 股票分析 | `GET /sapi/v1/stock_analyse` | `code`, `market` | ⭐⭐ |
| 评级 | `GET /sapi/v1/rating` | `code`, `market`, `financeType` | ⭐⭐ |
| 指标趋势 | `GET /sapi/v1/metrictrend` | `code`, `market`, `financeType` | ⭐⭐ |
| OpenData 聚合 | `GET /opendata` | `resource_id` | ⭐⭐⭐ |

---

### 3.8 技术分析事件

| 名称 | URL | 关键参数 | 稳定性 | 说明 |
| --- | --- | --- | --- | --- |
| 蜡烛图事件 | `GET /sapi/v1/get_candlestick_event` | `code`, `market`, `financeType`, `period` | ⭐⭐⭐⭐ | 技术形态信号，**非 K 线** |
| 事件列表 | `GET /sapi/v1/get_event_list` | 同上 | ⭐⭐⭐⭐ | |
| 同市场事件股 | `GET /sapi/v1/get_event_stocks` | 同上 | ⭐⭐⭐⭐ | |
| 事件筛选器 | `GET /sapi/v1/get_stock_filter` | `market`, `financeType` | ⭐⭐⭐⭐ | |
| 事件图形 | `GET /sapi/v1/get_event_graph` | `code`, `eventType`, `period` | ⭐⭐⭐ | |
| 指标图形 | `GET /sapi/v1/get_indicators_graph` | `code`, `eventType`, `period` | ⭐⭐⭐⭐ | MACD/RSI 等 |
| 指标说明 | `GET /sapi/v1/indicators_explain` | — | ⭐⭐⭐⭐ | |
| K 线事件数 | `GET /sapi/v1/kline_event_info` | `code`, `market`, `period` | ⭐⭐⭐⭐ | |
| 综合分析 | `GET /sapi/v1/get_analyse` | `code`, `market`, `financeType` | ⭐⭐⭐⭐ | 支撑/压力位 |
| 分析行情 | `GET /sapi/v1/get_analysis_quotation` | 同上 | ⭐⭐ | |
| 技术筛选 | `GET /sapi/v1/get_stock_filter` | `market`, `financeType` | ⭐⭐⭐⭐ | |

**eventType 示例**：`1004`=MACD, `1003`=RSI, `1017`=BOLL, `1000-1002`=MA

---

### 3.9 ETF / 基金

| 名称 | URL | 稳定性 |
| --- | --- | --- |
| ETF 持仓 | `GET /sapi/v1/etf_holdings` | ⭐⭐⭐ |
| 基金排行 | `GET /vapi/v1/getfundrank` | ⭐⭐⭐ |
| 基金排序列表 | `GET /vapi/v1/fundsortlist` | ⭐⭐ |
| 基金 OpenData | `GET /opendata` (resource_id) | ⭐⭐⭐ |

---

### 3.10 期货 / 外汇 / 债券

| 名称 | URL | 稳定性 |
| --- | --- | --- |
| 期货概览 | `GET /vapi/v1/futuresoverview` | ⭐⭐⭐ |
| 期货推荐 | `GET /vapi/v1/futures/recommend` | ⭐⭐ |
| 期货列表 | `GET /vapi/v1/getfutureslist` | ⭐⭐ |
| 外汇矩阵 | `GET /sapi/v1/forex/matrix` | ⭐⭐ |
| 外汇排行 | `GET /api/getforeignrank` | ⭐⭐⭐⭐ |
| 外汇兑换 | `GET /vapi/v1/getforeignexchange` | ⭐⭐ |
| 外汇异步 | `GET /vapi/async` (group=huilv_*) | ⭐⭐ |
| 外汇反向 | `GET /api/getrevforeigndata` | ⭐⭐⭐ |
| 货币兑换器 | `GET /vapi/v1/exchangebycode` | ⭐⭐⭐⭐⭐ |
| 债券元数据 | `GET /sapi/v1/bond/meta` | ⭐⭐ |
| 债券关联 | `GET /sapi/v1/bond/related` | ⭐⭐⭐ |
| 国债期货合约 | `GET /sapi/v1/contracts` | ⭐⭐⭐ |
| 纳斯达克数据 | `GET /sapi/v1/nasdaqdata` | ⭐⭐ |

---

### 3.11 日历 / 宏观

| 名称 | URL | 稳定性 |
| --- | --- | --- |
| 财经日历 | `GET /sapi/v1/financecalendar` | ⭐⭐ |
| 市场矩阵 | `GET /sapi/v1/matrix` | ⭐⭐ |

---

### 3.12 搜索 / 推荐

| 名称 | URL | 稳定性 |
| --- | --- | --- |
| 搜索建议 | `GET /vapi/v1/sug` | ⭐⭐⭐⭐⭐ |
| 搜索推荐 | `GET /sapi/v1/sug_recommendations` | ⭐⭐⭐ |
| 搜索历史 | `GET /vapi/v1/search/history` | ⭐⭐⭐（需登录态） |
| 股票推荐 | `GET /vapi/v1/stock/recommend` | ⭐⭐ |

---

### 3.13 社区 / 互动

| 名称 | URL | 稳定性 |
| --- | --- | --- |
| 股评列表 | `GET /vapi/v1/stocktalklist` | ⭐⭐⭐⭐ |
| 股评评论 | `GET /vapi/v1/stocktalkcomment` | ⭐⭐ |
| 看多看空投票 | `GET /vapi/v1/stockvoterecords` | ⭐⭐⭐⭐ |
| 弹幕 | `GET /sapi/v1/bulletscreen` | ⭐⭐ |

---

### 3.14 自选股（需登录）

| 名称 | URL | 说明 |
| --- | --- | --- |
| 自选股首页 | `GET /selfselect/gethomeinfo` | 需登录（ResultCode=10006） |
| 添加自选 | `GET /selfselect/addstock` | 需登录 |
| 删除自选 | `GET /selfselect/delstock` | 需登录 |
| 排序 | `GET /selfselect/sortstock` | 需登录 |
| 是否关注 | `GET /selfselect/stockisfollowed` | 需登录 |
| 自选趋势 | `GET /selfselect/gettrenddata` | 匿名可访问（空列表） |
| 市场排行 | `GET /selfselect/getmarketrank` | 需登录（3001） |

---

### 3.15 其他 / 内部

| 名称 | URL | 说明 |
| --- | --- | --- |
| 应用信息 | `GET /sapi/v1/appinfo` | 站点元数据 |
| PC 页面描述 | `GET /vapi/getpcdesc` | SEO 信息 |
| 股票 Widget | `GET /api/stockwidget` | 参数敏感 |
| 异步聚合 | `GET /vapi/async` | 多场景异步加载 |
| OpenAPI | `GET /selfselect/openapi` | 需登录 |
| Agent 交互 | `GET /sapi/v1/agent/interaction` | AI Agent |
| 用户记录 | `GET /vapi/v1/record` | 需登录 |
| 用户信息 | `GET /vapi/v1/userinfo` | 需登录 |

---

## 4. 功能分类与市场支持矩阵

| 分类 | A 股 | 港股 | 美股 | 基金 | ETF | 指数 | 期货 | 外汇 | 债券 | 板块 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 实时行情 | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ✅ |
| 分时 | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | ✅ | — | ✅ |
| K 线 | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | ✅ | — | — |
| 盘口 | ✅ | ✅ | ✅ | — | ✅ | — | — | — | — | — |
| 排行榜 | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ | ✅ | ⚠️ | ✅ | — | ✅ |
| 板块 | ✅ | ⚠️ | ⚠️ | — | — | — | — | — | — | ✅ |
| 资金流 | ✅ | ⚠️ | ⚠️ | — | — | ✅ | — | — | — | ✅ |
| 新闻快讯 | ✅ | ⚠️ | ⚠️ | — | — | — | ⚠️ | — | — | — |
| 公告 | ✅ | ⚠️ | ⚠️ | — | — | — | — | — | — | — |
| 财务分析 | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | — | — | — | — | — |
| 技术事件 | ✅ | ⚠️ | ⚠️ | — | — | — | — | — | — | — |
| 成分股 | — | — | — | — | — | ✅ | — | — | — | — |
| WebSocket | ✅ | ✅ | ✅ | — | ✅ | — | — | — | — | — |

✅ = 实测可用　⚠️ = 部分可用或参数敏感　— = 不支持

---

## 5. 稳定性评分与推荐

### 5.1 核心接口评分（1~5）

| 接口 | 评分 | 匿名 | 限流 | 版本 | 推荐场景 |
| --- | --- | --- | --- | --- | --- |
| `selfselect/getstockquotation` | 5 | ✅ | 低 | 稳定 | **Primary** — K 线/分时 |
| `vapi/v1/getquotation` | 5 | ✅ | 低 | 稳定 | **Primary** — 实时行情/盘口 |
| `sapi/v1/marketquote` | 5 | ✅ | 低 | 稳定 | **Primary** — 市场状态 |
| `api/indexbanner` / `getbanner` | 5 | ✅ | 低 | 稳定 | **Primary** — 指数快照 |
| `vapi/v1/sug` | 5 | ✅ | 低 | 稳定 | **Secondary** — 代码搜索 |
| `vapi/v3/blocks` | 5 | ✅ | 低 | v3 | **Primary** — 板块排行 |
| `vapi/v1/blocks/overview` | 5 | ✅ | 低 | v1 | **Secondary** — 板块概览 |
| `sapi/v1/constituents` | 5 | ✅ | 低 | 稳定 | **Secondary** — 指数成分 |
| `selfselect/expressnews` | 5 | ✅ | 低 | 稳定 | **Secondary** — 快讯 |
| `vapi/v1/hotrank` | 5 | ✅ | 低 | 稳定 | **Secondary** — 热搜 |
| `vapi/v1/analysis` | 4 | ✅ | 低 | 稳定 | **Secondary** — 评分 |
| `vapi/v1/widgets` | 4 | ✅ | 中 | 稳定 | **Secondary** — 聚合面板 |
| `wss://finance-ws.pae.baidu.com` | 4 | ✅ | 中 | 无版本 | **Primary** — 实时推送 |
| `sapi/v1/get_candlestick_event` | 4 | ✅ | 低 | 稳定 | **Fallback** — 技术事件 |
| `sapi/v1/get_indicators_graph` | 4 | ✅ | 低 | 稳定 | **Fallback** — 技术指标 |
| `vapi/v1/fundflow` | 2 | ✅ | — | 不稳定 | 实验 |
| `sapi/v1/ranks` | 2 | ✅ | — | 不稳定 | 实验 |
| `sapi/v1/financecalendar` | 2 | ✅ | — | 不稳定 | 实验 |
| `sapi/v1/basicinfo` | 2 | ✅ | — | 空返回 | 不推荐 |
| `selfselect/gethomeinfo` | 1 | ❌ | — | — | 需登录 |

### 5.2 Provider 角色建议

| 角色 | 接口 | 说明 |
| --- | --- | --- |
| **Primary Provider** | `getstockquotation` + `getquotation` + WS | A/H/US 股行情、K 线、分时 |
| **Secondary Provider** | `vapi/v3/blocks`, `indexbanner`, `hotrank`, `expressnews`, `analysis` | 板块、指数、热度、快讯 |
| **Fallback Provider** | `get_candlestick_event`, `get_indicators_graph` | 技术分析事件 |
| **不推荐** | `basicinfo`, `ranks`（无 bizType）, `fundflow`（空返回） |  Instable |

---

## 6. Go 接入建议

### 6.1 目录结构

```
internal/marketdata/ingest/baidu/
├── client.go          # HTTP 客户端 + 通用请求
├── types.go           # 公共类型 + 枚举
├── quote.go           # 实时行情 / 盘口
├── kline.go           # K 线 / 分时
├── sector.go          # 板块排行
├── ranking.go         # 热搜 / 排行
├── index.go           # 指数 Banner
├── news.go            # 快讯 / 公告
├── market_status.go   # 市场开闭市
├── search.go          # 代码搜索
├── websocket.go       # WS 实时推送
└── baidu_test.go
```

### 6.2 核心类型

```go
package baidu

const BaseURL = "https://finance.pae.baidu.com"
const WSURL   = "wss://finance-ws.pae.baidu.com"

// Market 市场枚举
type Market string

const (
    MarketAB  Market = "ab"
    MarketHK  Market = "hk"
    MarketUS  Market = "us"
    MarketSG  Market = "sg"
    MarketGlobal Market = "global"
)

// FinanceType 资产类型
type FinanceType string

const (
    FinanceStock   FinanceType = "stock"
    FinanceETF     FinanceType = "etf"
    FinanceIndex   FinanceType = "index"
    FinanceBlock   FinanceType = "block"
    FinanceFutures FinanceType = "futures"
    FinanceForeign FinanceType = "foreign"
    FinanceBond    FinanceType = "bond"
)

// KlinePeriod K 线周期
type KlinePeriod string

const (
    PeriodDay   KlinePeriod = "dayK"
    PeriodWeek  KlinePeriod = "weekK"
    PeriodMonth KlinePeriod = "monthK"
    PeriodYear  KlinePeriod = "yearK"
)

var PeriodToKType = map[KlinePeriod]int{
    PeriodDay: 1, PeriodWeek: 2, PeriodMonth: 3, PeriodYear: 4,
}
```

### 6.3 响应信封

```go
type APIResponse[T any] struct {
    QueryID    string `json:"QueryID"`
    ResultCode string `json:"ResultCode"`
    ResultMsg  string `json:"ResultMsg,omitempty"`
    Result     T      `json:"Result"`
}

func (r *APIResponse[T]) OK() bool {
    return r.ResultCode == "0" || r.ResultCode == ""
}
```

### 6.4 K 线

```go
type KlineMarketData struct {
    Headers    []string `json:"headers"`
    Keys       []string `json:"keys"`
    MarketData string   `json:"marketData"` // "ts,date,open,close,vol,high,low,amount,...;..."
}

type KlineRequest struct {
    Code       string
    Market     Market
    Period     KlinePeriod
    Count      int    // optional, with isPaged=1
    StartTime  int64  // optional, unix seconds
}

type Candle struct {
    Timestamp     int64
    Time          string
    Open          float64
    Close         float64
    High          float64
    Low           float64
    Volume        int64
    Amount        float64
    Change        float64
    ChangePercent float64
    Turnover      float64
    PreClose      float64
}
```

### 6.5 实时行情

```go
type QuoteResult struct {
    Cur struct {
        Price    string `json:"price"`
        Ratio    string `json:"ratio"`
        Increase string `json:"increase"`
        Volume   string `json:"volume"`
        Amount   string `json:"amount"`
        Unit     string `json:"unit"`
    } `json:"cur"`
    Pankouinfos struct {
        List []PankouItem `json:"list"`
    } `json:"pankouinfos"`
    AskInfos []OrderBookLevel `json:"askinfos"`
    BuyInfos []OrderBookLevel `json:"buyinfos"`
    UseWS    string `json:"useWS"`
}

type PankouItem struct {
    EName       string `json:"ename"`
    Name        string `json:"name"`
    Value       string `json:"value"`
    OriginValue string `json:"originValue"`
    Status      string `json:"status,omitempty"`
}
```

### 6.6 错误处理

```go
var (
    ErrAPIError    = errors.New("baidu: api error")
    ErrEmptyResult = errors.New("baidu: empty result")
    ErrRateLimit   = errors.New("baidu: rate limited")
)

// ResultCode 业务错误
// 10003 = 参数错误, 10006 = 未登录, 3001 = 权限不足, -1 = 服务端错误
```

### 6.7 重试与缓存建议

| 接口类型 | 重试 | 缓存 TTL | 轮询频率 |
| --- | --- | --- | --- |
| 实时行情（HTTP） | 3 次指数退避 | 3~5 秒 | 3~5 秒（非交易时段 30 秒） |
| K 线历史 | 2 次 | 1 小时（日 K）/ 5 分钟（分钟 K） | 按需 |
| 分时 | 2 次 | 不缓存 | WS 优先，降级 5 秒轮询 |
| 板块排行 | 2 次 | 30 秒 | 30 秒 |
| 指数 Banner | 2 次 | 5 秒 | 5 秒 |
| 快讯 | 2 次 | 60 秒 | 60 秒 |
| 市场状态 | 1 次 | 60 秒 | 60 秒 |
| 搜索建议 | 1 次 | 5 分钟 | 按需 |

### 6.8 HTTP Client 配置

```go
func NewClient() *Client {
    return &Client{
        http: &http.Client{
            Timeout: 10 * time.Second,
        },
        baseURL: BaseURL,
        headers: map[string]string{
            "User-Agent": "Mozilla/5.0 (compatible; MarketPulse/1.0)",
            "Referer":    "https://finance.baidu.com/",
            "Accept":     "application/json",
        },
    }
}
```

### 6.9 Provider 健康检查

建议注册名：`baidu_finance_quote`（Primary）、`baidu_finance_sector`（Secondary）

健康探测：`GET /sapi/v1/marketquote?bizType=marketStatus`，检查 `ResultCode == 0`。

---

## 7. MarketPulse 集成方案

### 7.1 与现有数据源的关系

| MarketPulse 数据域 | 现有 Primary | 百度可补充 |
| --- | --- | --- |
| A 股指数行情 | Tencent / Eastmoney | `indexbanner`（含分时点） |
| A 股 K 线 | Eastmoney | `getstockquotation`（A/H/US 统一） |
| 板块排行 | Eastmoney | `vapi/v3/blocks`（行业/概念/地域 + 资金流） |
| 市场广度 | Eastmoney | 暂无直接等价 |
| 美股参考 | Bitget / Binance Alpha | `getstockquotation`（真实美股行情） |
| 快讯 | — | `expressnews` |
| 热搜 | — | `hotrank` |
| 代码搜索 | — | `vapi/v1/sug` |

### 7.2 推荐集成优先级

**Phase 1（高价值、低风险）**：

1. `selfselect/getstockquotation` — 统一 K 线/分时 Provider，覆盖 A/H/US
2. `vapi/v1/getquotation` — 实时行情 + 盘口
3. `sapi/v1/marketquote` — 市场状态门控（决定是否轮询/WS）
4. `api/indexbanner` — 国内指数快照

**Phase 2（增强）**：

5. `vapi/v3/blocks` — 板块排行（含资金流 sort_key）
6. `wss://finance-ws.pae.baidu.com` — 实时推送替代 HTTP 轮询
7. `selfselect/expressnews` — 7×24 快讯 Feed
8. `vapi/v1/hotrank` — 热搜榜

**Phase 3（可选）**：

9. `vapi/v1/analysis` + `widgets` — 评分/舆情面板
10. `sapi/v1/constituents` — 指数成分股
11. `vapi/v1/sug` — 证券代码搜索

### 7.3 配置示例

```yaml
ingest:
  baidu:
    enabled: true
    base_url: "https://finance.pae.baidu.com"
    ws_url: "wss://finance-ws.pae.baidu.com"
    quote_interval: 5s
    index_banner_interval: 5s
    sector_interval: 30s
    news_interval: 60s
    markets: ["ab", "hk", "us"]
    # 指数 codes 用于 indexbanner
    index_codes: ["000001", "399001", "399006"]
```

### 7.4 数据归一化映射

| 百度字段 | MarketPulse `store.Quote` |
| --- | --- |
| `cur.price` | `Price` |
| `cur.ratio` | `ChangePercent`（去 `%` 解析） |
| `cur.increase` | `Change` |
| `cur.volume` | `Volume` |
| `code` + `market` | `Symbol`（如 `ab:600519`） |

| 百度 `Candle` | MarketPulse `binance.Candle` / 统一 K 线 |
| --- | --- |
| `timestamp` | `OpenTime` |
| `open/close/high/low` | OHLC |
| `volume` | `Volume` |
| `amount` | `QuoteVolume` |

### 7.5 风险与合规

1. **无官方 API 文档**：接口为前端内部使用，可能随时变更
2. **无限流公开文档**：建议自建速率限制（≤ 10 req/s per IP）
3. **数据延迟**：A 股行情通常 L1 实时，美股可能有 15 分钟延迟（需验证 `detailinfos` 中延迟标记）
4. **服务条款**：复用前确认百度服务协议是否允许程序化访问
5. **不做 Primary 的唯一依赖**：建议作为 Eastmoney/Tencent 的补充或 Fallback

---

## 8. 附录

### 8.0 OpenAPI 规范说明

`openapi.yaml` 包含：

- **111 个 HTTP path**（`sapi/v1` 42 + `vapi` 38 + `selfselect` 12 + `api` 9 + `opendata` 1 + 其他 9）
- 复用 `components/parameters`（`code`, `market`, `financeType`, `eprop`, `ktype` 等）
- 复用 `components/schemas`（`APIResponse`, `KlineMarketData`, `QuotationResponse` 等）
- 扩展字段：
  - `x-stability`：稳定性评分 1~5
  - `x-auth-required`：是否需登录
  - `x-websocket`：WebSocket 订阅协议
  - `x-endpoint-index`：路径统计

枚举与 JS 源码映射：

| OpenAPI Schema | JS 常量 |
| --- | --- |
| `Market` | `MarketEnum` |
| `FinanceType` | `FinanceTypeEnum` |
| `KlinePeriod` | `K_TYPE_ENUM` + 分钟周期 |
| `BlockType` | `HY` / `GN` / `DY` |

### 8.1 JS 扫描发现的完整路径列表

<details>
<summary>点击展开全部 100+ 路径（OpenAPI 已覆盖 111 个 HTTP path）</summary>

**sapi/v1/**

- `agent/interaction`
- `announcement`
- `appinfo`
- `assetmgmt`
- `basicinfo`
- `bond/meta`, `bond/related`
- `bulletscreen`
- `constituents`
- `contracts`
- `etf_holdings`
- `financecalendar`
- `financelsegtranscript/detail`
- `forex/matrix`
- `get_analyse`
- `get_analysis_quotation`
- `get_candlestick_event`
- `get_event_graph`
- `get_event_list`
- `get_event_stocks`
- `get_indicators_graph`
- `get_stock_filter`
- `getinsthotstock`
- `global/fundamental/industry`
- `global/fundamental/widget`
- `global/fundamental/statement`
- `index/fluctuation`
- `index/main`
- `index/main-fundflow`
- `index/marketvalue`
- `index/related`
- `indicators_explain`
- `kline_event_info`
- `marketquote`
- `matrix`
- `metrictrend`
- `nasdaqdata`
- `news_detail`
- `quotation`
- `quotes_knowledge`
- `ranks`
- `rating`
- `stock_analyse`
- `stock_notice`
- `stock_ranks`
- `sug_recommendations`

**vapi/**

- `async`
- `getfuturesnews`
- `getpcdesc`
- `index/overview`
- `sentimentlist`
- `sentimentrank`
- `stockshort`
- `v1/analysis`
- `v1/analysisrank`
- `v1/blocks/overview`
- `v1/exchangebycode`
- `v1/financial_briefing`
- `v1/fundflow`
- `v1/fundsortlist`
- `v1/futures/recommend`
- `v1/futuresoverview`
- `v1/getanalysisquotation`
- `v1/getforeignexchange`
- `v1/getfundrank`
- `v1/getfutureslist`
- `v1/getindustrylist`
- `v1/getquotation`
- `v1/getreportbasics`
- `v1/gettradeinfo`
- `v1/globalindexrank`
- `v1/hotrank`
- `v1/main-flow-direction`
- `v1/mainop`
- `v1/overviewwidget`
- `v1/record`
- `v1/search/history`
- `v1/stock/recommend`
- `v1/stockrelatedobjects`
- `v1/stocktalkcomment`
- `v1/stocktalklist`
- `v1/stockvoterecords`
- `v1/sug`
- `v1/userinfo`
- `v1/widgets`
- `v2/blocks`
- `v3/blocks`

**selfselect/**

- `addstock`, `delstock`, `sortstock`, `recstock`
- `expressnews`, `news`
- `gethomeinfo`, `getmarketrank`, `getstockquotation`, `gettrenddata`
- `listsugrecomm`, `openapi`, `selfsortstatus`, `stockisfollowed`

**api/**

- `getbanner`, `getforeignbanner`, `getforeignew`, `getforeignrank`
- `getrelatedblock`, `getrevforeigndata`, `indexbanner`, `relatedindex`, `stockwidget`

**WebSocket**

- `wss://finance-ws.pae.baidu.com`

</details>

### 8.2 get_candlestick_event 参数实测

```
GET /sapi/v1/get_candlestick_event
  ?code=600519
  &market=ab
  &financeType=stock
  &period=dayK
  &finClientType=pc
```

返回技术事件指标头（MA/BOLL/MACD/RSI/CCI/STOCH 等），**不含 OHLCV 数据**。

### 8.3 测试环境

- 测试时间：2026-07-10（A 股收盘后，美股交易中）
- 测试方式：curl 匿名请求，无 Cookie
- 测试标的：600519（A）、00700（HK）、AAPL（US）、000001（指数）、510300（ETF）、650100（板块）

---

*本文档与 [openapi.yaml](./openapi.yaml) 由 MarketPulse 数据源调研生成，仅供内部开发参考。*
