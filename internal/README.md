# internal/

后端私有实现，禁止被外部项目 import。

| 包 | 职责 |
|----|------|
| `config` | YAML + 环境变量 |
| `marketdata` | 行情服务门面，对外提供 snapshot、provider status、K 线与 WS |
| `marketdata/ingest` | 交易所 WS、REST 轮询 |
| `marketdata/store` | 行情内存读模型 |
| `marketdata/stream` | 行情 WebSocket 广播与 K 线订阅 |
| `api` | Gin HTTP handlers |
| `server` | Gin 引擎、CORS、路由挂载 |

**Phase A**：`config`、`marketdata/store`、`server`、`api`  
**Phase B Step 3–6**：Binance miniTicker ingest、`/api/v1/market/snapshot`、`/ws/v1/market/stream`、store→stream 防抖推送  

下一步：`alerts`、`portfolio`、`ai`、`users` 通过 `marketdata.MarketDataService` 消费行情能力。
