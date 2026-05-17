# internal/

后端私有实现，禁止被外部项目 import。

| 包 | 职责 |
|----|------|
| `config` | YAML + 环境变量 |
| `ingest` | 交易所 WS、REST 轮询 |
| `store` | 内存行情快照 |
| `hub` | WebSocket 广播 |
| `api` | Gin HTTP handlers |
| `server` | Gin 引擎、CORS、路由挂载 |

**Phase A**：`config`、`store`、`server`、`api`  
**Phase B Step 3–6**：Binance miniTicker ingest、`/api/v1/snapshot`、`/ws/v1/stream`、store→hub 防抖推送  

下一步：**Phase D** 汇率/股指/宏观，或前端 Step 13 联调验收。
