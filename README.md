# MarketPulse

个人加密货币行情看板：实时币价（WebSocket）、股指、宏观指标。

- 设计文档：[docs/README.md](./docs/README.md)
- 架构 RFC：[docs/RFC-001-architecture.md](./docs/RFC-001-architecture.md)

## 仓库结构

```
marketpulse/
├── cmd/marketd/      # Go 后端入口
├── internal/         # 后端实现
├── web/              # Vue 前端
├── docs/             # RFC 与设计
├── deploy/           # 部署模板
└── Makefile          # 构建 / 部署命令
```

## 快速开始

```bash
make help
make setup-config   # 首次：生成 config/config.yaml
make test           # Phase A 单元测试
make dev-api        # Gin :8080

# 另一个终端（前端占位）
make dev-web
```

```bash
curl -s http://127.0.0.1:8080/healthz | jq .
curl -s http://127.0.0.1:8080/api/v1/snapshot | jq .
```

## 部署（HK VPS）

默认 **nginx 分离静态与 API**，改前端可只执行 `make deploy-web`。

详见 [docs/RFC-003-deployment.md](./docs/RFC-003-deployment.md)。

## Vibe Coding

- 后端改动：`internal/`、`cmd/`
- 前端改动：`web/`
- 契约变更：先改 `docs/RFC-002-api-contract.md`
