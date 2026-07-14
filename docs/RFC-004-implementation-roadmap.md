# RFC-004：Vibe Coding 分步实现路线图

| 字段 | 内容 |
|------|------|
| 状态 | Active |
| 依赖 | RFC-001、RFC-002、RFC-003 |
| 日期 | 2026-05-16 |
| 最后对齐 | 2026-07-11 |

> **用法**：每次只做一个 Step，完成「验收标准」后再进入下一步。  
> 对 AI 说：「做 Step X」或「继续 Step X+1」即可。

---

## 总览

| 阶段 | Steps | 目标 | 状态 |
|------|-------|------|------|
| **A 地基** | 0–2 | 配置、目录包、开发环境跑通 | ✅ |
| **B 后端行情** | 3–6 | WS 入站 → 内存 → REST/WS 出站 | ✅ |
| **C 前端看板** | 7–14 | 首屏 + 实时 UI | ✅ |
| **D 慢数据** | 15–17 | 汇率、股指、宏观 | ✅ |
| **E 部署** | 18–19 | HK 上线、连通性 | ✅ |
| **F 增强** | 20–25 | K 线、美股参考、行情中心、Provider 健康 | ✅ 大部分 |
| **G 后续** | 26+ | 排行榜、设置页、板块钻取 | 规划中 |

预估：**MVP（A+B+C 核心）≈ Step 0–14**；完整看板 + 慢数据 ≈ Step 0–19。

---

## Phase A — 地基

### Step 0：环境与仓库就绪

**做什么**

- [x] `make setup-config` → `config/config.yaml`
- [x] `make test` / `go build -buildvcs=false -o bin/marketd ./cmd/marketd`
- [x] `cd web && npm ci && npm run dev`

**验收**

- `curl localhost:8080/healthz` 返回 ok（先 `make dev-api`）
- `npm run dev` 能打开占位页

**Vibe 提示词示例**

> 帮我完成 Step 0：检查 marketpulse 开发环境，修 Makefile 里 dev-api 默认配置路径。

---

### Step 1：后端配置模块 `internal/config` ✅

**做什么**

- [x] 实现 YAML 加载 + 环境变量覆盖（`MARKETPULSE_APP_ADDR` 等）
- [x] 结构体：`App`、`Symbols`、`Ingest` 各段
- [x] `cmd/marketd` 启动时加载，失败则退出
- [x] 单元测试：解析 example yaml

**验收**

- 改 `config.yaml` 中 `app.addr`，进程监听端口随之变化
- `/healthz` 返回 JSON 中含 `config.symbols` 数量（debug 字段即可）

**Vibe 提示词**

> 实现 RFC-004 Step 1：internal/config，并在 main 里接入。

---

### Step 2：内存 Store `internal/marketdata/store` ✅

**做什么**

- [x] `Quote`、`Rates`、`Snapshot` 结构体（对齐 RFC-002）
- [x] 线程安全：`RWMutex`
- [x] 方法：`UpdateQuote`、`UpdateRates`、`GetSnapshot`、`Version()`
- [x] 单测：并发读写不 panic

**验收**

- 写一个小测试或 `main` 里临时写入 BTC 价格，`GetSnapshot` 能读出

**Vibe 提示词**

> 实现 Step 2 internal/marketdata/store，字段与 RFC-002 snapshot 一致。

---

## Phase B — 后端行情（核心）

### Step 3：Binance WebSocket Ingest `internal/marketdata/ingest/binance` ✅

**做什么**

- [x] 根据 `config.symbols` 拼 combined stream URL
- [x] 连接 Binance combined `miniTicker`
- [x] 解析并 `store.UpdateQuote`
- [x] 断线重连指数退避（1s → 30s）

**验收**

- 日志每几秒打印 BTC 价格变化
- `GetSnapshot().quotes` 含 5 个币种且 `updatedAt` 在动

**Vibe 提示词**

> 实现 Step 3：Binance WS ingest，不要用 REST 轮询写 Redis。

---

### Step 4：REST 快照 API `internal/api` ✅

**做什么**

- [x] `GET /api/v1/snapshot` → `store.GetSnapshot()` JSON
- [x] `GET /healthz` → binance_ws 状态、last_quote、stream_clients
- [x] Gin 路由

**验收**

```bash
curl -s http://localhost:8080/api/v1/snapshot | jq '.quotes[0]'
```

**Vibe 提示词**

> 实现 Step 4：接入 chi/gin，替换 main 里手写 mux。

---

### Step 5：WebSocket Hub `internal/marketdata/stream` ✅

**做什么**

- [x] `GET /ws/v1/stream?channels=quotes,rates`
- [x] 首包 `snapshot`，后续 `quotes` / `rates`
- [x] 应用层 `ping` → `pong`

**验收**

- `websocat` 或浏览器连 `ws://localhost:8080/ws/v1/stream?channels=quotes`
- 币价变动时能收到 JSON 推送

**Vibe 提示词**

> 实现 Step 5：internal/marketdata/stream，store 更新时向订阅 quotes 的客户端推送。

---

### Step 6：Store 与 Hub 联动 ✅

**做什么**

- [x] `AddListener` + `notifyLocked` on bump
- [x] hub 100ms 防抖合并推送
- [x] 每轮推送全量 watchlist quotes（5 币）

**验收**

- WS 消息频率合理（不会每秒上百条压垮浏览器）
- `version` 单调递增

---

### Step 7：前端 API 层 `web/src/features/market/api` ✅

**做什么**

- [x] `fetchSnapshot()` → `/api/v1/market/snapshot`
- [x] `connectMarketStream()` → `/ws/v1/market/stream`
- [x] 自动重连、解析 RFC-002 消息类型
- [x] 类型在 `features/market/types/market.ts`

**验收**

- [x] 浏览器控制台能打印 snapshot 与 WS quotes 消息

---

### Step 8：Pinia Store `web/src/features/market/stores/market.ts` ✅

**做什么**

- [x] state：`snapshot`（quotes/rates/indices/alpha/macro）、`wsStatus`、`version`
- [x] actions：`initLive()`、`teardown()`、WS 消息处理
- [x] mock 降级：`initMock()` + 2.5s 随机波动

**验收**

- [x] Dashboard 实时跳动，断线自动重连

---

## Phase C — 前端看板（对齐旧站 P0）

### Step 9：布局与主题 `styles` + `App.vue` ✅

**做什么**

- [x] CSS 变量：背景、涨红跌绿（大陆习惯）
- [x] 响应式布局
- [x] `StatusBar`：连接状态、最后更新时间
- [x] `localStorage` 深浅色主题切换

---

### Step 10：币价表 `QuoteTable.vue` ✅

**做什么**

- [x] 列：图标、COIN、USDT/¥、当日%、24h%
- [x] 从 store 渲染；WS 更新
- [x] 点击行打开 K 线抽屉

---

### Step 11：宏观卡片 `MacroGrid.vue` ✅

**做什么**

- [x] 横向滚动卡片：总市值、恐惧贪婪、多空比、资金费率、爆仓等 14 项
- [x] 接真数据（CoinGecko + Binance 衍生品）
- [x] 部分字段 mock 兜底

---

### Step 12：股指条 `IndexGrid.vue` ✅

**做什么**

- [x] 地图气泡 / 方块双视图
- [x] 百度主源 + 腾讯/东财备用
- [x] 点击指数开 K 线（sge-au9999 除外）

---

### Step 13：联调与降级 ✅

**做什么**

- [x] Vite proxy 确认 `/api`、`/ws` 指向 8080
- [x] WS 断开显示「重连中」；5s 无数据降级 mock
- [x] 页面恢复时刷新 snapshot 并重连 WS

---

### Step 14：MVP 里程碑验收 ✅

**检查清单**

- [x] 5 个币种 USDT 价实时更新
- [x] 无 PHP、无 Redis、无 1s 轮询
- [x] 文档 RFC-002 与实现字段一致
- [x] README 补充「如何 dev」

---

## Phase D — 慢数据补齐

### Step 15：USDT/CNY + USD/CNY Ingest ✅

**做什么**

- [x] `internal/marketdata/ingest/otc`：OKX C2C REST，30s
- [x] `internal/marketdata/ingest/forex`：Frankfurter，1h
- [x] 写入 `store.UpdateRates`
- [x] WS 推送 `type: rates`

---

### Step 16：股指 + 黄金 Ingest ✅

**做什么**

- [x] `internal/marketdata/ingest/baidu`：百度主源（WS + REST）
- [x] `internal/marketdata/ingest/equity`：腾讯/东财备用
- [x] `internal/marketdata/ingest/metals`：上金所国内黄金
- [x] WS `type: indices`

---

### Step 17：宏观 Ingest ✅

**做什么**

- [x] `alternative.me` 恐惧贪婪
- [x] CoinGecko `/global` 市值、成交量、BTC 占比
- [x] Binance 衍生品：多空比、资金费率、OI、爆仓
- [x] `MacroGrid` 接真数据

---

## Phase E — 部署

### Step 18：HK 部署 ✅

- [x] `deploy/deploy.local.yaml` 配置
- [x] `make ship` / `make ship-commit`
- [x] 详见 `docs/RFC-003-deployment.md`

### Step 18b：Docker Compose 部署 ✅

- [x] `Dockerfile` 多阶段构建（前端 + `marketd`）
- [x] `docker-compose.yml`（默认行情；`--profile db` 可选 MySQL/Redis）
- [x] `make docker-up` / `make docker-up-db`
- [x] 文档：`docs/RFC-003-deployment.md` §1.1、`deploy/docker.md`

---

### Step 19：运维小闭环 ✅

- [x] `check-connectivity.sh` / `check-binance.sh`
- [x] `/healthz` + `/api/v1/market/providers/status` 监控
- [x] 日志：slog + 文件日志

---

## Phase F — 增强

| Step | 内容 | 状态 |
|------|------|------|
| 20 | K 线抽屉：lightweight-charts + WS/REST | ✅ |
| 21 | 美股参考：Bitget alpha + AlphaStockPanel | ✅ |
| 22 | Provider 健康：providers/status + ProviderStatusWidget | ✅ |
| 23 | 行情中心：market/center API + MarketCenterPanel | ✅ |
| 24 | 百度指数主源切换（specs/001） | ✅ |
| 25 | 排行榜：CoinGecko markets + RankTable | 规划中 |
| 26 | 自选币种：config.yaml + 设置页 | 规划中 |
| 27 | 板块详情钻取 | 规划中 |

---

## 每步通用流程（Vibe Coding）

```text
1. 打开 docs/RFC-004，确认当前 Step 编号
2. 若改 API → 先改 RFC-002
3. 只改一个目录（internal 或 web）
4. 本地验收（上表命令）
5. git commit：feat(api): step 3 binance ws ingest
6. 告诉我「Step N 完成，做 N+1」
```

---

## 建议执行顺序（压缩版）

若时间紧，**最小可用路径**：

```text
0 → 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 10 → 13 → 14
                              ↑ 后端一条线 ────┘   ↑ 前端 ─┘
然后：15 → 16 → 17 → 18
```

---

## 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 0.1 | 2026-05-16 | 初稿 |
| 0.2 | 2026-07-11 | 对齐实现：Step 7–24 完成状态、新增行情中心/美股参考/Provider 健康 |
