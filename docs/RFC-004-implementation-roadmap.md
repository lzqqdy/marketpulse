# RFC-004：Vibe Coding 分步实现路线图

| 字段 | 内容 |
|------|------|
| 状态 | Active |
| 依赖 | RFC-001、RFC-002、RFC-003 |
| 日期 | 2026-05-16 |

> **用法**：每次只做一个 Step，完成「验收标准」后再进入下一步。  
> 对 AI 说：「做 Step X」或「继续 Step X+1」即可。

---

## 总览

| 阶段 | Steps | 目标 |
|------|-------|------|
| **A 地基** | 0–2 | 配置、目录包、开发环境跑通 |
| **B 后端行情** | 3–8 | WS 入站 → 内存 → REST/WS 出站 |
| **C 前端看板** | 9–14 | 首屏 + 实时 UI，对齐旧站核心效果 |
| **D 慢数据** | 15–17 | 汇率、股指、宏观 |
| **E 部署** | 18–19 | HK 上线、连通性 |
| **F 增强** | 20+ | 排行榜、K 线、设置（可选） |

预估：**MVP（A+B+C 核心）≈ Step 0–14**；完整看板 + 慢数据 ≈ Step 0–19。

---

## Phase A — 地基

### Step 0：环境与仓库就绪

**做什么**

- [ ] `cd marketpulse && git init`（若未初始化）
- [x] `make setup-config` → `config/config.yaml`
- [ ] 运行 `make check`（HK 或本机）
- [x] `make test` / `go build -buildvcs=false -o bin/marketd ./cmd/marketd`
- [ ] `cd web && npm ci`（需网络，前端 Phase C 再做）

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

### Step 2：内存 Store `internal/store` ✅

**做什么**

- [x] `Quote`、`Rates`、`Snapshot` 结构体（对齐 RFC-002）
- [x] 线程安全：`RWMutex`
- [x] 方法：`UpdateQuote`、`UpdateRates`、`GetSnapshot`、`Version()`
- [x] 单测：并发读写不 panic

**验收**

- 写一个小测试或 `main` 里临时写入 BTC 价格，`GetSnapshot` 能读出

**Vibe 提示词**

> 实现 Step 2 internal/store，字段与 RFC-002 snapshot 一致。

---

## Phase B — 后端行情（核心）

### Step 3：Binance WebSocket Ingest `internal/ingest/binance` ✅

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

### Step 5：WebSocket Hub `internal/hub` ✅

**做什么**

- [x] `GET /ws/v1/stream?channels=quotes,rates`
- [x] 首包 `snapshot`，后续 `quotes` / `rates`
- [x] 应用层 `ping` → `pong`

**验收**

- `websocat` 或浏览器连 `ws://localhost:8080/ws/v1/stream?channels=quotes`
- 币价变动时能收到 JSON 推送

**Vibe 提示词**

> 实现 Step 5：internal/hub，store 更新时向订阅 quotes 的客户端推送。

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

### Step 7：前端 API 层 `web/src/api`

**做什么**

- [ ] `fetchSnapshot(): Promise<MarketSnapshot>`
- [ ] `createMarketSocket(channels): WebSocket` 封装
- [ ] 自动重连、解析 RFC-002 消息类型
- [ ] 类型使用 `src/types/market.ts`（已有则核对字段）

**验收**

- 浏览器控制台能打印 snapshot 与 WS quotes 消息

**Vibe 提示词**

> 实现 Step 7：web/src/api/http.ts 和 ws.ts，对齐 RFC-002。

---

### Step 8：Pinia Store `web/src/stores/market.ts`

**做什么**

- [ ] state：`quotes`、`rates`、`indices`、`macro`、`wsStatus`、`version`
- [ ] actions：`applySnapshot`、`applyWsMessage`
- [ ] `useMarketStream` composable：onMounted 拉 snapshot + 连 WS

**验收**

- `App.vue` 里能 `console.log(store.quotes)` 且数字随 WS 变

**Vibe 提示词**

> 实现 Step 8：Pinia store + useMarketStream composable。

---

## Phase C — 前端看板（对齐旧站 P0）

### Step 9：布局与主题 `styles` + `App.vue`

**做什么**

- [ ] CSS 变量：背景、涨 `#f6465d`、跌 `#0ecb81`（大陆习惯）
- [ ] 移动端：`max-width: 420px` 居中
- [ ] `StatusBar`：连接状态、最后更新时间
- [ ] `localStorage` 切换红涨绿跌 / 绿涨红跌（可选）

**验收**

- 深色页面，风格接近旧 `index.html`

---

### Step 10：币价表 `QuoteTable.vue`

**做什么**

- [ ] 列：图标占位、COIN、USDT/¥、当日%、24h%
- [ ] 从 store 渲染；WS 更新只改对应行
- [ ] 价格变动闪动动画（200ms）
- [ ] 数字格式化（千分位、小数位）

**验收**

- 与旧站表格信息密度类似，无需 1s 轮询接口

**Vibe 提示词**

> 实现 Step 10 QuoteTable，数据来自 Pinia，不要 axios 轮询。

---

### Step 11：宏观卡片 `MacroGrid.vue`

**做什么**

- [ ] 4×2 栅格：总市值、24h 成交额、恐惧贪婪、BTC 占比等
- [ ] MVP：宏观数据可先 mock 或等 Step 16 再接真数据
- [ ] 涨跌颜色 class

**验收**

- 布局与旧站 `layui-col-xs3` 卡片一致

---

### Step 12：股指条 `IndexGrid.vue`

**做什么**

- [ ] 三列卡片：名称、现价、涨跌幅
- [ ] MVP：可 mock；Step 16 接 REST
- [ ] 响应式换行

**验收**

- 与旧 `#stock_price` 区域等价

---

### Step 13：联调与降级

**做什么**

- [ ] Vite proxy 确认 `/api`、`/ws` 指向 8080
- [ ] WS 断开显示「重连中」；3 次失败后 `setInterval` 拉 snapshot（5s）并提示「非实时」
- [ ] 页面可见性：后台 tab 可选择暂停 WS（可选）

**验收**

- `make dev-api` + `make dev-web` 同时开，看板实时跳动
- 停后端再启，前端能恢复

**Vibe 提示词**

> 完成 Step 13：前后端联调与 WS 降级策略。

---

### Step 14：MVP 里程碑验收

**检查清单**

- [ ] 5 个币种 USDT 价实时更新
- [ ] 无 PHP、无 Redis、无 1s 轮询
- [ ] 文档 RFC-002 与实现字段一致
- [ ] README 补充「如何 dev」

**庆祝 🎉** — 可部署一版到 HK（Step 18）

---

## Phase D — 慢数据补齐

### Step 15：USDT/CNY + USD/CNY Ingest

**做什么**

- [ ] `internal/ingest/otc`：OKX C2C REST，30s
- [ ] `internal/ingest/forex`：Frankfurter，1h
- [ ] 写入 `store.UpdateRates`
- [ ] `QuoteTable` 中 `priceCny = priceUsdt * usdtCny`
- [ ] WS 推送 `type: rates`

**验收**

- snapshot 含 `rates.usdtCny`，CNY 列正确

---

### Step 16：股指 + 黄金 Ingest

**做什么**

- [ ] `internal/ingest/equity`：Yahoo symbols 映射（上证、恒生、道指等）
- [ ] 90s 轮询，休市仍返回昨收
- [ ] `IndexGrid` 接真实数据
- [ ] WS `type: indices` 或仅 REST 刷新（低频可只 snapshot）

**验收**

- `curl /api/v1/snapshot` 中 `indices` 长度 ≥ 10

---

### Step 17：宏观 Ingest

**做什么**

- [ ] `alternative.me` 恐惧贪婪
- [ ] CoinGecko `/global` 市值、成交量、BTC 占比
- [ ] `MacroGrid` 接真数据
- [ ] Ahr999：二期或显示 `--`

**验收**

- 宏观卡片与旧站字段大致对应

---

## Phase E — 部署

### Step 18：HK 部署（nginx 模式）

**做什么**

- [ ] VPS 安装 Go、Nginx
- [ ] `make deploy-api DEPLOY_HOST=...`
- [ ] `make deploy-web DEPLOY_HOST=...`
- [ ] 按 `deploy/nginx.conf.example` 配置 TLS
- [ ] systemd 启用 `marketpulse.service`

**验收**

- 手机浏览器 HTTPS 打开，币价实时

---

### Step 19：运维小闭环

**做什么**

- [ ] `check-connectivity.sh` 加入 cron 或手动巡检文档
- [ ] `/healthz` 监控（可选 Uptime Kuma）
- [ ] 日志：slog JSON 到 journal

---

## Phase F — 增强（可选）

| Step | 内容 |
|------|------|
| 20 | 排行榜：CoinGecko markets + `RankTable.vue` |
| 21 | K 线抽屉：TradingView Widget |
| 22 | 自选币种：`config.yaml` + 设置页写回 |
| 23 | embed 单二进制模式（`DEPLOY_MODE=embed`） |
| 24 | Huobi WS 作为第二 `ExchangeFeed` |
| 25 | 价格预警（内存触发 + 通知） |

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
