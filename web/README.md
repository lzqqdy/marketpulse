# web/

Vue 3 + Vite 6 + TypeScript 前端。

## 快速开始

```bash
npm install
npm run dev    # :5173，代理 /api /ws /healthz → :8080
npm run build  # → dist/
```

**需要后端**：`go run -buildvcs=false ./cmd/marketd -config config/config.yaml`（项目根目录）。

开发时 Vite 代理配置见 `vite.config.ts`：
- `/api` → `http://127.0.0.1:8080`
- `/ws` → `ws://127.0.0.1:8080`
- `/healthz` → `http://127.0.0.1:8080`

## 目录结构

```
web/src/
├── App.vue
├── views/DashboardView.vue          # 顶层视图
├── stores/theme.ts                  # 深浅色主题
├── features/market/
│   ├── views/MarketDashboard.vue    # 主看板布局
│   ├── components/                  # 业务组件
│   │   ├── QuoteTable.vue           # 币价表
│   │   ├── MacroGrid.vue            # 宏观指标
│   │   ├── IndexGrid.vue            # 全球速览
│   │   ├── MarketCenterPanel.vue    # 行情中心
│   │   ├── AlphaStockPanel.vue      # 美股参考
│   │   ├── KlineDrawer.vue          # K 线抽屉
│   │   ├── StatusBar.vue            # 连接状态
│   │   └── ProviderStatusWidget.vue # 数据源健康
│   ├── stores/
│   │   ├── market.ts                # 行情快照 + WS
│   │   ├── chart.ts                 # K 线抽屉状态
│   │   └── providers.ts             # 数据源健康
│   ├── api/                         # HTTP / WS 客户端
│   ├── types/                       # TypeScript 类型
│   ├── mock/                        # 降级演示数据
│   └── composables/                 # 图表、涨跌色、屏幕常亮
└── styles/main.css
```

## 数据流

1. `MarketDashboard` 挂载 → `marketStore.initLive()`
2. `GET /api/v1/market/snapshot` 填充首屏
3. 连接 `WS /ws/v1/market/stream?channels=quotes,rates,indices,alpha,macro`
4. 断线指数退避重连；5s 无数据降级 mock 演示
5. 行情中心独立 REST 轮询（60s），不走全局 WS

## Mock 降级

后端不可用时自动切换 mock 数据（`features/market/mock/`），StatusBar 显示「演示数据」。

契约：`../docs/RFC-002-api-contract.md`
