# MarketPulse

个人加密货币行情看板：实时币价（WebSocket）、全球股指、宏观指标、衍生品数据、美股参考、行情中心。

- 设计文档：[docs/README.md](./docs/README.md)
- 架构 RFC：[docs/RFC-001-architecture.md](./docs/RFC-001-architecture.md)
- API 契约：[docs/RFC-002-api-contract.md](./docs/RFC-002-api-contract.md)
- 数据源：[docs/DATA_SOURCES.md](./docs/DATA_SOURCES.md)

## 仓库结构

```
marketpulse/
├── cmd/marketd/      # Go 后端入口
├── internal/         # 后端实现
├── web/              # Vue 3 前端
├── docs/             # RFC 与设计
├── specs/            # Spec Kit 功能规格
├── deploy/           # 部署模板
└── Makefile          # 构建 / 部署命令（Linux/macOS/Git Bash）
```

## 快速开始

### Linux / macOS / Git Bash

```bash
make help
make setup-config   # 首次：生成 config/config.yaml
make test           # Go 单元测试
make dev-api        # Gin :8080

# 另一个终端
make dev-web        # Vite :5173
```

### Windows（PowerShell）

```powershell
cd F:\lzqqdy\marketpulse

# 推荐：用 .cmd 包装，无需改执行策略
.\scripts\dev.cmd help
.\scripts\dev.cmd setup-config
.\scripts\dev.cmd api            # 终端 1：后端 :8080
.\scripts\dev.cmd web            # 终端 2：前端 :5173
.\scripts\dev.cmd dev            # 同时启动前后端

# 或直接运行 .ps1（需先放开执行策略，见下方说明）
.\scripts\dev.ps1 api
```

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/api/v1/market/snapshot
```

浏览器打开 `http://localhost:5173`。

## 当前功能

| 模块 | 说明 |
|------|------|
| 币价表 | Binance WS 实时推送，USDT/¥ 双价 |
| 宏观指标 | 总市值、恐惧贪婪、多空比、资金费率、爆仓等 |
| 全球速览 | 14 个指数/商品（百度主源，腾讯/东财备用） |
| 行情中心 | A股/港股/美股涨跌分布、热力图、资金流、热门板块 |
| 美股参考 | Bitget USDT-FUTURES 代币化美股（Binance Alpha 备用） |
| K 线抽屉 | lightweight-charts，crypto/alpha WS 实时，指数 REST |
| 数据源健康 | Provider 状态面板，30s 轮询 |

## 部署（HK VPS）

推荐 **`make ship`**：单端口（默认 `:8080`）对外，源码同步到服务器 Git 仓库目录。

```bash
cp deploy/deploy.local.yaml.example deploy/deploy.local.yaml
# 配置 ssh_host、remote_dir: /home/lzqqdy/github/marketpulse

make ship                 # 构建 + 同步 + 重启
make ship-commit          # 同上，并在服务器 git commit
make ship SHIP_GIT_COMMIT=1   # 单次自动提交
```

- 部署详解：[docs/RFC-003-deployment.md](./docs/RFC-003-deployment.md)
- 模板说明：[deploy/README.md](./deploy/README.md)
- 使用 **Nginx + 域名** 时：`make deploy DEPLOY_HOST=user@host`

## 开发方式

### Spec Kit（新功能推荐）

已集成 [GitHub Spec Kit](https://github.com/github/spec-kit)。在 Cursor Agent 中使用 `/speckit-specify` 开始新功能开发。详见 [docs/SPEC_KIT_GUIDE.md](./docs/SPEC_KIT_GUIDE.md)。

### Vibe Coding（增量改造）

- 后端改动：`internal/`、`cmd/`
- 前端改动：`web/src/features/market/`
- 契约变更：先改 `docs/RFC-002-api-contract.md`
- 路线图：按 `docs/RFC-004-implementation-roadmap.md` 逐步推进
